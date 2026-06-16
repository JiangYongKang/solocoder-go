package jwtmgr

import (
	"sync"
	"time"
)

type RefreshTokenStore interface {
	Save(token *RefreshTokenInfo) error
	Get(token string) (*RefreshTokenInfo, error)
	Revoke(token string) error
	Close() error
}

type MemoryRefreshStore struct {
	mu     sync.RWMutex
	tokens map[string]*RefreshTokenInfo
}

func NewMemoryRefreshStore() *MemoryRefreshStore {
	return &MemoryRefreshStore{
		tokens: make(map[string]*RefreshTokenInfo),
	}
}

func (s *MemoryRefreshStore) Save(token *RefreshTokenInfo) error {
	if token == nil || token.Token == "" {
		return ErrInvalidRefreshToken
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token
	return nil
}

func (s *MemoryRefreshStore) Get(token string) (*RefreshTokenInfo, error) {
	if token == "" {
		return nil, ErrInvalidRefreshToken
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, exists := s.tokens[token]
	if !exists {
		return nil, ErrInvalidRefreshToken
	}
	if info.Revoked {
		return nil, ErrRefreshTokenRevoked
	}
	if time.Now().After(info.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}
	return info, nil
}

func (s *MemoryRefreshStore) Revoke(token string) error {
	if token == "" {
		return ErrInvalidRefreshToken
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info, exists := s.tokens[token]
	if !exists {
		return ErrInvalidRefreshToken
	}
	info.Revoked = true
	return nil
}

func (s *MemoryRefreshStore) Close() error {
	return nil
}

func (s *MemoryRefreshStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tokens)
}
