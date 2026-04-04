package leaderboards

import (
	"context"
	"errors"
)

var (
	// ErrInvalidQuery indicates query parameters are invalid.
	ErrInvalidQuery = errors.New("invalid query")
	// ErrNotFound indicates a requested leaderboard target does not exist.
	ErrNotFound = errors.New("leaderboard target not found")
)

// Repository reads current leaderboard projections from storage.
type Repository interface {
	ScenarioLeaderboard(ctx context.Context, req ScenarioRequest) (ScenarioLeaderboardResponse, error)
	BenchmarkDifficultyLeaderboard(ctx context.Context, req BenchmarkDifficultyRequest) (BenchmarkDifficultyLeaderboardResponse, error)
}
