package benchfrm

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

type benchmarker struct {
	mu           sync.RWMutex
	groups         []*BenchmarkGroup
	lastResults    []GroupStatistics
	baselineStore  BaselineStore
	reporter       Reporter
}

func NewBenchmarker() Benchmarker {
	return &benchmarker{
		groups: make([]*BenchmarkGroup, 0),
	}
}

func (b *benchmarker) AddGroup(name string, fn BenchmarkFunc, opts ...RunOption) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if fn == nil {
		panic(ErrNilFunction)
	}

	for _, g := range b.groups {
		if g.name == name {
			panic(ErrDuplicateGroupName)
		}
	}

	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := cfg.Validate(); err != nil {
		panic(err)
	}

	b.groups = append(b.groups, &BenchmarkGroup{
		name:   name,
		fn:     fn,
		config: cfg,
	})
}

func (b *benchmarker) RunAll() ([]GroupStatistics, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.groups) == 0 {
		return nil, ErrNoGroupsRegistered
	}

	results := make([]GroupStatistics, 0, len(b.groups))

	for _, group := range b.groups {
		stats, err := b.runGroup(group)
		if err != nil {
			return nil, err
		}
		results = append(results, stats)
	}

	b.lastResults = results
	return results, nil
}

func (b *benchmarker) runGroup(group *BenchmarkGroup) (GroupStatistics, error) {
	cfg := group.config

	for i := 0; i < cfg.WarmupIterations; i++ {
		_ = group.fn()
	}

	runResults := make([]RunResult, 0, cfg.Iterations)
	var firstErr error

	for i := 0; i < cfg.Iterations; i++ {
		result := b.runSingleWithTimeout(group, cfg.CollectMemory, cfg.Timeout)
		if result.Error != nil {
			if firstErr == nil {
				firstErr = result.Error
			}
			continue
		}
		runResults = append(runResults, result)
	}

	if len(runResults) == 0 {
		if firstErr != nil {
			return GroupStatistics{}, fmt.Errorf("%w: %w", ErrGroupEmptyResult, firstErr)
		}
		return GroupStatistics{}, ErrGroupEmptyResult
	}

	return calculateStatistics(group.name, runResults), nil
}

func (b *benchmarker) runSingle(group *BenchmarkGroup, collectMem bool) RunResult {
	var result RunResult

	if collectMem {
		var startMem, endMem runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&startMem)

		start := time.Now()
		result.Error = group.fn()
		result.Duration = time.Since(start)

		runtime.ReadMemStats(&endMem)
		result.AllocBytes = endMem.TotalAlloc - startMem.TotalAlloc
		result.AllocCount = endMem.Mallocs - startMem.Mallocs
	} else {
		start := time.Now()
		result.Error = group.fn()
		result.Duration = time.Since(start)
	}

	return result
}

func (b *benchmarker) runSingleWithTimeout(group *BenchmarkGroup, collectMem bool, timeout time.Duration) RunResult {
	if timeout <= 0 {
		return b.runSingle(group, collectMem)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan RunResult, 1)

	go func() {
		done <- b.runSingle(group, collectMem)
	}()

	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		return RunResult{Error: ctx.Err()}
	}
}

func calculateStatistics(name string, results []RunResult) GroupStatistics {
	if len(results) == 0 {
		return GroupStatistics{Name: name}
	}

	var totalDuration time.Duration
	var totalAllocBytes uint64
	var totalAllocCount uint64

	minDuration := results[0].Duration
	maxDuration := results[0].Duration

	for _, r := range results {
		totalDuration += r.Duration
		totalAllocBytes += r.AllocBytes
		totalAllocCount += r.AllocCount

		if r.Duration < minDuration {
			minDuration = r.Duration
		}
		if r.Duration > maxDuration {
			maxDuration = r.Duration
		}
	}

	n := float64(len(results))
	meanDuration := totalDuration / time.Duration(len(results))
	meanAllocBytes := totalAllocBytes / uint64(len(results))
	meanAllocCount := totalAllocCount / uint64(len(results))

	var variance float64
	for _, r := range results {
		diff := float64(r.Duration - meanDuration)
		variance += diff * diff
	}
	variance /= n
	stdDev := time.Duration(math.Sqrt(variance))

	return GroupStatistics{
		Name:           name,
		Iterations:     len(results),
		MeanDuration:   meanDuration,
		MinDuration:    minDuration,
		MaxDuration:    maxDuration,
		StdDevDuration: stdDev,
		MeanAllocBytes: meanAllocBytes,
		MeanAllocCount: meanAllocCount,
	}
}

