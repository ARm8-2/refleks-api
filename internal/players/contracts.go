package players

import (
	"context"
	"errors"
)

var (
	// ErrNotFound indicates the requested player does not exist.
	ErrNotFound = errors.New("player not found")
)

// Repository reads player account data from storage.
type Repository interface {
	ListPlayers(ctx context.Context, req ListRequest) ([]PlayerListItem, error)
	PlayerBySteamID(ctx context.Context, steamID string) (PlayerDetail, error)
}
