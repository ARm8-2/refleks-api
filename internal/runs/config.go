package runs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunSyncSettings are runtime-togglable options for run sync behavior.
// Each field maps to a row in the run_sync_config table.
type RunSyncSettings struct {
	SyncEnabled             bool
	StoreRunsEnabled        bool
	StoreAnonOnly           bool
	StoreWithMouseTraceOnly bool
}

// DefaultRunSyncSettings returns the default settings.
func DefaultRunSyncSettings() RunSyncSettings {
	return RunSyncSettings{
		SyncEnabled:             true,
		StoreRunsEnabled:        true,
		StoreAnonOnly:           false,
		StoreWithMouseTraceOnly: false,
	}
}

// PGConfigStore reads run sync settings from the database.
type PGConfigStore struct {
	pool *pgxpool.Pool
}

// NewPGConfigStore creates a run sync config store.
func NewPGConfigStore(pool *pgxpool.Pool) *PGConfigStore {
	return &PGConfigStore{pool: pool}
}

// Load reads the current run sync settings from the database.
// Missing keys are treated as their defaults.
func (s *PGConfigStore) Load(ctx context.Context) (RunSyncSettings, error) {
	out := DefaultRunSyncSettings()

	rows, err := s.pool.Query(ctx, `
		SELECT key, value
		FROM run_sync_config
		ORDER BY key
	`)
	if err != nil {
		return RunSyncSettings{}, fmt.Errorf("load run sync config: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var value bool
		if err := rows.Scan(&key, &value); err != nil {
			return RunSyncSettings{}, fmt.Errorf("scan run sync config: %w", err)
		}
		switch key {
		case "sync_enabled":
			out.SyncEnabled = value
		case "store_runs_enabled":
			out.StoreRunsEnabled = value
		case "store_anon_only":
			out.StoreAnonOnly = value
		case "store_with_mouse_trace_only":
			out.StoreWithMouseTraceOnly = value
		}
	}
	if err := rows.Err(); err != nil {
		return RunSyncSettings{}, fmt.Errorf("iterate run sync config: %w", err)
	}

	return out, nil
}
