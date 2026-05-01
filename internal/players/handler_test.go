package players

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
	repo.items["76561198012345678"] = PlayerDetail{
		SteamID:       "76561198012345678",
		SteamUsername: "alice",
		RunCount:      5,
		CreatedAt:     time.Now(),
	}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/players", nil)
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

	req := httptest.NewRequest(http.MethodGet, "/v1/players?limit=bad", nil)
	rec := httptest.NewRecorder()

	h.HandleList(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlerHandleGet_NotFound(t *testing.T) {
	t.Parallel()

	h := NewHandler(NewService(newMemRepo()))

	req := httptest.NewRequest(http.MethodGet, "/v1/players/76561198099999999", nil)
	req.SetPathValue("steam_id", "76561198099999999")
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandlerHandleGet_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items["76561198012345678"] = PlayerDetail{
		SteamID:       "76561198012345678",
		SteamUsername: "alice",
		RunCount:      7,
		CreatedAt:     time.Now(),
	}
	h := NewHandler(NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/v1/players/76561198012345678", nil)
	req.SetPathValue("steam_id", "76561198012345678")
	rec := httptest.NewRecorder()

	h.HandleGet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var player PlayerDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &player); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if player.SteamID != "76561198012345678" {
		t.Fatalf("unexpected steam_id: %s", player.SteamID)
	}
}

// Compile-time interface checks.
var _ Repository = (*memRepo)(nil)
var _ interface {
	HandleList(http.ResponseWriter, *http.Request)
	HandleGet(http.ResponseWriter, *http.Request)
} = (*Handler)(nil)

var _ context.Context = context.Background()
