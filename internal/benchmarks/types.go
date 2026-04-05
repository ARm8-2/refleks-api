package benchmarks

type ListView string

const (
	ListViewFull     ListView = "full"
	ListViewProgress ListView = "progress"
)

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
	RankColors         map[string]string   `json:"rankColors,omitempty"`
	Ranks              []BenchmarkRank     `json:"ranks,omitempty"`
	Scenarios          []BenchmarkScenario `json:"scenarios,omitempty"`
	Categories         []BenchmarkCategory `json:"categories"`
}

// BenchmarkRank models one ordered rank definition for a difficulty.
type BenchmarkRank struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// BenchmarkScenario models one scenario link on benchmark hierarchy nodes.
type BenchmarkScenario struct {
	ScenarioName   string    `json:"scenarioName"`
	RankThresholds []float64 `json:"rankThresholds,omitempty"`
}

// BenchmarkCategory models one benchmark category.
type BenchmarkCategory struct {
	CategoryName  string                 `json:"categoryName"`
	Color         string                 `json:"color,omitempty"`
	Scenarios     []BenchmarkScenario    `json:"scenarios,omitempty"`
	Subcategories []BenchmarkSubcategory `json:"subcategories"`
}

// BenchmarkSubcategory models one benchmark subcategory.
type BenchmarkSubcategory struct {
	SubcategoryName string              `json:"subcategoryName"`
	ScenarioCount   int                 `json:"scenarioCount"`
	Color           string              `json:"color,omitempty"`
	Scenarios       []BenchmarkScenario `json:"scenarios,omitempty"`
}

// ListRequest captures benchmark listing filters.
type ListRequest struct {
	Query string
	View  ListView
}

// ListResponse is the benchmark list API response payload.
type ListResponse struct {
	Benchmarks []Benchmark `json:"benchmarks"`
	Count      int         `json:"count"`
}
