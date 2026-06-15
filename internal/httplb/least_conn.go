package httplb

type LeastConnections struct {
	pool *ServerPool
}

func NewLeastConnections(servers []string) (*LeastConnections, error) {
	lc := &LeastConnections{
		pool: NewServerPool(),
	}
	for _, addr := range servers {
		if err := lc.pool.AddServer(addr, 1); err != nil {
			return nil, err
		}
	}
	return lc, nil
}

func (lc *LeastConnections) Next(key string) (*BackendServer, error) {
	healthy := lc.pool.GetHealthyServers()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyServer
	}

	minConn := healthy[0].ActiveConn()
	minIdx := 0

	for i := 1; i < len(healthy); i++ {
		conn := healthy[i].ActiveConn()
		if conn < minConn {
			minConn = conn
			minIdx = i
		}
	}

	server := healthy[minIdx]
	server.IncConn()
	return server, nil
}

func (lc *LeastConnections) Servers() []*BackendServer {
	return lc.pool.GetAllServers()
}

func (lc *LeastConnections) AddServer(address string, weight int) error {
	return lc.pool.AddServer(address, weight)
}

func (lc *LeastConnections) RemoveServer(address string) error {
	return lc.pool.RemoveServer(address)
}

func (lc *LeastConnections) DrainServer(address string) error {
	return lc.pool.DrainServer(address)
}

func (lc *LeastConnections) RestoreServer(address string) error {
	return lc.pool.RestoreServer(address)
}

func (lc *LeastConnections) ServerCount() int {
	return lc.pool.ServerCount()
}

func (lc *LeastConnections) HealthyCount() int {
	return lc.pool.HealthyCount()
}
