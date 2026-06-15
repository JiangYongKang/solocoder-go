package httplb

type Balancer interface {
	Next(key string) (*BackendServer, error)
	Servers() []*BackendServer
	AddServer(address string, weight int) error
	RemoveServer(address string) error
	DrainServer(address string) error
	RestoreServer(address string) error
	ServerCount() int
	HealthyCount() int
}
