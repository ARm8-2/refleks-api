package benchmarks

import (
	"context"
	"strings"
)

// Service provides benchmark query operations.
type Service struct {
	repo Repository
}

// NewService constructs a benchmarks service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// ListBenchmarks returns benchmark definitions for client consumption.
func (s *Service) ListBenchmarks(ctx context.Context, req ListRequest) (ListResponse, error) {
	req.Query = strings.TrimSpace(req.Query)

	items, err := s.repo.ListBenchmarks(ctx, req)
	if err != nil {
		return ListResponse{}, err
	}

	return ListResponse{
		Benchmarks: items,
		Count:      len(items),
	}, nil
}
