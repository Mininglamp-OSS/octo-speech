package config

import (
	"os"
	"testing"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Port != 8780 {
		t.Errorf("expected port 8780, got %d", cfg.Port)
	}
	if cfg.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", cfg.Timeout)
	}
	if cfg.TotalTimeout != 45 {
		t.Errorf("expected total timeout 45, got %d", cfg.TotalTimeout)
	}
	if cfg.MaxDuration != 60 {
		t.Errorf("expected max duration 60, got %d", cfg.MaxDuration)
	}
	if cfg.MaxFileSize != 3*1024*1024 {
		t.Errorf("expected max file size 3MB, got %d", cfg.MaxFileSize)
	}
	if cfg.Engine != EngineGemini {
		t.Errorf("expected engine gemini, got %s", cfg.Engine)
	}
	if cfg.EditMode != "edit" {
		t.Errorf("expected edit mode 'edit', got %s", cfg.EditMode)
	}
	if !cfg.EmotionEmoji {
		t.Error("expected emotion emoji true")
	}
	if cfg.LocalEnabled {
		t.Error("expected local enabled false")
	}
	if cfg.LocalTimeoutMs != 10000 {
		t.Errorf("expected local timeout 10000, got %d", cfg.LocalTimeoutMs)
	}
	if cfg.Hostname != "local" {
		t.Errorf("expected hostname 'local', got %s", cfg.Hostname)
	}
	if cfg.CacheTTL != 60 {
		t.Errorf("expected cache TTL 60, got %d", cfg.CacheTTL)
	}
	if cfg.MaxVoiceContextLength != 10000 {
		t.Errorf("expected max voice ctx 10000, got %d", cfg.MaxVoiceContextLength)
	}
	if cfg.MaxContextTextLength != 5000 {
		t.Errorf("expected max ctx text 5000, got %d", cfg.MaxContextTextLength)
	}
	if cfg.MaxChatContextLength != 20000 {
		t.Errorf("expected max chat ctx 20000, got %d", cfg.MaxChatContextLength)
	}
	if cfg.MaxMemberContextLength != 5000 {
		t.Errorf("expected max member ctx 5000, got %d", cfg.MaxMemberContextLength)
	}
	if len(cfg.Models) != 3 || cfg.Models[0] != "gemini-3.1-pro-preview" {
		t.Errorf("unexpected default models: %v", cfg.Models)
	}
	if cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog false by default")
	}
}

func TestLoadFromEnv_CustomValues(t *testing.T) {
	os.Clearenv()

	os.Setenv("SPEECH_SERVICE_PORT", "9090")
	os.Setenv("SPEECH_DB_DSN", "user:pass@tcp(localhost:3306)/speech")
	os.Setenv("SPEECH_APP_CACHE_TTL", "120")
	os.Setenv("HOSTNAME", "pod-1")
	os.Setenv("VOICE_LITELLM_URL", "http://litellm:8080")
	os.Setenv("VOICE_LITELLM_KEY", "sk-test")
	os.Setenv("VOICE_ENGINE", "gpt")
	os.Setenv("VOICE_GPT_MODELS", "gpt-4o-transcribe,gpt-4o-mini-transcribe")
	os.Setenv("VOICE_EMOTION_EMOJI", "false")
	os.Setenv("VOICE_LOCAL_TIMEOUT_MS", "90000")
	os.Setenv("VOICE_EDIT_MODE", "append")
	os.Setenv("VOICE_MODELS", "gemini-2.5-pro")

	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.DBDsn != "user:pass@tcp(localhost:3306)/speech?loc=Local&parseTime=true" {
		t.Errorf("unexpected DSN: %s", cfg.DBDsn)
	}
	if cfg.CacheTTL != 120 {
		t.Errorf("expected cache TTL 120, got %d", cfg.CacheTTL)
	}
	if cfg.Hostname != "pod-1" {
		t.Errorf("expected hostname 'pod-1', got %s", cfg.Hostname)
	}
	if cfg.Engine != EngineGPT {
		t.Errorf("expected engine gpt, got %s", cfg.Engine)
	}
	if len(cfg.GPTModels) != 2 {
		t.Errorf("expected 2 GPT models, got %d", len(cfg.GPTModels))
	}
	if cfg.EmotionEmoji {
		t.Error("expected emotion emoji false")
	}
	// local timeout capped at 60000
	if cfg.LocalTimeoutMs != 60000 {
		t.Errorf("expected local timeout capped at 60000, got %d", cfg.LocalTimeoutMs)
	}
	if cfg.EditMode != "append" {
		t.Errorf("expected edit mode 'append', got %s", cfg.EditMode)
	}
	if len(cfg.Models) != 1 || cfg.Models[0] != "gemini-2.5-pro" {
		t.Errorf("unexpected models: %v", cfg.Models)
	}
}

