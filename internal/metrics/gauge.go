package metrics

import (
	"sync"
	"time"
)

type gauge struct {
	mu     sync.RWMutex
	name   string
	labels Labels
	value  float64
}

func newGauge(name string, labels Labels) *gauge {
	g := &gauge{
		name:   name,
		labels: make(Labels, len(labels)),
	}
	copy(g.labels, labels)
	return g
}

func (g *gauge) Name() string {
	return g.name
}

func (g *gauge) Type() MetricType {
	return GaugeType
}

func (g *gauge) Labels() Labels {
	return g.labels
}

func (g *gauge) Set(value float64) {
	g.mu.Lock()
	g.value = value
	g.mu.Unlock()
}

func (g *gauge) Inc() {
	g.Add(1)
}

func (g *gauge) Dec() {
	g.Sub(1)
}

func (g *gauge) Add(delta float64) {
	g.mu.Lock()
	g.value += delta
	g.mu.Unlock()
}

func (g *gauge) Sub(delta float64) {
	g.mu.Lock()
	g.value -= delta
	g.mu.Unlock()
}

func (g *gauge) Value() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.value
}

func (g *gauge) Snapshot() MetricValue {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return MetricValue{
		Name:      g.name,
		Type:      GaugeType,
		Labels:    g.labels,
		Timestamp: time.Now(),
		Value:     g.value,
	}
}
