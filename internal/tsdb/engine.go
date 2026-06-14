package tsdb

import (
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	TTLDisabled time.Duration = -1
)

var (
	ErrInvalidTimeRange   = errors.New("invalid time range: start > end")
	ErrInvalidWindowSize  = errors.New("invalid window size: must be positive")
	ErrInvalidAggregator  = errors.New("invalid aggregator type")
	ErrInvalidTTL         = errors.New("invalid TTL: must be positive or TTLDisabled")
	ErrInvalidBatchSize   = errors.New("invalid batch size: must be positive")
	ErrInvalidInterval    = errors.New("invalid cleanup interval: must be positive")
	ErrEmptyTags          = errors.New("empty tags")
	ErrNilDataPoint       = errors.New("nil data point")
	ErrEngineClosed       = errors.New("engine is closed")
)

type DataPoint struct {
	Timestamp int64
	Value     float64
	Tags      map[string]string
}

type AggregatorType string

const (
	AggAvg   AggregatorType = "avg"
	AggMax   AggregatorType = "max"
	AggMin   AggregatorType = "min"
	AggSum   AggregatorType = "sum"
	AggCount AggregatorType = "count"
)

type AggregatedPoint struct {
	Timestamp int64
	Value     float64
	Count     int
}

type Config struct {
	TTL             time.Duration
	CleanupInterval time.Duration
	CleanupBatchSize int
}

func DefaultConfig() Config {
	return Config{
		TTL:             24 * time.Hour,
		CleanupInterval: 5 * time.Minute,
		CleanupBatchSize: 1000,
	}
}

type TSEngine struct {
	data       []*DataPoint
	dataMu     sync.RWMutex
	tagIndex   map[string]map[string][]int
	tagIndexMu sync.RWMutex
	ttl        time.Duration
	cleanupInt time.Duration
	batchSize  int
	stopCh     chan struct{}
	closed     bool
	closedMu   sync.RWMutex
	wg         sync.WaitGroup
}

func NewTSEngine() *TSEngine {
	e, err := NewTSEngineWithConfig(DefaultConfig())
	if err != nil {
		panic(err)
	}
	return e
}

func NewTSEngineWithConfig(cfg Config) (*TSEngine, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	e := &TSEngine{
		data:       make([]*DataPoint, 0),
		tagIndex:   make(map[string]map[string][]int),
		ttl:        cfg.TTL,
		cleanupInt: cfg.CleanupInterval,
		batchSize:  cfg.CleanupBatchSize,
		stopCh:     make(chan struct{}),
	}

	e.wg.Add(1)
	go e.cleanupLoop()

	return e, nil
}

func ValidateConfig(cfg Config) error {
	if cfg.TTL != TTLDisabled && cfg.TTL <= 0 {
		return ErrInvalidTTL
	}
	if cfg.CleanupInterval <= 0 {
		return ErrInvalidInterval
	}
	if cfg.CleanupBatchSize <= 0 {
		return ErrInvalidBatchSize
	}
	return nil
}

func (e *TSEngine) Write(points []*DataPoint) error {
	e.closedMu.RLock()
	defer e.closedMu.RUnlock()
	if e.closed {
		return ErrEngineClosed
	}

	if len(points) == 0 {
		return nil
	}

	validPoints := make([]*DataPoint, 0, len(points))
	for _, p := range points {
		if p == nil {
			return ErrNilDataPoint
		}
		if len(p.Tags) == 0 {
			return ErrEmptyTags
		}
		tagsCopy := make(map[string]string, len(p.Tags))
		for k, v := range p.Tags {
			tagsCopy[k] = v
		}
		validPoints = append(validPoints, &DataPoint{
			Timestamp: p.Timestamp,
			Value:     p.Value,
			Tags:      tagsCopy,
		})
	}

	e.dataMu.Lock()
	e.tagIndexMu.Lock()

	startIdx := len(e.data)
	e.data = append(e.data, validPoints...)

	for i, p := range validPoints {
		idx := startIdx + i
		for k, v := range p.Tags {
			if _, ok := e.tagIndex[k]; !ok {
				e.tagIndex[k] = make(map[string][]int)
			}
			e.tagIndex[k][v] = append(e.tagIndex[k][v], idx)
		}
	}

	sort.Slice(e.data, func(i, j int) bool {
		return e.data[i].Timestamp < e.data[j].Timestamp
	})

	e.rebuildTagIndex()

	e.tagIndexMu.Unlock()
	e.dataMu.Unlock()

	return nil
}

func (e *TSEngine) rebuildTagIndex() {
	e.tagIndex = make(map[string]map[string][]int)
	for i, p := range e.data {
		for k, v := range p.Tags {
			if _, ok := e.tagIndex[k]; !ok {
				e.tagIndex[k] = make(map[string][]int)
			}
			e.tagIndex[k][v] = append(e.tagIndex[k][v], i)
		}
	}
}

