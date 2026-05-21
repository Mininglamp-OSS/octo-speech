package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/Mininglamp-OSS/octo-speech/internal/adminconfig"
	"github.com/Mininglamp-OSS/octo-speech/internal/store"
)

//go:embed static/*
var staticFS embed.FS

func StaticFS() http.FileSystem {
	sub, _ := fs.Sub(staticFS, "static")
	return http.FS(sub)
}

type Handler struct {
	appStore   *store.AppStore
	auditStore *store.AuditStore
	cfg        *adminconfig.AdminConfig
	db         interface{ Ping() error }
	logger     *zap.Logger
}

func NewHandler(appStore *store.AppStore, auditStore *store.AuditStore, cfg *adminconfig.AdminConfig, db interface{ Ping() error }, logger *zap.Logger) *Handler {
	return &Handler{
		appStore:   appStore,
		auditStore: auditStore,
		cfg:        cfg,
		db:         db,
		logger:     logger,
	}
}

func (h *Handler) HealthCheck(c *gin.Context) {
	if err := h.db.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": 503,
			"msg":    "database unavailable",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  200,
		"msg":     "ok",
		"version": "1.0.0",
		"db":      "ok",
	})
}

func (h *Handler) ServeIndex(c *gin.Context) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "index.html not found")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "invalid request body"})
		return
	}

	if req.Username != h.cfg.Username {
		bcrypt.CompareHashAndPassword([]byte("$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"), []byte(req.Password))
		c.JSON(http.StatusUnauthorized, gin.H{"status": 401, "msg": "invalid credentials"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(h.cfg.PasswordHash), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"status": 401, "msg": "invalid credentials"})
		return
	}

	token, expiresAt, err := signJWT(req.Username, h.cfg.JWTSecret, h.cfg.TokenExpire)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to generate token"})
		return
	}

	csrfToken := generateCSRFToken()
	setAuthCookies(c, token, csrfToken, h.cfg)

	c.JSON(http.StatusOK, gin.H{
		"status":     200,
		"msg":        "ok",
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) Logout(c *gin.Context) {
	clearAuthCookies(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"status": 200, "msg": "ok"})
}

