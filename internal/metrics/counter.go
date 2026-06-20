package metrics

import (
	"sync"
	"time"
)

type counter struct {
	snapshotMu *sync.RWMutex
	mu         sync.RWMutex
	name       string
	labels     Labels
	value      float64
}

func newCounter(name string, labels Labels, snapshotMu *sync.RWMutex) *counter {
	c := &counter{
		snapshotMu: snapshotMu,
		name:       name,
		labels:     make(Labels, len(labels)),
	}
	copy(c.labels, labels)
	return c
}

func (c *counter) Name() string {
	return c.name
}

func (c *counter) Type() MetricType {
	return CounterType
}

func (c *counter) Labels() Labels {
	return c.labels
}

func (c *counter) Inc() {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *counter) Add(delta float64) {
	if delta < 0 {
		return
	}
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	c.mu.Lock()
	c.value += delta
	c.mu.Unlock()
}

func (c *counter) Value() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

func (c *counter) Reset() {
	c.snapshotMu.RLock()
	defer c.snapshotMu.RUnlock()
	c.mu.Lock()
	c.value = 0
	c.mu.Unlock()
}

func (c *counter) Snapshot() MetricValue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return MetricValue{
		Name:      c.name,
		Type:      CounterType,
		Labels:    c.labels,
		Timestamp: time.Now(),
		Value:     c.value,
	}
}
