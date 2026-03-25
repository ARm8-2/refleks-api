package adapters

import (
	"context"
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
func NewSupabaseRepository(ctx context.Context, pool *pgxpool.Pool) (*SupabaseRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("supabase pool is required")
	}

	repo := &SupabaseRepository{pool: pool}
	if err := repo.ensureSchema(ctx); err != nil {
		return nil, err
	}

	return repo, nil
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
			file_name,
			epoch_milli,
			size_bytes,
			object_key,
			checksum_hex,
			compression,
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
			$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
		)
		ON CONFLICT (hash) DO NOTHING
	`,
		accountID,
		scenarioID,
		meta.Hash,
		meta.FileName,
		meta.EpochMilli,
		meta.SizeBytes,
		meta.ObjectKey,
		meta.ChecksumHex,
		meta.Compression,
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

func (r *SupabaseRepository) ensureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id BIGSERIAL PRIMARY KEY,
			steam_id TEXT NOT NULL UNIQUE,
			steam_username TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS scenarios (
			id BIGSERIAL PRIMARY KEY,
			scenario_name TEXT NOT NULL UNIQUE,
			median_score DOUBLE PRECISION,
			stddev_score DOUBLE PRECISION,
			p95_score DOUBLE PRECISION,
			run_count BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id BIGSERIAL PRIMARY KEY,
			account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
			scenario_id BIGINT NOT NULL REFERENCES scenarios(id) ON DELETE RESTRICT,
			hash CHAR(64) NOT NULL,
			file_name TEXT NOT NULL,
			epoch_milli BIGINT NOT NULL,
			size_bytes BIGINT NOT NULL,
			object_key TEXT NOT NULL,
			checksum_hex CHAR(16) NOT NULL,
			compression SMALLINT NOT NULL,
			format_version SMALLINT NOT NULL,
			score DOUBLE PRECISION,
			accuracy DOUBLE PRECISION,
			avg_ttk_seconds DOUBLE PRECISION,
			duration_seconds DOUBLE PRECISION,
			sens_cm360 DOUBLE PRECISION,
			has_mouse_trace BOOLEAN NOT NULL DEFAULT FALSE,
			avg_mouse_speed DOUBLE PRECISION,
			mouse_vid TEXT,
			mouse_pid TEXT,
			uploaded_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (hash),
			UNIQUE (object_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_uploaded_at ON runs (uploaded_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_epoch ON runs (epoch_milli DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_scenario_id ON runs (scenario_id)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_account_id ON runs (account_id)`,
	}

	for _, statement := range statements {
		if _, err := r.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("ensure schema: %w", err)
		}
	}

	return nil
}
