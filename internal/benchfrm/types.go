package benchfrm

import "time"

type BenchmarkFunc func() error

type RunResult struct {
	Duration     time.Duration
	AllocBytes   uint64
	AllocCount   uint64
	Error        error
}

type GroupStatistics struct {
	Name            string
	Iterations      int
	MeanDuration    time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	StdDevDuration  time.Duration
	MeanAllocBytes  uint64
	MeanAllocCount  uint64
	AllocsPerOp     float64
	BytesPerOp      float64
}

type ComparisonItem struct {
	Group           string
	MeanDuration    time.Duration
	MeanAllocBytes  uint64
	MeanAllocCount  uint64
	VsBaselinePct   float64
}

type ComparisonReport struct {
	Baseline    string
	Items       []ComparisonItem
	GeneratedAt time.Time
}

type RegressionCheck struct {
	MetricName    string
	CurrentValue  float64
	BaselineValue float64
	DegradationPct float64
	ThresholdPct  float64
	IsDegraded    bool
}

type RegressionReport struct {
	IsRegression    bool
	Checks          []RegressionCheck
	GeneratedAt     time.Time
}

type BenchmarkGroup struct {
	name     string
	fn       BenchmarkFunc
	config   RunConfig
}

type BaselineStore interface {
	Save(groupName string, stats GroupStatistics) error
	Load(groupName string) (GroupStatistics, bool, error)
}

type Reporter interface {
	Report(stats []GroupStatistics) string
	ReportComparison(report ComparisonReport) string
	ReportRegression(report RegressionReport) string
}

type Benchmarker interface {
	AddGroup(name string, fn BenchmarkFunc, opts ...RunOption)
	RunAll() ([]GroupStatistics, error)
	Compare(baseline string) (ComparisonReport, error)
	CheckRegression(thresholdPct float64) (RegressionReport, error)
	SaveBaseline() error
	LoadBaseline() (map[string]GroupStatistics, error)
	SetBaselineStore(store BaselineStore)
	SetReporter(reporter Reporter)
}
