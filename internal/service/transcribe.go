package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/Mininglamp-OSS/octo-speech/internal/config"
)

type TranscribeService struct {
	cfg    *config.Config
	client *http.Client
}

func NewTranscribeService(cfg *config.Config) *TranscribeService {
	return &TranscribeService{
		cfg:    cfg,
		client: &http.Client{},
	}
}

type TranscribeOptions struct {
	Mode         string
	Model        string
	SkipMention  bool
	Engine       string
	EmotionEmoji *bool
}

type TranscribeResult struct {
	Text         string
	RawText      string
	Model        string
	Engine       string
	PromptText   string
	SystemPrompt string
	PromptType   string
	RequestBody  interface{}
}

var ErrGPTEditNotSupported = fmt.Errorf("edit mode is not supported with GPT engine")

func (s *TranscribeService) Transcribe(audioData []byte, mimeType, contextText, chatContext string, opts TranscribeOptions) (*TranscribeResult, error) {
	engine := s.cfg.Engine
	if opts.Engine != "" {
		engine = opts.Engine
	}

	mode := s.cfg.EditMode
	if opts.Mode != "" {
		mode = opts.Mode
	}

	if engine == config.EngineGPT && mode == "edit" {
		mode = "append"
	}

	if engine == config.EngineGPT && mode == "edit_only" {
		return nil, ErrGPTEditNotSupported
	}

	emotionEmoji := s.cfg.EmotionEmoji
	if opts.EmotionEmoji != nil {
		emotionEmoji = *opts.EmotionEmoji
	}

	models := s.modelsForEngine(engine)
	if opts.Model != "" {
		models = append([]string{opts.Model}, models...)
	}

	userMsg := BuildUserMessage(mode, contextText, chatContext, emotionEmoji)
	var systemMsg string
	if engine != config.EngineGPT {
		systemMsg = BuildSystemMessage(emotionEmoji, opts.SkipMention, mode)
	}

	var rawText, model string
	var requestBody interface{}
	var promptType string
	var err error

	switch engine {
	case config.EngineGPT:
		promptType = "audio_transcription"
		rawText, model, requestBody, err = s.callGPTWithModelFallback(audioData, mimeType, userMsg, models)
	default:
		promptType = "chat_completion"
		rawText, model, requestBody, err = s.callChatCompletionWithFallback(audioData, mimeType, systemMsg, userMsg, models, engine)
	}

	if err != nil {
		return &TranscribeResult{
			Engine:       engine,
			PromptText:   userMsg,
			SystemPrompt: systemMsg,
			PromptType:   promptType,
			RequestBody:  requestBody,
		}, err
	}

	result := &TranscribeResult{
		RawText:      rawText,
		Text:         rawText,
		Model:        model,
		Engine:       engine,
		PromptText:   userMsg,
		SystemPrompt: systemMsg,
		PromptType:   promptType,
		RequestBody:  requestBody,
	}

	switch mode {
	case "append":
		if IsNoSpeech(rawText) {
			if contextText != "" {
				result.Text = contextText
			} else {
				result.Text = ""
			}
		} else if contextText != "" {
			result.Text = joinContextAndText(contextText, rawText)
		}
	default:
		if rawText == NoSpeechSentinel {
			if contextText != "" {
				result.Text = contextText
			} else {
				result.Text = ""
			}
		} else if contextText != "" {
			result.Text = restoreTrailingWhitespace(contextText, rawText)
		}
	}

	return result, nil
}

func (s *TranscribeService) modelsForEngine(engine string) []string {
	switch engine {
	case config.EngineGPT:
		out := make([]string, len(s.cfg.GPTModels))
		copy(out, s.cfg.GPTModels)
		return out
	case config.EngineQwen:
		out := make([]string, len(s.cfg.QwenModels))
		copy(out, s.cfg.QwenModels)
		return out
	default:
		out := make([]string, len(s.cfg.Models))
		copy(out, s.cfg.Models)
		return out
	}
}

func (s *TranscribeService) effectiveURL(engine string) string {
	if engine == config.EngineQwen && s.cfg.QwenUrl != "" {
		return s.cfg.QwenUrl
	}
	return s.cfg.LiteLLMUrl
}

