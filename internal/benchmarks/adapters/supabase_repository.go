package adapters

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/benchmarks"
)

// SupabaseRepository reads benchmark metadata from Supabase Postgres.
type SupabaseRepository struct {
	pool *pgxpool.Pool
}

// NewSupabaseRepository constructs a Supabase-backed benchmark repository.
func NewSupabaseRepository(pool *pgxpool.Pool) (*SupabaseRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("supabase pool is required")
	}
	return &SupabaseRepository{pool: pool}, nil
}

// ListBenchmarks returns benchmark metadata in client-friendly hierarchy form.
func (r *SupabaseRepository) ListBenchmarks(ctx context.Context, req benchmarks.ListRequest) ([]benchmarks.Benchmark, error) {
	benchmarkItems, benchmarkIDs, err := r.listBenchmarkRows(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(benchmarkItems) == 0 {
		return []benchmarks.Benchmark{}, nil
	}

	benchmarkByID := make(map[int64]*benchmarks.Benchmark, len(benchmarkItems))
	for i := range benchmarkItems {
		benchmarkByID[benchmarkIDs[i]] = &benchmarkItems[i]
	}

	difficultyByID, err := r.attachDifficulties(ctx, benchmarkByID, benchmarkIDs)
	if err != nil {
		return nil, err
	}
	if len(difficultyByID) == 0 {
		return benchmarkItems, nil
	}

	categoryByID, err := r.attachCategories(ctx, difficultyByID)
	if err != nil {
		return nil, err
	}
	if len(categoryByID) == 0 {
		return benchmarkItems, nil
	}

	if err := r.attachSubcategories(ctx, categoryByID); err != nil {
		return nil, err
	}

	return benchmarkItems, nil
}

func (r *SupabaseRepository) listBenchmarkRows(ctx context.Context, req benchmarks.ListRequest) ([]benchmarks.Benchmark, []int64, error) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 3)

	if !req.IncludeInactive {
		clauses = append(clauses, "b.is_active = TRUE")
	}
	if req.Query != "" {
		args = append(args, "%"+req.Query+"%")
		p := len(args)
		clauses = append(clauses, fmt.Sprintf("(b.benchmark_name ILIKE $%d OR b.abbreviation ILIKE $%d)", p, p))
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	query := `
		SELECT
			b.id,
			b.benchmark_name,
			b.rank_calculation,
			b.abbreviation,
			b.color,
			b.spreadsheet_url,
			b.date_added
		FROM benchmarks b
	` + where + `
		ORDER BY b.benchmark_name ASC, b.id ASC
	`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]benchmarks.Benchmark, 0, 16)
	ids := make([]int64, 0, 16)

	for rows.Next() {
		var id int64
		var item benchmarks.Benchmark
		var dateAdded sql.NullTime

		if err := rows.Scan(
			&id,
			&item.BenchmarkName,
			&item.RankCalculation,
			&item.Abbreviation,
			&item.Color,
			&item.SpreadsheetURL,
			&dateAdded,
		); err != nil {
			return nil, nil, err
		}

		if dateAdded.Valid {
			formatted := dateAdded.Time.UTC().Format("2006-01-02")
			item.DateAdded = &formatted
		}
		item.Difficulties = []benchmarks.BenchmarkDifficulty{}

		items = append(items, item)
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return items, ids, nil
}

func (r *SupabaseRepository) attachDifficulties(ctx context.Context, benchmarkByID map[int64]*benchmarks.Benchmark, benchmarkIDs []int64) (map[int64]*benchmarks.BenchmarkDifficulty, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			d.id,
			d.benchmark_id,
			d.difficulty_name,
			d.kovaaks_benchmark_id,
			d.sharecode,
			d.rank_colors
		FROM benchmark_difficulties d
		WHERE d.benchmark_id = ANY($1)
		ORDER BY d.benchmark_id ASC, d.sort_order ASC, d.id ASC
	`, benchmarkIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]*benchmarks.BenchmarkDifficulty)
	for rows.Next() {
		var id int64
		var benchmarkID int64
		var diff benchmarks.BenchmarkDifficulty
		var rankColorsRaw []byte

		if err := rows.Scan(
			&id,
			&benchmarkID,
			&diff.DifficultyName,
			&diff.KovaaksBenchmarkID,
			&diff.Sharecode,
			&rankColorsRaw,
		); err != nil {
			return nil, err
		}

		rankColors := make(map[string]string)
		if len(rankColorsRaw) > 0 {
			if err := json.Unmarshal(rankColorsRaw, &rankColors); err != nil {
				return nil, err
			}
		}
		diff.RankColors = rankColors
		diff.Categories = []benchmarks.BenchmarkCategory{}

		parent, ok := benchmarkByID[benchmarkID]
		if !ok {
			continue
		}
		parent.Difficulties = append(parent.Difficulties, diff)
		out[id] = &parent.Difficulties[len(parent.Difficulties)-1]
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *SupabaseRepository) attachCategories(ctx context.Context, difficultyByID map[int64]*benchmarks.BenchmarkDifficulty) (map[int64]*benchmarks.BenchmarkCategory, error) {
	difficultyIDs := keysInt64(difficultyByID)

	rows, err := r.pool.Query(ctx, `
		SELECT
			c.id,
			c.difficulty_id,
			c.category_name,
			c.color
		FROM benchmark_categories c
		WHERE c.difficulty_id = ANY($1)
		ORDER BY c.difficulty_id ASC, c.sort_order ASC, c.id ASC
	`, difficultyIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]*benchmarks.BenchmarkCategory)
	for rows.Next() {
		var id int64
		var difficultyID int64
		var category benchmarks.BenchmarkCategory

		if err := rows.Scan(&id, &difficultyID, &category.CategoryName, &category.Color); err != nil {
			return nil, err
		}
		category.Subcategories = []benchmarks.BenchmarkSubcategory{}

		parent, ok := difficultyByID[difficultyID]
		if !ok {
			continue
		}
		parent.Categories = append(parent.Categories, category)
		out[id] = &parent.Categories[len(parent.Categories)-1]
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *SupabaseRepository) attachSubcategories(ctx context.Context, categoryByID map[int64]*benchmarks.BenchmarkCategory) error {
	categoryIDs := keysInt64(categoryByID)

	rows, err := r.pool.Query(ctx, `
		SELECT
			s.category_id,
			s.subcategory_name,
			s.scenario_count,
			s.color
		FROM benchmark_subcategories s
		WHERE s.category_id = ANY($1)
		ORDER BY s.category_id ASC, s.sort_order ASC, s.id ASC
	`, categoryIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var categoryID int64
		var sub benchmarks.BenchmarkSubcategory

		if err := rows.Scan(&categoryID, &sub.SubcategoryName, &sub.ScenarioCount, &sub.Color); err != nil {
			return err
		}

		parent, ok := categoryByID[categoryID]
		if !ok {
			continue
		}
		parent.Subcategories = append(parent.Subcategories, sub)
	}

	return rows.Err()
}

func keysInt64[T any](in map[int64]T) []int64 {
	out := make([]int64, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	return out
}
