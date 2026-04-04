package benchmarks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerHandleList_OK(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks?q=vt&include_inactive=true", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !repo.last.IncludeInactive {
		t.Fatalf("expected include_inactive=true")
	}
	if repo.last.Query != "vt" {
		t.Fatalf("expected trimmed query, got %q", repo.last.Query)
	}

	var body ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Count != 1 {
		t.Fatalf("expected count=1, got %d", body.Count)
	}
}

func TestHandlerHandleList_InvalidIncludeInactive(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&testRepo{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks?include_inactive=maybe", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandlerHandleList_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&testRepo{}))

	req := httptest.NewRequest(http.MethodPost, "/v1/benchmarks", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected allow %q, got %q", http.MethodGet, allow)
	}
}
