package httpapi

import (
	"log/slog"
	"net/http"
)

// AuthRoutes bundles optional auth handlers.
type AuthRoutes struct {
	SessionStub http.Handler
	SteamLogin  http.Handler
}

// RunSyncRoutes bundles optional run sync handlers.
type RunSyncRoutes struct {
	Sync          http.Handler
	BulkSync      http.Handler
	MissingHashes http.Handler
	RawDownload   http.Handler
	RawURL        http.Handler
}

// NewRouter configures API routes and middleware.
func NewRouter(logger *slog.Logger, statusHandler http.Handler, authRoutes *AuthRoutes, runSyncRoutes *RunSyncRoutes) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/status", statusHandler)
	if authRoutes != nil {
		if authRoutes.SessionStub != nil {
			mux.Handle("GET /v1/auth/session", authRoutes.SessionStub)
		}
		if authRoutes.SteamLogin != nil {
			mux.Handle("POST /v1/auth/steam/login", authRoutes.SteamLogin)
		}
	}
	if runSyncRoutes != nil {
		if runSyncRoutes.Sync != nil {
			mux.Handle("POST /v1/runs/sync", runSyncRoutes.Sync)
		}
		if runSyncRoutes.BulkSync != nil {
			mux.Handle("POST /v1/runs/sync/bulk", runSyncRoutes.BulkSync)
		}
		if runSyncRoutes.MissingHashes != nil {
			mux.Handle("POST /v1/runs/sync/missing", runSyncRoutes.MissingHashes)
		}
		if runSyncRoutes.RawDownload != nil {
			mux.Handle("GET /v1/runs/raw/{hash}", runSyncRoutes.RawDownload)
		}
		if runSyncRoutes.RawURL != nil {
			mux.Handle("GET /v1/runs/raw/{hash}/url", runSyncRoutes.RawURL)
		}
	}

	return Chain(
		mux,
		Recoverer(logger),
		RequestID(),
		AccessLog(logger),
	)
}
