package stats

// Response is the aggregate stats payload.
type Response struct {
	TotalRuns      int64 `json:"total_runs"`
	TotalPlayers   int64 `json:"total_players"`
	TotalScenarios int64 `json:"total_scenarios"`
}
