package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleSessionStub_OK(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(nil))
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	rec := httptest.NewRecorder()

	h.HandleSessionStub(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body SessionStubResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Status != "stub" {
		t.Fatalf("expected status=stub, got %q", body.Status)
	}
	if body.DatabaseConfigured {
		t.Fatalf("expected database_configured=false")
	}
}

func TestHandleSessionStub_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(nil))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/session", nil)
	rec := httptest.NewRecorder()

	h.HandleSessionStub(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodGet {
		t.Fatalf("expected allow %q, got %q", http.MethodGet, allow)
	}
}

func TestHandleSteamLoginStub_OK(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(nil))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/steam/login", nil)
	rec := httptest.NewRecorder()

	h.HandleSteamLoginStub(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var body SteamLoginStubResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Provider != "steam" {
		t.Fatalf("expected provider=steam, got %q", body.Provider)
	}
	if !body.RequiresSteamOIDC {
		t.Fatalf("expected requires_steam_oidc=true")
	}
}

func TestHandleSteamLoginStub_MethodNotAllowed(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(nil))
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/steam/login", nil)
	rec := httptest.NewRecorder()

	h.HandleSteamLoginStub(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected allow %q, got %q", http.MethodPost, allow)
	}
}
