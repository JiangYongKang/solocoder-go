package metrics

import "sync"

type snapshotGuard struct {
	mu *sync.RWMutex
}

func newSnapshotGuard(mu *sync.RWMutex) snapshotGuard {
	return snapshotGuard{mu: mu}
}

func (g snapshotGuard) write(fn func()) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	fn()
}

type snapshotProtected interface {
	Metric
	snapshotGuardPtr() *snapshotGuard
}
