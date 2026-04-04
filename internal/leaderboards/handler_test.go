package leaderboards

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerHandleScenario_OK(t *testing.T) {
	t.Parallel()

	repo := &testRepo{scenarioResp: ScenarioLeaderboardResponse{ScenarioID: 5, ScenarioName: "VT"}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboards/scenario?scenario_id=5&limit=20&offset=10", nil)
	rec := httptest.NewRecorder()

	h.HandleScenario(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if repo.lastScenario.ScenarioID == nil || *repo.lastScenario.ScenarioID != 5 {
		t.Fatalf("expected scenario id 5, got %#v", repo.lastScenario.ScenarioID)
	}
}

func TestHandlerHandleScenario_InvalidScenarioID(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&testRepo{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboards/scenario?scenario_id=nope", nil)
	rec := httptest.NewRecorder()

	h.HandleScenario(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandlerHandleScenario_NotFound(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&testRepo{err: ErrNotFound}))

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboards/scenario?scenario=missing", nil)
	rec := httptest.NewRecorder()

	h.HandleScenario(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestHandlerHandleBenchmarkDifficulty_InternalError(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&testRepo{err: errors.New("boom")}))

	req := httptest.NewRequest(http.MethodGet, "/v1/leaderboards/benchmark-difficulty?difficulty_id=1", nil)
	rec := httptest.NewRecorder()

	h.HandleBenchmarkDifficulty(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}
