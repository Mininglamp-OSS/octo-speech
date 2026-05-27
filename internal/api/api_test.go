package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"

	"github.com/Mininglamp-OSS/octo-speech/internal/asrlog"
	"github.com/Mininglamp-OSS/octo-speech/internal/config"
	"github.com/Mininglamp-OSS/octo-speech/internal/service"
	"github.com/Mininglamp-OSS/octo-speech/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthRouter(appStore *store.AppStore) *gin.Engine {
	r := gin.New()
	r.Use(AuthMiddleware(appStore))
	r.GET("/test", func(c *gin.Context) {
		appID, _ := c.Get("app_id")
		c.JSON(200, gin.H{"app_id": appID})
	})
	return r
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	s := store.NewAppStore(nil, 60)
	r := setupAuthRouter(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "missing authorization header" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	s := store.NewAppStore(nil, 60)
	r := setupAuthRouter(s)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestVocabularyHandler_PutValidation(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.PUT("/v1/speech/vocabularies", h.Put)

	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"missing subject_id",
			`{"scope_type":"global","scope_id":"default","content":"test"}`,
			400,
		},
		{
			"missing content",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default"}`,
			400,
		},
		{
			"invalid scope_type",
			`{"subject_id":"user1","scope_type":"invalid","scope_id":"default","content":"test"}`,
			400,
		},
		{
			"empty content",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","content":""}`,
			400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/v1/speech/vocabularies",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestVocabularyHandler_GetValidation(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.GET("/v1/speech/vocabularies", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/vocabularies", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without subject_id, got %d", w.Code)
	}
}

func TestVocabularyHandler_GetInvalidScope(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.GET("/v1/speech/vocabularies", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/vocabularies?subject_id=u1&scope_type=bad&scope_id=s1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid scope_type, got %d", w.Code)
	}
}

func TestVocabularyHandler_DeleteValidation(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.DELETE("/v1/speech/vocabularies", h.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/speech/vocabularies", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without subject_id, got %d", w.Code)
	}
}

