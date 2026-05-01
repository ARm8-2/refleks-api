package players

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memRepo struct {
	items map[string]PlayerDetail
}

func newMemRepo() *memRepo {
	return &memRepo{items: make(map[string]PlayerDetail)}
}

func (m *memRepo) ListPlayers(_ context.Context, req ListRequest) ([]PlayerListItem, error) {
	out := make([]PlayerListItem, 0, len(m.items))
	for _, d := range m.items {
		if req.Query != "" && d.SteamUsername != req.Query && d.SteamID != req.Query {
			continue
		}
		out = append(out, PlayerListItem{
			SteamID:       d.SteamID,
			SteamUsername: d.SteamUsername,
			RunCount:      d.RunCount,
			LastRunAt:     d.LastRunAt,
		})
	}
	if req.Offset >= len(out) {
		return []PlayerListItem{}, nil
	}
	out = out[req.Offset:]
	if req.Limit > 0 && len(out) > req.Limit {
		out = out[:req.Limit]
	}
	return out, nil
}

func (m *memRepo) PlayerBySteamID(_ context.Context, steamID string) (PlayerDetail, error) {
	d, ok := m.items[steamID]
	if !ok {
		return PlayerDetail{}, ErrNotFound
	}
	return d, nil
}

func TestServiceListPlayers_ReturnsAll(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items["7656119801"] = PlayerDetail{SteamID: "7656119801", SteamUsername: "alice", RunCount: 10, CreatedAt: time.Now()}
	repo.items["7656119802"] = PlayerDetail{SteamID: "7656119802", SteamUsername: "bob", RunCount: 5, CreatedAt: time.Now()}

	svc := NewService(repo)
	resp, err := svc.ListPlayers(context.Background(), ListRequest{})
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

func TestServiceListPlayers_Pagination(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	for i := 1; i <= 5; i++ {
		id := string(rune('0'+i)) + "76561198"
		repo.items[id] = PlayerDetail{SteamID: id, RunCount: int64(i), CreatedAt: time.Now()}
	}

	svc := NewService(repo)
	resp, err := svc.ListPlayers(context.Background(), ListRequest{Limit: 3})
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

func TestServiceGetPlayer_Success(t *testing.T) {
	t.Parallel()

	repo := newMemRepo()
	repo.items["76561198012345678"] = PlayerDetail{
		SteamID:       "76561198012345678",
		SteamUsername: "alice",
		RunCount:      42,
		CreatedAt:     time.Now(),
	}

	svc := NewService(repo)
	player, err := svc.GetPlayer(context.Background(), "76561198012345678")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if player.SteamID != "76561198012345678" {
		t.Fatalf("expected steam id 76561198012345678, got %s", player.SteamID)
	}
	if player.RunCount != 42 {
		t.Fatalf("expected run count 42, got %d", player.RunCount)
	}
}

func TestServiceGetPlayer_NotFound(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo())
	_, err := svc.GetPlayer(context.Background(), "76561198099999999")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestServiceGetPlayer_EmptySteamID(t *testing.T) {
	t.Parallel()

	svc := NewService(newMemRepo())
	_, err := svc.GetPlayer(context.Background(), "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for empty steam_id, got %v", err)
	}
}
