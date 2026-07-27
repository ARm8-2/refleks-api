package leaderboards

import "time"

// ScenarioRequest captures query options for one scenario leaderboard.
type ScenarioRequest struct {
	ScenarioID   *int64
	ScenarioName string
	Limit        int
	Offset       int
}

// BenchmarkDifficultyRequest captures query options for one benchmark-difficulty leaderboard.
type BenchmarkDifficultyRequest struct {
	DifficultyID       *int64
	KovaaksBenchmarkID *int64
	Limit              int
	Offset             int
}

// ScenarioEntry models one ranked scenario leaderboard row.
type ScenarioEntry struct {
	Rank          int     `json:"rank"`
	BestScore     float64 `json:"best_score"`
	BestPlayedAt  *int64  `json:"best_played_at,omitempty"`
	SteamID       string  `json:"steam_id"`
	SteamUsername string  `json:"steam_username,omitempty"`
}

// ScenarioLeaderboardResponse is the API payload for one scenario leaderboard page.
type ScenarioLeaderboardResponse struct {
	ScenarioID   int64           `json:"scenario_id"`
	ScenarioName string          `json:"scenario_name"`
	Entries      []ScenarioEntry `json:"entries"`
	Limit        int             `json:"limit"`
	Offset       int             `json:"offset"`
	Count        int             `json:"count"`
	HasMore      bool            `json:"has_more"`
	NextOffset   *int            `json:"next_offset,omitempty"`
	RefreshedAt  *time.Time      `json:"refreshed_at,omitempty"`
}

// BenchmarkDifficultyEntry models one benchmark difficulty leaderboard row.
type BenchmarkDifficultyEntry struct {
	Rank             int     `json:"rank"`
	CompositeScore   float64 `json:"composite_score"`
	MatchedScenarios int     `json:"matched_scenarios"`
	LastPlayedAt     *int64  `json:"last_played_at,omitempty"`
	SteamID          string  `json:"steam_id"`
	SteamUsername    string  `json:"steam_username,omitempty"`
}

// BenchmarkDifficultyLeaderboardResponse is the API payload for one benchmark-difficulty leaderboard page.
type BenchmarkDifficultyLeaderboardResponse struct {
	DifficultyID       int64                      `json:"difficulty_id"`
	KovaaksBenchmarkID int64                      `json:"kovaaks_benchmark_id"`
	BenchmarkName      string                     `json:"benchmark_name"`
	DifficultyName     string                     `json:"difficulty_name"`
	Entries            []BenchmarkDifficultyEntry `json:"entries"`
	Limit              int                        `json:"limit"`
	Offset             int                        `json:"offset"`
	Count              int                        `json:"count"`
	HasMore            bool                       `json:"has_more"`
	NextOffset         *int                       `json:"next_offset,omitempty"`
	RefreshedAt        *time.Time                 `json:"refreshed_at,omitempty"`
}
