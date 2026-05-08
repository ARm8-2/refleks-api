package adapters

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/stats"
)

// PostgresRepository reads aggregate stats from Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a Postgres-backed stats repository.
func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	return &PostgresRepository{pool: pool}, nil
}

// Counts returns overall totals for the main browseable entities.
func (r *PostgresRepository) Counts(ctx context.Context) (stats.Response, error) {
	var resp stats.Response
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM runs) AS total_runs,
			(SELECT COUNT(*) FROM players) AS total_players,
			(SELECT COUNT(*) FROM scenarios) AS total_scenarios
	`).Scan(
		&resp.TotalRuns,
		&resp.TotalPlayers,
		&resp.TotalScenarios,
	)
	if err != nil {
		return stats.Response{}, err
	}

	return resp, nil
}
