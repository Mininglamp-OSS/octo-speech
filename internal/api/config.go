package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Mininglamp-OSS/octo-speech/internal/config"
	"github.com/Mininglamp-OSS/octo-speech/internal/store"
)

type ConfigHandler struct {
	cfg          *config.Config
	localCfgStore *store.LocalConfigStore
}

func NewConfigHandler(cfg *config.Config, localCfgStore *store.LocalConfigStore) *ConfigHandler {
	return &ConfigHandler{cfg: cfg, localCfgStore: localCfgStore}
}

func (h *ConfigHandler) Handle(c *gin.Context) {
	subjectID := c.Query("subject_id")
	scopeType := c.Query("scope_type")
	scopeID := c.Query("scope_id")

	appID := ""
	if v, exists := c.Get("app_id"); exists {
		appID, _ = v.(string)
	}

	localCfg := h.localCfgStore.Query(appID, subjectID, scopeType, scopeID)

	resp := gin.H{
		"enabled":              true,
		"max_duration":         h.cfg.MaxDuration,
		"max_file_size":        h.cfg.MaxFileSize,
		"engine":               h.cfg.EngineShort(),
		"edit_mode":            h.cfg.EditMode,
		"local_enabled":        localCfg.Enabled,
		"local_timeout_ms":     localCfg.TimeoutMs,
		"local_probe_url":      localCfg.ProbeURL,
		"local_transcribe_url": localCfg.TranscribeURL,
	}

	if h.cfg.FeedbackURL != "" {
		resp["feedback_url"] = h.cfg.FeedbackURL
	}

	c.JSON(http.StatusOK, resp)
}
