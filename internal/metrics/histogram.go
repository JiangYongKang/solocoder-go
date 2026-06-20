package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

type histogram struct {
	guard   snapshotGuard
	mu      sync.RWMutex
	name    string
	labels  Labels
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
}

func (h *histogram) snapshotGuardPtr() *snapshotGuard { return &h.guard }

func newHistogram(name string, labels Labels, buckets []float64, guard snapshotGuard) *histogram {
	sortedBuckets := make([]float64, len(buckets))
	copy(sortedBuckets, buckets)
	sort.Float64s(sortedBuckets)

	h := &histogram{
		guard:   guard,
		name:    name,
		labels:  make(Labels, len(labels)),
		buckets: sortedBuckets,
		counts:  make([]uint64, len(sortedBuckets)+1),
	}
	copy(h.labels, labels)
	return h
}

func (h *histogram) Name() string {
	return h.name
}

func (h *histogram) Type() MetricType {
	return HistogramType
}

func (h *histogram) Labels() Labels {
	return h.labels
}

func (h *histogram) Observe(value float64) {
	h.guard.write(func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		idx := sort.SearchFloat64s(h.buckets, value)
		if idx < len(h.buckets) {
			h.counts[idx]++
		} else {
			h.counts[len(h.buckets)]++
		}
		h.count++
		h.sum += value
	})
}

func (h *histogram) Buckets() []BucketValue {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]BucketValue, len(h.buckets)+1)
	var cumulative uint64
	for i, bound := range h.buckets {
		cumulative += h.counts[i]
		result[i] = BucketValue{
			UpperBound: bound,
			Count:      cumulative,
		}
	}
	cumulative += h.counts[len(h.buckets)]
	result[len(h.buckets)] = BucketValue{
		UpperBound: math.Inf(1),
		Count:      cumulative,
	}
	return result
}

func (h *histogram) Count() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}

func (h *histogram) Sum() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sum
}

func (h *histogram) Snapshot() MetricValue {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buckets := make([]BucketValue, len(h.buckets)+1)
	var cumulative uint64
	for i, bound := range h.buckets {
		cumulative += h.counts[i]
		buckets[i] = BucketValue{
			UpperBound: bound,
			Count:      cumulative,
		}
	}
	cumulative += h.counts[len(h.buckets)]
	buckets[len(h.buckets)] = BucketValue{
		UpperBound: math.Inf(1),
		Count:      cumulative,
	}

	return MetricValue{
		Name:      h.name,
		Type:      HistogramType,
		Labels:    h.labels,
		Timestamp: time.Now(),
		Buckets:   buckets,
		Count:     h.count,
		Sum:       h.sum,
		Value:     h.sum,
	}
}

func DefaultBuckets() []float64 {
	return []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
}

func ExponentialBuckets(start, factor float64, count int) []float64 {
	if count < 1 {
		return nil
	}
	buckets := make([]float64, count)
	for i := 0; i < count; i++ {
		buckets[i] = start
		start *= factor
	}
	return buckets
}

func LinearBuckets(start, width float64, count int) []float64 {
	if count < 1 {
		return nil
	}
	buckets := make([]float64, count)
	for i := 0; i < count; i++ {
		buckets[i] = start + width*float64(i)
	}
	return buckets
}