func TestLoadFromEnv_FeedbackURL_Default(t *testing.T) {
	os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.FeedbackURL != "" {
		t.Errorf("expected empty feedback URL by default, got %q", cfg.FeedbackURL)
	}
}

func TestLoadFromEnv_FeedbackURL_Set(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_FEEDBACK_URL", "https://example.com/feedback")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.FeedbackURL != "https://example.com/feedback" {
		t.Errorf("expected feedback URL 'https://example.com/feedback', got %q", cfg.FeedbackURL)
	}
}

func TestLoadFromEnv_GPTForceAppend(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ENGINE", "gpt")
	os.Setenv("VOICE_EDIT_MODE", "edit")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EditMode != "append" {
		t.Errorf("expected GPT to force append, got %s", cfg.EditMode)
	}
}

func TestLoadFromEnv_GPTDefaultAppend(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ENGINE", "gpt")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EditMode != "append" {
		t.Errorf("expected GPT default append, got %s", cfg.EditMode)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid gemini",
			cfg: Config{
				DBDsn:         "dsn",
				LiteLLMUrl:    "http://url",
				LiteLLMKey:    "key",
				Engine:        EngineGemini,
				Models:        []string{"m1"},
				MaxUploadSize: 5 * 1024 * 1024,
				MaxFileSize:   3 * 1024 * 1024,
			},
		},
		{
			name: "missing dsn",
			cfg: Config{
				LiteLLMUrl: "http://url",
				LiteLLMKey: "key",
				Engine:     EngineGemini,
				Models:     []string{"m1"},
			},
			wantErr: true,
		},
		{
			name: "missing url",
			cfg: Config{
				DBDsn:      "dsn",
				LiteLLMKey: "key",
				Engine:     EngineGemini,
				Models:     []string{"m1"},
			},
			wantErr: true,
		},
		{
			name: "valid gpt",
			cfg: Config{
				DBDsn:         "dsn",
				LiteLLMUrl:    "http://url",
				LiteLLMKey:    "key",
				Engine:        EngineGPT,
				GPTModels:     []string{"gpt4"},
				MaxUploadSize: 5 * 1024 * 1024,
				MaxFileSize:   3 * 1024 * 1024,
			},
		},
		{
			name: "valid qwen with own url",
			cfg: Config{
				DBDsn:         "dsn",
				Engine:        EngineQwen,
				QwenUrl:       "http://qwen",
				QwenKey:       "qk",
				QwenModels:    []string{"qm"},
				MaxUploadSize: 5 * 1024 * 1024,
				MaxFileSize:   3 * 1024 * 1024,
			},
		},
		{
			name: "qwen missing url",
			cfg: Config{
				DBDsn:      "dsn",
				Engine:     EngineQwen,
				QwenKey:    "qk",
				QwenModels: []string{"qm"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeEngine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gm", EngineGemini},
		{"gemini", EngineGemini},
		{"GEMINI", EngineGemini},
		{"gp", EngineGPT},
		{"gpt", EngineGPT},
		{"qw", EngineQwen},
		{"qwen", EngineQwen},
		{"invalid", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeEngine(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeEngine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateRunesTail(t *testing.T) {
	tests := []struct {
		s    string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 3, "llo"},
		{"你好世界", 2, "世界"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := TruncateRunesTail(tt.s, tt.max)
		if got != tt.want {
			t.Errorf("TruncateRunesTail(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
		}
	}
}

func TestShortenModelName(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"gemini-3.1-pro-preview", "g31pp"},
		{"gpt-4o-mini-transcribe", "gpt4omt"},
		{"unknown-model", "unknown-model"},
		{"qwen3.5-omni-plus", "q35op"},
	}

	for _, tt := range tests {
		got := ShortenModelName(tt.model)
		if got != tt.want {
			t.Errorf("ShortenModelName(%q) = %q, want %q", tt.model, got, tt.want)
		}
	}
}

func TestEngineShort(t *testing.T) {
	tests := []struct {
		engine string
		want   string
	}{
		{EngineGemini, "gm"},
		{EngineGPT, "gp"},
		{EngineQwen, "qw"},
	}

	for _, tt := range tests {
		cfg := &Config{Engine: tt.engine}
		got := cfg.EngineShort()
		if got != tt.want {
			t.Errorf("EngineShort() for %s = %q, want %q", tt.engine, got, tt.want)
		}
	}
}

func TestLoadFromEnv_AllowFeedbackLog_Default(t *testing.T) {
	os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog false by default")
	}
}

func TestLoadFromEnv_AllowFeedbackLog_SetFalse(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ALLOW_FEEDBACK", "false")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog false when VOICE_ALLOW_FEEDBACK=false")
	}
}

func TestLoadFromEnv_AllowFeedbackLog_SetTrue(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ALLOW_FEEDBACK", "true")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog true when VOICE_ALLOW_FEEDBACK=true")
	}
}

