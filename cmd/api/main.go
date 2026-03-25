package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"refleks-api/internal/auth"
	"refleks-api/internal/config"
	"refleks-api/internal/httpapi"
	"refleks-api/internal/httpserver"
	"refleks-api/internal/runsync"
	"refleks-api/internal/runsync/adapters"
	"refleks-api/internal/status"
	"refleks-api/internal/supabase"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "application error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(version)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	statusService := status.NewService(cfg.AppName, cfg.Environment, cfg.Version, time.Now())
	var authRoutes *httpapi.AuthRoutes

	var supabaseClient *supabase.Client
	if cfg.SupabaseDBURL != "" {
		supabaseClient, err = supabase.NewClient(context.Background(), cfg.SupabaseDBURL)
		if err != nil {
			return fmt.Errorf("init supabase client: %w", err)
		}
		defer supabaseClient.Close()
		logger.Info("supabase client enabled")
	} else {
		logger.Warn("supabase client disabled: SUPABASE_DB_URL is empty")
	}

	authSvc := auth.NewService(supabaseClient)
	authHandler := auth.NewHandler(authSvc)
	authRoutes = &httpapi.AuthRoutes{
		SessionStub: http.HandlerFunc(authHandler.HandleSessionStub),
		SteamLogin:  http.HandlerFunc(authHandler.HandleSteamLoginStub),
	}

	var runSyncRoutes *httpapi.RunSyncRoutes
	if cfg.RunSyncEnabled {
		repo, err := adapters.NewSupabaseRepository(context.Background(), supabaseClient.Pool())
		if err != nil {
			return fmt.Errorf("init supabase repository: %w", err)
		}

		store, err := adapters.NewR2Store(context.Background(), adapters.R2Config{
			Endpoint:        cfg.R2Endpoint,
			Region:          cfg.R2Region,
			Bucket:          cfg.R2RawPublicBucket,
			PublicBaseURL:   cfg.R2RawPublicBaseURL,
			SignedURLTTL:    cfg.R2SignedURLTTL,
			AccessKeyID:     cfg.R2AccessKeyID,
			SecretAccessKey: cfg.R2SecretAccessKey,
		})
		if err != nil {
			return fmt.Errorf("init r2 store: %w", err)
		}

		svc := runsync.NewService(repo, store, cfg.R2KeyPrefix)
		h := runsync.NewHandler(svc, runsync.HandlerConfig{
			MaxSingleFileBytes: cfg.RunSyncMaxFileBytes,
			MaxBulkFileCount:   cfg.RunSyncMaxBulkFiles,
			MaxBulkTotalBytes:  cfg.RunSyncMaxBulkBytes,
			MaxMissingHashes:   cfg.RunSyncMaxMissingHashes,
		})

		runSyncRoutes = &httpapi.RunSyncRoutes{
			RunsList:      http.HandlerFunc(h.HandleListRuns),
			Sync:          http.HandlerFunc(h.HandleSync),
			BulkSync:      http.HandlerFunc(h.HandleBulkSync),
			MissingHashes: http.HandlerFunc(h.HandleMissingHashes),
			RawDownload:   http.HandlerFunc(h.HandleDownloadRaw),
			RawURL:        http.HandlerFunc(h.HandleDownloadRawURL),
		}

		logger.Info("run sync enabled")
	} else {
		logger.Warn("run sync disabled")
	}

	router := httpapi.NewRouter(logger, status.NewHandler(statusService), authRoutes, runSyncRoutes)
	server := httpserver.New(cfg, logger, router)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting api server",
		slog.String("service", cfg.AppName),
		slog.String("environment", cfg.Environment),
		slog.Int("port", cfg.Port),
		slog.String("version", cfg.Version),
	)

	if err := server.Run(ctx); err != nil {
		return fmt.Errorf("run server: %w", err)
	}

	logger.Info("server stopped")
	return nil
}
