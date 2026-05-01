package players

import (
	"context"
	"fmt"
	"strings"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Service provides player query operations.
type Service struct {
	repo Repository
}

// NewService constructs a players service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListPlayers returns a paginated, searchable list of players.
func (s *Service) ListPlayers(ctx context.Context, req ListRequest) (ListResponse, error) {
	req = normalizeListRequest(req)

	items, err := s.repo.ListPlayers(ctx, ListRequest{
		Query:  req.Query,
		Sort:   req.Sort,
		Limit:  req.Limit + 1,
		Offset: req.Offset,
	})
	if err != nil {
		return ListResponse{}, fmt.Errorf("list players: %w", err)
	}

	hasMore := len(items) > req.Limit
	if hasMore {
		items = items[:req.Limit]
	}

	resp := ListResponse{
		Players: items,
		Limit:   req.Limit,
		Offset:  req.Offset,
		Count:   len(items),
		HasMore: hasMore,
	}
	if hasMore {
		next := req.Offset + len(items)
		resp.NextOffset = &next
	}

	return resp, nil
}

// GetPlayer returns the profile for one player by their Steam ID.
func (s *Service) GetPlayer(ctx context.Context, steamID string) (PlayerDetail, error) {
	steamID = strings.TrimSpace(steamID)
	if steamID == "" {
		return PlayerDetail{}, ErrNotFound
	}
	return s.repo.PlayerBySteamID(ctx, steamID)
}

func normalizeListRequest(req ListRequest) ListRequest {
	req.Query = strings.TrimSpace(req.Query)
	if req.Limit <= 0 {
		req.Limit = defaultLimit
	}
	if req.Limit > maxLimit {
		req.Limit = maxLimit
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	switch req.Sort {
	case PlayerSortRunCountDesc, PlayerSortRunCountAsc, PlayerSortNameAsc:
	default:
		req.Sort = PlayerSortRunCountDesc
	}
	return req
}
