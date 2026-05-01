package players

import "time"

// PlayerSort controls list ordering for player browsing endpoints.
type PlayerSort string

const (
	PlayerSortRunCountDesc PlayerSort = "run_count_desc"
	PlayerSortRunCountAsc  PlayerSort = "run_count_asc"
	PlayerSortNameAsc      PlayerSort = "name_asc"
)

// ListRequest captures filters and pagination for listing players.
type ListRequest struct {
	Query  string
	Sort   PlayerSort
	Limit  int
	Offset int
}

// PlayerListItem is one row in the player browser.
type PlayerListItem struct {
	SteamID       string     `json:"steam_id"`
	SteamUsername string     `json:"steam_username,omitempty"`
	RunCount      int64      `json:"run_count"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
}

// PlayerDetail contains full profile metadata for one player.
type PlayerDetail struct {
	SteamID       string     `json:"steam_id"`
	SteamUsername string     `json:"steam_username,omitempty"`
	RunCount      int64      `json:"run_count"`
	LastRunAt     *time.Time `json:"last_run_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ListResponse is the paginated player list API payload.
type ListResponse struct {
	Players    []PlayerListItem `json:"players"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
	Count      int              `json:"count"`
	HasMore    bool             `json:"has_more"`
	NextOffset *int             `json:"next_offset,omitempty"`
}
