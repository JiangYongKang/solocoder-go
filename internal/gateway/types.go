package gateway

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type HandlerFunc func(http.ResponseWriter, *http.Request)

type Middleware func(HandlerFunc) HandlerFunc

type UpstreamHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	Name() string
	HealthCheck() bool
}

type RouteType int

const (
	ExactMatch RouteType = iota
	WildcardMatch
)

type Route struct {
	Path     string
	Type     RouteType
	Upstream string
}

type UserInfo struct {
	UserID string
	Roles  []string
}

type contextKey string

const UserContextKey contextKey = "user"

func WithUser(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

func UserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value(UserContextKey).(*UserInfo)
	return user, ok
}

type TokenStore interface {
	Validate(token string) (*UserInfo, bool)
}

type MemoryTokenStore struct {
	tokens map[string]*UserInfo
	mu     sync.RWMutex
}

func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{tokens: make(map[string]*UserInfo)}
}

func (s *MemoryTokenStore) Add(token string, user *UserInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[token] = user
}

func (s *MemoryTokenStore) Validate(token string) (*UserInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.tokens[token]
	return user, ok
}

type FailureEntry struct {
	Time    time.Time
	Success bool
}

type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	name               string
	state              CircuitBreakerState
	failures           []FailureEntry
	windowSize         time.Duration
	failureThreshold   int
	openDuration       time.Duration
	halfOpenMaxRequests int
	halfOpenRequests   int
	lastOpenTime       time.Time
	halfOpenSuccesses  int
	halfOpenFailures   int
	mu                 sync.Mutex
}

type RateLimiter struct {
	tokens     map[string]*tokenBucket
	rate       int
	capacity   int
	mu         sync.Mutex
	refillRate time.Duration
}

type tokenBucket struct {
	tokens     int
	capacity   int
	rate       int
	lastRefill time.Time
	refillRate time.Duration
	mu         sync.Mutex
}

type HealthStatus struct {
	Name      string
	Healthy   bool
	LastCheck time.Time
	FailCount int
}

type HealthChecker struct {
	upstreams   map[string]*upstreamHealth
	checkInterval time.Duration
	failThreshold int
	passThreshold int
	stopCh      chan struct{}
	running     bool
	mu          sync.RWMutex
}

type upstreamHealth struct {
	handler      UpstreamHandler
	healthy      bool
	lastCheck    time.Time
	failCount    int
	passCount    int
}

type Router struct {
	routes     map[string]string
	wildcards  []wildcardRoute
	mu         sync.RWMutex
	health     *HealthChecker
}

type wildcardRoute struct {
	Prefix   string
	Upstream string
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

type Gateway struct {
	router        *Router
	upstreams     map[string]UpstreamHandler
	middlewares   []Middleware
	auth          *AuthMiddleware
	limiter       *RateLimiter
	circuits      map[string]*CircuitBreaker
	health        *HealthChecker
	fallback      map[string]HandlerFunc
	logger        *LoggerMiddleware
	server        *http.Server
}
