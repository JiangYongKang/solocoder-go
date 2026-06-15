package httplb

import (
	"errors"
	"sync"
	"sync/atomic"
)

var (
	ErrServerExists       = errors.New("httplb: server already exists")
	ErrServerNotFound     = errors.New("httplb: server not found")
	ErrNoHealthyServer    = errors.New("httplb: no healthy backend servers available")
	ErrInvalidWeight      = errors.New("httplb: invalid weight, must be positive")
	ErrServerHasConns     = errors.New("httplb: server still has active connections")
	ErrServerNotDraining  = errors.New("httplb: server is not in draining state, call DrainServer first")
)

type ServerStatus int

const (
	StatusUp ServerStatus = iota
	StatusDraining
	StatusDown
)

type BackendServer struct {
	activeConn int64
	Address    string
	Weight     int
	status     ServerStatus
	mu         sync.RWMutex
}

func NewBackendServer(address string, weight int) (*BackendServer, error) {
	if weight <= 0 {
		return nil, ErrInvalidWeight
	}
	return &BackendServer{
		Address: address,
		Weight:  weight,
		status:  StatusUp,
	}, nil
}

func (s *BackendServer) Status() ServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *BackendServer) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == StatusUp
}

func (s *BackendServer) ActiveConn() int64 {
	return atomic.LoadInt64(&s.activeConn)
}

func (s *BackendServer) IncConn() {
	atomic.AddInt64(&s.activeConn, 1)
}

func (s *BackendServer) DecConn() {
	atomic.AddInt64(&s.activeConn, -1)
}

func (s *BackendServer) MarkDraining() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == StatusUp {
		s.status = StatusDraining
	}
}

func (s *BackendServer) MarkDown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = StatusDown
}

func (s *BackendServer) MarkUp() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = StatusUp
}

func (s *BackendServer) IsDraining() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == StatusDraining
}

type ServerPool struct {
	servers map[string]*BackendServer
	order   []string
	mu      sync.RWMutex
}

func NewServerPool() *ServerPool {
	return &ServerPool{
		servers: make(map[string]*BackendServer),
		order:   make([]string, 0),
	}
}

func (sp *ServerPool) AddServer(address string, weight int) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if _, exists := sp.servers[address]; exists {
		return ErrServerExists
	}

	server, err := NewBackendServer(address, weight)
	if err != nil {
		return err
	}

	sp.servers[address] = server
	sp.order = append(sp.order, address)
	return nil
}

func (sp *ServerPool) RemoveServer(address string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	s, exists := sp.servers[address]
	if !exists {
		return ErrServerNotFound
	}

	if !s.IsDraining() {
		return ErrServerNotDraining
	}

	if s.ActiveConn() > 0 {
		return ErrServerHasConns
	}

	delete(sp.servers, address)

	newOrder := make([]string, 0, len(sp.order)-1)
	for _, addr := range sp.order {
		if addr != address {
			newOrder = append(newOrder, addr)
		}
	}
	sp.order = newOrder
	return nil
}

func (sp *ServerPool) GetServer(address string) (*BackendServer, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	s, ok := sp.servers[address]
	return s, ok
}

func (sp *ServerPool) GetHealthyServers() []*BackendServer {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	result := make([]*BackendServer, 0, len(sp.servers))
	for _, addr := range sp.order {
		s := sp.servers[addr]
		if s.IsHealthy() {
			result = append(result, s)
		}
	}
	return result
}

func (sp *ServerPool) GetAllServers() []*BackendServer {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	result := make([]*BackendServer, 0, len(sp.servers))
	for _, addr := range sp.order {
		result = append(result, sp.servers[addr])
	}
	return result
}

func (sp *ServerPool) ServerCount() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	return len(sp.servers)
}

func (sp *ServerPool) HealthyCount() int {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	count := 0
	for _, s := range sp.servers {
		if s.IsHealthy() {
			count++
		}
	}
	return count
}

func (sp *ServerPool) DrainServer(address string) error {
	sp.mu.RLock()
	s, exists := sp.servers[address]
	sp.mu.RUnlock()

	if !exists {
		return ErrServerNotFound
	}

	s.MarkDraining()
	return nil
}

func (sp *ServerPool) RestoreServer(address string) error {
	sp.mu.RLock()
	s, exists := sp.servers[address]
	sp.mu.RUnlock()

	if !exists {
		return ErrServerNotFound
	}

	s.MarkUp()
	return nil
}