func (s *TranscribeService) effectiveKey(engine string) string {
	if engine == config.EngineQwen && s.cfg.QwenKey != "" {
		return s.cfg.QwenKey
	}
	return s.cfg.LiteLLMKey
}

func (s *TranscribeService) effectiveTimeout(engine string) int {
	if engine == config.EngineQwen && s.cfg.QwenTimeout > 0 {
		return s.cfg.QwenTimeout
	}
	return s.cfg.Timeout
}

func (s *TranscribeService) callChatCompletionWithFallback(audioData []byte, mimeType, systemMsg, userMsg string, models []string, engine string) (string, string, interface{}, error) {
	totalCtx, totalCancel := context.WithTimeout(context.Background(),
		time.Duration(s.cfg.TotalTimeout)*time.Second)
	defer totalCancel()

	var lastErr error
	var lastReqBody interface{}
	for _, model := range models {
		if totalCtx.Err() != nil {
			break
		}

		text, reqBody, err := s.callChatCompletion(totalCtx, model, audioData, mimeType, systemMsg, userMsg, engine)
		lastReqBody = reqBody
		if err == nil {
			return text, model, reqBody, nil
		}

		lastErr = err
		if isNonRetryableError(err) {
			return "", model, reqBody, err
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no models configured")
	}
	return "", "", lastReqBody, fmt.Errorf("all models failed: %w", lastErr)
}

func (s *TranscribeService) callGPTWithModelFallback(audioData []byte, mimeType, prompt string, models []string) (string, string, interface{}, error) {
	totalCtx, totalCancel := context.WithTimeout(context.Background(),
		time.Duration(s.cfg.TotalTimeout)*time.Second)
	defer totalCancel()

	audioBase64 := base64.StdEncoding.EncodeToString(audioData)

	var lastErr error
	var lastReqBody interface{}
	for _, model := range models {
		if totalCtx.Err() != nil {
			break
		}

		text, err := s.callAudioTranscriptions(totalCtx, model, audioData, mimeType, prompt)

		reqBody := map[string]interface{}{
			"model":        model,
			"language":     s.cfg.Language,
			"prompt":       prompt,
			"file":         "(multipart binary, see audio_file in input)",
			"audio_base64": audioBase64,
		}
		lastReqBody = reqBody

		if err == nil {
			return text, model, reqBody, nil
		}

		lastErr = err
		if isNonRetryableError(err) {
			return "", model, reqBody, err
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no GPT models configured")
	}
	return "", "", lastReqBody, fmt.Errorf("all GPT models failed: %w", lastErr)
}

func (s *TranscribeService) callChatCompletion(totalCtx context.Context, model string, audioData []byte, mimeType, systemMsg, userMsg, engine string) (string, interface{}, error) {
	b64Audio := base64.StdEncoding.EncodeToString(audioData)

	// qwen3.5-omni-plus requires data URI prefix
	if engine == config.EngineQwen {
		b64Audio = "data:;base64," + b64Audio
	}

	var reasoningEffort string
	if engine == config.EngineGemini && strings.Contains(model, "3.1-pro") {
		reasoningEffort = "low"
	}

	var messages []chatMessage
	if systemMsg != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemMsg})
	}
	messages = append(messages, chatMessage{
		Role: "user",
		Content: []contentPart{
			{
				Type: "text",
				Text: userMsg,
			},
			{
				Type: "input_audio",
				InputAudio: &inputAudio{
					Data:   b64Audio,
					Format: MimeTypeToFormat(mimeType),
				},
			},
		},
	})

	reqBody := chatCompletionRequest{
		Model:           model,
		ReasoningEffort: reasoningEffort,
		Messages:        messages,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", reqBody, fmt.Errorf("marshal request: %w", err)
	}

	perModelTimeout := time.Duration(s.effectiveTimeout(engine)) * time.Second
	ctx, cancel := context.WithTimeout(totalCtx, perModelTimeout)
	defer cancel()

	url := strings.TrimRight(s.effectiveURL(engine), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", reqBody, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.effectiveKey(engine))

	resp, err := s.client.Do(req)
	if err != nil {
		return "", reqBody, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", reqBody, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", reqBody, &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	var chatResp chatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", reqBody, fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", reqBody, fmt.Errorf("empty response from model")
	}

	result := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	return result, reqBody, nil
}

