package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/runs"
)

// PostgresRepository stores run metadata in Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository builds a run sync repository from a shared Postgres pool.
func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}

	return &PostgresRepository{pool: pool}, nil
}

// ExistingHashes returns the subset of hashes that already exist.
func (r *PostgresRepository) ExistingHashes(ctx context.Context, hashes []string) (map[string]struct{}, error) {
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

// RunDetail returns metadata for a single run by hash, shaped as a list item.
func (r *PostgresRepository) RunDetail(ctx context.Context, hash string) (runs.RunListItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			r.hash,
			r.file_name,
			s.scenario_name,
			p.steam_id,
			p.steam_username,
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
		LEFT JOIN players p ON p.id = r.player_id
		WHERE r.hash = $1
		LIMIT 1
	`, hash)
	if err != nil {
		return runs.RunListItem{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return runs.RunListItem{}, err
		}
		return runs.RunListItem{}, runs.ErrObjectNotFound
	}

	var item runs.RunListItem
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
		return runs.RunListItem{}, err
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

	return item, rows.Err()
}

// RunByHash returns one persisted run lookup by hash.
func (r *PostgresRepository) RunByHash(ctx context.Context, hash string) (runs.StoredRun, error) {
	var run runs.StoredRun
	err := r.pool.QueryRow(ctx,
		`SELECT object_key, file_name FROM runs WHERE hash = $1 LIMIT 1`,
		hash,
	).Scan(&run.ObjectKey, &run.FileName)
	if err == nil {
		return run, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return runs.StoredRun{}, runs.ErrObjectNotFound
	}
	return runs.StoredRun{}, err
}

// ListRuns returns filtered/sorted run rows for frontend browse pages.
func (r *PostgresRepository) ListRuns(ctx context.Context, req runs.RunListRequest) ([]runs.RunListItem, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 10)

	addArgClause := func(expr string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(expr, len(args)))
	}

	if req.ScenarioID > 0 {
		addArgClause("r.scenario_id = $%d", req.ScenarioID)
	}
	if req.ScenarioName != "" {
		addArgClause("s.scenario_name ILIKE $%d", "%"+req.ScenarioName+"%")
	}
	if req.SteamID != "" {
		addArgClause("COALESCE(p.steam_id, '') ILIKE $%d", "%"+req.SteamID+"%")
	}
	if req.SteamUsername != "" {
		addArgClause("COALESCE(p.steam_username, '') ILIKE $%d", "%"+req.SteamUsername+"%")
	}
	if req.Query != "" {
		args = append(args, "%"+req.Query+"%")
		p := len(args)
		clauses = append(clauses,
			fmt.Sprintf("(r.file_name ILIKE $%d OR s.scenario_name ILIKE $%d OR COALESCE(p.steam_username, '') ILIKE $%d OR COALESCE(p.steam_id, '') ILIKE $%d)", p, p, p, p),
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
	if req.MinAccuracy != nil {
		addArgClause("r.accuracy >= $%d", *req.MinAccuracy)
	}
	if req.MaxAccuracy != nil {
		addArgClause("r.accuracy <= $%d", *req.MaxAccuracy)
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
			p.steam_id,
			p.steam_username,
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
		LEFT JOIN players p ON p.id = r.player_id
		%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]runs.RunListItem, 0, req.Limit)
	for rows.Next() {
		var item runs.RunListItem
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
func (r *PostgresRepository) InsertRun(ctx context.Context, meta runs.RunMetadata) (bool, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	playerID, err := ensurePlayer(ctx, tx, meta)
	if err != nil {
		return false, err
	}
	scenarioID, err := ensureScenario(ctx, tx, meta.ScenarioName)
	if err != nil {
		return false, err
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO runs (
			player_id,
			scenario_id,
			hash,
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
			$11,$12,$13,$14,$15,$16,$17,$18
		)
		ON CONFLICT DO NOTHING
	`,
		playerID,
		scenarioID,
		meta.Hash,
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

func ensurePlayer(ctx context.Context, tx pgx.Tx, meta runs.RunMetadata) (*int64, error) {
	steamID := strings.TrimSpace(meta.SteamID)
	if steamID == "" {
		return nil, nil
	}

	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO players (steam_id, steam_username, updated_at)
		VALUES ($1, NULLIF($2, ''), NOW())
		ON CONFLICT (steam_id) DO UPDATE
			SET steam_username = COALESCE(NULLIF(EXCLUDED.steam_username, ''), players.steam_username),
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

func runsOrderBySQL(sort runs.RunsSort) string {
	switch sort {
	case runs.RunsSortUploadedAtAsc:
		return "r.uploaded_at ASC, r.id ASC"
	case runs.RunsSortEpochDesc:
		return "r.epoch_milli DESC, r.id DESC"
	case runs.RunsSortEpochAsc:
		return "r.epoch_milli ASC, r.id ASC"
	case runs.RunsSortScoreDesc:
		return "r.score DESC NULLS LAST, r.id DESC"
	case runs.RunsSortScoreAsc:
		return "r.score ASC NULLS LAST, r.id ASC"
	case runs.RunsSortAccuracyDesc:
		return "r.accuracy DESC NULLS LAST, r.id DESC"
	case runs.RunsSortAccuracyAsc:
		return "r.accuracy ASC NULLS LAST, r.id ASC"
	case runs.RunsSortAvgTTKDesc:
		return "r.avg_ttk_seconds DESC NULLS LAST, r.id DESC"
	case runs.RunsSortAvgTTKAsc:
		return "r.avg_ttk_seconds ASC NULLS LAST, r.id ASC"
	default:
		return "r.uploaded_at DESC, r.id DESC"
	}
}
