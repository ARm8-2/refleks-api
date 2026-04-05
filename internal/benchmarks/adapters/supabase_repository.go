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

type categoryLookupKey struct {
	difficultyID int64
	name         string
}

type subcategoryLookupKey struct {
	difficultyID int64
	categoryName string
	name         string
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

	if err := r.attachRanks(ctx, difficultyByID, req.View == benchmarks.ListViewFull); err != nil {
		return nil, err
	}

	categoryByID, err := r.attachCategories(ctx, difficultyByID)
	if err != nil {
		return nil, err
	}
	if len(categoryByID) > 0 {
		if err := r.attachSubcategories(ctx, categoryByID); err != nil {
			return nil, err
		}
	}

	if req.View != benchmarks.ListViewProgress {
		if err := r.attachScenarioLinks(ctx, difficultyByID); err != nil {
			return nil, err
		}
	}

	return benchmarkItems, nil
}

func (r *SupabaseRepository) listBenchmarkRows(ctx context.Context, req benchmarks.ListRequest) ([]benchmarks.Benchmark, []int64, error) {
	clauses := make([]string, 0, 1)
	args := make([]any, 0, 3)

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
			d.sharecode
		FROM benchmark_difficulties d
		WHERE d.benchmark_id = ANY($1)
		ORDER BY d.benchmark_id ASC, d.sort_order ASC, d.id ASC
	`, benchmarkIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type difficultyRow struct {
		id               int64
		benchmarkID      int64
		difficultyName   string
		kovaaksBenchmark int64
		sharecode        string
	}

	rowsData := make([]difficultyRow, 0, 64)
	countsByBenchmark := make(map[int64]int)
	for rows.Next() {
		var row difficultyRow
		if err := rows.Scan(
			&row.id,
			&row.benchmarkID,
			&row.difficultyName,
			&row.kovaaksBenchmark,
			&row.sharecode,
		); err != nil {
			return nil, err
		}
		rowsData = append(rowsData, row)
		countsByBenchmark[row.benchmarkID]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for benchmarkID, count := range countsByBenchmark {
		parent, ok := benchmarkByID[benchmarkID]
		if !ok {
			continue
		}
		parent.Difficulties = make([]benchmarks.BenchmarkDifficulty, 0, count)
	}

	out := make(map[int64]*benchmarks.BenchmarkDifficulty)
	for _, row := range rowsData {
		var diff benchmarks.BenchmarkDifficulty
		diff.DifficultyName = row.difficultyName
		diff.KovaaksBenchmarkID = row.kovaaksBenchmark
		diff.Sharecode = row.sharecode
		diff.RankColors = nil
		diff.Ranks = nil
		diff.Scenarios = []benchmarks.BenchmarkScenario{}
		diff.Categories = []benchmarks.BenchmarkCategory{}

		parent, ok := benchmarkByID[row.benchmarkID]
		if !ok {
			continue
		}
		parent.Difficulties = append(parent.Difficulties, diff)
		out[row.id] = &parent.Difficulties[len(parent.Difficulties)-1]
	}

	return out, nil
}

func (r *SupabaseRepository) attachRanks(ctx context.Context, difficultyByID map[int64]*benchmarks.BenchmarkDifficulty, includeOrdered bool) error {
	difficultyIDs := keysInt64(difficultyByID)
	if len(difficultyIDs) == 0 {
		return nil
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			difficulty_id,
			rank_name,
			rank_color
		FROM benchmark_difficulty_ranks
		WHERE difficulty_id = ANY($1)
		ORDER BY difficulty_id ASC, sort_order ASC
	`, difficultyIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var difficultyID int64
		var rankName string
		var rankColor string

		if err := rows.Scan(&difficultyID, &rankName, &rankColor); err != nil {
			return err
		}

		difficulty, ok := difficultyByID[difficultyID]
		if !ok {
			continue
		}

		rankName = strings.TrimSpace(rankName)
		if rankName == "" || strings.EqualFold(rankName, "no rank") {
			continue
		}

		rankColor = strings.TrimSpace(rankColor)
		if rankColor == "" {
			rankColor = "#60a5fa"
		}

		if includeOrdered {
			difficulty.Ranks = append(difficulty.Ranks, benchmarks.BenchmarkRank{
				Name:  rankName,
				Color: rankColor,
			})
			continue
		}

		if difficulty.RankColors == nil {
			difficulty.RankColors = map[string]string{}
		}
		difficulty.RankColors[rankName] = rankColor
	}

	return rows.Err()
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

	type categoryRow struct {
		id           int64
		difficultyID int64
		name         string
		color        string
	}

	rowsData := make([]categoryRow, 0, 128)
	countsByDifficulty := make(map[int64]int)
	for rows.Next() {
		var row categoryRow
		if err := rows.Scan(&row.id, &row.difficultyID, &row.name, &row.color); err != nil {
			return nil, err
		}
		rowsData = append(rowsData, row)
		countsByDifficulty[row.difficultyID]++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for difficultyID, count := range countsByDifficulty {
		parent, ok := difficultyByID[difficultyID]
		if !ok {
			continue
		}
		parent.Categories = make([]benchmarks.BenchmarkCategory, 0, count)
	}

	out := make(map[int64]*benchmarks.BenchmarkCategory)
	for _, row := range rowsData {
		var category benchmarks.BenchmarkCategory
		category.CategoryName = row.name
		category.Color = row.color
		category.Scenarios = []benchmarks.BenchmarkScenario{}
		category.Subcategories = []benchmarks.BenchmarkSubcategory{}

		parent, ok := difficultyByID[row.difficultyID]
		if !ok {
			continue
		}
		parent.Categories = append(parent.Categories, category)
		out[row.id] = &parent.Categories[len(parent.Categories)-1]
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
		sub.Scenarios = []benchmarks.BenchmarkScenario{}

		parent, ok := categoryByID[categoryID]
		if !ok {
			continue
		}
		parent.Subcategories = append(parent.Subcategories, sub)
	}

	return rows.Err()
}

