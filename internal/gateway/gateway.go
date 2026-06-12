package gateway

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

func NewHealthChecker(checkInterval time.Duration, failThreshold, passThreshold int) *HealthChecker {
	return &HealthChecker{
		upstreams:     make(map[string]*upstreamHealth),
		checkInterval: checkInterval,
		failThreshold: failThreshold,
		passThreshold: passThreshold,
		stopCh:        make(chan struct{}),
	}
}

func (hc *HealthChecker) AddUpstream(name string, handler UpstreamHandler) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.upstreams[name] = &upstreamHealth{
		handler:   handler,
		healthy:   true,
		lastCheck: time.Now(),
	}
}

func (hc *HealthChecker) RemoveUpstream(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.upstreams, name)
}

func (hc *HealthChecker) IsHealthy(name string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	uh, ok := hc.upstreams[name]
	if !ok {
		return false
	}
	return uh.healthy
}

func (hc *HealthChecker) GetStatus(name string) (HealthStatus, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	uh, ok := hc.upstreams[name]
	if !ok {
		return HealthStatus{}, false
	}
	return HealthStatus{
		Name:      name,
		Healthy:   uh.healthy,
		LastCheck: uh.lastCheck,
		FailCount: uh.failCount,
	}, true
}

func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.mu.Unlock()

	if hc.checkInterval > 0 {
		go hc.checkLoop()
	}
}

func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if !hc.running {
		return
	}
	hc.running = false
	close(hc.stopCh)
}

func (hc *HealthChecker) checkLoop() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkAll()
		}
	}
}

func (hc *HealthChecker) checkAll() {
	hc.mu.RLock()
	names := make([]string, 0, len(hc.upstreams))
	for name := range hc.upstreams {
		names = append(names, name)
	}
	hc.mu.RUnlock()

	for _, name := range names {
		hc.checkOne(name)
	}
}

func (hc *HealthChecker) checkOne(name string) {
	hc.mu.RLock()
	uh, ok := hc.upstreams[name]
	if !ok {
		hc.mu.RUnlock()
		return
	}
	handler := uh.handler
	hc.mu.RUnlock()

	healthy := handler.HealthCheck()
	now := time.Now()

	hc.mu.Lock()
	defer hc.mu.Unlock()

	uh, ok = hc.upstreams[name]
	if !ok {
		return
	}
	uh.lastCheck = now

	if healthy {
		uh.failCount = 0
		uh.passCount++
		if !uh.healthy && uh.passCount >= hc.passThreshold {
			uh.healthy = true
		}
	} else {
		uh.passCount = 0
		uh.failCount++
		if uh.healthy && uh.failCount >= hc.failThreshold {
			uh.healthy = false
		}
	}
}

func (hc *HealthChecker) SetHealthy(name string, healthy bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	uh, ok := hc.upstreams[name]
	if !ok {
		return
	}
	uh.healthy = healthy
	uh.lastCheck = time.Now()
	if healthy {
		uh.passCount = hc.passThreshold
		uh.failCount = 0
	} else {
		uh.failCount = hc.failThreshold
		uh.passCount = 0
	}
}

func (hc *HealthChecker) Running() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}

type GatewayConfig struct {
	TokenStore      TokenStore
	Rate            int
	RateCapacity    int
	RefillRate      time.Duration
	CheckInterval   time.Duration
	FailThreshold   int
	PassThreshold   int
	CircuitConfigs  map[string]CircuitBreakerConfig
	AuthExemptPaths []string
	EnableAuth      bool
	EnableRateLimit bool
	EnableLogger    bool
}

func NewGateway(config GatewayConfig) *Gateway {
	health := NewHealthChecker(config.CheckInterval, config.FailThreshold, config.PassThreshold)

	g := &Gateway{
		router:    NewRouter(health),
		upstreams: make(map[string]UpstreamHandler),
		circuits:  make(map[string]*CircuitBreaker),
		fallback:  make(map[string]HandlerFunc),
		health:    health,
	}

	if config.EnableLogger {
		g.logger = NewLoggerMiddleware()
	}

	if config.EnableAuth && config.TokenStore != nil {
		g.auth = NewAuthMiddleware(config.TokenStore)
		for _, p := range config.AuthExemptPaths {
			g.auth.ExemptPath(p)
		}
	}

	if config.EnableRateLimit && config.Rate > 0 {
		g.limiter = NewRateLimiter(config.Rate, config.RateCapacity, config.RefillRate)
	}

	for name, cbCfg := range config.CircuitConfigs {
		g.circuits[name] = NewCircuitBreaker(
			cbCfg.Name,
			cbCfg.WindowSize,
			cbCfg.FailureThreshold,
			cbCfg.OpenDuration,
			cbCfg.HalfOpenMaxRequests,
		)
		if cbCfg.Fallback != nil {
			g.fallback[name] = cbCfg.Fallback
		}
	}

	g.buildMiddlewareChain()

	return g
}

