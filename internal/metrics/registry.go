package metrics

import (
	"fmt"
	"strings"
	"sync"
)

type registry struct {
	mu         sync.RWMutex
	snapshotMu sync.RWMutex
	guard      snapshotGuard
	counters   map[string]map[string]*counter
	gauges     map[string]map[string]*gauge
	histograms map[string]map[string]*histogram
	summaries  map[string]map[string]*summary
	allMetrics map[string]snapshotProtected
}

func NewRegistry() Registry {
	r := &registry{
		counters:   make(map[string]map[string]*counter),
		gauges:     make(map[string]map[string]*gauge),
		histograms: make(map[string]map[string]*histogram),
		summaries:  make(map[string]map[string]*summary),
		allMetrics: make(map[string]snapshotProtected),
	}
	r.guard = newSnapshotGuard(&r.snapshotMu)
	return r
}

var DefaultRegistry = NewRegistry()

func (r *registry) RegisterCounter(name string, labels Labels) CounterMetric {
	if !isValidMetricName(name) {
		panic(ErrInvalidMetricName)
	}
	for _, l := range labels {
		if !isValidLabelName(l.Name) {
			panic(ErrInvalidLabelName)
		}
	}

	hash := labels.Hash()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.counters[name]; ok {
		if _, ok := r.counters[name][hash]; ok {
			panic(ErrMetricExists)
		}
	} else {
		r.counters[name] = make(map[string]*counter)
	}

	c := newCounter(name, labels, r.guard)
	r.counters[name][hash] = c
	r.allMetrics[name+"\x00"+hash] = c
	return c
}

func (r *registry) RegisterGauge(name string, labels Labels) GaugeMetric {
	if !isValidMetricName(name) {
		panic(ErrInvalidMetricName)
	}
	for _, l := range labels {
		if !isValidLabelName(l.Name) {
			panic(ErrInvalidLabelName)
		}
	}

	hash := labels.Hash()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.gauges[name]; ok {
		if _, ok := r.gauges[name][hash]; ok {
			panic(ErrMetricExists)
		}
	} else {
		r.gauges[name] = make(map[string]*gauge)
	}

	g := newGauge(name, labels, r.guard)
	r.gauges[name][hash] = g
	r.allMetrics[name+"\x00"+hash] = g
	return g
}

func (r *registry) RegisterHistogram(name string, labels Labels, buckets []float64) HistogramMetric {
	if !isValidMetricName(name) {
		panic(ErrInvalidMetricName)
	}
	for _, l := range labels {
		if !isValidLabelName(l.Name) {
			panic(ErrInvalidLabelName)
		}
	}
	if len(buckets) == 0 {
		panic(ErrEmptyBuckets)
	}

	hash := labels.Hash()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.histograms[name]; ok {
		if _, ok := r.histograms[name][hash]; ok {
			panic(ErrMetricExists)
		}
	} else {
		r.histograms[name] = make(map[string]*histogram)
	}

	h := newHistogram(name, labels, buckets, r.guard)
	r.histograms[name][hash] = h
	r.allMetrics[name+"\x00"+hash] = h
	return h
}

func (r *registry) RegisterSummary(name string, labels Labels, quantiles []float64) SummaryMetric {
	if !isValidMetricName(name) {
		panic(ErrInvalidMetricName)
	}
	for _, l := range labels {
		if !isValidLabelName(l.Name) {
			panic(ErrInvalidLabelName)
		}
	}
	if len(quantiles) == 0 {
		panic(ErrEmptyQuantiles)
	}
	for _, q := range quantiles {
		if q < 0 || q > 1 {
			panic(ErrInvalidQuantile)
		}
	}

	hash := labels.Hash()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.summaries[name]; ok {
		if _, ok := r.summaries[name][hash]; ok {
			panic(ErrMetricExists)
		}
	} else {
		r.summaries[name] = make(map[string]*summary)
	}

	s := newSummary(name, labels, quantiles, r.guard)
	r.summaries[name][hash] = s
	r.allMetrics[name+"\x00"+hash] = s
	return s
}

func (r *registry) GetCounter(name string, labels Labels) (CounterMetric, bool) {
	hash := labels.Hash()
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.counters[name]; ok {
		if c, ok := m[hash]; ok {
			return c, true
		}
	}
	return nil, false
}

func (r *registry) GetGauge(name string, labels Labels) (GaugeMetric, bool) {
	hash := labels.Hash()
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.gauges[name]; ok {
		if g, ok := m[hash]; ok {
			return g, true
		}
	}
	return nil, false
}

func (r *registry) GetHistogram(name string, labels Labels) (HistogramMetric, bool) {
	hash := labels.Hash()
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.histograms[name]; ok {
		if h, ok := m[hash]; ok {
			return h, true
		}
	}
	return nil, false
}

func (r *registry) GetSummary(name string, labels Labels) (SummaryMetric, bool) {
	hash := labels.Hash()
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.summaries[name]; ok {
		if s, ok := m[hash]; ok {
			return s, true
		}
	}
	return nil, false
}

func (r *registry) Snapshot() []MetricValue {
	r.snapshotMu.Lock()
	defer r.snapshotMu.Unlock()

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []MetricValue
	for _, m := range r.allMetrics {
		result = append(result, m.Snapshot())
	}

	return result
}

func (r *registry) Unregister(name string, labels Labels) bool {
	hash := labels.Hash()
	r.mu.Lock()
	defer r.mu.Unlock()

	if m, ok := r.counters[name]; ok {
		if _, ok := m[hash]; ok {
			delete(m, hash)
			if len(m) == 0 {
				delete(r.counters, name)
			}
			delete(r.allMetrics, name+"\x00"+hash)
			return true
		}
	}

	if m, ok := r.gauges[name]; ok {
		if _, ok := m[hash]; ok {
			delete(m, hash)
			if len(m) == 0 {
				delete(r.gauges, name)
			}
			delete(r.allMetrics, name+"\x00"+hash)
			return true
		}
	}

	if m, ok := r.histograms[name]; ok {
		if _, ok := m[hash]; ok {
			delete(m, hash)
			if len(m) == 0 {
				delete(r.histograms, name)
			}
			delete(r.allMetrics, name+"\x00"+hash)
			return true
		}
	}

	if m, ok := r.summaries[name]; ok {
		if _, ok := m[hash]; ok {
			delete(m, hash)
			if len(m) == 0 {
				delete(r.summaries, name)
			}
			delete(r.allMetrics, name+"\x00"+hash)
			return true
		}
	}

	return false
}

func isValidMetricName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i, c := range name {
		if i == 0 {
			if !isAlpha(c) && c != '_' && c != ':' {
				return false
			}
		} else {
			if !isAlphaNum(c) && c != '_' && c != ':' {
				return false
			}
		}
	}
	return true
}

func isValidLabelName(name string) bool {
	if len(name) == 0 {
		return false
	}
	if strings.HasPrefix(name, "__") {
		return false
	}
	for i, c := range name {
		if i == 0 {
			if !isAlpha(c) && c != '_' {
				return false
			}
		} else {
			if !isAlphaNum(c) && c != '_' {
				return false
			}
		}
	}
	return true
}

func isAlpha(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isAlphaNum(c rune) bool {
	return isAlpha(c) || (c >= '0' && c <= '9')
}

func formatLabels(labels Labels) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		parts[i] = fmt.Sprintf("%s=\"%s\"", l.Name, escapeString(l.Value))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}