func (s *TranscribeService) callAudioTranscriptions(totalCtx context.Context, model string,
	audioData []byte, mimeType string, prompt string) (string, error) {

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	writer.WriteField("model", model)

	if s.cfg.Language != "" {
		writer.WriteField("language", s.cfg.Language)
	}

	if prompt != "" {
		writer.WriteField("prompt", prompt)
	}

	ext := MimeTypeToFormat(mimeType)
	part, err := writer.CreateFormFile("file", "audio."+ext)
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	part.Write(audioData)
	writer.Close()

	perModelTimeout := time.Duration(s.cfg.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(totalCtx, perModelTimeout)
	defer cancel()

	url := strings.TrimRight(s.cfg.LiteLLMUrl, "/") + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.cfg.LiteLLMKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &APIError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return strings.TrimSpace(result.Text), nil
}

func joinContextAndText(contextText, newText string) string {
	if contextText == "" || newText == "" {
		return contextText + newText
	}
	ctxRunes := []rune(contextText)
	newRunes := []rune(newText)
	last := ctxRunes[len(ctxRunes)-1]
	first := newRunes[0]

	if unicode.IsSpace(last) || isPunctuation(last) {
		return contextText + newText
	}
	if isCJK(last) || isCJK(first) {
		return contextText + newText
	}
	return contextText + " " + newText
}

func isCJK(r rune) bool {
	return (r >= 0x4e00 && r <= 0x9fff) ||
		(r >= 0x3400 && r <= 0x4dbf) ||
		(r >= 0x3000 && r <= 0x303f) ||
		(r >= 0xff00 && r <= 0xffef) ||
		(r >= 0x3040 && r <= 0x309f) ||
		(r >= 0x30a0 && r <= 0x30ff) ||
		(r >= 0xac00 && r <= 0xd7af)
}

func isPunctuation(r rune) bool {
	return strings.ContainsRune(`，。！？；：、,.!?;:…—·"'）」】》〉`+"“”‘’", r)
}

func restoreTrailingWhitespace(contextText, text string) string {
	trimmedCtx := strings.TrimRight(contextText, " \t\n\r")
	trailing := contextText[len(trimmedCtx):]

	if trailing == "" || trimmedCtx == "" {
		return text
	}

	if strings.HasPrefix(text, trimmedCtx) {
		rest := text[len(trimmedCtx):]
		return trimmedCtx + trailing + strings.TrimLeft(rest, " \t")
	}

	return strings.TrimRight(text, " \t\n\r") + trailing
}

func MimeTypeToFormat(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "wav"):
		return "wav"
	case strings.Contains(mimeType, "mp3"), strings.Contains(mimeType, "mpeg"):
		return "mp3"
	case strings.Contains(mimeType, "ogg"):
		return "ogg"
	case strings.Contains(mimeType, "webm"):
		return "webm"
	case strings.Contains(mimeType, "mp4"), strings.Contains(mimeType, "m4a"):
		return "m4a"
	case strings.Contains(mimeType, "flac"):
		return "flac"
	default:
		return "wav"
	}
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

func isNonRetryableError(err error) bool {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.StatusCode >= 400 && ae.StatusCode < 500 && ae.StatusCode != 429
	}
	return false
}

type chatCompletionRequest struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func (m *chatMessage) UnmarshalJSON(data []byte) error {
	var raw struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Role = raw.Role

	var s string
	if err := json.Unmarshal(raw.Content, &s); err == nil {
		m.Content = s
		return nil
	}

	var parts []contentPart
	if err := json.Unmarshal(raw.Content, &parts); err == nil {
		m.Content = parts
		return nil
	}

	return fmt.Errorf("message content is neither string nor []contentPart")
}

type contentPart struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	InputAudio *inputAudio `json:"input_audio,omitempty"`
}

type inputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

type chatCompletionResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message responseMessage `json:"message"`
}

type responseMessage struct {
	Content string `json:"content"`
}
