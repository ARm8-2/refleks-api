package stats

import (
	"context"
	"fmt"
)

// Service provides aggregate stats queries.
type Service struct {
	repo Repository
}

// NewService constructs a stats service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetCounts returns overall database totals.
func (s *Service) GetCounts(ctx context.Context) (Response, error) {
	counts, err := s.repo.Counts(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("get stats counts: %w", err)
	}
	return counts, nil
}
