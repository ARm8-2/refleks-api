package scenarios

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandlerHandleList_OK(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items[1] = ScenarioDetail{ID: 1, ScenarioName: "VT Pat", RunCount: 10, UpdatedAt: time.Now()}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
}

func TestHandlerHandleList_InvalidLimit(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(newMemRepo()))

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios?limit=nope", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerHandleGet_NotFound(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(newMemRepo()))

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios/99", nil)
	req.SetPathValue("id", "99")
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerHandleGet_InvalidID(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(newMemRepo()))

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios/abc", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerHandleGet_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items[3] = ScenarioDetail{ID: 3, ScenarioName: "Air Angelic 4", RunCount: 5, UpdatedAt: time.Now()}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios/3", nil)
	req.SetPathValue("id", "3")
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var detail ScenarioDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.ID != 3 {
		t.Fatalf("expected ID 3, got %d", detail.ID)
	}
}

// Ensure the Handler's memRepo satisfies the Repository interface at compile time.
var _ Repository = (*memRepo)(nil)

// Ensure the Handler satisfies the expected method set at compile time.
var _ interface {
	HandleList(http.ResponseWriter, *http.Request)
	HandleGet(http.ResponseWriter, *http.Request)
} = (*Handler)(nil)

type contextKey struct{}

var _ context.Context = context.Background()
