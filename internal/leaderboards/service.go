package leaderboards

import (
	"context"
	"fmt"
	"strings"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

// Service provides leaderboard query operations.
type Service struct {
	repo Repository
}

// NewService constructs a leaderboards service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ScenarioLeaderboard returns a scenario leaderboard page.
func (s *Service) ScenarioLeaderboard(ctx context.Context, req ScenarioRequest) (ScenarioLeaderboardResponse, error) {
	normalized, err := normalizeScenarioRequest(req)
	if err != nil {
		return ScenarioLeaderboardResponse{}, err
	}
	return s.repo.ScenarioLeaderboard(ctx, normalized)
}

// BenchmarkDifficultyLeaderboard returns a benchmark difficulty leaderboard page.
func (s *Service) BenchmarkDifficultyLeaderboard(ctx context.Context, req BenchmarkDifficultyRequest) (BenchmarkDifficultyLeaderboardResponse, error) {
	normalized, err := normalizeBenchmarkDifficultyRequest(req)
	if err != nil {
		return BenchmarkDifficultyLeaderboardResponse{}, err
	}
	return s.repo.BenchmarkDifficultyLeaderboard(ctx, normalized)
}

func normalizeScenarioRequest(req ScenarioRequest) (ScenarioRequest, error) {
	out := req
	out.ScenarioName = strings.TrimSpace(out.ScenarioName)
	out.Limit = normalizeLimit(out.Limit)
	out.Offset = normalizeOffset(out.Offset)

	if out.ScenarioID != nil && *out.ScenarioID <= 0 {
		return ScenarioRequest{}, fmt.Errorf("%w: scenario_id must be greater than zero", ErrInvalidQuery)
	}
	if out.ScenarioID == nil && out.ScenarioName == "" {
		return ScenarioRequest{}, fmt.Errorf("%w: scenario_id or scenario is required", ErrInvalidQuery)
	}

	return out, nil
}

func normalizeBenchmarkDifficultyRequest(req BenchmarkDifficultyRequest) (BenchmarkDifficultyRequest, error) {
	out := req
	out.Limit = normalizeLimit(out.Limit)
	out.Offset = normalizeOffset(out.Offset)

	if out.DifficultyID != nil && *out.DifficultyID <= 0 {
		return BenchmarkDifficultyRequest{}, fmt.Errorf("%w: difficulty_id must be greater than zero", ErrInvalidQuery)
	}
	if out.KovaaksBenchmarkID != nil && *out.KovaaksBenchmarkID <= 0 {
		return BenchmarkDifficultyRequest{}, fmt.Errorf("%w: kovaaks_benchmark_id must be greater than zero", ErrInvalidQuery)
	}
	if out.DifficultyID == nil && out.KovaaksBenchmarkID == nil {
		return BenchmarkDifficultyRequest{}, fmt.Errorf("%w: difficulty_id or kovaaks_benchmark_id is required", ErrInvalidQuery)
	}

	return out, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}
