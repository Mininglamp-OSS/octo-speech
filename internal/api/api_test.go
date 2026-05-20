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
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

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
		MaxFileSize:  3145728,
		EmotionEmoji: true,
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
