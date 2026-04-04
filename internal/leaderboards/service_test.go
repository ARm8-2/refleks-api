package leaderboards

import (
	"context"
	"errors"
	"testing"
)

type testRepo struct {
	scenarioResp   ScenarioLeaderboardResponse
	benchmarkResp  BenchmarkDifficultyLeaderboardResponse
	err            error
	lastScenario   ScenarioRequest
	lastDifficulty BenchmarkDifficultyRequest
}

func (r *testRepo) ScenarioLeaderboard(_ context.Context, req ScenarioRequest) (ScenarioLeaderboardResponse, error) {
	r.lastScenario = req
	if r.err != nil {
		return ScenarioLeaderboardResponse{}, r.err
	}
	return r.scenarioResp, nil
}

func (r *testRepo) BenchmarkDifficultyLeaderboard(_ context.Context, req BenchmarkDifficultyRequest) (BenchmarkDifficultyLeaderboardResponse, error) {
	r.lastDifficulty = req
	if r.err != nil {
		return BenchmarkDifficultyLeaderboardResponse{}, r.err
	}
	return r.benchmarkResp, nil
}

func TestServiceScenarioLeaderboard_NormalizesRequest(t *testing.T) {
	t.Parallel()

	repo := &testRepo{scenarioResp: ScenarioLeaderboardResponse{ScenarioID: 1}}
	svc := NewService(repo)

	resp, err := svc.ScenarioLeaderboard(context.Background(), ScenarioRequest{
		ScenarioName: "  VT  ",
		Limit:        999,
		Offset:       -4,
	})
	if err != nil {
		t.Fatalf("scenario leaderboard: %v", err)
	}

	if resp.ScenarioID != 1 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if repo.lastScenario.ScenarioName != "VT" {
		t.Fatalf("expected trimmed scenario, got %q", repo.lastScenario.ScenarioName)
	}
	if repo.lastScenario.Limit != maxLimit {
		t.Fatalf("expected normalized limit %d, got %d", maxLimit, repo.lastScenario.Limit)
	}
	if repo.lastScenario.Offset != 0 {
		t.Fatalf("expected normalized offset 0, got %d", repo.lastScenario.Offset)
	}
}

func TestServiceScenarioLeaderboard_RequiresSelector(t *testing.T) {
	t.Parallel()

	svc := NewService(&testRepo{})
	_, err := svc.ScenarioLeaderboard(context.Background(), ScenarioRequest{})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("expected ErrInvalidQuery, got %v", err)
	}
}

func TestServiceBenchmarkDifficultyLeaderboard_RequiresSelector(t *testing.T) {
	t.Parallel()

	svc := NewService(&testRepo{})
	_, err := svc.BenchmarkDifficultyLeaderboard(context.Background(), BenchmarkDifficultyRequest{})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("expected ErrInvalidQuery, got %v", err)
	}
}
