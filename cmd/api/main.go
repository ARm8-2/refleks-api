package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"refleks-api/internal/config"
	"refleks-api/internal/httpapi"
	"refleks-api/internal/httpserver"
	"refleks-api/internal/status"
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
	router := httpapi.NewRouter(logger, status.NewHandler(statusService))
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
