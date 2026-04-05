package benchmarks

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidView = errors.New("invalid benchmark view")

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
	if req.View == "" {
		req.View = ListViewFull
	}
	switch req.View {
	case ListViewFull, ListViewProgress:
	default:
		return ListResponse{}, fmt.Errorf("%w: view must be one of full|progress", ErrInvalidView)
	}

	items, err := s.repo.ListBenchmarks(ctx, req)
	if err != nil {
		return ListResponse{}, err
	}
	normalizeBenchmarks(items)

	return ListResponse{
		Benchmarks: items,
		Count:      len(items),
	}, nil
}

func normalizeBenchmarks(items []Benchmark) {
	for benchmarkIndex := range items {
		benchmark := &items[benchmarkIndex]
		if benchmark.Difficulties == nil {
			benchmark.Difficulties = []BenchmarkDifficulty{}
		}

		for difficultyIndex := range benchmark.Difficulties {
			difficulty := &benchmark.Difficulties[difficultyIndex]
			if difficulty.Ranks == nil {
				difficulty.Ranks = []BenchmarkRank{}
			}
			if difficulty.Categories == nil {
				difficulty.Categories = []BenchmarkCategory{}
			}

			for categoryIndex := range difficulty.Categories {
				category := &difficulty.Categories[categoryIndex]
				if category.Subcategories == nil {
					category.Subcategories = []BenchmarkSubcategory{}
				}
			}
		}
	}
}
