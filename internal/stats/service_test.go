package stats

import (
	"context"
	"errors"
	"testing"
)

func TestServiceGetCounts_OK(t *testing.T) {
	t.Parallel()

	svc := NewService(stubRepo{
		response: Response{
			TotalRuns:      42,
			TotalPlayers:   7,
			TotalScenarios: 12,
		},
	})

	resp, err := svc.GetCounts(context.Background())
	if err != nil {
		t.Fatalf("GetCounts returned error: %v", err)
	}
	if resp.TotalRuns != 42 {
		t.Fatalf("expected total_runs 42, got %d", resp.TotalRuns)
	}
	if resp.TotalPlayers != 7 {
		t.Fatalf("expected total_players 7, got %d", resp.TotalPlayers)
	}
	if resp.TotalScenarios != 12 {
		t.Fatalf("expected total_scenarios 12, got %d", resp.TotalScenarios)
	}
}

func TestServiceGetCounts_Error(t *testing.T) {
	t.Parallel()

	svc := NewService(stubRepo{err: errors.New("boom")})

	_, err := svc.GetCounts(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

type stubRepo struct {
	response Response
	err      error
}

func (s stubRepo) Counts(_ context.Context) (Response, error) {
	if s.err != nil {
		return Response{}, s.err
	}
	return s.response, nil
}
