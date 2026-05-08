package stats

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerHandleGet_OK(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(stubRepo{
		response: Response{
			TotalRuns:      123,
			TotalPlayers:   45,
			TotalScenarios: 9,
		},
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("expected content-type application/json; charset=utf-8, got %q", got)
	}

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.TotalRuns != 123 {
		t.Fatalf("expected total_runs 123, got %d", body.TotalRuns)
	}
	if body.TotalPlayers != 45 {
		t.Fatalf("expected total_players 45, got %d", body.TotalPlayers)
	}
	if body.TotalScenarios != 9 {
		t.Fatalf("expected total_scenarios 9, got %d", body.TotalScenarios)
	}
}

func TestHandlerHandleGet_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(stubRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/v1/stats", nil)
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
}

func TestHandlerHandleGet_InternalError(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(stubRepo{err: errors.New("boom")}))

	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Fatalf("expected internal server error message, got %q", body["error"])
	}
}
