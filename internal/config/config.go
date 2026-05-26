package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Mininglamp-OSS/octo-speech/internal/dsnutil"
	"go.uber.org/zap"
)

const (
	EngineGemini = "gemini"
	EngineGPT    = "gpt"
	EngineQwen   = "qwen"
)

const (
	defaultMaxUploadSize = 5 * 1024 * 1024
	defaultPort          = 8780
	defaultTimeout       = 30
	defaultTotalTimeout  = 45
	defaultMaxDuration   = 60
	defaultMaxFileSize   = 3 * 1024 * 1024
	defaultCacheTTL      = 60
	defaultLogBufSize    = 256
	defaultRetention     = 7
)

const (
	DefaultMaxVoiceContextLength  = 10000
	DefaultMaxContextTextLength   = 5000
	DefaultMaxChatContextLength   = 20000
	DefaultMaxMemberContextLength = 5000
)

var defaultModels = []string{"gemini-3.1-pro-preview", "gemini-3-flash-preview", "gemini-2.5-pro"}
var defaultGPTModels = []string{"gpt-4o-mini-transcribe"}
var defaultQwenModels = []string{"qwen3.5-omni-plus"}

type Config struct {
	Port     int
	DBDsn    string
	CacheTTL int

	LiteLLMUrl   string
	LiteLLMKey   string
	Timeout      int
	TotalTimeout int
	Models       []string
	MaxDuration int
	// MaxUploadSize is the hard limit on the entire HTTP request body size (including
	// multipart boundaries and headers). Requests exceeding this are rejected with 413.
	// VOICE_MAX_FILE_SIZE must be smaller than this value, otherwise the file size
	// check will never trigger (the request body will be truncated first).
	MaxUploadSize int64
	MaxFileSize   int64
	Engine       string
	GPTModels    []string
	Language     string
	EditMode     string

	QwenModels  []string
	QwenUrl     string
	QwenKey     string
	QwenTimeout int

	PromptFile   string
	EmotionEmoji bool

	LocalEnabled       bool
	LocalTimeoutMs     int
	LocalProbeURL      string
	LocalTranscribeURL string

	MaxVoiceContextLength  int
	MaxContextTextLength   int
	MaxChatContextLength   int
	MaxMemberContextLength int

	ASRLogDir          string
	ASRLogBufferSize   int
	ASRLogRetentionDays int

	Hostname         string
	FeedbackURL      string
	AllowFeedbackLog bool

	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func LoadFromEnv(logger *zap.Logger) (*Config, error) {
	models := make([]string, len(defaultModels))
	copy(models, defaultModels)
	gptModels := make([]string, len(defaultGPTModels))
	copy(gptModels, defaultGPTModels)
	qwenModels := make([]string, len(defaultQwenModels))
	copy(qwenModels, defaultQwenModels)

	cfg := &Config{
		Port:         defaultPort,
		CacheTTL:     defaultCacheTTL,
		LiteLLMUrl:   os.Getenv("VOICE_LITELLM_URL"),
		LiteLLMKey:   os.Getenv("VOICE_LITELLM_KEY"),
		Timeout:      defaultTimeout,
		TotalTimeout: defaultTotalTimeout,
		Models:       models,
		MaxDuration:  defaultMaxDuration,
		MaxUploadSize: defaultMaxUploadSize,
		MaxFileSize:  defaultMaxFileSize,
		Engine:       EngineGemini,
		GPTModels:    gptModels,
		QwenModels:   qwenModels,

		EmotionEmoji:       true,
		LocalEnabled:       true,
		LocalTimeoutMs:     10000,
		LocalProbeURL:      "http://localhost:8787/",
		LocalTranscribeURL: "http://localhost:8787/v1/voice/transcribe",

		MaxVoiceContextLength:  DefaultMaxVoiceContextLength,
		MaxContextTextLength:   DefaultMaxContextTextLength,
		MaxChatContextLength:   DefaultMaxChatContextLength,
		MaxMemberContextLength: DefaultMaxMemberContextLength,

		ASRLogBufferSize:    defaultLogBufSize,
		ASRLogRetentionDays: defaultRetention,
		AllowFeedbackLog:    false,

		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	cfg.DBDsn = dsnutil.EnsureDSNTimeParams(os.Getenv("SPEECH_DB_DSN"))

	if v := os.Getenv("SPEECH_SERVICE_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Port = n
		}
	}

	if v := os.Getenv("SPEECH_APP_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.CacheTTL = n
		}
	}

	cfg.Hostname = os.Getenv("HOSTNAME")
	if cfg.Hostname == "" {
		cfg.Hostname = "local"
	}

	if v := os.Getenv("VOICE_LITELLM_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Timeout = n
		}
	}

	if v := os.Getenv("VOICE_TOTAL_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.TotalTimeout = n
		}
	}

	if v := os.Getenv("VOICE_MODELS"); v != "" {
		if parsed := splitModels(v); len(parsed) > 0 {
			cfg.Models = parsed
		}
	}

	if v := os.Getenv("VOICE_MAX_DURATION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxDuration = n
		}
	}

	if v := os.Getenv("VOICE_MAX_FILE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.MaxFileSize = n
		}
	}

	if v := os.Getenv("VOICE_MAX_UPLOAD_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.MaxUploadSize = n
		}
	}

	if v := os.Getenv("VOICE_ENGINE"); v == EngineGPT || v == EngineGemini || v == EngineQwen {
		cfg.Engine = v
	}

	if v := os.Getenv("VOICE_GPT_MODELS"); v != "" {
		if parsed := splitModels(v); len(parsed) > 0 {
			cfg.GPTModels = parsed
		}
	}

	if v := os.Getenv("VOICE_QWEN_MODELS"); v != "" {
		if parsed := splitModels(v); len(parsed) > 0 {
			cfg.QwenModels = parsed
		}
	}

	if v := os.Getenv("VOICE_QWEN_URL"); v != "" {
		cfg.QwenUrl = v
	}
	if v := os.Getenv("VOICE_QWEN_KEY"); v != "" {
		cfg.QwenKey = v
	}
	if v := os.Getenv("VOICE_QWEN_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.QwenTimeout = n
		}
	}

	if v := os.Getenv("VOICE_LANGUAGE"); v != "" {
		cfg.Language = v
	}

	if v := os.Getenv("VOICE_PROMPT_FILE"); v != "" {
		cfg.PromptFile = v
	}

	if v := os.Getenv("VOICE_EMOTION_EMOJI"); v == "false" || v == "0" {
		cfg.EmotionEmoji = false
	}

	cfg.MaxVoiceContextLength = getEnvInt("VOICE_MAX_VOICE_CONTEXT_LENGTH", DefaultMaxVoiceContextLength)
	cfg.MaxContextTextLength = getEnvInt("VOICE_MAX_CONTEXT_TEXT_LENGTH", DefaultMaxContextTextLength)
	cfg.MaxChatContextLength = getEnvInt("VOICE_MAX_CHAT_CONTEXT_LENGTH", DefaultMaxChatContextLength)
	cfg.MaxMemberContextLength = getEnvInt("VOICE_MAX_MEMBER_CONTEXT_LENGTH", DefaultMaxMemberContextLength)

	if v := os.Getenv("VOICE_EDIT_MODE"); v == "edit" || v == "append" {
		cfg.EditMode = v
	} else {
		if cfg.Engine == EngineGPT {
			cfg.EditMode = "append"
		} else {
			cfg.EditMode = "edit"
		}
	}

	if cfg.Engine == EngineGPT && cfg.EditMode == "edit" {
		if logger != nil {
			logger.Warn("GPT engine does not support edit mode, forcing append")
		}
		cfg.EditMode = "append"
	}

	if v := os.Getenv("VOICE_LOCAL_ENABLED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.LocalEnabled = b
		}
	}

	if v := os.Getenv("VOICE_LOCAL_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 60000 {
				n = 60000
			}
			cfg.LocalTimeoutMs = n
		}
	}

	if v := os.Getenv("VOICE_LOCAL_PROBE_URL"); v != "" {
		cfg.LocalProbeURL = v
	}
	if v := os.Getenv("VOICE_LOCAL_TRANSCRIBE_URL"); v != "" {
		cfg.LocalTranscribeURL = v
	}

	cfg.ASRLogDir = os.Getenv("ASR_LOG_DIR")

	cfg.FeedbackURL = os.Getenv("VOICE_FEEDBACK_URL")

	if v := os.Getenv("VOICE_ALLOW_FEEDBACK"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid VOICE_ALLOW_FEEDBACK value %q: %w", v, err)
		}
		cfg.AllowFeedbackLog = b
	}

	if v := os.Getenv("ASR_LOG_BUFFER_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ASRLogBufferSize = n
		}
	}

	if v := os.Getenv("ASR_LOG_RETENTION_DAYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ASRLogRetentionDays = n
		}
	}

	if v := os.Getenv("SPEECH_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.ReadTimeout = d
		}
	}
	if v := os.Getenv("SPEECH_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.WriteTimeout = d
		}
	}
	if v := os.Getenv("SPEECH_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.IdleTimeout = d
		}
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.DBDsn == "" {
		return errors.New("SPEECH_DB_DSN is required")
	}

	if c.MaxUploadSize <= 0 {
		return errors.New("VOICE_MAX_UPLOAD_SIZE must be positive")
	}
	if c.MaxFileSize >= c.MaxUploadSize {
		return errors.New("VOICE_MAX_FILE_SIZE must be smaller than VOICE_MAX_UPLOAD_SIZE")
	}

	if c.Engine == EngineQwen {
		url := c.QwenUrl
		if url == "" {
			url = c.LiteLLMUrl
		}
		if url == "" {
			return errors.New("VOICE_QWEN_URL or VOICE_LITELLM_URL is required for qwen engine")
		}
		key := c.QwenKey
		if key == "" {
			key = c.LiteLLMKey
		}
		if key == "" {
			return errors.New("VOICE_QWEN_KEY or VOICE_LITELLM_KEY is required for qwen engine")
		}
		if len(c.QwenModels) == 0 {
			return errors.New("VOICE_QWEN_MODELS is required for qwen engine")
		}
		return nil
	}

	if c.LiteLLMUrl == "" {
		return errors.New("VOICE_LITELLM_URL is required")
	}
	if c.LiteLLMKey == "" {
		return errors.New("VOICE_LITELLM_KEY is required")
	}
	if c.Engine == EngineGPT {
		if len(c.GPTModels) == 0 {
			return errors.New("VOICE_GPT_MODELS is required for GPT engine")
		}
	} else {
		if len(c.Models) == 0 {
			return errors.New("VOICE_MODELS is required")
		}
	}
	return nil
}

