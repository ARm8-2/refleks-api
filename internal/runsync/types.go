package runsync

import "time"

const (
	// RunFileExtension is the canonical extension for raw run files.
	RunFileExtension = ".refleks"
)

// RunMetadata stores normalized metadata for one uploaded .refleks file.
type RunMetadata struct {
	Hash          string
	FileName      string
	ScenarioName  string
	SteamID       string
	SteamUsername string
	EpochMilli    int64
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

// SyncResult describes the outcome for a single sync operation.
type SyncResult struct {
	Hash           string `json:"hash"`
	AlreadyPresent bool   `json:"already_present"`
	Stored         bool   `json:"stored"`
	FileName       string `json:"file_name"`
	EpochMilli     int64  `json:"epoch_milli"`
	SizeBytes      int64  `json:"size_bytes"`
}

// ParsedRefleksFile represents verified binary metadata extracted from a .refleks file.
type ParsedRefleksFile struct {
	FileName      string
	ScenarioName  string
	SteamID       string
	SteamUsername string
	EpochMilli    int64
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

// RunsSort controls list ordering for run browsing endpoints.
type RunsSort string

const (
	RunsSortUploadedAtDesc RunsSort = "uploaded_at_desc"
	RunsSortUploadedAtAsc  RunsSort = "uploaded_at_asc"
	RunsSortEpochDesc      RunsSort = "epoch_desc"
	RunsSortEpochAsc       RunsSort = "epoch_asc"
	RunsSortScoreDesc      RunsSort = "score_desc"
	RunsSortScoreAsc       RunsSort = "score_asc"
)

// RunListRequest captures filters, sorting, and pagination for listing runs.
type RunListRequest struct {
	Limit         int
	Offset        int
	Sort          RunsSort
	ScenarioName  string
	SteamID       string
	SteamUsername string
	Query         string
	HasMouseTrace *bool
	MinScore      *float64
	MaxScore      *float64
	FromEpoch     *int64
	ToEpoch       *int64
}

// RunListItem contains one run card row for frontend listing.
type RunListItem struct {
	Hash          string    `json:"hash"`
	FileName      string    `json:"file_name"`
	ScenarioName  string    `json:"scenario_name"`
	SteamID       string    `json:"steam_id,omitempty"`
	SteamUsername string    `json:"steam_username,omitempty"`
	EpochMilli    int64     `json:"epoch_milli"`
	UploadedAt    time.Time `json:"uploaded_at"`
	SizeBytes     int64     `json:"size_bytes"`
	Score         *float64  `json:"score,omitempty"`
	Accuracy      *float64  `json:"accuracy,omitempty"`
	AvgTTKSeconds *float64  `json:"avg_ttk_seconds,omitempty"`
	DurationSecs  *float64  `json:"duration_seconds,omitempty"`
	SensCM360     *float64  `json:"sens_cm360,omitempty"`
	HasMouseTrace bool      `json:"has_mouse_trace"`
	AvgMouseSpeed *float64  `json:"avg_mouse_speed,omitempty"`
	MouseVID      string    `json:"mouse_vid,omitempty"`
	MousePID      string    `json:"mouse_pid,omitempty"`
}

// RunListResponse is the paginated API payload for run listing.
type RunListResponse struct {
	Runs       []RunListItem `json:"runs"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
	Count      int           `json:"count"`
	HasMore    bool          `json:"has_more"`
	NextOffset *int          `json:"next_offset,omitempty"`
}
