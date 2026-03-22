package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"refleks-api/internal/config"
)

// Server wraps an HTTP server and its runtime behavior.
type Server struct {
	logger *slog.Logger
	http   *http.Server
	config config.Config
}

// New creates a new HTTP server instance.
func New(cfg config.Config, logger *slog.Logger, handler http.Handler) *Server {
	return &Server{
		logger: logger,
		config: cfg,
		http: &http.Server{
			Addr:              ":" + strconv.Itoa(cfg.Port),
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
	}
}

// Run starts the HTTP server and performs graceful shutdown on context cancellation.
func (s *Server) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)

	go func() {
		serverErr <- s.http.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen and serve: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()

		s.logger.Info("shutdown signal received, stopping server")
		if err := s.http.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		err := <-serverErr
		if errors.Is(err, http.ErrServerClosed) || err == nil {
			return nil
		}
		return fmt.Errorf("server closed with error: %w", err)
	}
}
