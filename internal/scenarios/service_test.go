package scenarios

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memRepo struct {
	items map[int64]ScenarioDetail
}

func newMemRepo() *memRepo {
	return &memRepo{items: make(map[int64]ScenarioDetail)}
}

func (m *memRepo) ListScenarios(_ context.Context, req ListRequest) ([]ScenarioListItem, error) {
	out := make([]ScenarioListItem, 0, len(m.items))
	for _, d := range m.items {
		if req.Query != "" && d.ScenarioName != req.Query {
			continue
		}
		out = append(out, ScenarioListItem{
			ID:               d.ID,
			ScenarioName:     d.ScenarioName,
			RunCount:         d.RunCount,
			ScoreSampleCount: d.ScoreSampleCount,
			UpdatedAt:        d.UpdatedAt,
		})
	}
	if req.Offset >= len(out) {
		return []ScenarioListItem{}, nil
	}
	out = out[req.Offset:]
	if req.Limit > 0 && len(out) > req.Limit {
		out = out[:req.Limit]
	}
	return out, nil
}

func (m *memRepo) ScenarioByID(_ context.Context, id int64) (ScenarioDetail, error) {
	d, ok := m.items[id]
	if !ok {
		return ScenarioDetail{}, ErrNotFound
	}
	return d, nil
}

func TestServiceListScenarios_ReturnsAll(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items[1] = ScenarioDetail{ID: 1, ScenarioName: "VT Pat", RunCount: 100, UpdatedAt: time.Now()}
	repo.items[2] = ScenarioDetail{ID: 2, ScenarioName: "Air Angelic 4", RunCount: 50, UpdatedAt: time.Now()}

	svc := NewService(repo)
	resp, err := svc.ListScenarios(context.Background(), ListRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 2 {
		t.Fatalf("expected count 2, got %d", resp.Count)
	}
	if resp.HasMore {
		t.Fatalf("expected no more pages")
	}
}

func TestServiceListScenarios_Pagination(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	for i := int64(1); i <= 5; i++ {
		repo.items[i] = ScenarioDetail{ID: i, ScenarioName: "Scenario", RunCount: i, UpdatedAt: time.Now()}
	}

	svc := NewService(repo)
	resp, err := svc.ListScenarios(context.Background(), ListRequest{Limit: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Count != 3 {
		t.Fatalf("expected count 3, got %d", resp.Count)
	}
	if !resp.HasMore {
		t.Fatalf("expected has_more=true")
	}
	if resp.NextOffset == nil || *resp.NextOffset != 3 {
		t.Fatalf("expected next_offset 3, got %v", resp.NextOffset)
	}
}

func TestServiceGetScenario_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items[7] = ScenarioDetail{ID: 7, ScenarioName: "VT Pat", RunCount: 42, UpdatedAt: time.Now()}

	svc := NewService(repo)
	detail, err := svc.GetScenario(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.ID != 7 {
		t.Fatalf("expected ID 7, got %d", detail.ID)
	}
	if detail.ScenarioName != "VT Pat" {
		t.Fatalf("expected VT Pat, got %s", detail.ScenarioName)
	}
}

func TestServiceGetScenario_NotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo())
	_, err := svc.GetScenario(context.Background(), 999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestServiceGetScenario_InvalidID(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo())
	_, err := svc.GetScenario(context.Background(), 0)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for id=0, got %v", err)
	}
}
