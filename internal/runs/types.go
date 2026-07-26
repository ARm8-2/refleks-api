package runs

import "time"

const (
	RunFileExtension = ".refleks"
)

type RunMetadata struct {
	Hash          string
	FileName      string
	ScenarioName  string
	SteamID       string
	SteamUsername string
	PlayedAt      time.Time
	SizeBytes     int64
	ObjectKey     string
	UploadedAt    time.Time
	FormatVersion uint8
	Score         *float64
	Accuracy      *float64
	AvgTTKSeconds *float64
	DurationSecs  *float64
	SensCM360     *float64
	HasMouseTrace bool
	AvgMouseSpeed *float64
	MouseVID      string
	MousePID      string
}

type SyncResult struct {
	Hash           string `json:"hash"`
	AlreadyPresent bool   `json:"already_present"`
	Stored         bool   `json:"stored"`
	FileName       string `json:"file_name"`
	PlayedAt       int64  `json:"played_at"`
	SizeBytes      int64  `json:"size_bytes"`
}

type ParsedRefleksFile struct {
	FileName      string
	ScenarioName  string
	SteamID       string
	SteamUsername string
	PlayedAt      int64
	FormatVersion uint8
	Score         *float64
	Accuracy      *float64
	AvgTTKSeconds *float64
	DurationSecs  *float64
	SensCM360     *float64
	HasMouseTrace bool
	AvgMouseSpeed *float64
	MouseVID      string
	MousePID      string
}

type RunsSort string

const (
	RunsSortUploadedAtDesc RunsSort = "uploaded_at_desc"
	RunsSortUploadedAtAsc  RunsSort = "uploaded_at_asc"
	RunsSortPlayedAtDesc   RunsSort = "played_at_desc"
	RunsSortPlayedAtAsc    RunsSort = "played_at_asc"
	RunsSortScoreDesc      RunsSort = "score_desc"
	RunsSortScoreAsc       RunsSort = "score_asc"
	RunsSortAccuracyDesc   RunsSort = "accuracy_desc"
	RunsSortAccuracyAsc    RunsSort = "accuracy_asc"
	RunsSortAvgTTKDesc     RunsSort = "avg_ttk_desc"
	RunsSortAvgTTKAsc      RunsSort = "avg_ttk_asc"
)

type RunListRequest struct {
	Limit         int
	Offset        int
	Sort          RunsSort
	ScenarioID    int64
	ScenarioName  string
	SteamID       string
	SteamUsername string
	Query         string
	HasMouseTrace *bool
	MinScore      *float64
	MaxScore      *float64
	MinAccuracy   *float64
	MaxAccuracy   *float64
	FromPlayedAt  *int64
	ToPlayedAt    *int64
	FromUploaded  *int64
	ToUploaded    *int64
}

type RunListItem struct {
	Hash          string   `json:"hash"`
	FileName      string   `json:"file_name"`
	ScenarioName  string   `json:"scenario_name"`
	SteamID       string   `json:"steam_id,omitempty"`
	SteamUsername string   `json:"steam_username,omitempty"`
	PlayedAt      int64    `json:"played_at"`
	UploadedAt    int64    `json:"uploaded_at"`
	SizeBytes     int64    `json:"size_bytes"`
	Score         *float64 `json:"score,omitempty"`
	Accuracy      *float64 `json:"accuracy,omitempty"`
	AvgTTKSeconds *float64 `json:"avg_ttk_seconds,omitempty"`
	DurationSecs  *float64 `json:"duration_seconds,omitempty"`
	SensCM360     *float64 `json:"sens_cm360,omitempty"`
	HasMouseTrace bool     `json:"has_mouse_trace"`
	AvgMouseSpeed *float64 `json:"avg_mouse_speed,omitempty"`
	MouseVID      string   `json:"mouse_vid,omitempty"`
	MousePID      string   `json:"mouse_pid,omitempty"`
}

type RunListResponse struct {
	Runs       []RunListItem `json:"runs"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
	Count      int           `json:"count"`
	HasMore    bool          `json:"has_more"`
	NextOffset *int          `json:"next_offset,omitempty"`
}
