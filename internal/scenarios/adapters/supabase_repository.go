package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/scenarios"
)

// SupabaseRepository reads scenario metadata from Supabase Postgres.
type SupabaseRepository struct {
	pool *pgxpool.Pool
}

// NewSupabaseRepository constructs a Supabase-backed scenario repository.
func NewSupabaseRepository(pool *pgxpool.Pool) (*SupabaseRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("supabase pool is required")
	}
	return &SupabaseRepository{pool: pool}, nil
}

// ListScenarios returns a filtered, sorted, paginated list of scenarios.
func (r *SupabaseRepository) ListScenarios(ctx context.Context, req scenarios.ListRequest) ([]scenarios.ScenarioListItem, error) {
	args := make([]any, 0, 4)
	where := ""
	if req.Query != "" {
		args = append(args, "%"+req.Query+"%")
		where = fmt.Sprintf("WHERE s.scenario_name ILIKE $%d", len(args))
	}

	orderBy := scenariosOrderBySQL(req.Sort)
	args = append(args, req.Limit)
	limitPos := len(args)
	args = append(args, req.Offset)
	offsetPos := len(args)

	query := fmt.Sprintf(`
		SELECT s.id, s.scenario_name, s.run_count, s.score_sample_count, s.updated_at
		FROM scenarios s
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]scenarios.ScenarioListItem, 0, req.Limit)
	for rows.Next() {
		var item scenarios.ScenarioListItem
		if err := rows.Scan(
			&item.ID,
			&item.ScenarioName,
			&item.RunCount,
			&item.ScoreSampleCount,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ScenarioByID returns full detail for one scenario including score and sensitivity distributions.
func (r *SupabaseRepository) ScenarioByID(ctx context.Context, id int64) (scenarios.ScenarioDetail, error) {
	var detail scenarios.ScenarioDetail
	var scoreDistJSON []byte
	var sensDistJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, scenario_name, run_count, score_sample_count, score_distribution,
		       sens_sample_count, sens_distribution, updated_at
		FROM scenarios
		WHERE id = $1
		LIMIT 1
	`, id).Scan(
		&detail.ID,
		&detail.ScenarioName,
		&detail.RunCount,
		&detail.ScoreSampleCount,
		&scoreDistJSON,
		&detail.SensSampleCount,
		&sensDistJSON,
		&detail.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return scenarios.ScenarioDetail{}, scenarios.ErrNotFound
		}
		return scenarios.ScenarioDetail{}, err
	}

	if len(scoreDistJSON) > 0 && string(scoreDistJSON) != "{}" {
		detail.ScoreDistribution = scoreDistJSON
	}
	if len(sensDistJSON) > 0 && string(sensDistJSON) != "{}" {
		detail.SensDistribution = sensDistJSON
	}

	return detail, nil
}

func scenariosOrderBySQL(sort scenarios.ScenarioSort) string {
	switch sort {
	case scenarios.ScenarioSortRunCountAsc:
		return "s.run_count ASC, s.id ASC"
	case scenarios.ScenarioSortNameAsc:
		return "s.scenario_name ASC, s.id ASC"
	case scenarios.ScenarioSortNameDesc:
		return "s.scenario_name DESC, s.id ASC"
	case scenarios.ScenarioSortUpdatedDesc:
		return "s.updated_at DESC, s.id ASC"
	default: // ScenarioSortRunCountDesc
		return "s.run_count DESC, s.id ASC"
	}
}