func TestConfigHandler(t *testing.T) {
	cfg := &config.Config{
		MaxDuration:        60,
		MaxFileSize:        3145728,
		Engine:             config.EngineGemini,
		EditMode:           "edit",
		LocalEnabled:       true,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
	}

	localStore := store.NewLocalConfigStore(nil, cfg)
	h := NewConfigHandler(cfg, localStore)

	r := gin.New()
	r.GET("/v1/speech/config", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["engine"] != "gm" {
		t.Errorf("expected engine 'gm', got %v", resp["engine"])
	}
	if resp["edit_mode"] != "edit" {
		t.Errorf("expected edit_mode 'edit', got %v", resp["edit_mode"])
	}
	if resp["max_duration"] != float64(60) {
		t.Errorf("expected max_duration 60, got %v", resp["max_duration"])
	}
	if resp["local_enabled"] != true {
		t.Errorf("expected local_enabled true, got %v", resp["local_enabled"])
	}
}

func TestConfigHandler_FeedbackURL_Present(t *testing.T) {
	cfg := &config.Config{
		MaxDuration:        60,
		MaxFileSize:        3145728,
		Engine:             config.EngineGemini,
		EditMode:           "edit",
		LocalEnabled:       true,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
		FeedbackURL:        "https://example.com/feedback",
	}

	localStore := store.NewLocalConfigStore(nil, cfg)
	h := NewConfigHandler(cfg, localStore)

	r := gin.New()
	r.GET("/v1/speech/config", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["feedback_url"] != "https://example.com/feedback" {
		t.Errorf("expected feedback_url 'https://example.com/feedback', got %v", resp["feedback_url"])
	}
}

func TestConfigHandler_FeedbackURL_Absent(t *testing.T) {
	cfg := &config.Config{
		MaxDuration:        60,
		MaxFileSize:        3145728,
		Engine:             config.EngineGemini,
		EditMode:           "edit",
		LocalEnabled:       true,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
	}

	localStore := store.NewLocalConfigStore(nil, cfg)
	h := NewConfigHandler(cfg, localStore)

	r := gin.New()
	r.GET("/v1/speech/config", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/config", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if _, exists := resp["feedback_url"]; exists {
		t.Errorf("expected feedback_url to be absent, but got %v", resp["feedback_url"])
	}
}

func TestTranscribeHandler_MissingAudio(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize: 5 * 1024 * 1024,
		MaxFileSize:   3145728,
		EmotionEmoji:  true,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTranscribeHandler_InvalidMode(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.WriteField("mode", "invalid_mode")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid mode, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTranscribeHandler_InvalidEngine(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.WriteField("engine", "invalid_engine")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid engine, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVocabularyHandler_PutMissingUpdatedBy(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.PUT("/v1/speech/vocabularies", h.Put)

	body := `{"subject_id":"user1","scope_type":"global","scope_id":"default","content":"test"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/speech/vocabularies", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "updated_by is required" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func TestVocabularyHandler_GetScopePairing(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.GET("/v1/speech/vocabularies", h.Get)

	tests := []struct {
		name string
		url  string
		want int
	}{
		{
			"scope_type without scope_id",
			"/v1/speech/vocabularies?subject_id=u1&scope_type=global",
			400,
		},
		{
			"scope_id without scope_type",
			"/v1/speech/vocabularies?subject_id=u1&scope_id=s1",
			400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tt.url, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["msg"] != "scope_type and scope_id must be provided together" {
				t.Errorf("unexpected msg: %v", resp["msg"])
			}
		})
	}
}

func TestVocabularyHandler_DeleteScopePairing(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.DELETE("/v1/speech/vocabularies", h.Delete)

	tests := []struct {
		name string
		url  string
		want int
	}{
		{
			"scope_type without scope_id",
			"/v1/speech/vocabularies?subject_id=u1&scope_type=global",
			400,
		},
		{
			"scope_id without scope_type",
			"/v1/speech/vocabularies?subject_id=u1&scope_id=s1",
			400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", tt.url, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["msg"] != "scope_type and scope_id must be provided together" {
				t.Errorf("unexpected msg: %v", resp["msg"])
			}
		})
	}
}

func TestVocabularyHandler_GetDefaultScope(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.GET("/v1/speech/vocabularies", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/vocabularies?subject_id=u1", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		t.Errorf("expected empty scope to default to global/default, got 400: %s", w.Body.String())
	}
}

func TestVocabularyHandler_DeleteDefaultScope(t *testing.T) {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})

	h := NewVocabularyHandler(nil)
	r.DELETE("/v1/speech/vocabularies", h.Delete)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/speech/vocabularies?subject_id=u1", nil)
	r.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		t.Errorf("expected empty scope to default to global/default, got 400: %s", w.Body.String())
	}
}

func TestTranscribeHandler_EditOnlyWithoutContextText(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.WriteField("mode", "edit_only")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for edit_only without context_text, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "edit_only mode requires context_text" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func TestTranscribeHandler_InvalidChannelType(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.WriteField("channel_type", "invalid_channel")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid channel_type, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "invalid channel_type, expected: dm, group, thread, 1, 2, 5" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func TestTranscribeHandler_ChannelTypeThread(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	tests := []struct {
		name        string
		channelType string
	}{
		{"numeric 5", "5"},
		{"string thread", "thread"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("audio", "test.wav")
			part.Write([]byte("fake audio"))
			writer.WriteField("channel_type", tt.channelType)
			writer.Close()

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			r.ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				msg, _ := resp["msg"].(string)
				if strings.Contains(msg, "channel_type") {
					t.Fatalf("channel_type=%q was rejected by validation: %s", tt.channelType, msg)
				}
				t.Fatalf("unexpected 400 for channel_type=%q: %s", tt.channelType, w.Body.String())
			}
		})
	}
}

func TestTranscribeHandler_ChannelTypeThread_SkipMentionFalse(t *testing.T) {
	service.ResetPromptsToDefaults()

	tests := []struct {
		name        string
		channelType string
	}{
		{"string thread", "thread"},
		{"numeric 5", "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedSystem string
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				var req struct {
					Messages []struct {
						Role    string `json:"role"`
						Content json.RawMessage `json:"content"`
					} `json:"messages"`
				}
				json.Unmarshal(body, &req)
				for _, m := range req.Messages {
					if m.Role == "system" {
						var s string
						json.Unmarshal(m.Content, &s)
						capturedSystem = s
					}
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"choices": []map[string]interface{}{
						{"message": map[string]string{"content": "transcribed text"}},
					},
				})
			}))
			defer backend.Close()

			cfg := &config.Config{
				MaxUploadSize:          5 * 1024 * 1024,
				MaxFileSize:            3145728,
				EmotionEmoji:           true,
				MaxContextTextLength:   5000,
				MaxChatContextLength:   20000,
				MaxVoiceContextLength:  10000,
				MaxMemberContextLength: 5000,
				Engine:                 config.EngineGemini,
				Models:                 []string{"test-model"},
				LiteLLMUrl:             backend.URL,
				LiteLLMKey:             "test-key",
				Timeout:                10,
				TotalTimeout:           15,
			}

			svc := service.NewTranscribeService(cfg)
			h := NewTranscribeHandler(svc, cfg, nil, nil)

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("app_id", "test-app")
				c.Next()
			})
			r.POST("/v1/speech/transcribe", h.Handle)

			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("audio", "test.wav")
			part.Write([]byte("fake audio"))
			writer.WriteField("channel_type", tt.channelType)
			writer.Close()

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			r.ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest {
				t.Fatalf("channel_type=%q rejected by validation: %s", tt.channelType, w.Body.String())
			}
			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			if capturedSystem == "" {
				t.Fatal("system message was not sent to the backend")
			}
			if !strings.Contains(capturedSystem, "@") {
				t.Errorf("expected mention section in system prompt (skipMention should be false for channel_type=%q), but it was absent", tt.channelType)
			}
		})
	}
}

type mockNetError struct {
	timeout bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false }

func TestTranscribeHandler_RequestBodyTooLarge(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	largeData := make([]byte, 6*1024*1024)
	part.Write(largeData)
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "request body too large" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func TestTranscribeHandler_InvalidModel(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
		Models:                 []string{"gemini-2.5-pro"},
		GPTModels:              []string{"gpt-4o-mini-transcribe"},
		QwenModels:             []string{"qwen3.5-omni-plus"},
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.WriteField("model", "malicious-model")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid model, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "unsupported model" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func TestTranscribeHandler_ValidModel(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
		Models:                 []string{"gemini-2.5-pro"},
		GPTModels:              []string{"gpt-4o-mini-transcribe"},
		QwenModels:             []string{"qwen3.5-omni-plus"},
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.WriteField("model", "gemini-2.5-pro")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["msg"] == "unsupported model" {
			t.Error("valid model should not be rejected")
		}
	}
}

func TestClassifyTranscribeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"deadline exceeded", context.DeadlineExceeded, "transcription failed: timeout"},
		{"wrapped deadline", fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "transcription failed: timeout"},
		{"net timeout", &mockNetError{timeout: true}, "transcription failed: timeout"},
		{"net connection error", &mockNetError{timeout: false}, "transcription failed: service unavailable"},
		{"generic error", fmt.Errorf("something went wrong"), "transcription failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTranscribeError(tt.err); got != tt.want {
				t.Errorf("classifyTranscribeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTranscribeHandler_AllowFeedback_InvalidValue(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		AllowFeedbackLog:       true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	invalidValues := []string{"maybe", "yes", "no", "2", " "}
	for _, val := range invalidValues {
		t.Run("allow_feedback="+val, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("audio", "test.wav")
			part.Write([]byte("fake audio"))
			writer.WriteField("allow_feedback", val)
			writer.Close()

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for allow_feedback=%q, got %d: %s", val, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["msg"] != "invalid allow_feedback value, expected: true or false" {
				t.Errorf("unexpected msg: %v", resp["msg"])
			}
		})
	}
}

func TestTranscribeHandler_AllowFeedback_ValidValues(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		AllowFeedbackLog:       true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	validValues := []string{"true", "false", "1", "0", "t", "f", "TRUE", "FALSE"}
	for _, val := range validValues {
		t.Run("allow_feedback="+val, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("audio", "test.wav")
			part.Write([]byte("fake audio"))
			writer.WriteField("allow_feedback", val)
			writer.Close()

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			r.ServeHTTP(w, req)

			if w.Code == http.StatusBadRequest {
				var resp map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &resp)
				if resp["msg"] == "invalid allow_feedback value, expected: true or false" {
					t.Errorf("valid allow_feedback=%q was rejected", val)
				}
			}
		})
	}
}

func TestTranscribeHandler_AllowFeedback_NotProvided(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3145728,
		EmotionEmoji:           true,
		AllowFeedbackLog:       true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write([]byte("fake audio"))
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp["msg"] == "invalid allow_feedback value, expected: true or false" {
			t.Error("missing allow_feedback should not cause 400")
		}
	}
}

func TestTranscribeHandler_AllowFeedback_LoggingBehavior(t *testing.T) {
	service.ResetPromptsToDefaults()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "transcribed text"}},
			},
		})
	}))
	defer backend.Close()

	tests := []struct {
		name             string
		allowFeedbackLog bool
		paramValue       string
		expectLogged     bool
	}{
		{"config=true, param=true", true, "true", true},
		{"config=true, param=false", true, "false", false},
		{"config=false, param=true", false, "true", true},
		{"config=false, param=false", false, "false", false},
		{"config=true, no param", true, "", true},
		{"config=false, no param", false, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			asrLogger := asrlog.NewLogger(logDir, 256, "test-pod", nil)
			if asrLogger == nil {
				t.Fatal("failed to create asr logger")
			}
			defer asrLogger.Close()

			cfg := &config.Config{
				MaxUploadSize:          5 * 1024 * 1024,
				MaxFileSize:            3145728,
				EmotionEmoji:           true,
				AllowFeedbackLog:       tt.allowFeedbackLog,
				MaxContextTextLength:   5000,
				MaxChatContextLength:   20000,
				MaxVoiceContextLength:  10000,
				MaxMemberContextLength: 5000,
				Engine:                 config.EngineGemini,
				Models:                 []string{"test-model"},
				LiteLLMUrl:             backend.URL,
				LiteLLMKey:             "test-key",
				Timeout:                10,
				TotalTimeout:           15,
			}

			svc := service.NewTranscribeService(cfg)
			h := NewTranscribeHandler(svc, cfg, asrLogger, nil)

			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("app_id", "test-app")
				c.Next()
			})
			r.POST("/v1/speech/transcribe", h.Handle)

			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("audio", "test.wav")
			part.Write([]byte("fake audio"))
			if tt.paramValue != "" {
				writer.WriteField("allow_feedback", tt.paramValue)
			}
			writer.Close()

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
			}

			asrLogger.Close()

			hasLogFiles := false
			filepath.WalkDir(logDir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".json") {
					hasLogFiles = true
				}
				return nil
			})

			if tt.expectLogged && !hasLogFiles {
				t.Error("expected log files to be written, but none found")
			}
			if !tt.expectLogged && hasLogFiles {
				t.Error("expected no log files, but some were found")
			}
		})
	}
}

func TestTranscribeHandler_RequestBodyWithinLimit(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          5 * 1024 * 1024,
		MaxFileSize:            3 * 1024 * 1024,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	part.Write(make([]byte, 100))
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code == http.StatusRequestEntityTooLarge {
		t.Errorf("expected request within limit to not return 413, got %d", w.Code)
	}
}

func TestTranscribeHandler_MaxUploadSize(t *testing.T) {
	cfg := &config.Config{
		MaxUploadSize:          1024,
		MaxFileSize:            512,
		EmotionEmoji:           true,
		MaxContextTextLength:   5000,
		MaxChatContextLength:   20000,
		MaxVoiceContextLength:  10000,
		MaxMemberContextLength: 5000,
	}

	h := NewTranscribeHandler(nil, cfg, nil, nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.POST("/v1/speech/transcribe", h.Handle)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("audio", "test.wav")
	largeData := make([]byte, 2048)
	part.Write(largeData)
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/speech/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "request body too large" {
		t.Errorf("unexpected msg: %v", resp["msg"])
	}
}

func setupLocalConfigRouter() (*gin.Engine, *LocalConfigHandler) {
	cfg := &config.Config{
		LocalEnabled:       false,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
	}
	localStore := store.NewLocalConfigStore(nil, cfg)
	h := NewLocalConfigHandler(localStore)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.PUT("/v1/speech/local-config", h.Put)
	r.GET("/v1/speech/local-config", h.Get)
	r.DELETE("/v1/speech/local-config", h.Delete)
	return r, h
}

func setupLocalConfigRouterWithMock(t *testing.T) (*gin.Engine, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{
		LocalEnabled:       false,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
	}
	localStore := store.NewLocalConfigStore(db, cfg)
	h := NewLocalConfigHandler(localStore)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "test-app")
		c.Next()
	})
	r.PUT("/v1/speech/local-config", h.Put)
	r.GET("/v1/speech/local-config", h.Get)
	r.DELETE("/v1/speech/local-config", h.Delete)
	return r, mock
}

func TestLocalConfigHandler_PutValidation(t *testing.T) {
	r, _ := setupLocalConfigRouter()

	tests := []struct {
		name string
		body string
		want int
		msg  string
	}{
		{
			"invalid json",
			`{bad`,
			400,
			"invalid request body",
		},
		{
			"missing subject_id",
			`{"scope_type":"global","scope_id":"default","enabled":true}`,
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"missing scope_type",
			`{"subject_id":"user1","scope_id":"default","enabled":true}`,
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"missing scope_id",
			`{"subject_id":"user1","scope_type":"global","enabled":true}`,
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"invalid scope_type",
			`{"subject_id":"user1","scope_type":"invalid","scope_id":"default","enabled":true}`,
			400,
			"invalid scope_type, expected: global, space, org, project",
		},
		{
			"missing enabled",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","timeout_ms":5000}`,
			400,
			"enabled is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/v1/speech/local-config",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["msg"] != tt.msg {
				t.Errorf("expected msg %q, got %v", tt.msg, resp["msg"])
			}
		})
	}
}

func TestLocalConfigHandler_GetValidation(t *testing.T) {
	r, _ := setupLocalConfigRouter()

	tests := []struct {
		name string
		url  string
		want int
		msg  string
	}{
		{
			"missing all params",
			"/v1/speech/local-config",
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"missing scope_type",
			"/v1/speech/local-config?subject_id=u1&scope_id=s1",
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"missing scope_id",
			"/v1/speech/local-config?subject_id=u1&scope_type=global",
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"invalid scope_type",
			"/v1/speech/local-config?subject_id=u1&scope_type=bad&scope_id=s1",
			400,
			"invalid scope_type, expected: global, space, org, project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", tt.url, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["msg"] != tt.msg {
				t.Errorf("expected msg %q, got %v", tt.msg, resp["msg"])
			}
		})
	}
}

func TestLocalConfigHandler_GetDefaultValues(t *testing.T) {
	cfg := &config.Config{
		LocalEnabled:       false,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",
	}
	localStore := store.NewLocalConfigStore(nil, cfg)
	h := NewLocalConfigHandler(localStore)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app_id", "")
		c.Next()
	})
	r.GET("/v1/speech/local-config", h.Get)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/local-config?subject_id=u1&scope_type=global&scope_id=default", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["enabled"] != false {
		t.Errorf("expected enabled false, got %v", resp["enabled"])
	}
	if resp["timeout_ms"] != float64(10000) {
		t.Errorf("expected timeout_ms 10000, got %v", resp["timeout_ms"])
	}
	if resp["probe_url"] != "http://localhost:8787/" {
		t.Errorf("expected default probe_url, got %v", resp["probe_url"])
	}
	if resp["transcribe_url"] != "http://localhost:8787/v1/voice/transcribe" {
		t.Errorf("expected default transcribe_url, got %v", resp["transcribe_url"])
	}
}