func (g *Gateway) buildMiddlewareChain() {
	g.middlewares = nil
	if g.logger != nil {
		g.middlewares = append(g.middlewares, g.logger.Middleware())
	}
	if g.limiter != nil {
		g.middlewares = append(g.middlewares, g.limiter.Middleware())
	}
	if g.auth != nil {
		g.middlewares = append(g.middlewares, g.auth.Middleware())
	}
}

func (g *Gateway) RegisterUpstream(name string, handler UpstreamHandler) {
	g.upstreams[name] = handler
	g.health.AddUpstream(name, handler)
}

func (g *Gateway) AddRoute(path string, routeType RouteType, upstream string) {
	g.router.AddRoute(path, routeType, upstream)
}

func (g *Gateway) RegisterFallback(upstream string, fb HandlerFunc) {
	g.fallback[upstream] = fb
}

func (g *Gateway) RegisterCircuitBreaker(name string, cb *CircuitBreaker) {
	g.circuits[name] = cb
}

func (g *Gateway) StartHealthCheck() {
	g.health.Start()
}

func (g *Gateway) StopHealthCheck() {
	g.health.Stop()
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := g.router.Handler(g.upstreams, g.circuits, g.fallback)

	for i := len(g.middlewares) - 1; i >= 0; i-- {
		handler = g.middlewares[i](handler)
	}

	handler(w, r)
}

func (g *Gateway) Handler() http.Handler {
	return http.HandlerFunc(g.ServeHTTP)
}

func (g *Gateway) Start(addr string) error {
	if g.server != nil {
		return errors.New("gateway: server already started")
	}
	g.server = &http.Server{
		Addr:    addr,
		Handler: g.Handler(),
	}
	g.health.Start()
	err := g.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (g *Gateway) Stop(ctx context.Context) error {
	g.health.Stop()
	if g.server == nil {
		return nil
	}
	err := g.server.Shutdown(ctx)
	g.server = nil
	return err
}

func (g *Gateway) GetRouter() *Router {
	return g.router
}

func (g *Gateway) GetHealthChecker() *HealthChecker {
	return g.health
}

func (g *Gateway) GetCircuitBreaker(name string) (*CircuitBreaker, bool) {
	cb, ok := g.circuits[name]
	return cb, ok
}

func (g *Gateway) GetUpstream(name string) (UpstreamHandler, bool) {
	u, ok := g.upstreams[name]
	return u, ok
}

type MockUpstreamHandler struct {
	name           string
	healthy        bool
	healthyMu      sync.RWMutex
	statusCode     int
	responseBody   string
	requestCount   int64
	countMu        sync.Mutex
	customHandler  HandlerFunc
	latency        time.Duration
}

func NewMockUpstreamHandler(name string) *MockUpstreamHandler {
	return &MockUpstreamHandler{
		name:         name,
		healthy:      true,
		statusCode:   http.StatusOK,
		responseBody: "OK from " + name,
	}
}

func (m *MockUpstreamHandler) Name() string {
	return m.name
}

func (m *MockUpstreamHandler) HealthCheck() bool {
	m.healthyMu.RLock()
	defer m.healthyMu.RUnlock()
	return m.healthy
}

func (m *MockUpstreamHandler) SetHealthy(healthy bool) {
	m.healthyMu.Lock()
	defer m.healthyMu.Unlock()
	m.healthy = healthy
}

func (m *MockUpstreamHandler) SetResponse(statusCode int, body string) {
	m.statusCode = statusCode
	m.responseBody = body
}

func (m *MockUpstreamHandler) SetLatency(d time.Duration) {
	m.latency = d
}

func (m *MockUpstreamHandler) SetCustomHandler(h HandlerFunc) {
	m.customHandler = h
}

func (m *MockUpstreamHandler) RequestCount() int64 {
	m.countMu.Lock()
	defer m.countMu.Unlock()
	return m.requestCount
}

func (m *MockUpstreamHandler) ResetCount() {
	m.countMu.Lock()
	defer m.countMu.Unlock()
	m.requestCount = 0
}

func (m *MockUpstreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.countMu.Lock()
	m.requestCount++
	m.countMu.Unlock()

	if m.latency > 0 {
		time.Sleep(m.latency)
	}

	if m.customHandler != nil {
		m.customHandler(w, r)
		return
	}

	w.WriteHeader(m.statusCode)
	w.Write([]byte(m.responseBody))
}
