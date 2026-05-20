package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/Mininglamp-OSS/octo-speech/internal/asrlog"
	"github.com/Mininglamp-OSS/octo-speech/internal/config"
	"github.com/Mininglamp-OSS/octo-speech/internal/service"
)

type TranscribeHandler struct {
	svc       *service.TranscribeService
	cfg       *config.Config
	asrLogger *asrlog.Logger
	logger    *zap.Logger
}

func NewTranscribeHandler(svc *service.TranscribeService, cfg *config.Config, asrLogger *asrlog.Logger, logger *zap.Logger) *TranscribeHandler {
	return &TranscribeHandler{svc: svc, cfg: cfg, asrLogger: asrLogger, logger: logger}
}

func (h *TranscribeHandler) Handle(c *gin.Context) {
	startTime := time.Now()
	appID, _ := c.Get("app_id")
	appIDStr, _ := appID.(string)

	var requestID string
	if h.asrLogger != nil {
		requestID = h.asrLogger.GenerateRequestID()
	} else {
		b := make([]byte, 3)
		rand.Read(b)
		requestID = fmt.Sprintf("nolog_%d_%s", time.Now().UnixMilli(), hex.EncodeToString(b))
	}

	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     http.StatusBadRequest,
			"msg":        "audio file is required",
			"request_id": requestID,
		})
		return
	}
	defer file.Close()

	if header.Size > h.cfg.MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     http.StatusBadRequest,
			"msg":        "file size exceeds limit",
			"request_id": requestID,
		})
		return
	}

	audioData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     http.StatusBadRequest,
			"msg":        "failed to read audio file",
			"request_id": requestID,
		})
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "audio/wav"
	}

	contextText := c.PostForm("context_text")
	chatContext := c.PostForm("chat_context")
	personalContext := c.PostForm("personal_context")
	memberContext := c.PostForm("member_context")
	channelType := c.PostForm("channel_type")
	mode := c.PostForm("mode")
	engineParam := c.PostForm("engine")
	modelParam := c.PostForm("model")
	emotionParam := c.PostForm("emotion_emoji")

	if channelType != "" && channelType != "dm" && channelType != "group" && channelType != "thread" && channelType != "1" && channelType != "2" && channelType != "5" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     http.StatusBadRequest,
			"msg":        "invalid channel_type, expected: dm, group, thread, 1, 2, 5",
			"request_id": requestID,
		})
		return
	}

	contextText = config.TruncateRunesTail(contextText, h.cfg.MaxContextTextLength)
	chatContext = config.TruncateRunesTail(chatContext, h.cfg.MaxChatContextLength)
	personalContext = config.TruncateRunesTail(personalContext, h.cfg.MaxVoiceContextLength)
	memberContext = config.TruncateRunesTail(memberContext, h.cfg.MaxMemberContextLength)

	isDM := channelType == "dm" || channelType == "1"

	vocabRef := service.BuildVocabularyReference(personalContext, memberContext, chatContext)

	var opts service.TranscribeOptions
	opts.SkipMention = isDM

	if mode != "" {
		switch mode {
		case "smart":
			// use default edit mode
		case "append_only":
			opts.Mode = "append"
		case "edit_only":
			opts.Mode = "edit_only"
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status":     http.StatusBadRequest,
				"msg":        "invalid mode, expected: smart, append_only, edit_only",
				"request_id": requestID,
			})
			return
		}
	}

	if opts.Mode == "edit_only" && contextText == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":     http.StatusBadRequest,
			"msg":        "edit_only mode requires context_text",
			"request_id": requestID,
		})
		return
	}

	if engineParam != "" {
		normalized := config.NormalizeEngine(engineParam)
		if normalized == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":     http.StatusBadRequest,
				"msg":        "invalid engine, expected: gm/gemini, gp/gpt, qw/qwen",
				"request_id": requestID,
			})
			return
		}
		opts.Engine = normalized
	}

	if modelParam != "" {
		opts.Model = modelParam
	}

	if emotionParam != "" {
		b, err := strconv.ParseBool(emotionParam)
		if err == nil {
			opts.EmotionEmoji = &b
		}
	}

	result, err := h.svc.Transcribe(audioData, mimeType, contextText, vocabRef, opts)

	duration := time.Since(startTime)

	engine := h.cfg.Engine
	if opts.Engine != "" {
		engine = opts.Engine
	}

	if h.asrLogger != nil {
		entry := asrlog.ASREntry{
			RequestID:      requestID,
			Timestamp:      startTime.UTC().Format(time.RFC3339Nano),
			Source:         "api",
			AppID:          appIDStr,
			Engine:         engine,
			ModelRequested: modelParam,
			Input: asrlog.ASRInput{
				Mode:            mode,
				MimeType:        mimeType,
				AudioSize:       len(audioData),
				ContextText:     contextText,
				ChatContext:     chatContext,
				PersonalContext: personalContext,
				MemberContext:   memberContext,
				Model:           modelParam,
				Language:        h.cfg.Language,
				ChannelType:     channelType,
			},
			DurationMs: duration.Milliseconds(),
			AudioData:  audioData,
		}

		if result != nil {
			entry.ModelUsed = result.Model
			entry.RawResultText = result.RawText
			entry.ResultText = result.Text
			entry.ResultLength = len([]rune(result.Text))
			entry.IsNoSpeech = service.IsNoSpeech(result.RawText)
			entry.Prompt = &asrlog.ASRPrompt{
				Type:        result.PromptType,
				Text:        result.PromptText,
				RequestBody: result.RequestBody,
			}
		}

		if err != nil {
			entry.Error = err.Error()
		}

		h.asrLogger.Enqueue(entry)
	}

	if err != nil {
		if err == service.ErrGPTEditNotSupported {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":     http.StatusBadRequest,
				"msg":        err.Error(),
				"request_id": requestID,
			})
			return
		}

		statusCode := http.StatusInternalServerError

		h.logger.Error("transcribe failed",
			zap.String("request_id", requestID),
			zap.String("app_id", appIDStr),
			zap.Error(err))

		c.JSON(statusCode, gin.H{
			"status":     statusCode,
			"msg":        classifyTranscribeError(err),
			"request_id": requestID,
		})
		return
	}

	engineShort := config.EngineToShort(engine)

	c.JSON(http.StatusOK, gin.H{
		"status":     http.StatusOK,
		"text":       result.Text,
		"m":          config.ShortenModelName(result.Model),
		"engine":     engineShort,
		"request_id": requestID,
	})
}

func classifyTranscribeError(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "transcription failed: timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "transcription failed: timeout"
		}
		return "transcription failed: service unavailable"
	}
	return "transcription failed"
}