func TestLocalConfigHandler_DeleteValidation(t *testing.T) {
	r, _ := setupLocalConfigRouter()

	tests := []struct {
		name string
		url  string
		want int
		msg  string
	}{
		{
			"missing all params",
			"/v1/speech/local-config",
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"missing subject_id",
			"/v1/speech/local-config?scope_type=global&scope_id=default",
			400,
			"subject_id, scope_type, and scope_id are required",
		},
		{
			"invalid scope_type",
			"/v1/speech/local-config?subject_id=u1&scope_type=bad&scope_id=s1",
			400,
			"invalid scope_type, expected: global, space, org, project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", tt.url, nil)
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["msg"] != tt.msg {
				t.Errorf("expected msg %q, got %v", tt.msg, resp["msg"])
			}
		})
	}
}

func TestLocalConfigHandler_PatchPartialUpdate(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"only timeout_ms without enabled",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","timeout_ms":5000}`,
			400,
		},
		{
			"only enabled",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true}`,
			200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r *gin.Engine
			if tt.want == 200 {
				var mock sqlmock.Sqlmock
				r, mock = setupLocalConfigRouterWithMock(t)
				mock.ExpectExec("INSERT INTO local_asr_config").
					WillReturnResult(sqlmock.NewResult(1, 1))
			} else {
				r, _ = setupLocalConfigRouter()
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/v1/speech/local-config",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestLocalConfigHandler_PatchEnabledRequired(t *testing.T) {
	r, _ := setupLocalConfigRouter()

	body := `{"subject_id":"user1","scope_type":"global","scope_id":"default","timeout_ms":5000}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/speech/local-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "enabled is required" {
		t.Errorf("expected msg 'enabled is required', got %v", resp["msg"])
	}
}