func (b *benchmarker) Compare(baseline string) (ComparisonReport, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.lastResults) == 0 {
		return ComparisonReport{}, ErrNoGroupsRegistered
	}

	var baselineStats *GroupStatistics
	for i := range b.lastResults {
		if b.lastResults[i].Name == baseline {
			baselineStats = &b.lastResults[i]
			break
		}
	}

	if baselineStats == nil {
		return ComparisonReport{}, ErrGroupNotFound
	}

	items := make([]ComparisonItem, 0, len(b.lastResults))

	for _, stats := range b.lastResults {
		var vsBaselinePct float64
		var allocBytesPct, allocCountPct float64

		if baselineStats.MeanDuration > 0 {
			vsBaselinePct = float64(stats.MeanDuration-baselineStats.MeanDuration) / float64(baselineStats.MeanDuration) * 100
		}
		if baselineStats.MeanAllocBytes > 0 {
			allocBytesPct = float64(stats.MeanAllocBytes-baselineStats.MeanAllocBytes) / float64(baselineStats.MeanAllocBytes) * 100
		}
		if baselineStats.MeanAllocCount > 0 {
			allocCountPct = float64(stats.MeanAllocCount-baselineStats.MeanAllocCount) / float64(baselineStats.MeanAllocCount) * 100
		}

		items = append(items, ComparisonItem{
			Group:            stats.Name,
			MeanDuration:     stats.MeanDuration,
			MeanAllocBytes:   stats.MeanAllocBytes,
			MeanAllocCount:   stats.MeanAllocCount,
			VsBaselinePct:   vsBaselinePct,
			AllocBytesPct:    allocBytesPct,
			AllocCountPct:    allocCountPct,
		})
	}

	return ComparisonReport{
		Baseline:    baseline,
		Items:       items,
		GeneratedAt: time.Now(),
	}, nil
}

func (b *benchmarker) CheckRegression(thresholdPct float64) (RegressionReport, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if thresholdPct <= 0 {
		return RegressionReport{}, ErrInvalidThreshold
	}

	if b.baselineStore == nil {
		return RegressionReport{}, ErrNoBaselineStore
	}

	if len(b.lastResults) == 0 {
		return RegressionReport{}, ErrNoGroupsRegistered
	}

	var checks []RegressionCheck
	isRegression := false

	for _, current := range b.lastResults {
		baseline, found, err := b.baselineStore.Load(current.Name)
		if err != nil {
			return RegressionReport{}, err
		}
		if !found {
			return RegressionReport{}, fmt.Errorf("%w: %s", ErrBaselineNotFound, current.Name)
		}

		durationCheck := checkMetric("MeanDuration",
			float64(current.MeanDuration),
			float64(baseline.MeanDuration),
			thresholdPct)
		allocBytesCheck := checkMetric("MeanAllocBytes",
			float64(current.MeanAllocBytes),
			float64(baseline.MeanAllocBytes),
			thresholdPct)
		allocCountCheck := checkMetric("MeanAllocCount",
			float64(current.MeanAllocCount),
			float64(baseline.MeanAllocCount),
			thresholdPct)

		checks = append(checks, durationCheck, allocBytesCheck, allocCountCheck)

		if durationCheck.IsDegraded || allocBytesCheck.IsDegraded || allocCountCheck.IsDegraded {
			isRegression = true
		}
	}

	return RegressionReport{
		IsRegression: isRegression,
		Checks:       checks,
		GeneratedAt: time.Now(),
	}, nil
}

func checkMetric(name string, current, baseline, thresholdPct float64) RegressionCheck {
	var degradationPct float64
	if baseline > 0 {
		degradationPct = (current - baseline) / baseline * 100
	}

	return RegressionCheck{
		MetricName:    name,
		CurrentValue:  current,
		BaselineValue: baseline,
		DegradationPct: degradationPct,
		ThresholdPct:  thresholdPct,
		IsDegraded:    degradationPct > thresholdPct,
	}
}

func (b *benchmarker) SetBaselineStore(store BaselineStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.baselineStore = store
}

func (b *benchmarker) SaveBaseline() error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.baselineStore == nil {
		return ErrNoBaselineStore
	}

	if len(b.lastResults) == 0 {
		return ErrNoGroupsRegistered
	}

	for _, stats := range b.lastResults {
		if err := b.baselineStore.Save(stats.Name, stats); err != nil {
			return err
		}
	}

	return nil
}

func (b *benchmarker) LoadBaseline() (map[string]GroupStatistics, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.baselineStore == nil {
		return nil, ErrNoBaselineStore
	}

	baselines := make(map[string]GroupStatistics)
	for _, group := range b.groups {
		stats, found, err := b.baselineStore.Load(group.name)
		if err != nil {
			return nil, err
		}
		if found {
			baselines[group.name] = stats
		}
	}

	return baselines, nil
}

func (b *benchmarker) SetReporter(reporter Reporter) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reporter = reporter
}
