package benchmarks

// Benchmark models one benchmark document served to clients.
type Benchmark struct {
	BenchmarkName   string                `json:"benchmarkName"`
	RankCalculation string                `json:"rankCalculation"`
	Abbreviation    string                `json:"abbreviation"`
	Color           string                `json:"color"`
	SpreadsheetURL  string                `json:"spreadsheetURL"`
	DateAdded       *string               `json:"dateAdded,omitempty"`
	Difficulties    []BenchmarkDifficulty `json:"difficulties"`
}

// BenchmarkDifficulty models one difficulty under a benchmark.
type BenchmarkDifficulty struct {
	DifficultyName     string              `json:"difficultyName"`
	KovaaksBenchmarkID int64               `json:"kovaaksBenchmarkId"`
	Sharecode          string              `json:"sharecode"`
	RankColors         map[string]string   `json:"rankColors"`
	Categories         []BenchmarkCategory `json:"categories"`
}

// BenchmarkCategory models one benchmark category.
type BenchmarkCategory struct {
	CategoryName  string                 `json:"categoryName"`
	Color         string                 `json:"color,omitempty"`
	Subcategories []BenchmarkSubcategory `json:"subcategories"`
}

// BenchmarkSubcategory models one benchmark subcategory.
type BenchmarkSubcategory struct {
	SubcategoryName string `json:"subcategoryName"`
	ScenarioCount   int    `json:"scenarioCount"`
	Color           string `json:"color,omitempty"`
}

// ListRequest captures benchmark listing filters.
type ListRequest struct {
	Query           string
	IncludeInactive bool
}

// ListResponse is the benchmark list API response payload.
type ListResponse struct {
	Benchmarks []Benchmark `json:"benchmarks"`
	Count      int         `json:"count"`
}
