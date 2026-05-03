package adapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"refleks-api/internal/players"
)

// PostgresRepository reads player account data from Postgres.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository constructs a Postgres-backed player repository.
func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

// ListPlayers returns a filtered, sorted, paginated list of players with their run counts.
func (r *PostgresRepository) ListPlayers(ctx context.Context, req players.ListRequest) ([]players.PlayerListItem, error) {
	args := make([]any, 0, 4)
	where := ""
	if req.Query != "" {
		args = append(args, "%"+req.Query+"%")
		p := len(args)
		where = fmt.Sprintf("WHERE (a.steam_username ILIKE $%d OR a.steam_id ILIKE $%d)", p, p)
	}

	orderBy := playersOrderBySQL(req.Sort)
	args = append(args, req.Limit)
	limitPos := len(args)
	args = append(args, req.Offset)
	offsetPos := len(args)

	query := fmt.Sprintf(`
		SELECT
			a.steam_id,
			a.steam_username,
			COUNT(r.id) AS run_count,
			MAX(r.uploaded_at) AS last_run_at
		FROM accounts a
		LEFT JOIN runs r ON r.account_id = a.id
		%s
		GROUP BY a.id, a.steam_id, a.steam_username
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, where, orderBy, limitPos, offsetPos)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]players.PlayerListItem, 0, req.Limit)
	for rows.Next() {
		var item players.PlayerListItem
		var steamUsername sql.NullString
		var lastRunAt sql.NullTime

		if err := rows.Scan(
			&item.SteamID,
			&steamUsername,
			&item.RunCount,
			&lastRunAt,
		); err != nil {
			return nil, err
		}

		if steamUsername.Valid {
			item.SteamUsername = steamUsername.String
		}
		if lastRunAt.Valid {
			t := lastRunAt.Time.UTC()
			item.LastRunAt = &t
		}

		out = append(out, item)
	}
	return out, rows.Err()
}

// PlayerBySteamID returns profile detail for one player.
func (r *PostgresRepository) PlayerBySteamID(ctx context.Context, steamID string) (players.PlayerDetail, error) {
	var detail players.PlayerDetail
	var steamUsername sql.NullString
	var lastRunAt sql.NullTime
	var runCount int64
	var createdAt time.Time

	err := r.pool.QueryRow(ctx, `
		SELECT
			a.steam_id,
			a.steam_username,
			a.created_at,
			COUNT(r.id) AS run_count,
			MAX(r.uploaded_at) AS last_run_at
		FROM accounts a
		LEFT JOIN runs r ON r.account_id = a.id
		WHERE a.steam_id = $1
		GROUP BY a.id, a.steam_id, a.steam_username, a.created_at
		LIMIT 1
	`, strings.TrimSpace(steamID)).Scan(
		&detail.SteamID,
		&steamUsername,
		&createdAt,
		&runCount,
		&lastRunAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return players.PlayerDetail{}, players.ErrNotFound
		}
		return players.PlayerDetail{}, err
	}

	detail.CreatedAt = createdAt.UTC()
	detail.RunCount = runCount
	if steamUsername.Valid {
		detail.SteamUsername = steamUsername.String
	}
	if lastRunAt.Valid {
		t := lastRunAt.Time.UTC()
		detail.LastRunAt = &t
	}

	return detail, nil
}

func playersOrderBySQL(sort players.PlayerSort) string {
	switch sort {
	case players.PlayerSortRunCountAsc:
		return "run_count ASC, a.id ASC"
	case players.PlayerSortNameAsc:
		return "a.steam_username ASC NULLS LAST, a.id ASC"
	default: // PlayerSortRunCountDesc
		return "run_count DESC, a.id ASC"
	}
}
