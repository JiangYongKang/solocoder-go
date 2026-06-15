package httplb

import (
	"sync"
	"sync/atomic"
)

type RoundRobin struct {
	counter uint64
	pool    *ServerPool
	mu      sync.Mutex
}

func NewRoundRobin(servers []string) (*RoundRobin, error) {
	rr := &RoundRobin{
		pool: NewServerPool(),
	}
	for _, addr := range servers {
		if err := rr.pool.AddServer(addr, 1); err != nil {
			return nil, err
		}
	}
	return rr, nil
}

func (rr *RoundRobin) Next(key string) (*BackendServer, error) {
	healthy := rr.pool.GetHealthyServers()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyServer
	}

	counter := atomic.AddUint64(&rr.counter, 1)
	idx := int(counter-1) % len(healthy)
	server := healthy[idx]

	server.IncConn()
	return server, nil
}

func (rr *RoundRobin) Servers() []*BackendServer {
	return rr.pool.GetAllServers()
}

func (rr *RoundRobin) AddServer(address string, weight int) error {
	return rr.pool.AddServer(address, weight)
}

func (rr *RoundRobin) RemoveServer(address string) error {
	return rr.pool.RemoveServer(address)
}

func (rr *RoundRobin) DrainServer(address string) error {
	return rr.pool.DrainServer(address)
}

func (rr *RoundRobin) RestoreServer(address string) error {
	return rr.pool.RestoreServer(address)
}

func (rr *RoundRobin) ServerCount() int {
	return rr.pool.ServerCount()
}

func (rr *RoundRobin) HealthyCount() int {
	return rr.pool.HealthyCount()
}
