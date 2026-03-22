package status

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandler_ServeHTTP_StatusOK(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)
	now := startedAt.Add(42 * time.Second)

	service := NewService("refleks-api", "test", "v0.0.1", startedAt)
	handler := NewHandler(service)
	handler.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("expected content-type application/json; charset=utf-8, got %q", got)
	}

	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Status != "ok" {
		t.Fatalf("expected status field ok, got %q", body.Status)
	}
	if body.Service != "refleks-api" {
		t.Fatalf("expected service refleks-api, got %q", body.Service)
	}
	if body.Environment != "test" {
		t.Fatalf("expected env test, got %q", body.Environment)
	}
	if body.Version != "v0.0.1" {
		t.Fatalf("expected version v0.0.1, got %q", body.Version)
	}
	if body.Timestamp != now.Format(time.RFC3339Nano) {
		t.Fatalf("expected timestamp %q, got %q", now.Format(time.RFC3339Nano), body.Timestamp)
	}
	if body.UptimeSeconds != 42 {
		t.Fatalf("expected uptime 42, got %d", body.UptimeSeconds)
	}
}

func TestHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	service := NewService("refleks-api", "test", "v0.0.1", time.Now())
	handler := NewHandler(service)

	req := httptest.NewRequest(http.MethodPost, "/v1/status", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != http.MethodGet {
		t.Fatalf("expected Allow header %q, got %q", http.MethodGet, got)
	}
}
