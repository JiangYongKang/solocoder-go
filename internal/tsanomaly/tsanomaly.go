package tsanomaly

import (
	"container/list"
	"errors"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrInvalidWindowSize      = errors.New("tsanomaly: invalid window size, must be positive")
	ErrInvalidStdDevFactor    = errors.New("tsanomaly: invalid standard deviation factor, must be non-negative")
	ErrInvalidPeriodLength    = errors.New("tsanomaly: invalid period length, must be positive when seasonal mode is enabled")
	ErrInvalidPeriodSlot      = errors.New("tsanomaly: invalid period slot, must be positive duration when seasonal mode is enabled")
	ErrInvalidMinSamples      = errors.New("tsanomaly: invalid min samples, must be positive and <= window size")
	ErrNilDataPoint           = errors.New("tsanomaly: nil data point")
	ErrDetectorClosed         = errors.New("tsanomaly: detector is closed")
	ErrInvalidDirection       = errors.New("tsanomaly: invalid deviation direction")
)

type DeviationDirection int

const (
	DirectionBoth DeviationDirection = iota
	DirectionUp
	DirectionDown
)

func (d DeviationDirection) String() string {
	switch d {
	case DirectionBoth:
		return "both"
	case DirectionUp:
		return "up"
	case DirectionDown:
		return "down"
	default:
		return "unknown"
	}
}

type Config struct {
	WindowSize        int
	StdDevFactor      float64
	MinSamples        int
	EnableSeasonal    bool
	PeriodLength      int
	PeriodSlot        time.Duration
	SeasonalEpoch     time.Time
	Direction         DeviationDirection
	MaxAnomalyHistory int
}

func DefaultConfig() Config {
	return Config{
		WindowSize:        100,
		StdDevFactor:      3.0,
		MinSamples:        10,
		EnableSeasonal:    false,
		PeriodLength:      0,
		PeriodSlot:        0,
		SeasonalEpoch:     time.Unix(0, 0).UTC(),
		Direction:         DirectionBoth,
		MaxAnomalyHistory: 1000,
	}
}

func ValidateConfig(cfg Config) error {
	if cfg.WindowSize <= 0 {
		return ErrInvalidWindowSize
	}
	if cfg.StdDevFactor < 0 {
		return ErrInvalidStdDevFactor
	}
	if cfg.MinSamples <= 0 || cfg.MinSamples > cfg.WindowSize {
		return ErrInvalidMinSamples
	}
	if cfg.EnableSeasonal {
		if cfg.PeriodLength <= 0 {
			return ErrInvalidPeriodLength
		}
		if cfg.PeriodSlot <= 0 {
			return ErrInvalidPeriodSlot
		}
	}
	switch cfg.Direction {
	case DirectionBoth, DirectionUp, DirectionDown:
	default:
		return ErrInvalidDirection
	}
	return nil
}

type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

type AnomalySeverity string

const (
	SeverityWarning  AnomalySeverity = "warning"
	SeverityCritical AnomalySeverity = "critical"
)

type AnomalyEvent struct {
	Timestamp      time.Time
	ActualValue    float64
	BaselineValue  float64
	StdDev         float64
	Deviation      float64
	DeviationRatio float64
	Threshold      float64
	Direction      DeviationDirection
	Severity       AnomalySeverity
	SeasonalIndex  int
}

type windowStats struct {
	values *list.List
	sum    float64
	sumSq  float64
}

func newWindowStats() *windowStats {
	return &windowStats{
		values: list.New(),
	}
}

func cloneWindowStats(src *windowStats) *windowStats {
	dst := newWindowStats()
	dst.sum = src.sum
	dst.sumSq = src.sumSq
	for e := src.values.Front(); e != nil; e = e.Next() {
		dst.values.PushBack(e.Value.(float64))
	}
	return dst
}

func (ws *windowStats) count() int {
	return ws.values.Len()
}

func (ws *windowStats) mean() float64 {
	if ws.count() == 0 {
		return 0
	}
	return ws.sum / float64(ws.count())
}

func (ws *windowStats) variance() float64 {
	n := ws.count()
	if n < 2 {
		return 0
	}
	mean := ws.mean()
	return (ws.sumSq/float64(n) - mean*mean) * float64(n) / float64(n-1)
}

func (ws *windowStats) stdDev() float64 {
	return math.Sqrt(ws.variance())
}

func (ws *windowStats) add(value float64, maxSize int) {
	ws.values.PushBack(value)
	ws.sum += value
	ws.sumSq += value * value
	for ws.values.Len() > maxSize {
		e := ws.values.Front()
		oldVal := e.Value.(float64)
		ws.sum -= oldVal
		ws.sumSq -= oldVal * oldVal
		ws.values.Remove(e)
	}
}

func (ws *windowStats) reset() {
	ws.values.Init()
	ws.sum = 0
	ws.sumSq = 0
}

type Detector struct {
	mu            sync.RWMutex
	cfg           Config
	globalStats   *windowStats
	seasonalStats []*windowStats
	anomalies     []*AnomalyEvent
	pointCount    int64
	closed        bool
}

