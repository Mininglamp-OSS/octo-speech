package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"github.com/Mininglamp-OSS/octo-speech/internal/api"
	"github.com/Mininglamp-OSS/octo-speech/internal/asrlog"
	"github.com/Mininglamp-OSS/octo-speech/internal/config"
	"github.com/Mininglamp-OSS/octo-speech/internal/migration"
	"github.com/Mininglamp-OSS/octo-speech/internal/service"
	"github.com/Mininglamp-OSS/octo-speech/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.LoadFromEnv(logger)
	if err != nil {
		logger.Fatal("failed to load configuration", zap.Error(err))
	}
	if err := cfg.Validate(); err != nil {
		logger.Fatal("invalid configuration", zap.Error(err))
	}

	db, err := sql.Open("mysql", cfg.DBDsn)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	n, err := migration.Run(db)
	if err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}
	if n > 0 {
		logger.Info("applied migrations", zap.Int("count", n))
	}

	service.LoadPrompts(cfg.PromptFile, logger)

	appStore := store.NewAppStore(db, cfg.CacheTTL)
	vocabStore := store.NewVocabularyStore(db)
	localCfgStore := store.NewLocalConfigStore(db, cfg)

	svc := service.NewTranscribeService(cfg)

	var asrLogger *asrlog.Logger
	var asrCleaner *asrlog.Cleaner
	if cfg.ASRLogDir != "" {
		asrLogger = asrlog.NewLogger(cfg.ASRLogDir, cfg.ASRLogBufferSize, cfg.Hostname, logger)
		if asrLogger != nil {
			asrCleaner = asrlog.NewCleaner(cfg.ASRLogDir, cfg.ASRLogRetentionDays, logger)
			asrCleaner.Start()
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	transcribeHandler := api.NewTranscribeHandler(svc, cfg, asrLogger, logger)
	configHandler := api.NewConfigHandler(cfg, localCfgStore)
	vocabHandler := api.NewVocabularyHandler(vocabStore)

	auth := api.AuthMiddleware(appStore)

	protected := r.Group("/v1/speech")
	protected.Use(auth)
	{
		protected.GET("/config", configHandler.Handle)
		protected.POST("/transcribe", transcribeHandler.Handle)
		protected.PUT("/vocabularies", vocabHandler.Put)
		protected.GET("/vocabularies", vocabHandler.Get)
		protected.DELETE("/vocabularies", vocabHandler.Delete)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	logger.Info("starting octo-speech", zap.String("addr", addr))

	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	if asrLogger != nil {
		asrLogger.Close()
	}
	if asrCleaner != nil {
		asrCleaner.Close()
	}
}
