package benchmarks

import (
	"context"
	"errors"
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
	if repo.last.View != ListViewFull {
		t.Fatalf("expected default view %q, got %q", ListViewFull, repo.last.View)
	}
	if resp.Count != 1 {
		t.Fatalf("expected count 1, got %d", resp.Count)
	}
}

func TestServiceListBenchmarks_PassesProgressView(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{BenchmarkName: "Voltaic"}}}
	svc := NewService(repo)

	_, err := svc.ListBenchmarks(context.Background(), ListRequest{View: ListViewProgress})
	if err != nil {
		t.Fatalf("list benchmarks: %v", err)
	}
	if repo.last.View != ListViewProgress {
		t.Fatalf("expected view %q, got %q", ListViewProgress, repo.last.View)
	}
}

func TestServiceListBenchmarks_InvalidView(t *testing.T) {
	t.Parallel()

	svc := NewService(&testRepo{})
	_, err := svc.ListBenchmarks(context.Background(), ListRequest{View: ListView("nope")})
	if !errors.Is(err, ErrInvalidView) {
		t.Fatalf("expected ErrInvalidView, got %v", err)
	}
}

func TestServiceListBenchmarks_NormalizesSliceFields(t *testing.T) {
	t.Parallel()

	repo := &testRepo{items: []Benchmark{{
		BenchmarkName: "Voltaic",
		Difficulties: []BenchmarkDifficulty{{
			DifficultyName:     "Intermediate",
			KovaaksBenchmarkID: 123,
			Sharecode:          "KOVAAKSXYZ",
		}},
	}}}
	svc := NewService(repo)

	resp, err := svc.ListBenchmarks(context.Background(), ListRequest{})
	if err != nil {
		t.Fatalf("list benchmarks: %v", err)
	}

	if len(resp.Benchmarks) != 1 || len(resp.Benchmarks[0].Difficulties) != 1 {
		t.Fatalf("expected one benchmark with one difficulty")
	}
	difficulty := resp.Benchmarks[0].Difficulties[0]
	if difficulty.Ranks == nil {
		t.Fatalf("expected ranks to be normalized to empty slice")
	}
	if difficulty.Categories == nil {
		t.Fatalf("expected categories to be normalized to empty slice")
	}
}