func NewDetector(cfg Config) (*Detector, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	d := &Detector{
		cfg:         cfg,
		globalStats: newWindowStats(),
		anomalies:   make([]*AnomalyEvent, 0),
	}
	if cfg.EnableSeasonal {
		d.seasonalStats = make([]*windowStats, cfg.PeriodLength)
		for i := 0; i < cfg.PeriodLength; i++ {
			d.seasonalStats[i] = newWindowStats()
		}
	}
	return d, nil
}

func NewDetectorWithDefault() *Detector {
	d, err := NewDetector(DefaultConfig())
	if err != nil {
		panic(err)
	}
	return d
}

func (d *Detector) Config() Config {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cfg
}

func (d *Detector) computeSeasonalIndex(timestamp time.Time) int {
	slotCount := int64(timestamp.Sub(d.cfg.SeasonalEpoch) / d.cfg.PeriodSlot)
	idx := int(slotCount % int64(d.cfg.PeriodLength))
	if idx < 0 {
		idx += d.cfg.PeriodLength
	}
	return idx
}

func mergeWindowStats(dst *windowStats, src *windowStats, maxSize int) {
	var srcVals []float64
	for e := src.values.Front(); e != nil; e = e.Next() {
		srcVals = append(srcVals, e.Value.(float64))
	}
	sort.Slice(srcVals, func(i, j int) bool {
		return i < j
	})
	for _, v := range srcVals {
		dst.add(v, maxSize)
	}
}

func (d *Detector) UpdateConfig(cfg Config) error {
	if err := ValidateConfig(cfg); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	oldCfg := d.cfg
	oldSeasonal := d.seasonalStats
	oldEnable := oldCfg.EnableSeasonal
	newEnable := cfg.EnableSeasonal

	d.cfg = cfg

	newSeasonalLen := 0
	if newEnable {
		newSeasonalLen = cfg.PeriodLength
	}

	if !oldEnable && !newEnable {
		d.seasonalStats = nil
		return nil
	}

	if oldEnable && !newEnable {
		if d.globalStats.count() > cfg.WindowSize {
			for d.globalStats.count() > cfg.WindowSize {
				e := d.globalStats.values.Front()
				oldVal := e.Value.(float64)
				d.globalStats.sum -= oldVal
				d.globalStats.sumSq -= oldVal * oldVal
				d.globalStats.values.Remove(e)
			}
		}
		d.seasonalStats = nil
		return nil
	}

	if !oldEnable && newEnable {
		newSeasonal := make([]*windowStats, newSeasonalLen)
		for i := 0; i < newSeasonalLen; i++ {
			newSeasonal[i] = newWindowStats()
		}
		if d.globalStats.count() > 0 {
			globalClone := cloneWindowStats(d.globalStats)
			idx := 0
			for e := globalClone.values.Front(); e != nil; e = e.Next() {
				newSeasonal[idx%newSeasonalLen].add(e.Value.(float64), cfg.WindowSize)
				idx++
			}
		}
		d.seasonalStats = newSeasonal
		return nil
	}

	oldLen := oldCfg.PeriodLength
	newLen := cfg.PeriodLength
	newSeasonal := make([]*windowStats, newLen)
	for i := 0; i < newLen; i++ {
		newSeasonal[i] = newWindowStats()
	}

	if oldLen > 0 && len(oldSeasonal) > 0 {
		if oldLen == newLen && oldCfg.PeriodSlot == cfg.PeriodSlot && oldCfg.SeasonalEpoch.Equal(cfg.SeasonalEpoch) {
			for i := 0; i < newLen; i++ {
				if i < len(oldSeasonal) && oldSeasonal[i] != nil {
					newSeasonal[i] = cloneWindowStats(oldSeasonal[i])
					for newSeasonal[i].count() > cfg.WindowSize {
						e := newSeasonal[i].values.Front()
						oldVal := e.Value.(float64)
						newSeasonal[i].sum -= oldVal
						newSeasonal[i].sumSq -= oldVal * oldVal
						newSeasonal[i].values.Remove(e)
					}
				}
			}
		} else {
			for oldIdx := 0; oldIdx < oldLen && oldIdx < len(oldSeasonal); oldIdx++ {
				if oldSeasonal[oldIdx] == nil {
					continue
				}
				newIdx := oldIdx % newLen
				mergeWindowStats(newSeasonal[newIdx], oldSeasonal[oldIdx], cfg.WindowSize)
			}
		}
	}

	d.seasonalStats = newSeasonal
	return nil
}

