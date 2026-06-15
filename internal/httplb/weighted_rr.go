package httplb

import (
	"sync"
)

type weightedServer struct {
	server       *BackendServer
	currentWeight int
}

type WeightedRoundRobin struct {
	pool     *ServerPool
	weighted []*weightedServer
	mu       sync.Mutex
}

func NewWeightedRoundRobin(servers []string, weights []int) (*WeightedRoundRobin, error) {
	if len(servers) != len(weights) {
		return nil, ErrInvalidWeight
	}

	wrr := &WeightedRoundRobin{
		pool:     NewServerPool(),
		weighted: make([]*weightedServer, 0, len(servers)),
	}

	for i, addr := range servers {
		if err := wrr.pool.AddServer(addr, weights[i]); err != nil {
			return nil, err
		}
		s, _ := wrr.pool.GetServer(addr)
		wrr.weighted = append(wrr.weighted, &weightedServer{
			server:        s,
			currentWeight: 0,
		})
	}

	return wrr, nil
}

func (wrr *WeightedRoundRobin) Next(key string) (*BackendServer, error) {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	healthy := wrr.getHealthyWeighted()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyServer
	}

	totalWeight := 0
	for _, ws := range healthy {
		totalWeight += ws.server.Weight
	}

	var best *weightedServer
	for _, ws := range healthy {
		ws.currentWeight += ws.server.Weight
		if best == nil || ws.currentWeight > best.currentWeight {
			best = ws
		}
	}

	best.currentWeight -= totalWeight
	best.server.IncConn()
	return best.server, nil
}

func (wrr *WeightedRoundRobin) getHealthyWeighted() []*weightedServer {
	result := make([]*weightedServer, 0, len(wrr.weighted))
	for _, ws := range wrr.weighted {
		if ws.server.IsHealthy() {
			result = append(result, ws)
		}
	}
	return result
}

func (wrr *WeightedRoundRobin) Servers() []*BackendServer {
	return wrr.pool.GetAllServers()
}

func (wrr *WeightedRoundRobin) AddServer(address string, weight int) error {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	if err := wrr.pool.AddServer(address, weight); err != nil {
		return err
	}

	s, _ := wrr.pool.GetServer(address)
	wrr.weighted = append(wrr.weighted, &weightedServer{
		server:        s,
		currentWeight: 0,
	})
	return nil
}

func (wrr *WeightedRoundRobin) RemoveServer(address string) error {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	if err := wrr.pool.RemoveServer(address); err != nil {
		return err
	}

	newWeighted := make([]*weightedServer, 0, len(wrr.weighted)-1)
	for _, ws := range wrr.weighted {
		if ws.server.Address != address {
			newWeighted = append(newWeighted, ws)
		}
	}
	wrr.weighted = newWeighted
	return nil
}

func (wrr *WeightedRoundRobin) DrainServer(address string) error {
	return wrr.pool.DrainServer(address)
}

func (wrr *WeightedRoundRobin) RestoreServer(address string) error {
	return wrr.pool.RestoreServer(address)
}

func (wrr *WeightedRoundRobin) ServerCount() int {
	return wrr.pool.ServerCount()
}

func (wrr *WeightedRoundRobin) HealthyCount() int {
	return wrr.pool.HealthyCount()
}