func (e *TSEngine) Query(start, end int64, tags map[string]string) ([]*DataPoint, error) {
	e.closedMu.RLock()
	defer e.closedMu.RUnlock()
	if e.closed {
		return nil, ErrEngineClosed
	}

	if start > end {
		return nil, ErrInvalidTimeRange
	}

	e.dataMu.RLock()
	defer e.dataMu.RUnlock()

	if len(e.data) == 0 {
		return []*DataPoint{}, nil
	}

	indices := e.filterByTags(tags)
	if len(indices) == 0 {
		return []*DataPoint{}, nil
	}

	result := make([]*DataPoint, 0)
	for _, idx := range indices {
		p := e.data[idx]
		if p.Timestamp >= start && p.Timestamp <= end {
			result = append(result, &DataPoint{
				Timestamp: p.Timestamp,
				Value:     p.Value,
				Tags:      e.copyTags(p.Tags),
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result, nil
}

func (e *TSEngine) filterByTags(tags map[string]string) []int {
	if len(tags) == 0 {
		indices := make([]int, len(e.data))
		for i := range e.data {
			indices[i] = i
		}
		return indices
	}

	e.tagIndexMu.RLock()
	defer e.tagIndexMu.RUnlock()

	var resultSet map[int]bool
	first := true

	for tagKey, tagValue := range tags {
		valueMap, ok := e.tagIndex[tagKey]
		if !ok {
			return []int{}
		}
		indices, ok := valueMap[tagValue]
		if !ok {
			return []int{}
		}

		currentSet := make(map[int]bool, len(indices))
		for _, idx := range indices {
			currentSet[idx] = true
		}

		if first {
			resultSet = currentSet
			first = false
		} else {
			intersection := make(map[int]bool)
			for idx := range resultSet {
				if currentSet[idx] {
					intersection[idx] = true
				}
			}
			resultSet = intersection
		}

		if len(resultSet) == 0 {
			return []int{}
		}
	}

	result := make([]int, 0, len(resultSet))
	for idx := range resultSet {
		result = append(result, idx)
	}
	return result
}

func (e *TSEngine) Downsample(start, end int64, windowSize time.Duration, agg AggregatorType, tags map[string]string) ([]*AggregatedPoint, error) {
	e.closedMu.RLock()
	defer e.closedMu.RUnlock()
	if e.closed {
		return nil, ErrEngineClosed
	}

	if start > end {
		return nil, ErrInvalidTimeRange
	}
	if windowSize <= 0 {
		return nil, ErrInvalidWindowSize
	}

	switch agg {
	case AggAvg, AggMax, AggMin, AggSum, AggCount:
	default:
		return nil, ErrInvalidAggregator
	}

	points, err := e.Query(start, end, tags)
	if err != nil {
		return nil, err
	}

	if len(points) == 0 {
		return []*AggregatedPoint{}, nil
	}

	windowMs := windowSize.Milliseconds()

	type bucket struct {
		startTs int64
		sum     float64
		count   int
		min     float64
		max     float64
	}

	buckets := make(map[int64]*bucket)

	for _, p := range points {
		bucketStart := (p.Timestamp / windowMs) * windowMs
		b, ok := buckets[bucketStart]
		if !ok {
			b = &bucket{
				startTs: bucketStart,
				min:     p.Value,
				max:     p.Value,
			}
			buckets[bucketStart] = b
		}
		b.sum += p.Value
		b.count++
		if p.Value < b.min {
			b.min = p.Value
		}
		if p.Value > b.max {
			b.max = p.Value
		}
	}

	result := make([]*AggregatedPoint, 0, len(buckets))
	for _, b := range buckets {
		ap := &AggregatedPoint{
			Timestamp: b.startTs,
			Count:     b.count,
		}
		switch agg {
		case AggAvg:
			ap.Value = b.sum / float64(b.count)
		case AggSum:
			ap.Value = b.sum
		case AggMin:
			ap.Value = b.min
		case AggMax:
			ap.Value = b.max
		case AggCount:
			ap.Value = float64(b.count)
		}
		result = append(result, ap)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result, nil
}

func (e *TSEngine) cleanupLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.cleanupInt)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.cleanupExpired()
		case <-e.stopCh:
			return
		}
	}
}

func (e *TSEngine) cleanupExpired() {
	e.closedMu.RLock()
	if e.closed {
		e.closedMu.RUnlock()
		return
	}
	e.closedMu.RUnlock()

	if e.ttl == TTLDisabled {
		return
	}

	cutoff := time.Now().Add(-e.ttl).UnixMilli()

	for {
		e.dataMu.Lock()
		e.tagIndexMu.Lock()

		if len(e.data) == 0 || e.data[0].Timestamp >= cutoff {
			e.tagIndexMu.Unlock()
			e.dataMu.Unlock()
			break
		}

		removeCount := 0
		for removeCount < e.batchSize && removeCount < len(e.data) {
			if e.data[removeCount].Timestamp >= cutoff {
				break
			}
			removeCount++
		}

		if removeCount == 0 {
			e.tagIndexMu.Unlock()
			e.dataMu.Unlock()
			break
		}

		e.data = e.data[removeCount:]
		e.rebuildTagIndex()

		e.tagIndexMu.Unlock()
		e.dataMu.Unlock()

		if removeCount < e.batchSize {
			break
		}
	}
}

func (e *TSEngine) Close() {
	e.closedMu.Lock()
	if e.closed {
		e.closedMu.Unlock()
		return
	}
	e.closed = true
	close(e.stopCh)
	e.closedMu.Unlock()

	e.wg.Wait()
}

func (e *TSEngine) Count() int {
	e.dataMu.RLock()
	defer e.dataMu.RUnlock()
	return len(e.data)
}

func (e *TSEngine) copyTags(tags map[string]string) map[string]string {
	result := make(map[string]string, len(tags))
	for k, v := range tags {
		result[k] = v
	}
	return result
}

func (e *TSEngine) ForceCleanup() {
	e.cleanupExpired()
}

func (e *TSEngine) GetTTL() time.Duration {
	return e.ttl
}
