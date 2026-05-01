package scenarios

import (
	"encoding/json"
	"time"
)

// ScenarioSort controls list ordering for scenario browsing endpoints.
type ScenarioSort string

const (
	ScenarioSortRunCountDesc ScenarioSort = "run_count_desc"
	ScenarioSortRunCountAsc  ScenarioSort = "run_count_asc"
	ScenarioSortNameAsc      ScenarioSort = "name_asc"
	ScenarioSortNameDesc     ScenarioSort = "name_desc"
	ScenarioSortUpdatedDesc  ScenarioSort = "updated_at_desc"
)

// ListRequest captures filters and pagination for listing scenarios.
type ListRequest struct {
	Query  string
	Sort   ScenarioSort
	Limit  int
	Offset int
}

// ScenarioListItem is one row in the scenario browser.
type ScenarioListItem struct {
	ID               int64     `json:"id"`
	ScenarioName     string    `json:"scenario_name"`
	RunCount         int64     `json:"run_count"`
	ScoreSampleCount int64     `json:"score_sample_count"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ScenarioDetail contains full scenario metadata including score and sensitivity distributions.
type ScenarioDetail struct {
	ID                int64           `json:"id"`
	ScenarioName      string          `json:"scenario_name"`
	RunCount          int64           `json:"run_count"`
	ScoreSampleCount  int64           `json:"score_sample_count"`
	ScoreDistribution json.RawMessage `json:"score_distribution,omitempty"`
	SensSampleCount   int64           `json:"sens_sample_count"`
	SensDistribution  json.RawMessage `json:"sens_distribution,omitempty"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// ListResponse is the paginated scenario list API payload.
type ListResponse struct {
	Scenarios  []ScenarioListItem `json:"scenarios"`
	Limit      int                `json:"limit"`
	Offset     int                `json:"offset"`
	Count      int                `json:"count"`
	HasMore    bool               `json:"has_more"`
	NextOffset *int               `json:"next_offset,omitempty"`
}
