package httplb

import (
	"errors"
	"net/http"
)

type Algorithm string

const (
	AlgorithmRoundRobin       Algorithm = "round_robin"
	AlgorithmLeastConnections Algorithm = "least_connections"
	AlgorithmWeightedRR       Algorithm = "weighted_round_robin"
	AlgorithmConsistentHash   Algorithm = "consistent_hash"
)

type Config struct {
	Algorithm     Algorithm
	Servers       []string
	Weights       []int
	VirtualNodes  int
	HashKeyFunc   func(*http.Request) string
}

type HTTPLoadBalancer struct {
	balancer    Balancer
	hashKeyFunc func(*http.Request) string
}

func NewHTTPLoadBalancer(cfg Config) (*HTTPLoadBalancer, error) {
	var balancer Balancer
	var err error

	switch cfg.Algorithm {
	case AlgorithmRoundRobin:
		balancer, err = NewRoundRobin(cfg.Servers)
	case AlgorithmLeastConnections:
		balancer, err = NewLeastConnections(cfg.Servers)
	case AlgorithmWeightedRR:
		balancer, err = NewWeightedRoundRobin(cfg.Servers, cfg.Weights)
	case AlgorithmConsistentHash:
		balancer, err = NewConsistentHash(cfg.Servers, cfg.VirtualNodes)
	default:
		return nil, errors.New("httplb: unknown load balancing algorithm")
	}

	if err != nil {
		return nil, err
	}

	hashKeyFunc := cfg.HashKeyFunc
	if hashKeyFunc == nil {
		hashKeyFunc = defaultHashKeyFunc
	}

	return &HTTPLoadBalancer{
		balancer:    balancer,
		hashKeyFunc: hashKeyFunc,
	}, nil
}

func defaultHashKeyFunc(r *http.Request) string {
	return r.URL.Path
}

func (lb *HTTPLoadBalancer) NextServer(r *http.Request) (*BackendServer, error) {
	key := lb.hashKeyFunc(r)
	return lb.balancer.Next(key)
}

func (lb *HTTPLoadBalancer) Balancer() Balancer {
	return lb.balancer
}

func (lb *HTTPLoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server, err := lb.NextServer(r)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}
	defer server.DecConn()

	w.Header().Set("X-Backend-Server", server.Address)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Routed to: " + server.Address))
}

func (lb *HTTPLoadBalancer) AddServer(address string, weight int) error {
	return lb.balancer.AddServer(address, weight)
}

func (lb *HTTPLoadBalancer) RemoveServer(address string) error {
	return lb.balancer.RemoveServer(address)
}

func (lb *HTTPLoadBalancer) DrainServer(address string) error {
	return lb.balancer.DrainServer(address)
}

func (lb *HTTPLoadBalancer) RestoreServer(address string) error {
	return lb.balancer.RestoreServer(address)
}

func (lb *HTTPLoadBalancer) ServerCount() int {
	return lb.balancer.ServerCount()
}

func (lb *HTTPLoadBalancer) HealthyCount() int {
	return lb.balancer.HealthyCount()
}
