package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/runsync"
)

// SupabaseRepository stores run metadata in Supabase Postgres.
type SupabaseRepository struct {
	pool *pgxpool.Pool
}

// NewSupabaseRepository builds a run sync repository from a shared Supabase pool.
func NewSupabaseRepository(pool *pgxpool.Pool) (*SupabaseRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("supabase pool is required")
	}

	return &SupabaseRepository{pool: pool}, nil
}

// ExistingHashes returns the subset of hashes that already exist.
func (r *SupabaseRepository) ExistingHashes(ctx context.Context, hashes []string) (map[string]struct{}, error) {
	found := make(map[string]struct{})
	if len(hashes) == 0 {
		return found, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT hash FROM runs WHERE hash = ANY($1)`,
		hashes,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		found[hash] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return found, nil
}

// ExistingStatsHashes returns the subset of stats hashes that already exist.
func (r *SupabaseRepository) ExistingStatsHashes(ctx context.Context, hashes []string) (map[string]struct{}, error) {
	found := make(map[string]struct{})
	if len(hashes) == 0 {
		return found, nil
	}

	rows, err := r.pool.Query(ctx,
		`SELECT stats_hash FROM runs WHERE stats_hash = ANY($1) AND stats_hash <> ''`,
		hashes,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return nil, err
		}
		found[strings.TrimSpace(hash)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return found, nil
}

// RunByHash returns one persisted run lookup by hash.
func (r *SupabaseRepository) RunByHash(ctx context.Context, hash string) (runsync.StoredRun, error) {
	var run runsync.StoredRun
	err := r.pool.QueryRow(ctx,
		`SELECT object_key, file_name FROM runs WHERE hash = $1 LIMIT 1`,
		hash,
	).Scan(&run.ObjectKey, &run.FileName)
	if err == nil {
		return run, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return runsync.StoredRun{}, runsync.ErrObjectNotFound
	}
	return runsync.StoredRun{}, err
}

// ListRuns returns filtered/sorted run rows for frontend browse pages.
func (r *SupabaseRepository) ListRuns(ctx context.Context, req runsync.RunListRequest) ([]runsync.RunListItem, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 10)

	addArgClause := func(expr string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(expr, len(args)))
	}

	if req.ScenarioName != "" {
		addArgClause("s.scenario_name ILIKE $%d", "%"+req.ScenarioName+"%")
	}
	if req.SteamID != "" {
		addArgClause("COALESCE(a.steam_id, '') ILIKE $%d", "%"+req.SteamID+"%")
	}
	if req.SteamUsername != "" {
		addArgClause("COALESCE(a.steam_username, '') ILIKE $%d", "%"+req.SteamUsername+"%")
	}
	if req.Query != "" {
		args = append(args, "%"+req.Query+"%")
		p := len(args)
		clauses = append(clauses,
			fmt.Sprintf("(r.file_name ILIKE $%d OR s.scenario_name ILIKE $%d OR COALESCE(a.steam_username, '') ILIKE $%d)", p, p, p),
		)
	}
	if req.HasMouseTrace != nil {
		addArgClause("r.has_mouse_trace = $%d", *req.HasMouseTrace)
	}
	if req.MinScore != nil {
		addArgClause("r.score >= $%d", *req.MinScore)
	}
	if req.MaxScore != nil {
		addArgClause("r.score <= $%d", *req.MaxScore)
	}
	if req.FromEpoch != nil {
		addArgClause("r.epoch_milli >= $%d", *req.FromEpoch)
	}
	if req.ToEpoch != nil {
		addArgClause("r.epoch_milli <= $%d", *req.ToEpoch)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	orderBy := runsOrderBySQL(req.Sort)
	args = append(args, req.Limit)
	limitPos := len(args)
	args = append(args, req.Offset)
	offsetPos := len(args)

	query := fmt.Sprintf(`
		SELECT
			r.hash,
			r.file_name,
			s.scenario_name,
			a.steam_id,
			a.steam_username,
			r.epoch_milli,
			r.uploaded_at,
			r.size_bytes,
			r.score,
			r.accuracy,
			r.avg_ttk_seconds,
			r.duration_seconds,
			r.sens_cm360,
			r.has_mouse_trace,
			r.avg_mouse_speed,
			r.mouse_vid,
			r.mouse_pid
		FROM runs r
		JOIN scenarios s ON s.id = r.scenario_id
		LEFT JOIN accounts a ON a.id = r.account_id
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]runsync.RunListItem, 0, req.Limit)
	for rows.Next() {
		var item runsync.RunListItem
		var steamID sql.NullString
		var steamUsername sql.NullString
		var score sql.NullFloat64
		var accuracy sql.NullFloat64
		var avgTTK sql.NullFloat64
		var duration sql.NullFloat64
		var sens sql.NullFloat64
		var avgSpeed sql.NullFloat64
		var mouseVID sql.NullString
		var mousePID sql.NullString

		if err := rows.Scan(
			&item.Hash,
			&item.FileName,
			&item.ScenarioName,
			&steamID,
			&steamUsername,
			&item.EpochMilli,
			&item.UploadedAt,
			&item.SizeBytes,
			&score,
			&accuracy,
			&avgTTK,
			&duration,
			&sens,
			&item.HasMouseTrace,
			&avgSpeed,
			&mouseVID,
			&mousePID,
		); err != nil {
			return nil, err
		}

		if steamID.Valid {
			item.SteamID = steamID.String
		}
		if steamUsername.Valid {
			item.SteamUsername = steamUsername.String
		}
		item.Score = nullFloatPtr(score)
		item.Accuracy = nullFloatPtr(accuracy)
		item.AvgTTKSeconds = nullFloatPtr(avgTTK)
		item.DurationSecs = nullFloatPtr(duration)
		item.SensCM360 = nullFloatPtr(sens)
		item.AvgMouseSpeed = nullFloatPtr(avgSpeed)
		if mouseVID.Valid {
			item.MouseVID = mouseVID.String
		}
		if mousePID.Valid {
			item.MousePID = mousePID.String
		}

		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// InsertRun inserts run metadata. It returns false when the row already exists.
func (r *SupabaseRepository) InsertRun(ctx context.Context, meta runsync.RunMetadata) (bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	accountID, err := ensureAccount(ctx, tx, meta)
	if err != nil {
		return false, err
	}
	scenarioID, err := ensureScenario(ctx, tx, meta.ScenarioName)
	if err != nil {
		return false, err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO runs (
			account_id,
			scenario_id,
			hash,
			stats_hash,
			file_name,
			epoch_milli,
			size_bytes,
			object_key,
			format_version,
			score,
			accuracy,
			avg_ttk_seconds,
			duration_seconds,
			sens_cm360,
			has_mouse_trace,
			avg_mouse_speed,
			mouse_vid,
			mouse_pid,
			uploaded_at
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,
			$11,$12,$13,$14,$15,$16,$17,$18,$19
		)
		ON CONFLICT DO NOTHING
	`,
		accountID,
		scenarioID,
		meta.Hash,
		meta.StatsHash,
		meta.FileName,
		meta.EpochMilli,
		meta.SizeBytes,
		meta.ObjectKey,
		meta.FormatVersion,
		meta.Score,
		meta.Accuracy,
		meta.AvgTTKSeconds,
		meta.DurationSecs,
		meta.SensCM360,
		meta.HasMouseTrace,
		meta.AvgMouseSpeed,
		nullIfEmpty(meta.MouseVID),
		nullIfEmpty(meta.MousePID),
		meta.UploadedAt,
	)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if _, err := tx.Exec(ctx,
		`UPDATE scenarios SET run_count = run_count + 1, updated_at = NOW() WHERE id = $1`,
		scenarioID,
	); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func ensureAccount(ctx context.Context, tx pgx.Tx, meta runsync.RunMetadata) (*int64, error) {
	steamID := strings.TrimSpace(meta.SteamID)
	if steamID == "" {
		return nil, nil
	}

	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO accounts (steam_id, steam_username, updated_at)
		VALUES ($1, NULLIF($2, ''), NOW())
		ON CONFLICT (steam_id) DO UPDATE
			SET steam_username = COALESCE(NULLIF(EXCLUDED.steam_username, ''), accounts.steam_username),
				updated_at = NOW()
		RETURNING id
	`, steamID, strings.TrimSpace(meta.SteamUsername)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func ensureScenario(ctx context.Context, tx pgx.Tx, scenarioName string) (int64, error) {
	name := strings.TrimSpace(scenarioName)
	if name == "" {
		name = "unknown"
	}

	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO scenarios (scenario_name, updated_at)
		VALUES ($1, NOW())
		ON CONFLICT (scenario_name) DO UPDATE
			SET updated_at = NOW()
		RETURNING id
	`, name).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func nullIfEmpty(v string) any {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func nullFloatPtr(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	copy := v.Float64
	return &copy
}

func runsOrderBySQL(sort runsync.RunsSort) string {
	switch sort {
	case runsync.RunsSortUploadedAtAsc:
		return "r.uploaded_at ASC, r.id ASC"
	case runsync.RunsSortEpochDesc:
		return "r.epoch_milli DESC, r.id DESC"
	case runsync.RunsSortEpochAsc:
		return "r.epoch_milli ASC, r.id ASC"
	case runsync.RunsSortScoreDesc:
		return "r.score DESC NULLS LAST, r.id DESC"
	case runsync.RunsSortScoreAsc:
		return "r.score ASC NULLS LAST, r.id ASC"
	default:
		return "r.uploaded_at DESC, r.id DESC"
	}
}