func (h *Handler) ListApps(c *gin.Context) {
	var statusPtr *int
	if s := c.Query("status"); s != "" {
		v := 0
		if s == "1" {
			v = 1
		}
		statusPtr = &v
	}
	keyword := c.Query("keyword")

	apps, err := h.appStore.List(c.Request.Context(), statusPtr, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to list apps"})
		return
	}

	items := make([]gin.H, 0, len(apps))
	for _, a := range apps {
		items = append(items, gin.H{
			"app_id":     a.AppID,
			"app_name":   a.AppName,
			"status":     a.Status,
			"created_at": a.CreatedAt.Format(time.RFC3339),
			"updated_at": a.UpdatedAt.Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": 200,
		"msg":    "ok",
		"items":  items,
	})
}

func (h *Handler) CreateApp(c *gin.Context) {
	var req struct {
		AppName string `json:"app_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "invalid request body"})
		return
	}

	appName := strings.TrimSpace(req.AppName)
	if appName == "" || len([]rune(appName)) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "app_name must be 1-100 characters"})
		return
	}

	appID, err := GenerateAppID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to generate app ID"})
		return
	}
	apiKey, err := GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to generate API key"})
		return
	}
	apiKeyHash := hashAPIKey(apiKey)

	now := time.Now()
	if err := h.appStore.Create(c.Request.Context(), appID, appName, apiKeyHash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to create app"})
		return
	}

	username, _ := c.Get("username")
	if err := h.auditStore.Log(c.Request.Context(), store.AuditEntry{
		Action:   "create",
		AppID:    appID,
		AppName:  appName,
		Operator: username.(string),
	}); err != nil {
		h.logger.Error("failed to write audit log", zap.Error(err))
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":     201,
		"msg":        "ok",
		"app_id":     appID,
		"app_name":   appName,
		"api_key":    apiKey,
		"created_at": now.Format(time.RFC3339),
	})
}

func isValidAppID(id string) bool {
	if len(id) != 20 || id[:4] != "app_" {
		return false
	}
	for _, c := range id[4:] {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func (h *Handler) UpdateStatus(c *gin.Context) {
	appID := c.Param("app_id")
	if !isValidAppID(appID) {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "invalid app_id format"})
		return
	}

	var req struct {
		Status *int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "invalid request body"})
		return
	}

	if *req.Status != 0 && *req.Status != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "status must be 0 or 1"})
		return
	}

	if err := h.appStore.UpdateStatus(c.Request.Context(), appID, *req.Status); err != nil {
		if err == store.ErrAppNotFound {
			c.JSON(http.StatusNotFound, gin.H{"status": 404, "msg": "app not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to update status"})
		return
	}

	action := "enable"
	if *req.Status == 0 {
		action = "disable"
	}

	username, _ := c.Get("username")
	appName := ""
	app, err := h.appStore.GetByAppID(c.Request.Context(), appID)
	if err != nil && err != store.ErrAppNotFound {
		h.logger.Error("failed to get app for audit log", zap.String("app_id", appID), zap.Error(err))
	}
	if app != nil {
		appName = app.AppName
	}
	if err := h.auditStore.Log(c.Request.Context(), store.AuditEntry{
		Action:   action,
		AppID:    appID,
		AppName:  appName,
		Operator: username.(string),
	}); err != nil {
		h.logger.Error("failed to write audit log", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"status": 200, "msg": "ok"})
}

func (h *Handler) DeleteApp(c *gin.Context) {
	appID := c.Param("app_id")
	if !isValidAppID(appID) {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "invalid app_id format"})
		return
	}

	app, err := h.appStore.GetByAppID(c.Request.Context(), appID)
	if err != nil && err != store.ErrAppNotFound {
		h.logger.Error("failed to get app for audit log", zap.String("app_id", appID), zap.Error(err))
	}

	if err := h.appStore.Delete(c.Request.Context(), appID); err != nil {
		if err == store.ErrAppNotFound {
			c.JSON(http.StatusNotFound, gin.H{"status": 404, "msg": "app not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to delete app"})
		return
	}

	username, _ := c.Get("username")
	appName := ""
	if app != nil {
		appName = app.AppName
	}
	if err := h.auditStore.Log(c.Request.Context(), store.AuditEntry{
		Action:   "delete",
		AppID:    appID,
		AppName:  appName,
		Operator: username.(string),
	}); err != nil {
		h.logger.Error("failed to write audit log", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"status": 200, "msg": "ok"})
}

func (h *Handler) ResetKey(c *gin.Context) {
	appID := c.Param("app_id")
	if !isValidAppID(appID) {
		c.JSON(http.StatusBadRequest, gin.H{"status": 400, "msg": "invalid app_id format"})
		return
	}

	apiKey, err := GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to generate API key"})
		return
	}
	apiKeyHash := hashAPIKey(apiKey)

	if err := h.appStore.UpdateAPIKey(c.Request.Context(), appID, apiKeyHash); err != nil {
		if err == store.ErrAppNotFound {
			c.JSON(http.StatusNotFound, gin.H{"status": 404, "msg": "app not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": 500, "msg": "failed to reset key"})
		return
	}

	username, _ := c.Get("username")
	appName := ""
	app, err := h.appStore.GetByAppID(c.Request.Context(), appID)
	if err != nil && err != store.ErrAppNotFound {
		h.logger.Error("failed to get app for audit log", zap.String("app_id", appID), zap.Error(err))
	}
	if app != nil {
		appName = app.AppName
	}
	if err := h.auditStore.Log(c.Request.Context(), store.AuditEntry{
		Action:   "reset_key",
		AppID:    appID,
		AppName:  appName,
		Operator: username.(string),
	}); err != nil {
		h.logger.Error("failed to write audit log", zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  200,
		"msg":     "ok",
		"api_key": apiKey,
	})
}
