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

// BenchmarkRoutes bundles optional benchmark handlers.
type BenchmarkRoutes struct {
	List http.Handler
}

// LeaderboardRoutes bundles optional leaderboard handlers.
type LeaderboardRoutes struct {
	Scenario            http.Handler
	BenchmarkDifficulty http.Handler
}

// ScenarioRoutes bundles optional scenario browsing handlers.
type ScenarioRoutes struct {
	List http.Handler
	Get  http.Handler
}

// PlayerRoutes bundles optional player browsing handlers.
type PlayerRoutes struct {
	List http.Handler
	Get  http.Handler
}

// RunRoutes bundles optional run handlers.
type RunRoutes struct {
	RunsList      http.Handler
	GetRun        http.Handler
	Sync          http.Handler
	BulkSync      http.Handler
	MissingHashes http.Handler
	RawDownload   http.Handler
	RawURL        http.Handler
}

// NewRouter configures API routes and middleware.
func NewRouter(logger *slog.Logger, statusHandler http.Handler, statsHandler http.Handler, authRoutes *AuthRoutes, benchmarkRoutes *BenchmarkRoutes, leaderboardRoutes *LeaderboardRoutes, scenarioRoutes *ScenarioRoutes, playerRoutes *PlayerRoutes, runRoutes *RunRoutes) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/status", statusHandler)
	if statsHandler != nil {
		mux.Handle("GET /v1/stats", statsHandler)
	}
	if authRoutes != nil {
		if authRoutes.SessionStub != nil {
			mux.Handle("GET /v1/auth/session", authRoutes.SessionStub)
		}
		if authRoutes.SteamLogin != nil {
			mux.Handle("POST /v1/auth/steam/login", authRoutes.SteamLogin)
		}
	}
	if benchmarkRoutes != nil {
		if benchmarkRoutes.List != nil {
			mux.Handle("GET /v1/benchmarks", benchmarkRoutes.List)
		}
	}
	if leaderboardRoutes != nil {
		if leaderboardRoutes.Scenario != nil {
			mux.Handle("GET /v1/leaderboards/scenario", leaderboardRoutes.Scenario)
		}
		if leaderboardRoutes.BenchmarkDifficulty != nil {
			mux.Handle("GET /v1/leaderboards/benchmark-difficulty", leaderboardRoutes.BenchmarkDifficulty)
		}
	}
	if scenarioRoutes != nil {
		if scenarioRoutes.List != nil {
			mux.Handle("GET /v1/scenarios", scenarioRoutes.List)
		}
		if scenarioRoutes.Get != nil {
			mux.Handle("GET /v1/scenarios/{id}", scenarioRoutes.Get)
		}
	}
	if playerRoutes != nil {
		if playerRoutes.List != nil {
			mux.Handle("GET /v1/players", playerRoutes.List)
		}
		if playerRoutes.Get != nil {
			mux.Handle("GET /v1/players/{steam_id}", playerRoutes.Get)
		}
	}
	if runRoutes != nil {
		if runRoutes.RunsList != nil {
			mux.Handle("GET /v1/runs", runRoutes.RunsList)
		}
		if runRoutes.GetRun != nil {
			mux.Handle("GET /v1/runs/{hash}", runRoutes.GetRun)
		}
		if runRoutes.Sync != nil {
			mux.Handle("POST /v1/runs/sync", runRoutes.Sync)
		}
		if runRoutes.BulkSync != nil {
			mux.Handle("POST /v1/runs/sync/bulk", runRoutes.BulkSync)
		}
		if runRoutes.MissingHashes != nil {
			mux.Handle("POST /v1/runs/sync/missing", runRoutes.MissingHashes)
		}
		if runRoutes.RawDownload != nil {
			mux.Handle("GET /v1/runs/raw/{hash}", runRoutes.RawDownload)
		}
		if runRoutes.RawURL != nil {
			mux.Handle("GET /v1/runs/raw/{hash}/url", runRoutes.RawURL)
		}
	}

	return Chain(
		mux,
		Recoverer(logger),
		RequestID(),
		AccessLog(logger),
	)
}
