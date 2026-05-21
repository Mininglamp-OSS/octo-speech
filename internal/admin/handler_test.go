package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/Mininglamp-OSS/octo-speech/internal/adminconfig"
)

type mockDB struct {
	pingErr error
}

func (m *mockDB) Ping() error { return m.pingErr }

func newTestHandler() *Handler {
	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.MinCost)
	cfg := &adminconfig.AdminConfig{
		Username:     "admin",
		PasswordHash: string(hash),
		JWTSecret:    "test-secret-key",
		TokenExpire:  24,
		SecureCookie: false,
	}
	return NewHandler(nil, nil, cfg, &mockDB{}, zap.NewNop())
}

func TestHandler_HealthCheck_OK(t *testing.T) {
	h := NewHandler(nil, nil, nil, &mockDB{}, zap.NewNop())
	r := gin.New()
	r.GET("/healthz", h.HealthCheck)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["db"] != "ok" {
		t.Errorf("expected db=ok, got %v", resp["db"])
	}
}

func TestHandler_HealthCheck_DBDown(t *testing.T) {
	h := NewHandler(nil, nil, nil, &mockDB{pingErr: http.ErrServerClosed}, zap.NewNop())
	r := gin.New()
	r.GET("/healthz", h.HealthCheck)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	r.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandler_Login_Success(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/api/login", h.Login)

	body := `{"username":"admin","password":"testpass"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cookies := w.Result().Cookies()
	var hasToken, hasCSRF bool
	for _, c := range cookies {
		if c.Name == "token" {
			hasToken = true
		}
		if c.Name == "csrf_token" {
			hasCSRF = true
		}
	}
	if !hasToken {
		t.Error("expected token cookie")
	}
	if !hasCSRF {
		t.Error("expected csrf_token cookie")
	}
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/api/login", h.Login)

	body := `{"username":"admin","password":"wrongpass"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_Login_WrongUsername(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/api/login", h.Login)

	body := `{"username":"wronguser","password":"testpass"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHandler_Login_InvalidBody(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/api/login", h.Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_Logout(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.POST("/api/logout", h.Logout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/logout", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "token" && c.MaxAge > 0 {
			t.Error("expected token cookie to be cleared")
		}
	}
}

func TestHandler_CreateApp_Validation(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	r.POST("/api/apps", h.CreateApp)

	tests := []struct {
		name string
		body string
		want int
	}{
		{"empty name", `{"app_name":""}`, 400},
		{"whitespace only", `{"app_name":"   "}`, 400},
		{"invalid json", `not json`, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/apps", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_UpdateStatus_Validation(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	r.PUT("/api/apps/:app_id/status", h.UpdateStatus)

	validAppID := "app_AbCdEfGhIjKlMnOp"
	tests := []struct {
		name string
		body string
		want int
	}{
		{"invalid status", `{"status":2}`, 400},
		{"missing status", `{}`, 400},
		{"invalid json", `bad`, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/api/apps/"+validAppID+"/status", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.want {
				t.Errorf("expected %d, got %d: %s", tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestJWTMiddleware_MissingCookie(t *testing.T) {
	r := gin.New()
	r.Use(JWTMiddleware("secret"))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_InvalidToken(t *testing.T) {
	r := gin.New()
	r.Use(JWTMiddleware("secret"))
	r.GET("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "invalid-token"})
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret"
	token, _, err := signJWT("admin", secret, 1)
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(JWTMiddleware(secret))
	r.GET("/test", func(c *gin.Context) {
		username, _ := c.Get("username")
		c.String(200, username.(string))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "admin" {
		t.Errorf("expected 'admin', got %q", w.Body.String())
	}
}

func TestCSRFMiddleware_MissingCookie(t *testing.T) {
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFMiddleware_Mismatch(t *testing.T) {
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "token-abc"})
	req.Header.Set("X-CSRF-Token", "different-token")
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFMiddleware_Match(t *testing.T) {
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "token-abc"})
	req.Header.Set("X-CSRF-Token", "token-abc")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSRFMiddleware_ConstantTimeCompare(t *testing.T) {
	r := gin.New()
	r.Use(CSRFMiddleware())
	r.POST("/test", func(c *gin.Context) {
		c.String(200, "ok")
	})

	token := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200 for matching CSRF tokens, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: token})
	req.Header.Set("X-CSRF-Token", token+"x")
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403 for mismatched CSRF tokens, got %d", w.Code)
	}
}

func TestIsValidAppID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", "app_AbCdEfGhIjKlMnOp", true},
		{"valid all digits", "app_1234567890123456", true},
		{"valid all lowercase", "app_abcdefghijklmnop", true},
		{"valid all uppercase", "app_ABCDEFGHIJKLMNOP", true},
		{"too short", "app_abc", false},
		{"too long", "app_AbCdEfGhIjKlMnOpQ", false},
		{"wrong prefix", "xxx_AbCdEfGhIjKlMnOp", false},
		{"no prefix", "AbCdEfGhIjKlMnOpQrSt", false},
		{"special chars", "app_AbCdEfGh!jKlMnOp", false},
		{"newline", "app_AbCdEfGh\njKlMnOp", false},
		{"control char", "app_AbCdEfGh\x00jKlMnOp", false},
		{"space", "app_AbCdEfGh IjKlMnO", false},
		{"underscore in suffix", "app_AbCd_fGhIjKlMnOp", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidAppID(tt.input)
			if got != tt.want {
				t.Errorf("isValidAppID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestHandler_UpdateStatus_InvalidAppID(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	r.PUT("/api/apps/:app_id/status", h.UpdateStatus)

	tests := []struct {
		name  string
		appID string
	}{
		{"too short", "app_short"},
		{"special chars", "app_AbCdEfGh!jKlMnOp"},
		{"wrong prefix", "xxx_AbCdEfGhIjKlMnOp"},
		{"underscore in suffix", "app_AbCd_fGhIjKlMnOp"},
		{"space in suffix", "app_AbCdEfGh%20KlMnOp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("PUT", "/api/apps/"+tt.appID+"/status", strings.NewReader(`{"status":1}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != 400 {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_DeleteApp_InvalidAppID(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	r.DELETE("/api/apps/:app_id", h.DeleteApp)

	tests := []struct {
		name  string
		appID string
	}{
		{"too short", "app_x"},
		{"special chars", "app_AbCdEfGh!jKlMnOp"},
		{"wrong length", "app_AbCdEfGhIjKl"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/api/apps/"+tt.appID, nil)
			r.ServeHTTP(w, req)

			if w.Code != 400 {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHandler_ResetKey_InvalidAppID(t *testing.T) {
	h := newTestHandler()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("username", "admin")
		c.Next()
	})
	r.POST("/api/apps/:app_id/reset-key", h.ResetKey)

	tests := []struct {
		name  string
		appID string
	}{
		{"special chars", "app_AbCd;DROP_TABLE!"},
		{"empty suffix", "app_"},
		{"too long", "app_AbCdEfGhIjKlMnOpQ"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/apps/"+tt.appID+"/reset-key", nil)
			r.ServeHTTP(w, req)

			if w.Code != 400 {
				t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
