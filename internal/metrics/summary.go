package metrics

import (
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

const defaultSummaryMaxSamples = 1024

type summary struct {
	guard     snapshotGuard
	mu        sync.RWMutex
	name      string
	labels    Labels
	quantiles []float64
	reservoir []float64
	capacity  int
	count     uint64
	sum       float64
	rand      *rand.Rand
}

var _ snapshotProtected = (*summary)(nil)

func (s *summary) snapshotGuardPtr() *snapshotGuard { return &s.guard }

func newSummary(name string, labels Labels, quantiles []float64, guard snapshotGuard) *summary {
	sortedQuantiles := make([]float64, len(quantiles))
	copy(sortedQuantiles, quantiles)
	sort.Float64s(sortedQuantiles)

	s := &summary{
		guard:     guard,
		name:      name,
		labels:    make(Labels, len(labels)),
		quantiles: sortedQuantiles,
		reservoir: make([]float64, 0, defaultSummaryMaxSamples),
		capacity:  defaultSummaryMaxSamples,
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	copy(s.labels, labels)
	return s
}

func (s *summary) Name() string {
	return s.name
}

func (s *summary) Type() MetricType {
	return SummaryType
}

func (s *summary) Labels() Labels {
	return s.labels
}

func (s *summary) Observe(value float64) {
	s.guard.write(func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.count++
		s.sum += value

		if len(s.reservoir) < s.capacity {
			s.reservoir = append(s.reservoir, value)
		} else {
			j := s.rand.Intn(int(s.count))
			if j < s.capacity {
				s.reservoir[j] = value
			}
		}
	})
}

func (s *summary) Quantiles() []QuantileValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.reservoir) == 0 {
		result := make([]QuantileValue, len(s.quantiles))
		for i, q := range s.quantiles {
			result[i] = QuantileValue{Quantile: q, Value: 0}
		}
		return result
	}

	sorted := make([]float64, len(s.reservoir))
	copy(sorted, s.reservoir)
	sort.Float64s(sorted)

	result := make([]QuantileValue, len(s.quantiles))
	for i, q := range s.quantiles {
		result[i] = QuantileValue{
			Quantile: q,
			Value:    quantile(sorted, q),
		}
	}
	return result
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}

	pos := q * float64(len(sorted)-1)
	idx := int(math.Floor(pos))
	frac := pos - float64(idx)

	if idx+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	return sorted[idx] + frac*(sorted[idx+1]-sorted[idx])
}

func (s *summary) Count() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

func (s *summary) Sum() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sum
}

func (s *summary) Snapshot() MetricValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	quantileValues := make([]QuantileValue, len(s.quantiles))
	if len(s.reservoir) > 0 {
		sorted := make([]float64, len(s.reservoir))
		copy(sorted, s.reservoir)
		sort.Float64s(sorted)

		for i, q := range s.quantiles {
			quantileValues[i] = QuantileValue{
				Quantile: q,
				Value:    quantile(sorted, q),
			}
		}
	} else {
		for i, q := range s.quantiles {
			quantileValues[i] = QuantileValue{Quantile: q, Value: 0}
		}
	}

	return MetricValue{
		Name:      s.name,
		Type:      SummaryType,
		Labels:    s.labels,
		Timestamp: time.Now(),
		Quantiles: quantileValues,
		Count:     s.count,
		Sum:       s.sum,
		Value:     s.sum,
	}
}

func DefaultQuantiles() []float64 {
	return []float64{0.5, 0.9, 0.99}
}
