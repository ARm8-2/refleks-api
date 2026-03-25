package runsync

import "time"

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
	ChecksumHex   string
	UploadedAt    time.Time
	Compression   uint8
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
	Compression   uint8
	FormatVersion uint8
	Checksum      uint64
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
