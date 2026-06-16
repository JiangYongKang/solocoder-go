package jwtmgr

import (
	"sync"
	"time"
)

type Blacklist interface {
	Add(tokenID string, ttl time.Duration) error
	Contains(tokenID string) (bool, error)
	Remove(tokenID string) error
	Close() error
}

type MemoryBlacklist struct {
	mu       sync.RWMutex
	items    map[string]time.Time
	stopCh   chan struct{}
	closed   bool
	cleanupInt time.Duration
}

func NewMemoryBlacklist(cleanupInterval time.Duration) *MemoryBlacklist {
	bl := &MemoryBlacklist{
		items:      make(map[string]time.Time),
		stopCh:     make(chan struct{}),
		cleanupInt: cleanupInterval,
	}
	if cleanupInterval > 0 {
		go bl.cleanupLoop()
	}
	return bl
}

func (b *MemoryBlacklist) cleanupLoop() {
	ticker := time.NewTicker(b.cleanupInt)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.cleanup()
		case <-b.stopCh:
			return
		}
	}
}

func (b *MemoryBlacklist) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	for id, expiresAt := range b.items {
		if now.After(expiresAt) {
			delete(b.items, id)
		}
	}
}

func (b *MemoryBlacklist) Add(tokenID string, ttl time.Duration) error {
	if tokenID == "" {
		return ErrInvalidToken
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	expiresAt := time.Now().Add(ttl)
	b.items[tokenID] = expiresAt
	return nil
}

func (b *MemoryBlacklist) Contains(tokenID string) (bool, error) {
	if tokenID == "" {
		return false, nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	expiresAt, exists := b.items[tokenID]
	if !exists {
		return false, nil
	}
	if time.Now().After(expiresAt) {
		return false, nil
	}
	return true, nil
}

func (b *MemoryBlacklist) Remove(tokenID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.items, tokenID)
	return nil
}

func (b *MemoryBlacklist) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.closed = true
		close(b.stopCh)
	}
	return nil
}

func (b *MemoryBlacklist) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.items)
}