func (d *Detector) addLocked(point *DataPoint) (*AnomalyEvent, error) {
	if point == nil {
		return nil, ErrNilDataPoint
	}
	if d.closed {
		return nil, ErrDetectorClosed
	}

	d.pointCount++

	var seasonalIdx int
	var stats *windowStats
	if d.cfg.EnableSeasonal {
		seasonalIdx = d.computeSeasonalIndex(point.Timestamp)
		stats = d.seasonalStats[seasonalIdx]
	} else {
		stats = d.globalStats
	}

	var event *AnomalyEvent
	if stats.count() >= d.cfg.MinSamples {
		mean := stats.mean()
		stdDev := stats.stdDev()
		threshold := d.cfg.StdDevFactor * stdDev
		deviation := point.Value - mean
		absDeviation := math.Abs(deviation)

		var isAnomaly bool
		var direction DeviationDirection

		switch d.cfg.Direction {
		case DirectionBoth:
			isAnomaly = absDeviation > threshold
			if deviation > 0 {
				direction = DirectionUp
			} else {
				direction = DirectionDown
			}
		case DirectionUp:
			isAnomaly = deviation > threshold
			direction = DirectionUp
		case DirectionDown:
			isAnomaly = deviation < -threshold
			direction = DirectionDown
		}

		if isAnomaly {
			var ratio float64
			if stdDev > 0 {
				ratio = absDeviation / stdDev
			} else {
				if mean != 0 {
					ratio = absDeviation / math.Abs(mean)
				} else {
					ratio = 0
				}
			}

			severity := SeverityWarning
			if ratio >= 2*d.cfg.StdDevFactor {
				severity = SeverityCritical
			}

			event = &AnomalyEvent{
				Timestamp:      point.Timestamp,
				ActualValue:    point.Value,
				BaselineValue:  mean,
				StdDev:         stdDev,
				Deviation:      deviation,
				DeviationRatio: ratio,
				Threshold:      threshold,
				Direction:      direction,
				Severity:       severity,
				SeasonalIndex:  seasonalIdx,
			}

			d.anomalies = append(d.anomalies, event)
			if d.cfg.MaxAnomalyHistory > 0 && len(d.anomalies) > d.cfg.MaxAnomalyHistory {
				removeCount := len(d.anomalies) - d.cfg.MaxAnomalyHistory
				d.anomalies = d.anomalies[removeCount:]
			}
			sort.SliceStable(d.anomalies, func(i, j int) bool {
				return d.anomalies[i].Timestamp.Before(d.anomalies[j].Timestamp)
			})
		}
	}

	d.globalStats.add(point.Value, d.cfg.WindowSize)
	if d.cfg.EnableSeasonal {
		stats.add(point.Value, d.cfg.WindowSize)
	}

	return event, nil
}

func (d *Detector) Add(point *DataPoint) (*AnomalyEvent, error) {
	if point == nil {
		return nil, ErrNilDataPoint
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.addLocked(point)
}

func (d *Detector) BatchAdd(points []*DataPoint) ([]*AnomalyEvent, error) {
	if len(points) == 0 {
		return nil, nil
	}
	for _, p := range points {
		if p == nil {
			return nil, ErrNilDataPoint
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil, ErrDetectorClosed
	}

	events := make([]*AnomalyEvent, 0, len(points))
	for _, p := range points {
		event, err := d.addLocked(p)
		if err != nil {
			return events, err
		}
		if event != nil {
			events = append(events, event)
		}
	}
	return events, nil
}

func (d *Detector) GetBaseline() (mean, stdDev float64, count int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	mean = d.globalStats.mean()
	stdDev = d.globalStats.stdDev()
	count = d.globalStats.count()
	return
}

func (d *Detector) GetSeasonalBaseline(index int) (mean, stdDev float64, count int, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if !d.cfg.EnableSeasonal {
		return 0, 0, 0, errors.New("tsanomaly: seasonal mode is not enabled")
	}
	if index < 0 || index >= d.cfg.PeriodLength {
		return 0, 0, 0, errors.New("tsanomaly: seasonal index out of range")
	}
	stats := d.seasonalStats[index]
	mean = stats.mean()
	stdDev = stats.stdDev()
	count = stats.count()
	return
}

type AnomalyQuery struct {
	StartTime *time.Time
	EndTime   *time.Time
	Direction *DeviationDirection
	Severity  *AnomalySeverity
	Limit     int
}

func (d *Detector) GetAnomalies(query *AnomalyQuery) []*AnomalyEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*AnomalyEvent, 0, len(d.anomalies))
	for _, a := range d.anomalies {
		if query != nil {
			if query.StartTime != nil && a.Timestamp.Before(*query.StartTime) {
				continue
			}
			if query.EndTime != nil && a.Timestamp.After(*query.EndTime) {
				continue
			}
			if query.Direction != nil && a.Direction != *query.Direction {
				continue
			}
			if query.Severity != nil && a.Severity != *query.Severity {
				continue
			}
		}
		result = append(result, a)
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	if query != nil && query.Limit > 0 && len(result) > query.Limit {
		result = result[len(result)-query.Limit:]
	}

	return result
}

func (d *Detector) AnomalyCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.anomalies)
}

func (d *Detector) PointCount() int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.pointCount
}

func (d *Detector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.globalStats.reset()
	if d.cfg.EnableSeasonal {
		for i := range d.seasonalStats {
			d.seasonalStats[i].reset()
		}
	}
	d.anomalies = make([]*AnomalyEvent, 0)
	d.pointCount = 0
}

func (d *Detector) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
}

func (d *Detector) IsClosed() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.closed
}
