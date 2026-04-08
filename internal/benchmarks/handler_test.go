package benchmarks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerHandleList_OK(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks?q=vt", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if cacheControl := rec.Header().Get("Cache-Control"); cacheControl != benchmarkListCacheControl {
		t.Fatalf("expected cache-control %q, got %q", benchmarkListCacheControl, cacheControl)
	}
	if etag := rec.Header().Get("ETag"); etag == "" {
		t.Fatalf("expected etag header to be set")
	}
	if repo.last.Query != "vt" {
		t.Fatalf("expected trimmed query, got %q", repo.last.Query)
	}
	if repo.last.View != ListViewFull {
		t.Fatalf("expected default view %q, got %q", ListViewFull, repo.last.View)
	}

	var body ListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Count != 1 {
		t.Fatalf("expected count=1, got %d", body.Count)
	}
}

func TestHandlerHandleList_ConditionalRequestReturnsNotModified(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	h := NewHandler(NewService(repo))

	firstReq := httptest.NewRequest(http.MethodGet, "/v1/benchmarks", nil)
	firstRec := httptest.NewRecorder()

	h.HandleList(firstRec, firstReq)

	etag := firstRec.Header().Get("ETag")
	if etag == "" {
		t.Fatalf("expected etag on initial response")
	}

	conditionalReq := httptest.NewRequest(http.MethodGet, "/v1/benchmarks", nil)
	conditionalReq.Header.Set("If-None-Match", etag)
	conditionalRec := httptest.NewRecorder()

	h.HandleList(conditionalRec, conditionalReq)

	if conditionalRec.Code != http.StatusNotModified {
		t.Fatalf("expected status 304, got %d", conditionalRec.Code)
	}
	if body := conditionalRec.Body.String(); body != "" {
		t.Fatalf("expected empty body for 304 response, got %q", body)
	}
	if got := conditionalRec.Header().Get("ETag"); got != etag {
		t.Fatalf("expected etag %q on 304 response, got %q", etag, got)
	}
}

func TestHandlerHandleList_ConditionalRequestIgnoresMismatchedETag(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks", nil)
	req.Header.Set("If-None-Match", `"different"`)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if etag := rec.Header().Get("ETag"); etag == `"different"` {
		t.Fatalf("expected a fresh etag, got %q", etag)
	}
}

func TestHandlerHandleList_DisablesHTMLEscaping(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Dark & Rafal SpeedTS"}}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "\\u0026") {
		t.Fatalf("expected literal ampersand in response body, got %q", body)
	}
}

func TestHandlerHandleList_AlwaysEmitsRanksArray(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{
		BenchmarkName: "Voltaic",
		Difficulties: []BenchmarkDifficulty{{
			DifficultyName:     "Intermediate",
			KovaaksBenchmarkID: 123,
			Sharecode:          "KOVAAKSXYZ",
		}},
	}}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "\"ranks\":[]") {
		t.Fatalf("expected ranks to be serialized as empty array, got %q", body)
	}
}

func TestHandlerHandleList_ProgressView(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks?view=progress", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if repo.last.View != ListViewProgress {
		t.Fatalf("expected view %q, got %q", ListViewProgress, repo.last.View)
	}
}

func TestHandlerHandleList_InvalidView(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(&testRepo{}))

	req := httptest.NewRequest(http.MethodGet, "/v1/benchmarks?view=invalid", nil)
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
