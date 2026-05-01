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
	"refleks-api/internal/benchmarks"
	benchadapters "refleks-api/internal/benchmarks/adapters"
	"refleks-api/internal/config"
	"refleks-api/internal/httpapi"
	"refleks-api/internal/httpserver"
	"refleks-api/internal/leaderboards"
	leaderboardadapters "refleks-api/internal/leaderboards/adapters"
	"refleks-api/internal/players"
	playeradapters "refleks-api/internal/players/adapters"
	"refleks-api/internal/runs"
	runadapters "refleks-api/internal/runs/adapters"
	"refleks-api/internal/scenarios"
	scenarioadapters "refleks-api/internal/scenarios/adapters"
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
	var benchmarkRoutes *httpapi.BenchmarkRoutes
	var leaderboardRoutes *httpapi.LeaderboardRoutes

	var supabaseClient *supabase.Client
	if cfg.SupabaseDBURL != "" {
		supabaseClient, err = supabase.NewClient(context.Background(), cfg.SupabaseDBURL)
		if err != nil {
			return fmt.Errorf("init supabase client: %w", err)
		}
		pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelPing()
		if err := supabaseClient.Ping(pingCtx); err != nil {
			return fmt.Errorf("ping supabase: %w", err)
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

	if supabaseClient != nil {
		benchmarkRepo, err := benchadapters.NewSupabaseRepository(supabaseClient.Pool())
		if err != nil {
			return fmt.Errorf("init benchmark repository: %w", err)
		}
		benchmarkSvc := benchmarks.NewService(benchmarkRepo)
		benchmarkHandler := benchmarks.NewHandler(benchmarkSvc)
		benchmarkRoutes = &httpapi.BenchmarkRoutes{List: http.HandlerFunc(benchmarkHandler.HandleList)}

		leaderboardRepo, err := leaderboardadapters.NewSupabaseRepository(supabaseClient.Pool())
		if err != nil {
			return fmt.Errorf("init leaderboard repository: %w", err)
		}
		leaderboardSvc := leaderboards.NewService(leaderboardRepo)
		leaderboardHandler := leaderboards.NewHandler(leaderboardSvc)
		leaderboardRoutes = &httpapi.LeaderboardRoutes{
			Scenario:            http.HandlerFunc(leaderboardHandler.HandleScenario),
			BenchmarkDifficulty: http.HandlerFunc(leaderboardHandler.HandleBenchmarkDifficulty),
		}

		logger.Info("benchmark and leaderboard endpoints enabled")
	} else {
		logger.Warn("benchmark and leaderboard endpoints disabled: SUPABASE_DB_URL is empty")
	}

	var runRoutes *httpapi.RunRoutes
	var scenarioRoutes *httpapi.ScenarioRoutes
	var playerRoutes *httpapi.PlayerRoutes
	if supabaseClient != nil {
		runRepo, err := runadapters.NewSupabaseRepository(supabaseClient.Pool())
		if err != nil {
			return fmt.Errorf("init run repository: %w", err)
		}
		// Read-only run service: store is not needed for browse/get endpoints.
		readSvc := runs.NewService(runRepo, nil, "")
		readHandler := runs.NewHandler(readSvc, runs.HandlerConfig{})
		runRoutes = &httpapi.RunRoutes{
			RunsList: http.HandlerFunc(readHandler.HandleListRuns),
			GetRun:   http.HandlerFunc(readHandler.HandleGetRun),
		}

		hasR2Config := cfg.R2Endpoint != "" && cfg.R2RawPublicBucket != "" && cfg.R2AccessKeyID != "" && cfg.R2SecretAccessKey != ""
		if hasR2Config {
			store, err := runadapters.NewR2Store(context.Background(), runadapters.R2Config{
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

			storeSvc := runs.NewService(runRepo, store, cfg.R2KeyPrefix)
			storeHandler := runs.NewHandler(storeSvc, runs.HandlerConfig{
				MaxSingleFileBytes: cfg.RunSyncMaxFileBytes,
				MaxBulkFileCount:   cfg.RunSyncMaxBulkFiles,
				MaxBulkTotalBytes:  cfg.RunSyncMaxBulkBytes,
				MaxMissingHashes:   cfg.RunSyncMaxMissingHashes,
			})
			runRoutes.RawDownload = http.HandlerFunc(storeHandler.HandleDownloadRaw)
			runRoutes.RawURL = http.HandlerFunc(storeHandler.HandleDownloadRawURL)

			if cfg.RunSyncEnabled {
				runRoutes.Sync = http.HandlerFunc(storeHandler.HandleSync)
				runRoutes.BulkSync = http.HandlerFunc(storeHandler.HandleBulkSync)
				runRoutes.MissingHashes = http.HandlerFunc(storeHandler.HandleMissingHashes)
				logger.Info("run sync enabled")
			} else {
				logger.Info("run sync disabled: upload endpoints inactive")
			}

			logger.Info("run raw download endpoints enabled")
		} else if cfg.RunSyncEnabled {
			return fmt.Errorf("R2 endpoint, bucket, and credentials are required when RUNSYNC_ENABLED=true")
		} else {
			logger.Warn("run raw download endpoints disabled: R2 configuration is incomplete")
		}

		scenarioRepo, err := scenarioadapters.NewSupabaseRepository(supabaseClient.Pool())
		if err != nil {
			return fmt.Errorf("init scenario repository: %w", err)
		}
		scenarioSvc := scenarios.NewService(scenarioRepo)
		scenarioHandler := scenarios.NewHandler(scenarioSvc)
		scenarioRoutes = &httpapi.ScenarioRoutes{
			List: http.HandlerFunc(scenarioHandler.HandleList),
			Get:  http.HandlerFunc(scenarioHandler.HandleGet),
		}

		playerRepo, err := playeradapters.NewSupabaseRepository(supabaseClient.Pool())
		if err != nil {
			return fmt.Errorf("init player repository: %w", err)
		}
		playerSvc := players.NewService(playerRepo)
		playerHandler := players.NewHandler(playerSvc)
		playerRoutes = &httpapi.PlayerRoutes{
			List: http.HandlerFunc(playerHandler.HandleList),
			Get:  http.HandlerFunc(playerHandler.HandleGet),
		}

		logger.Info("run, scenario, and player read endpoints enabled")
	} else {
		logger.Warn("run, scenario, and player endpoints disabled: SUPABASE_DB_URL is empty")
	}

	router := httpapi.NewRouter(logger, status.NewHandler(statusService), authRoutes, benchmarkRoutes, leaderboardRoutes, scenarioRoutes, playerRoutes, runRoutes)
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