func TestLocalConfigHandler_TimeoutMsValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"timeout_ms=0",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"timeout_ms":0}`,
			400,
		},
		{
			"timeout_ms=-5",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"timeout_ms":-5}`,
			400,
		},
		{
			"timeout_ms=60001",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"timeout_ms":60001}`,
			400,
		},
		{
			"timeout_ms=30000",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"timeout_ms":30000}`,
			200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r *gin.Engine
			if tt.want == 200 {
				var mock sqlmock.Sqlmock
				r, mock = setupLocalConfigRouterWithMock(t)
				mock.ExpectExec("INSERT INTO local_asr_config").
					WillReturnResult(sqlmock.NewResult(1, 1))
			} else {
				r, _ = setupLocalConfigRouter()
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/v1/speech/local-config",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestLocalConfigHandler_URLValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			"probe_url not-a-url",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"probe_url":"not-a-url"}`,
			400,
		},
		{
			"probe_url ftp scheme",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"probe_url":"ftp://x"}`,
			400,
		},
		{
			"probe_url valid http",
			`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"probe_url":"http://localhost:8787/"}`,
			200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var r *gin.Engine
			if tt.want == 200 {
				var mock sqlmock.Sqlmock
				r, mock = setupLocalConfigRouterWithMock(t)
				mock.ExpectExec("INSERT INTO local_asr_config").
					WillReturnResult(sqlmock.NewResult(1, 1))
			} else {
				r, _ = setupLocalConfigRouter()
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/v1/speech/local-config",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestLocalConfigHandler_GetQueryError(t *testing.T) {
	r, mock := setupLocalConfigRouterWithMock(t)

	mock.ExpectQuery("SELECT enabled, timeout_ms, probe_url, transcribe_url").
		WillReturnError(fmt.Errorf("connection refused"))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/v1/speech/local-config?subject_id=u1&scope_type=global&scope_id=default", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "failed to query local config" {
		t.Errorf("expected msg 'failed to query local config', got %v", resp["msg"])
	}
}

func TestLocalConfigHandler_DeleteNotFound(t *testing.T) {
	r, mock := setupLocalConfigRouterWithMock(t)

	mock.ExpectExec("DELETE FROM local_asr_config").
		WithArgs("test-app", "u1", "global", "default").
		WillReturnResult(sqlmock.NewResult(0, 0))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/speech/local-config?subject_id=u1&scope_type=global&scope_id=default", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["msg"] != "config not found" {
		t.Errorf("expected msg 'config not found', got %v", resp["msg"])
	}
}

func TestLocalConfigHandler_DeleteSuccess(t *testing.T) {
	r, mock := setupLocalConfigRouterWithMock(t)

	mock.ExpectExec("DELETE FROM local_asr_config").
		WithArgs("test-app", "u1", "global", "default").
		WillReturnResult(sqlmock.NewResult(0, 1))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/speech/local-config?subject_id=u1&scope_type=global&scope_id=default", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLocalConfigHandler_URLTooLong(t *testing.T) {
	r, _ := setupLocalConfigRouter()

	longURL := "http://example.com/" + strings.Repeat("a", 500)
	body := fmt.Sprintf(`{"subject_id":"user1","scope_type":"global","scope_id":"default","enabled":true,"probe_url":"%s"}`, longURL)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/v1/speech/local-config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	msg, _ := resp["msg"].(string)
	if !strings.Contains(msg, "500 characters") {
		t.Errorf("expected error about 500 characters, got %v", resp["msg"])
	}
}
