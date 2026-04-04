package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/leaderboards"
)

// SupabaseRepository reads precomputed leaderboards from Supabase Postgres.
type SupabaseRepository struct {
	pool *pgxpool.Pool
}

// NewSupabaseRepository constructs a Supabase-backed leaderboard repository.
func NewSupabaseRepository(pool *pgxpool.Pool) (*SupabaseRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("supabase pool is required")
	}
	return &SupabaseRepository{pool: pool}, nil
}

// ScenarioLeaderboard returns one scenario leaderboard page.
func (r *SupabaseRepository) ScenarioLeaderboard(ctx context.Context, req leaderboards.ScenarioRequest) (leaderboards.ScenarioLeaderboardResponse, error) {
	scenarioID, scenarioName, err := r.resolveScenario(ctx, req)
	if err != nil {
		return leaderboards.ScenarioLeaderboardResponse{}, err
	}

	refreshedAt, err := r.fetchScenarioRefreshedAt(ctx, scenarioID)
	if err != nil {
		return leaderboards.ScenarioLeaderboardResponse{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			sl.rank,
			sl.best_score,
			sl.best_epoch_milli,
			a.steam_id,
			a.steam_username
		FROM scenario_leaderboard_current sl
		JOIN accounts a ON a.id = sl.account_id
		WHERE sl.scenario_id = $1
		ORDER BY sl.rank ASC
		LIMIT $2 OFFSET $3
	`, scenarioID, req.Limit+1, req.Offset)
	if err != nil {
		return leaderboards.ScenarioLeaderboardResponse{}, err
	}
	defer rows.Close()

	entries := make([]leaderboards.ScenarioEntry, 0, req.Limit+1)
	for rows.Next() {
		var entry leaderboards.ScenarioEntry
		var bestEpoch sql.NullInt64
		var steamUsername sql.NullString

		if err := rows.Scan(&entry.Rank, &entry.BestScore, &bestEpoch, &entry.SteamID, &steamUsername); err != nil {
			return leaderboards.ScenarioLeaderboardResponse{}, err
		}
		if bestEpoch.Valid {
			copy := bestEpoch.Int64
			entry.BestEpochMilli = &copy
		}
		if steamUsername.Valid {
			entry.SteamUsername = steamUsername.String
		}

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return leaderboards.ScenarioLeaderboardResponse{}, err
	}

	hasMore := len(entries) > req.Limit
	if hasMore {
		entries = entries[:req.Limit]
	}

	resp := leaderboards.ScenarioLeaderboardResponse{
		ScenarioID:   scenarioID,
		ScenarioName: scenarioName,
		Entries:      entries,
		Limit:        req.Limit,
		Offset:       req.Offset,
		Count:        len(entries),
		HasMore:      hasMore,
		RefreshedAt:  refreshedAt,
	}
	if hasMore {
		next := req.Offset + len(entries)
		resp.NextOffset = &next
	}

	return resp, nil
}

// BenchmarkDifficultyLeaderboard returns one benchmark difficulty leaderboard page.
func (r *SupabaseRepository) BenchmarkDifficultyLeaderboard(ctx context.Context, req leaderboards.BenchmarkDifficultyRequest) (leaderboards.BenchmarkDifficultyLeaderboardResponse, error) {
	difficultyID, kovaaksID, benchmarkName, difficultyName, err := r.resolveDifficulty(ctx, req)
	if err != nil {
		return leaderboards.BenchmarkDifficultyLeaderboardResponse{}, err
	}

	refreshedAt, err := r.fetchDifficultyRefreshedAt(ctx, difficultyID)
	if err != nil {
		return leaderboards.BenchmarkDifficultyLeaderboardResponse{}, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			l.rank,
			l.composite_score,
			l.matched_scenarios,
			l.last_epoch_milli,
			a.steam_id,
			a.steam_username
		FROM benchmark_difficulty_leaderboard_current l
		JOIN accounts a ON a.id = l.account_id
		WHERE l.difficulty_id = $1
		ORDER BY l.rank ASC
		LIMIT $2 OFFSET $3
	`, difficultyID, req.Limit+1, req.Offset)
	if err != nil {
		return leaderboards.BenchmarkDifficultyLeaderboardResponse{}, err
	}
	defer rows.Close()

	entries := make([]leaderboards.BenchmarkDifficultyEntry, 0, req.Limit+1)
	for rows.Next() {
		var entry leaderboards.BenchmarkDifficultyEntry
		var lastEpoch sql.NullInt64
		var steamUsername sql.NullString

		if err := rows.Scan(
			&entry.Rank,
			&entry.CompositeScore,
			&entry.MatchedScenarios,
			&lastEpoch,
			&entry.SteamID,
			&steamUsername,
		); err != nil {
			return leaderboards.BenchmarkDifficultyLeaderboardResponse{}, err
		}
		if lastEpoch.Valid {
			copy := lastEpoch.Int64
			entry.LastEpochMilli = &copy
		}
		if steamUsername.Valid {
			entry.SteamUsername = steamUsername.String
		}

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return leaderboards.BenchmarkDifficultyLeaderboardResponse{}, err
	}

	hasMore := len(entries) > req.Limit
	if hasMore {
		entries = entries[:req.Limit]
	}

	resp := leaderboards.BenchmarkDifficultyLeaderboardResponse{
		DifficultyID:       difficultyID,
		KovaaksBenchmarkID: kovaaksID,
		BenchmarkName:      benchmarkName,
		DifficultyName:     difficultyName,
		Entries:            entries,
		Limit:              req.Limit,
		Offset:             req.Offset,
		Count:              len(entries),
		HasMore:            hasMore,
		RefreshedAt:        refreshedAt,
	}
	if hasMore {
		next := req.Offset + len(entries)
		resp.NextOffset = &next
	}

	return resp, nil
}

func (r *SupabaseRepository) resolveScenario(ctx context.Context, req leaderboards.ScenarioRequest) (int64, string, error) {
	var id int64
	var name string

	var err error
	if req.ScenarioID != nil {
		err = r.pool.QueryRow(ctx,
			`SELECT id, scenario_name FROM scenarios WHERE id = $1 LIMIT 1`,
			*req.ScenarioID,
		).Scan(&id, &name)
	} else {
		err = r.pool.QueryRow(ctx,
			`SELECT id, scenario_name FROM scenarios WHERE lower(scenario_name) = lower($1) LIMIT 1`,
			req.ScenarioName,
		).Scan(&id, &name)
	}

	if err == nil {
		return id, name, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", leaderboards.ErrNotFound
	}
	return 0, "", err
}

func (r *SupabaseRepository) resolveDifficulty(ctx context.Context, req leaderboards.BenchmarkDifficultyRequest) (int64, int64, string, string, error) {
	var id int64
	var kovaaksID int64
	var benchmarkName string
	var difficultyName string

	var err error
	if req.DifficultyID != nil {
		err = r.pool.QueryRow(ctx, `
			SELECT d.id, d.kovaaks_benchmark_id, b.benchmark_name, d.difficulty_name
			FROM benchmark_difficulties d
			JOIN benchmarks b ON b.id = d.benchmark_id
			WHERE d.id = $1
			LIMIT 1
		`, *req.DifficultyID).Scan(&id, &kovaaksID, &benchmarkName, &difficultyName)
	} else {
		err = r.pool.QueryRow(ctx, `
			SELECT d.id, d.kovaaks_benchmark_id, b.benchmark_name, d.difficulty_name
			FROM benchmark_difficulties d
			JOIN benchmarks b ON b.id = d.benchmark_id
			WHERE d.kovaaks_benchmark_id = $1
			LIMIT 1
		`, *req.KovaaksBenchmarkID).Scan(&id, &kovaaksID, &benchmarkName, &difficultyName)
	}

	if err == nil {
		return id, kovaaksID, benchmarkName, difficultyName, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, "", "", leaderboards.ErrNotFound
	}
	return 0, 0, "", "", err
}

func (r *SupabaseRepository) fetchScenarioRefreshedAt(ctx context.Context, scenarioID int64) (*time.Time, error) {
	var refreshed sql.NullTime
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(updated_at) FROM scenario_leaderboard_current WHERE scenario_id = $1`,
		scenarioID,
	).Scan(&refreshed)
	if err != nil {
		return nil, err
	}
	if !refreshed.Valid {
		return nil, nil
	}
	t := refreshed.Time.UTC()
	return &t, nil
}

func (r *SupabaseRepository) fetchDifficultyRefreshedAt(ctx context.Context, difficultyID int64) (*time.Time, error) {
	var refreshed sql.NullTime
	err := r.pool.QueryRow(ctx,
		`SELECT MAX(updated_at) FROM benchmark_difficulty_leaderboard_current WHERE difficulty_id = $1`,
		difficultyID,
	).Scan(&refreshed)
	if err != nil {
		return nil, err
	}
	if !refreshed.Valid {
		return nil, nil
	}
	t := refreshed.Time.UTC()
	return &t, nil
}