func (r *SupabaseRepository) attachScenarioLinks(ctx context.Context, difficultyByID map[int64]*benchmarks.BenchmarkDifficulty) error {
	difficultyIDs := keysInt64(difficultyByID)

	categoryLookup := make(map[categoryLookupKey]*benchmarks.BenchmarkCategory)
	subcategoryLookup := make(map[subcategoryLookupKey]*benchmarks.BenchmarkSubcategory)
	for difficultyID, difficulty := range difficultyByID {
		for categoryIndex := range difficulty.Categories {
			category := &difficulty.Categories[categoryIndex]
			categoryLookup[categoryLookupKey{difficultyID: difficultyID, name: normalizedHierarchyKey(category.CategoryName)}] = category
			for subIndex := range category.Subcategories {
				sub := &category.Subcategories[subIndex]
				subcategoryLookup[subcategoryLookupKey{
					difficultyID: difficultyID,
					categoryName: normalizedHierarchyKey(category.CategoryName),
					name:         normalizedHierarchyKey(sub.SubcategoryName),
				}] = sub
			}
		}
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			bds.difficulty_id,
			bds.category_name,
			bds.subcategory_name,
			s.scenario_name,
			bds.rank_thresholds
		FROM benchmark_difficulty_scenarios bds
		JOIN scenarios s ON s.id = bds.scenario_id
		WHERE bds.difficulty_id = ANY($1)
		ORDER BY bds.difficulty_id ASC, bds.sort_order ASC, s.scenario_name ASC
	`, difficultyIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var difficultyID int64
		var categoryName string
		var subcategoryName string
		var scenarioName string
		var rankThresholdsRaw []byte

		if err := rows.Scan(&difficultyID, &categoryName, &subcategoryName, &scenarioName, &rankThresholdsRaw); err != nil {
			return err
		}

		difficulty, ok := difficultyByID[difficultyID]
		if !ok {
			continue
		}

		rankThresholds := make([]float64, 0)
		if len(rankThresholdsRaw) > 0 {
			if err := json.Unmarshal(rankThresholdsRaw, &rankThresholds); err != nil {
				return err
			}
		}

		scenario := benchmarks.BenchmarkScenario{
			ScenarioName:   scenarioName,
			RankThresholds: rankThresholds,
		}

		normalizedCategory := normalizedHierarchyKey(categoryName)
		normalizedSubcategory := normalizedHierarchyKey(subcategoryName)

		appendScenarioToHierarchy(
			difficulty,
			categoryLookup,
			subcategoryLookup,
			difficultyID,
			normalizedCategory,
			normalizedSubcategory,
			scenario,
		)
	}

	return rows.Err()
}

func appendScenarioToHierarchy(
	difficulty *benchmarks.BenchmarkDifficulty,
	categoryLookup map[categoryLookupKey]*benchmarks.BenchmarkCategory,
	subcategoryLookup map[subcategoryLookupKey]*benchmarks.BenchmarkSubcategory,
	difficultyID int64,
	normalizedCategory string,
	normalizedSubcategory string,
	scenario benchmarks.BenchmarkScenario,
) {
	if sub, ok := subcategoryLookup[subcategoryLookupKey{difficultyID: difficultyID, categoryName: normalizedCategory, name: normalizedSubcategory}]; ok {
		sub.Scenarios = append(sub.Scenarios, scenario)
		return
	}
	if category, ok := categoryLookup[categoryLookupKey{difficultyID: difficultyID, name: normalizedCategory}]; ok {
		category.Scenarios = append(category.Scenarios, scenario)
		return
	}
	difficulty.Scenarios = append(difficulty.Scenarios, scenario)
}

func normalizedHierarchyKey(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func keysInt64[T any](in map[int64]T) []int64 {
	out := make([]int64, 0, len(in))
	for key := range in {
		out = append(out, key)
	}
	return out
}
