package oauth2svc

import (
	"sync"
	"time"
)

type ClientStore interface {
	GetClient(clientID string) (*Client, error)
	SaveClient(client *Client) error
	ValidateClient(clientID, clientSecret string) (*Client, error)
}

type AuthorizationCodeStore interface {
	SaveCode(code *AuthorizationCode) error
	GetCode(code string) (*AuthorizationCode, error)
	MarkCodeUsed(code string) error
}

type RefreshTokenStore interface {
	SaveToken(token *RefreshToken) error
	GetToken(token string) (*RefreshToken, error)
	RevokeToken(token string) error
}

type MemoryClientStore struct {
	clients map[string]*Client
	mu      sync.RWMutex
}

func NewMemoryClientStore() *MemoryClientStore {
	return &MemoryClientStore{
		clients: make(map[string]*Client),
	}
}

func (s *MemoryClientStore) GetClient(clientID string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	client, ok := s.clients[clientID]
	if !ok {
		return nil, ErrInvalidClient
	}
	return client, nil
}

func (s *MemoryClientStore) SaveClient(client *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID] = client
	return nil
}

func (s *MemoryClientStore) ValidateClient(clientID, clientSecret string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	client, ok := s.clients[clientID]
	if !ok {
		return nil, ErrInvalidClient
	}
	if client.Secret != clientSecret {
		return nil, ErrInvalidClient
	}
	return client, nil
}

type MemoryAuthorizationCodeStore struct {
	codes map[string]*AuthorizationCode
	mu    sync.RWMutex
}

func NewMemoryAuthorizationCodeStore() *MemoryAuthorizationCodeStore {
	return &MemoryAuthorizationCodeStore{
		codes: make(map[string]*AuthorizationCode),
	}
}

func (s *MemoryAuthorizationCodeStore) SaveCode(code *AuthorizationCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[code.Code] = code
	return nil
}

func (s *MemoryAuthorizationCodeStore) GetCode(code string) (*AuthorizationCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	authCode, ok := s.codes[code]
	if !ok {
		return nil, ErrInvalidGrant
	}
	if time.Now().After(authCode.ExpiresAt) {
		return nil, ErrCodeExpired
	}
	return authCode, nil
}

func (s *MemoryAuthorizationCodeStore) MarkCodeUsed(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	authCode, ok := s.codes[code]
	if !ok {
		return ErrInvalidGrant
	}
	if authCode.Used {
		return ErrCodeUsed
	}
	authCode.Used = true
	return nil
}

type MemoryRefreshTokenStore struct {
	tokens map[string]*RefreshToken
	mu     sync.RWMutex
}

func NewMemoryRefreshTokenStore() *MemoryRefreshTokenStore {
	return &MemoryRefreshTokenStore{
		tokens: make(map[string]*RefreshToken),
	}
}

func (s *MemoryRefreshTokenStore) SaveToken(token *RefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token.Token] = token
	return nil
}

func (s *MemoryRefreshTokenStore) GetToken(token string) (*RefreshToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rt, ok := s.tokens[token]
	if !ok {
		return nil, ErrInvalidGrant
	}
	if rt.Revoked {
		return nil, ErrRefreshTokenRevoked
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrExpiredToken
	}
	return rt, nil
}

func (s *MemoryRefreshTokenStore) RevokeToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rt, ok := s.tokens[token]
	if !ok {
		return ErrInvalidGrant
	}
	rt.Revoked = true
	return nil
}