func TestLoadFromEnv_AllowFeedbackLog_InvalidReturnsError(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ALLOW_FEEDBACK", "yes")
	defer os.Clearenv()

	_, err := LoadFromEnv(nil)
	if err == nil {
		t.Error("expected error when VOICE_ALLOW_FEEDBACK has invalid value")
	}
}

func TestLoadFromEnv_AllowFeedbackLog_Zero(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ALLOW_FEEDBACK", "0")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog false when VOICE_ALLOW_FEEDBACK=0")
	}
}

func TestLoadFromEnv_AllowFeedbackLog_One(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_ALLOW_FEEDBACK", "1")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog true when VOICE_ALLOW_FEEDBACK=1")
	}
}

func TestLoadFromEnv_AllowFeedbackLog_Unset(t *testing.T) {
	os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AllowFeedbackLog {
		t.Error("expected AllowFeedbackLog false when VOICE_ALLOW_FEEDBACK is not set")
	}
}

func TestLoadFromEnv_MaxUploadSize_Default(t *testing.T) {
	os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxUploadSize != 5*1024*1024 {
		t.Errorf("expected max upload size 5MB, got %d", cfg.MaxUploadSize)
	}
}

func TestLoadFromEnv_MaxUploadSize_FromEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_MAX_UPLOAD_SIZE", "10485760")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxUploadSize != 10485760 {
		t.Errorf("expected max upload size 10485760, got %d", cfg.MaxUploadSize)
	}
}

func TestLoadFromEnv_MaxUploadSize_ZeroFallsBackToDefault(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_MAX_UPLOAD_SIZE", "0")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxUploadSize != 5*1024*1024 {
		t.Errorf("expected default 5MB when VOICE_MAX_UPLOAD_SIZE=0, got %d", cfg.MaxUploadSize)
	}
}

func TestLoadFromEnv_MaxUploadSize_NegativeFallsBackToDefault(t *testing.T) {
	os.Clearenv()
	os.Setenv("VOICE_MAX_UPLOAD_SIZE", "-100")
	defer os.Clearenv()

	cfg, err := LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.MaxUploadSize != 5*1024*1024 {
		t.Errorf("expected default 5MB when VOICE_MAX_UPLOAD_SIZE=-100, got %d", cfg.MaxUploadSize)
	}
}

func TestValidate_MaxUploadSizeZero(t *testing.T) {
	cfg := Config{
		DBDsn:         "dsn",
		LiteLLMUrl:    "http://url",
		LiteLLMKey:    "key",
		Engine:        EngineGemini,
		Models:        []string{"m1"},
		MaxUploadSize: 0,
		MaxFileSize:   3 * 1024 * 1024,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error when MaxUploadSize is 0")
	}
	if err != nil && err.Error() != "VOICE_MAX_UPLOAD_SIZE must be positive" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MaxFileSizeOneLessThanMaxUploadSize(t *testing.T) {
	cfg := Config{
		DBDsn:         "dsn",
		LiteLLMUrl:    "http://url",
		LiteLLMKey:    "key",
		Engine:        EngineGemini,
		Models:        []string{"m1"},
		MaxUploadSize: 5 * 1024 * 1024,
		MaxFileSize:   5*1024*1024 - 1,
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("expected no error when MaxFileSize == MaxUploadSize-1, got: %v", err)
	}
}

func TestValidate_MaxFileSizeExceedsMaxUploadSize(t *testing.T) {
	cfg := Config{
		DBDsn:        "dsn",
		LiteLLMUrl:   "http://url",
		LiteLLMKey:   "key",
		Engine:       EngineGemini,
		Models:       []string{"m1"},
		MaxUploadSize: 5 * 1024 * 1024,
		MaxFileSize:   5 * 1024 * 1024,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("expected error when MaxFileSize >= MaxUploadSize")
	}
	if err != nil && err.Error() != "VOICE_MAX_FILE_SIZE must be smaller than VOICE_MAX_UPLOAD_SIZE" {
		t.Errorf("unexpected error: %v", err)
	}
}
