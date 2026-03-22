package httpapi

import (
	"log/slog"
	"net/http"
)

// NewRouter configures API routes and middleware.
func NewRouter(logger *slog.Logger, statusHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/status", statusHandler)

	return Chain(
		mux,
		Recoverer(logger),
		RequestID(),
		AccessLog(logger),
	)
}
