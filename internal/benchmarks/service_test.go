package benchmarks

import (
	"context"
	"testing"
)

type testRepo struct {
	items []Benchmark
	err   error
	last  ListRequest
}

func (r *testRepo) ListBenchmarks(_ context.Context, req ListRequest) ([]Benchmark, error) {
	r.last = req
	if r.err != nil {
		return nil, r.err
	}
	return r.items, nil
}

func TestServiceListBenchmarks_TrimsQuery(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	svc := NewService(repo)

	resp, err := svc.ListBenchmarks(context.Background(), ListRequest{Query: "  vt  "})
	if err != nil {
		t.Fatalf("list benchmarks: %v", err)
	}

	if repo.last.Query != "vt" {
		t.Fatalf("expected trimmed query, got %q", repo.last.Query)
	}
	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
}