func (c *Config) EngineShort() string {
	switch c.Engine {
	case EngineGPT:
		return "gp"
	case EngineQwen:
		return "qw"
	default:
		return "gm"
	}
}

func NormalizeEngine(s string) string {
	switch strings.ToLower(s) {
	case "gm", "gemini":
		return EngineGemini
	case "gp", "gpt":
		return EngineGPT
	case "qw", "qwen":
		return EngineQwen
	default:
		return ""
	}
}

func EngineToShort(engine string) string {
	switch engine {
	case EngineGPT:
		return "gp"
	case EngineQwen:
		return "qw"
	default:
		return "gm"
	}
}

func splitModels(v string) []string {
	parts := strings.Split(v, ",")
	var out []string
	for _, m := range parts {
		m = strings.TrimSpace(m)
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}

func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}

func TruncateRunesTail(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[len(runes)-max:])
}

var modelAbbreviations = map[string]string{
	"gemini-3.1-pro-preview": "g31pp",
	"gemini-3-flash-preview": "g3fp",
	"gemini-2.5-pro":         "g25p",
	"gemini-2.0-flash":       "g20f",
	"gemini-2.0-flash-lite":  "g20fl",
	"gpt-4o-transcribe":      "gpt4ot",
	"gpt-4o-mini-transcribe": "gpt4omt",
	"whisper-1":              "w1",
	"whisper-large-v3":       "wlv3",
	"qwen3.5-omni-plus":      "q35op",
}

func ShortenModelName(model string) string {
	if short, ok := modelAbbreviations[model]; ok {
		return short
	}
	return model
}
