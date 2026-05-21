package adminconfig

import (
	"os"
	"testing"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func TestLoadFromEnv_DefaultPort(t *testing.T) {
	os.Unsetenv("ADMIN_PORT")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "pass")
	defer os.Unsetenv("ADMIN_USERNAME")
	defer os.Unsetenv("ADMIN_PASSWORD")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.Port != 8781 {
		t.Errorf("expected default port 8781, got %d", cfg.Port)
	}
}

func TestLoadFromEnv_CustomPort(t *testing.T) {
	os.Setenv("ADMIN_PORT", "9090")
	defer os.Unsetenv("ADMIN_PORT")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	os.Setenv("ADMIN_PORT", "not-a-number")
	defer os.Unsetenv("ADMIN_PORT")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.Port != 8781 {
		t.Errorf("expected default port 8781 for invalid input, got %d", cfg.Port)
	}
}

func TestLoadFromEnv_PasswordHash(t *testing.T) {
	os.Setenv("ADMIN_PASSWORD", "mypassword")
	defer os.Unsetenv("ADMIN_PASSWORD")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.PasswordHash == "" {
		t.Fatal("expected non-empty password hash")
	}

	err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte("mypassword"))
	if err != nil {
		t.Errorf("bcrypt hash does not match password: %v", err)
	}
}

func TestLoadFromEnv_EmptyPassword(t *testing.T) {
	os.Unsetenv("ADMIN_PASSWORD")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.PasswordHash != "" {
		t.Errorf("expected empty password hash for empty password, got %q", cfg.PasswordHash)
	}
}

func TestLoadFromEnv_TokenExpire(t *testing.T) {
	os.Setenv("ADMIN_TOKEN_EXPIRE", "8")
	defer os.Unsetenv("ADMIN_TOKEN_EXPIRE")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.TokenExpire != 8 {
		t.Errorf("expected token expire 8, got %d", cfg.TokenExpire)
	}
}

func TestLoadFromEnv_DefaultTokenExpire(t *testing.T) {
	os.Unsetenv("ADMIN_TOKEN_EXPIRE")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.TokenExpire != 24 {
		t.Errorf("expected default token expire 24, got %d", cfg.TokenExpire)
	}
}

func TestLoadFromEnv_SecureCookie(t *testing.T) {
	os.Unsetenv("ADMIN_SECURE_COOKIE")
	cfg := LoadFromEnv(zap.NewNop())
	if !cfg.SecureCookie {
		t.Error("expected SecureCookie=true by default")
	}

	os.Setenv("ADMIN_SECURE_COOKIE", "false")
	defer os.Unsetenv("ADMIN_SECURE_COOKIE")
	cfg = LoadFromEnv(zap.NewNop())
	if cfg.SecureCookie {
		t.Error("expected SecureCookie=false when set to 'false'")
	}
}

func TestLoadFromEnv_TrustedProxies(t *testing.T) {
	os.Setenv("ADMIN_TRUSTED_PROXIES", " 10.0.0.1 , 10.0.0.2 ")
	defer os.Unsetenv("ADMIN_TRUSTED_PROXIES")

	cfg := LoadFromEnv(zap.NewNop())
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(cfg.TrustedProxies))
	}
	if cfg.TrustedProxies[0] != "10.0.0.1" || cfg.TrustedProxies[1] != "10.0.0.2" {
		t.Errorf("unexpected proxies: %v", cfg.TrustedProxies)
	}
}

func TestLoadFromEnv_JWTSecretFromEnv(t *testing.T) {
	os.Setenv("ADMIN_JWT_SECRET", "my-secret")
	defer os.Unsetenv("ADMIN_JWT_SECRET")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.JWTSecret != "my-secret" {
		t.Errorf("expected 'my-secret', got %q", cfg.JWTSecret)
	}
}

func TestLoadFromEnv_JWTSecretGenerated(t *testing.T) {
	os.Unsetenv("ADMIN_JWT_SECRET")

	cfg := LoadFromEnv(zap.NewNop())
	if cfg.JWTSecret == "" {
		t.Fatal("expected non-empty generated JWT secret")
	}
	if len(cfg.JWTSecret) < 32 {
		t.Errorf("expected generated secret to be at least 32 chars, got %d", len(cfg.JWTSecret))
	}
}

func TestLoadFromEnv_DBDsnEnriched(t *testing.T) {
	os.Setenv("SPEECH_DB_DSN", "user:pass@tcp(127.0.0.1:3306)/testdb")
	defer os.Unsetenv("SPEECH_DB_DSN")

	cfg := LoadFromEnv(zap.NewNop())

	want := "user:pass@tcp(127.0.0.1:3306)/testdb?loc=Local&parseTime=true"
	if cfg.DBDsn != want {
		t.Errorf("DBDsn not enriched\n got: %q\nwant: %q", cfg.DBDsn, want)
	}
}
