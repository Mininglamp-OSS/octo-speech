package api

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/Mininglamp-OSS/octo-speech/internal/store"
)

type LocalConfigHandler struct {
	localCfgStore *store.LocalConfigStore
}

func NewLocalConfigHandler(localCfgStore *store.LocalConfigStore) *LocalConfigHandler {
	return &LocalConfigHandler{localCfgStore: localCfgStore}
}

func (h *LocalConfigHandler) Put(c *gin.Context) {
	appID, _ := c.Get("app_id")
	appIDStr, _ := appID.(string)

	var req struct {
		SubjectID     string  `json:"subject_id"`
		ScopeType     string  `json:"scope_type"`
		ScopeID       string  `json:"scope_id"`
		Enabled       *bool   `json:"enabled"`
		TimeoutMs     *int    `json:"timeout_ms"`
		ProbeURL      *string `json:"probe_url"`
		TranscribeURL *string `json:"transcribe_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "invalid request body"})
		return
	}

	if req.SubjectID == "" || req.ScopeType == "" || req.ScopeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "subject_id, scope_type, and scope_id are required"})
		return
	}

	if !store.IsValidScopeType(req.ScopeType) {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "invalid scope_type, expected: global, space, org, project"})
		return
	}

	if req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "enabled is required"})
		return
	}

	if req.TimeoutMs != nil {
		if *req.TimeoutMs < 1 || *req.TimeoutMs > 60000 {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "timeout_ms must be between 1 and 60000"})
			return
		}
	}

	if req.ProbeURL != nil {
		if err := validateLocalURL(*req.ProbeURL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "probe_url: " + err.Error()})
			return
		}
	}

	if req.TranscribeURL != nil {
		if err := validateLocalURL(*req.TranscribeURL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "transcribe_url: " + err.Error()})
			return
		}
	}

	if err := h.localCfgStore.Upsert(appIDStr, req.SubjectID, req.ScopeType, req.ScopeID, *req.Enabled, req.TimeoutMs, req.ProbeURL, req.TranscribeURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "msg": "failed to save local config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "msg": "ok"})
}

func (h *LocalConfigHandler) Get(c *gin.Context) {
	appID, _ := c.Get("app_id")
	appIDStr, _ := appID.(string)

	subjectID := c.Query("subject_id")
	scopeType := c.Query("scope_type")
	scopeID := c.Query("scope_id")

	if subjectID == "" || scopeType == "" || scopeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "subject_id, scope_type, and scope_id are required"})
		return
	}

	if !store.IsValidScopeType(scopeType) {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "invalid scope_type, expected: global, space, org, project"})
		return
	}

	localCfg, err := h.localCfgStore.Query(appIDStr, subjectID, scopeType, scopeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "msg": "failed to query local config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         http.StatusOK,
		"enabled":        localCfg.Enabled,
		"timeout_ms":     localCfg.TimeoutMs,
		"probe_url":      localCfg.ProbeURL,
		"transcribe_url": localCfg.TranscribeURL,
	})
}

func (h *LocalConfigHandler) Delete(c *gin.Context) {
	appID, _ := c.Get("app_id")
	appIDStr, _ := appID.(string)

	subjectID := c.Query("subject_id")
	scopeType := c.Query("scope_type")
	scopeID := c.Query("scope_id")

	if subjectID == "" || scopeType == "" || scopeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "subject_id, scope_type, and scope_id are required"})
		return
	}

	if !store.IsValidScopeType(scopeType) {
		c.JSON(http.StatusBadRequest, gin.H{"status": http.StatusBadRequest, "msg": "invalid scope_type, expected: global, space, org, project"})
		return
	}

	rows, err := h.localCfgStore.Delete(appIDStr, subjectID, scopeType, scopeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "msg": "failed to delete local config"})
		return
	}
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": http.StatusNotFound, "msg": "config not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": http.StatusOK, "msg": "ok"})
}

func validateLocalURL(raw string) error {
	if len(raw) > 500 {
		return fmt.Errorf("URL must not exceed 500 characters")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	if u.Host == "" {
		return fmt.Errorf("host is required")
	}
	return nil
}
