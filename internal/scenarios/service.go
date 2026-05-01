package scenarios

import (
	"context"
	"fmt"
	"strings"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Service provides scenario query operations.
type Service struct {
	repo Repository
}

// NewService constructs a scenarios service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListScenarios returns a paginated, filtered list of scenarios.
func (s *Service) ListScenarios(ctx context.Context, req ListRequest) (ListResponse, error) {
	req = normalizeListRequest(req)

	items, err := s.repo.ListScenarios(ctx, ListRequest{
		Query:  req.Query,
		Sort:   req.Sort,
		Limit:  req.Limit + 1,
		Offset: req.Offset,
	})
	if err != nil {
		return ListResponse{}, fmt.Errorf("list scenarios: %w", err)
	}

	hasMore := len(items) > req.Limit
	if hasMore {
		items = items[:req.Limit]
	}

	resp := ListResponse{
		Scenarios: items,
		Limit:     req.Limit,
		Offset:    req.Offset,
		Count:     len(items),
		HasMore:   hasMore,
	}
	if hasMore {
		next := req.Offset + len(items)
		resp.NextOffset = &next
	}

	return resp, nil
}

// GetScenario returns the full detail for one scenario by its database ID.
func (s *Service) GetScenario(ctx context.Context, id int64) (ScenarioDetail, error) {
	if id <= 0 {
		return ScenarioDetail{}, ErrNotFound
	}
	return s.repo.ScenarioByID(ctx, id)
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
	case ScenarioSortRunCountDesc, ScenarioSortRunCountAsc,
		ScenarioSortNameAsc, ScenarioSortNameDesc, ScenarioSortUpdatedDesc:
	default:
		req.Sort = ScenarioSortRunCountDesc
	}
	return req
}
