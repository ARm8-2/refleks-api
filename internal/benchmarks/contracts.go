package benchmarks

import "context"

// Repository fetches benchmark definitions from storage.
type Repository interface {
	ListBenchmarks(ctx context.Context, req ListRequest) ([]Benchmark, error)
}
