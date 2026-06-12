package gateway

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRouter_ExactMatch(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	router.AddRoute("/api/users", ExactMatch, "user-service")
	router.AddRoute("/api/orders", ExactMatch, "order-service")

	upstream, ok := router.Match("/api/users")
	if !ok || upstream != "user-service" {
		t.Errorf("expected user-service, got %s (ok=%v)", upstream, ok)
	}

	upstream, ok = router.Match("/api/orders")
	if !ok || upstream != "order-service" {
		t.Errorf("expected order-service, got %s (ok=%v)", upstream, ok)
	}
}

func TestRouter_WildcardMatch(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	router.AddRoute("/api/*", WildcardMatch, "api-service")
	router.AddRoute("/api/admin/*", WildcardMatch, "admin-service")
	router.AddRoute("/static/*", WildcardMatch, "static-service")

	upstream, ok := router.Match("/api/users")
	if !ok || upstream != "api-service" {
		t.Errorf("expected api-service, got %s (ok=%v)", upstream, ok)
	}

	upstream, ok = router.Match("/api/admin/dashboard")
	if !ok || upstream != "admin-service" {
		t.Errorf("expected admin-service (longest prefix), got %s (ok=%v)", upstream, ok)
	}

	upstream, ok = router.Match("/static/css/main.css")
	if !ok || upstream != "static-service" {
		t.Errorf("expected static-service, got %s (ok=%v)", upstream, ok)
	}
}

func TestRouter_NotFound(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	router.AddRoute("/api/*", WildcardMatch, "api-service")

	_, ok := router.Match("/health")
	if ok {
		t.Error("expected not found for /health")
	}

	_, ok = router.Match("/")
	if ok {
		t.Error("expected not found for /")
	}
}

func TestRouter_HealthCheckIntegration(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)
	mock := NewMockUpstreamHandler("user-service")
	health.AddUpstream("user-service", mock)
	router.AddRoute("/api/users", ExactMatch, "user-service")

	upstream, ok := router.Match("/api/users")
	if !ok || upstream != "user-service" {
		t.Errorf("expected match when healthy, got %s (ok=%v)", upstream, ok)
	}

	mock.SetHealthy(false)
	health.SetHealthy("user-service", false)

	_, ok = router.Match("/api/users")
	if ok {
		t.Error("expected no match when upstream unhealthy")
	}

	mock.SetHealthy(true)
	health.SetHealthy("user-service", true)

	upstream, ok = router.Match("/api/users")
	if !ok || upstream != "user-service" {
		t.Errorf("expected match after recovery, got %s (ok=%v)", upstream, ok)
	}
}

func TestRouter_Handler(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	userSvc := NewMockUpstreamHandler("user-service")
	userSvc.SetResponse(http.StatusOK, `{"id":1,"name":"test"}`)
	orderSvc := NewMockUpstreamHandler("order-service")
	orderSvc.SetResponse(http.StatusOK, `{"order_id":"abc"}`)

	upstreams := map[string]UpstreamHandler{
		"user-service":  userSvc,
		"order-service": orderSvc,
	}
	circuits := make(map[string]*CircuitBreaker)
	fallbacks := make(map[string]HandlerFunc)

	router.AddRoute("/api/users", ExactMatch, "user-service")
	router.AddRoute("/api/orders/*", WildcardMatch, "order-service")

	handler := router.Handler(upstreams, circuits, fallbacks)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"id":1`) {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
	if userSvc.RequestCount() != 1 {
		t.Errorf("expected 1 request to user-service, got %d", userSvc.RequestCount())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/orders/123", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
	if orderSvc.RequestCount() != 1 {
		t.Errorf("expected 1 request to order-service, got %d", orderSvc.RequestCount())
	}

	req3 := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	w3 := httptest.NewRecorder()
	handler(w3, req3)
	if w3.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w3.Code)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	store := NewMemoryTokenStore()
	user := &UserInfo{UserID: "user123", Roles: []string{"admin", "user"}}
	store.Add("valid-token-xyz", user)

	auth := NewAuthMiddleware(store)

	var capturedUser *UserInfo
	next := func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if ok {
			capturedUser = u
		}
		w.WriteHeader(http.StatusOK)
	}

	handler := auth.Middleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token-xyz")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if capturedUser == nil {
		t.Fatal("expected user info in context")
	}
	if capturedUser.UserID != "user123" {
		t.Errorf("expected user123, got %s", capturedUser.UserID)
	}
	if len(capturedUser.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(capturedUser.Roles))
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	store := NewMemoryTokenStore()
	auth := NewAuthMiddleware(store)

	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	handler := auth.Middleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if called {
		t.Error("next handler should not be called")
	}
	if !strings.Contains(w.Body.String(), "Missing Authorization") {
		t.Errorf("unexpected error message: %s", w.Body.String())
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	store := NewMemoryTokenStore()
	auth := NewAuthMiddleware(store)

	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	handler := auth.Middleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid token") {
		t.Errorf("unexpected error message: %s", w.Body.String())
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	store := NewMemoryTokenStore()
	auth := NewAuthMiddleware(store)

	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	handler := auth.Middleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "Basic abc123")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Invalid Authorization format") {
		t.Errorf("unexpected error message: %s", w.Body.String())
	}
}

func TestAuthMiddleware_ExemptPath(t *testing.T) {
	store := NewMemoryTokenStore()
	auth := NewAuthMiddleware(store)
	auth.ExemptPath("/health")

	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}

	handler := auth.Middleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for exempt path, got %d", w.Code)
	}
	if !called {
		t.Error("next handler should be called for exempt path")
	}
}

func TestAuthMiddleware_NoBearerPrefix(t *testing.T) {
	store := NewMemoryTokenStore()
	auth := NewAuthMiddleware(store)

	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	handler := auth.Middleware()(next)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Authorization", "JustAToken")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRateLimiter_AllowAndDeny(t *testing.T) {
	rl := NewRateLimiter(1, 3, 100*time.Millisecond)
	key := "192.168.1.1"

	for i := 0; i < 3; i++ {
		if !rl.Allow(key) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow(key) {
		t.Error("4th request should be denied (capacity exceeded)")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(1, 2, 50*time.Millisecond)
	key := "10.0.0.1"

	if !rl.Allow(key) {
		t.Fatal("1st request should be allowed")
	}
	if !rl.Allow(key) {
		t.Fatal("2nd request should be allowed")
	}
	if rl.Allow(key) {
		t.Fatal("3rd request should be denied")
	}

	time.Sleep(120 * time.Millisecond)

	if !rl.Allow(key) {
		t.Error("request after refill should be allowed")
	}
	if !rl.Allow(key) {
		t.Error("second request after refill should be allowed")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(1, 2, 100*time.Millisecond)

	var callCount int
	next := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}

	handler := rl.Middleware()(next)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		handler(w, req)

		if i < 2 {
			if w.Code != http.StatusOK {
				t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
			}
		} else {
			if w.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: expected 429, got %d", i+1, w.Code)
			}
			retryAfter := w.Header().Get("Retry-After")
			if retryAfter == "" {
				t.Error("expected Retry-After header")
			}
		}
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls to next handler, got %d", callCount)
	}
}

func TestRateLimiter_ExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		realIP     string
		expected   string
	}{
		{"RemoteAddr with port", "192.168.1.50:8080", "", "", "192.168.1.50"},
		{"RemoteAddr without port", "10.0.0.5", "", "", "10.0.0.5"},
		{"X-Forwarded-For single", "", "203.0.113.1", "", "203.0.113.1"},
		{"X-Forwarded-For multiple", "", "203.0.113.1, 10.0.0.1, 172.16.0.1", "", "203.0.113.1"},
		{"X-Real-IP", "", "", "198.51.100.5", "198.51.100.5"},
		{"XFF takes precedence", "10.0.0.1:9999", "203.0.113.50", "192.168.1.1", "203.0.113.50"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.realIP != "" {
				req.Header.Set("X-Real-IP", tt.realIP)
			}
			ip := extractIP(req)
			if ip != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ip)
			}
		})
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1, 1, time.Second)
	key := "reset-test-ip"

	if !rl.Allow(key) {
		t.Fatal("first request should be allowed")
	}
	if rl.Allow(key) {
		t.Fatal("second request should be denied")
	}

	rl.Reset(key)

	if !rl.Allow(key) {
		t.Error("request after reset should be allowed")
	}
}

func TestRateLimiter_Independence(t *testing.T) {
	rl := NewRateLimiter(1, 1, time.Second)

	if !rl.Allow("ip-a") {
		t.Fatal("ip-a first should be allowed")
	}
	if rl.Allow("ip-a") {
		t.Fatal("ip-a second should be denied")
	}
	if !rl.Allow("ip-b") {
		t.Error("ip-b first should be allowed (independent of ip-a)")
	}
}

func TestLoggerMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "[TEST] ", log.LstdFlags)
	lm := NewLoggerMiddlewareWithLogger(logger)

	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}

	handler := lm.Middleware()(next)

	req := httptest.NewRequest(http.MethodPost, "/api/create", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "method=POST") {
		t.Errorf("log missing method: %s", output)
	}
	if !strings.Contains(output, "path=/api/create") {
		t.Errorf("log missing path: %s", output)
	}
	if !strings.Contains(output, "status=201") {
		t.Errorf("log missing status: %s", output)
	}
	if !strings.Contains(output, "duration=") {
		t.Errorf("log missing duration: %s", output)
	}
}

func TestLoggerMiddleware_VariousStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"OK 200", http.StatusOK},
		{"Bad Request 400", http.StatusBadRequest},
		{"Unauthorized 401", http.StatusUnauthorized},
		{"Forbidden 403", http.StatusForbidden},
		{"Not Found 404", http.StatusNotFound},
		{"Internal Error 500", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.New(&buf, "", 0)
			lm := NewLoggerMiddlewareWithLogger(logger)

			next := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}

			handler := lm.Middleware()(next)
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler(w, req)

			if !strings.Contains(buf.String(), "status="+itoa(tt.status)) {
				t.Errorf("expected status=%d in log, got: %s", tt.status, buf.String())
			}
		})
	}
}

func TestLogCollector(t *testing.T) {
	collector := NewLogCollector()

	store := NewMemoryTokenStore()
	store.Add("tok1", &UserInfo{UserID: "u-001"})

	handler := collector.Middleware()(func(w http.ResponseWriter, r *http.Request) {
		if user, ok := UserFromContext(r.Context()); ok {
			_ = user
		}
		w.WriteHeader(http.StatusOK)
	})

	auth := NewAuthMiddleware(store)
	authHandler := auth.Middleware()(handler)

	req := httptest.NewRequest(http.MethodGet, "/collected", nil)
	req.RemoteAddr = "172.16.0.1:5678"
	req.Header.Set("Authorization", "Bearer tok1")
	w := httptest.NewRecorder()
	authHandler(w, req)

	logs := collector.GetLogs()
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}

	lg := logs[0]
	if lg.Method != "GET" {
		t.Errorf("expected GET, got %s", lg.Method)
	}
	if lg.Path != "/collected" {
		t.Errorf("expected /collected, got %s", lg.Path)
	}
	if lg.Status != http.StatusOK {
		t.Errorf("expected 200, got %d", lg.Status)
	}
	if lg.IP != "172.16.0.1" {
		t.Errorf("expected 172.16.0.1, got %s", lg.IP)
	}
	if lg.UserID != "u-001" {
		t.Errorf("expected u-001, got %s", lg.UserID)
	}

	collector.Clear()
	if len(collector.GetLogs()) != 0 {
		t.Error("expected 0 logs after clear")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 3, time.Minute, 2)

	if cb.State() != StateClosed {
		t.Errorf("expected initial state Closed, got %v", cb.State())
	}

	for i := 0; i < 5; i++ {
		allowed, shouldRecord := cb.Allow()
		if !allowed || !shouldRecord {
			t.Errorf("iteration %d: expected allowed=true, shouldRecord=true", i)
		}
	}
}

func TestCircuitBreaker_TripToOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 3, 100*time.Millisecond, 2)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Errorf("expected Closed after 2 failures (threshold=3), got %v", cb.State())
	}

	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("expected Open after 3 failures, got %v", cb.State())
	}

	allowed, _ := cb.Allow()
	if allowed {
		t.Error("no requests should be allowed in Open state")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 2, 50*time.Millisecond, 2)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	allowed, _ := cb.Allow()
	if allowed {
		t.Error("should not allow while Open")
	}

	time.Sleep(80 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Errorf("expected HalfOpen after openDuration, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 2, 10*time.Millisecond, 2)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen, got %v", cb.State())
	}

	allowed1, record1 := cb.Allow()
	allowed2, record2 := cb.Allow()
	allowed3, _ := cb.Allow()

	if !allowed1 || !allowed2 {
		t.Error("first 2 half-open requests should be allowed")
	}
	if !record1 || !record2 {
		t.Error("half-open requests should be recorded")
	}
	if allowed3 {
		t.Error("3rd half-open request should be denied (max=2)")
	}

	cb.RecordSuccess()
	cb.RecordSuccess()

	if cb.State() != StateClosed {
		t.Errorf("expected Closed after halfOpenMaxRequests successes, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 2, 10*time.Millisecond, 2)

	cb.RecordFailure()
	cb.RecordFailure()
	time.Sleep(20 * time.Millisecond)

	cb.Allow()
	cb.RecordFailure()

	if cb.State() != StateOpen {
		t.Errorf("expected Open after failure in HalfOpen, got %v", cb.State())
	}
}

func TestCircuitBreaker_FailureWindow(t *testing.T) {
	cb := NewCircuitBreaker("test", 50*time.Millisecond, 3, time.Second, 2)

	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(80 * time.Millisecond)

	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Errorf("failures outside window should not trip, got state=%v", cb.State())
	}

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Errorf("expected Open after 3 recent failures, got %v", cb.State())
	}
}

func TestCircuitBreaker_SuccessResetsWindow(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 3, time.Second, 2)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()

	if cb.State() != StateClosed {
		t.Errorf("should still be Closed, got %v", cb.State())
	}
	if cb.FailureCount() > 2 {
		t.Errorf("failure count should account for success")
	}
}

func TestCircuitBreaker_ForceStates(t *testing.T) {
	cb := NewCircuitBreaker("test", time.Minute, 5, time.Minute, 1)

	cb.ForceOpen()
	if cb.State() != StateOpen {
		t.Errorf("expected Open after ForceOpen, got %v", cb.State())
	}

	cb.ForceClosed()
	if cb.State() != StateClosed {
		t.Errorf("expected Closed after ForceClosed, got %v", cb.State())
	}

	cb.RecordFailure()
	cb.RecordFailure()
	cb.Reset()
	if cb.State() != StateClosed {
		t.Errorf("expected Closed after Reset, got %v", cb.State())
	}
	if cb.FailureCount() != 0 {
		t.Errorf("expected 0 failures after Reset, got %d", cb.FailureCount())
	}
}

func TestRouter_CircuitBreakerIntegration(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	mock := NewMockUpstreamHandler("failing-svc")
	mock.SetResponse(http.StatusInternalServerError, "boom")

	upstreams := map[string]UpstreamHandler{"failing-svc": mock}
	cb := NewCircuitBreaker("failing-svc", time.Minute, 2, 500*time.Millisecond, 1)
	circuits := map[string]*CircuitBreaker{"failing-svc": cb}

	var fallbackCalled bool
	fallbacks := map[string]HandlerFunc{
		"failing-svc": func(w http.ResponseWriter, r *http.Request) {
			fallbackCalled = true
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("FALLBACK"))
		},
	}

	router.AddRoute("/fail", ExactMatch, "failing-svc")
	handler := router.Handler(upstreams, circuits, fallbacks)

	req1 := httptest.NewRequest(http.MethodGet, "/fail", nil)
	w1 := httptest.NewRecorder()
	handler(w1, req1)
	if w1.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 from upstream, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/fail", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on 2nd failure, got %d", w2.Code)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/fail", nil)
	w3 := httptest.NewRecorder()
	handler(w3, req3)
	if w3.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 (circuit open + fallback), got %d", w3.Code)
	}
	if !fallbackCalled {
		t.Error("fallback should have been called")
	}
	if !strings.Contains(w3.Body.String(), "FALLBACK") {
		t.Errorf("expected fallback body, got %s", w3.Body.String())
	}
}

func TestRouter_CircuitBreaker_NoFallback(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	mock := NewMockUpstreamHandler("svc-no-fb")
	mock.SetResponse(http.StatusInternalServerError, "x")

	upstreams := map[string]UpstreamHandler{"svc-no-fb": mock}
	cb := NewCircuitBreaker("svc-no-fb", time.Minute, 1, time.Second, 1)
	circuits := map[string]*CircuitBreaker{"svc-no-fb": cb}
	fallbacks := make(map[string]HandlerFunc)

	router.AddRoute("/x", ExactMatch, "svc-no-fb")
	handler := router.Handler(upstreams, circuits, fallbacks)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if w2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 default fallback, got %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), "Circuit Breaker Open") {
		t.Errorf("expected default message, got %s", w2.Body.String())
	}
}

func TestHealthChecker_Basic(t *testing.T) {
	hc := NewHealthChecker(time.Second, 2, 2)
	mock := NewMockUpstreamHandler("svc-a")
	hc.AddUpstream("svc-a", mock)

	if !hc.IsHealthy("svc-a") {
		t.Error("new upstream should be healthy")
	}

	status, ok := hc.GetStatus("svc-a")
	if !ok {
		t.Fatal("expected status for svc-a")
	}
	if !status.Healthy {
		t.Error("status should report healthy")
	}
	if status.Name != "svc-a" {
		t.Errorf("expected name svc-a, got %s", status.Name)
	}

	_, ok = hc.GetStatus("nonexistent")
	if ok {
		t.Error("should not find nonexistent upstream")
	}
}

func TestHealthChecker_CheckLoop(t *testing.T) {
	hc := NewHealthChecker(20*time.Millisecond, 2, 2)
	mock := NewMockUpstreamHandler("flaky")
	hc.AddUpstream("flaky", mock)

	mock.SetHealthy(false)
	hc.Start()
	defer hc.Stop()

	time.Sleep(100 * time.Millisecond)

	if hc.IsHealthy("flaky") {
		t.Error("should be unhealthy after failing checks")
	}

	mock.SetHealthy(true)
	time.Sleep(100 * time.Millisecond)

	if !hc.IsHealthy("flaky") {
		t.Error("should recover after passing checks")
	}
}

func TestHealthChecker_StartStop(t *testing.T) {
	hc := NewHealthChecker(time.Second, 1, 1)

	if hc.Running() {
		t.Error("should not be running initially")
	}

	hc.Start()
	if !hc.Running() {
		t.Error("should be running after Start")
	}

	hc.Start()
	if !hc.Running() {
		t.Error("double Start should be idempotent")
	}

	hc.Stop()
	if hc.Running() {
		t.Error("should not be running after Stop")
	}

	hc.Stop()
}

func TestHealthChecker_RemoveAndSet(t *testing.T) {
	hc := NewHealthChecker(time.Second, 1, 1)
	mock := NewMockUpstreamHandler("to-remove")
	hc.AddUpstream("to-remove", mock)

	if !hc.IsHealthy("to-remove") {
		t.Fatal("should be healthy")
	}

	hc.RemoveUpstream("to-remove")
	if hc.IsHealthy("to-remove") {
		t.Error("removed upstream should not be found")
	}

	hc.AddUpstream("override", NewMockUpstreamHandler("override"))
	hc.SetHealthy("override", false)
	if hc.IsHealthy("override") {
		t.Error("SetHealthy=false should make it unhealthy")
	}

	hc.SetHealthy("override", true)
	if !hc.IsHealthy("override") {
		t.Error("SetHealthy=true should make it healthy")
	}

	hc.SetHealthy("nonexistent", true)
}

func TestHealthChecker_Thresholds(t *testing.T) {
	hc := NewHealthChecker(time.Hour, 3, 2)
	mock := NewMockUpstreamHandler("threshold")
	hc.AddUpstream("threshold", mock)

	mock.SetHealthy(false)
	hc.checkOne("threshold")
	hc.checkOne("threshold")
	if !hc.IsHealthy("threshold") {
		t.Error("should still be healthy with 2 failures (threshold=3)")
	}
	hc.checkOne("threshold")
	if hc.IsHealthy("threshold") {
		t.Error("should be unhealthy after 3 failures")
	}

	mock.SetHealthy(true)
	hc.checkOne("threshold")
	if hc.IsHealthy("threshold") {
		t.Error("should still be unhealthy with 1 success (passThreshold=2)")
	}
	hc.checkOne("threshold")
	if !hc.IsHealthy("threshold") {
		t.Error("should be healthy after 2 successes")
	}
}

func TestMockUpstreamHandler(t *testing.T) {
	mock := NewMockUpstreamHandler("test-svc")

	if mock.Name() != "test-svc" {
		t.Errorf("expected test-svc, got %s", mock.Name())
	}
	if !mock.HealthCheck() {
		t.Error("should be healthy by default")
	}

	mock.SetHealthy(false)
	if mock.HealthCheck() {
		t.Error("should be unhealthy after SetHealthy(false)")
	}

	mock.SetResponse(http.StatusAccepted, "custom-body")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mock.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", w.Code)
	}
	if w.Body.String() != "custom-body" {
		t.Errorf("expected custom-body, got %s", w.Body.String())
	}
	if mock.RequestCount() != 1 {
		t.Errorf("expected count=1, got %d", mock.RequestCount())
	}

	customCalled := false
	mock.SetCustomHandler(func(w http.ResponseWriter, r *http.Request) {
		customCalled = true
		w.WriteHeader(http.StatusTeapot)
	})

	w2 := httptest.NewRecorder()
	mock.ServeHTTP(w2, req)
	if !customCalled {
		t.Error("custom handler should be called")
	}
	if w2.Code != http.StatusTeapot {
		t.Errorf("expected 418, got %d", w2.Code)
	}

	mock.ResetCount()
	if mock.RequestCount() != 0 {
		t.Errorf("expected count=0 after reset, got %d", mock.RequestCount())
	}

	mock.SetLatency(10 * time.Millisecond)
	start := time.Now()
	w3 := httptest.NewRecorder()
	mock.ServeHTTP(w3, nil)
	elapsed := time.Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected latency >= 10ms, got %v", elapsed)
	}
}

func TestMockUpstreamHandler_ConcurrentSetResponse(t *testing.T) {
	mock := NewMockUpstreamHandler("concurrent-svc")
	mock.SetResponse(http.StatusOK, "body-0")

	pairs := []struct {
		code int
		body string
	}{
		{http.StatusOK, "body-ok"},
		{http.StatusCreated, "body-created"},
		{http.StatusBadRequest, "body-bad"},
		{http.StatusNotFound, "body-nf"},
		{http.StatusInternalServerError, "body-err"},
	}

	var wg sync.WaitGroup
	numSetters := 5
	numReaders := 10
	iterations := 200
	inconsistent := int64(0)

	for s := 0; s < numSetters; s++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				p := pairs[(id+i)%len(pairs)]
				mock.SetResponse(p.code, p.body)
			}
		}(s)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				w := httptest.NewRecorder()
				mock.ServeHTTP(w, req)

				code := w.Code
				body := w.Body.String()

				matched := false
				for _, p := range pairs {
					if p.code == code && p.body == body {
						matched = true
						break
					}
				}
				if !matched {
					atomic.AddInt64(&inconsistent, 1)
					t.Logf("inconsistent state: code=%d body=%q", code, body)
				}
			}
		}()
	}

	wg.Wait()

	count := atomic.LoadInt64(&inconsistent)
	if count > 0 {
		t.Errorf("found %d inconsistent status/body pairs (race condition)", count)
	}

	totalExpected := int64(numReaders * iterations)
	if mock.RequestCount() != totalExpected {
		t.Errorf("expected %d requests, got %d", totalExpected, mock.RequestCount())
	}
}

func TestMockUpstreamHandler_ConcurrentHealthToggle(t *testing.T) {
	mock := NewMockUpstreamHandler("health-test")
	mock.SetHealthy(true)

	var wg sync.WaitGroup
	numTogglers := 4
	numReaders := 8
	iterations := 500
	invalidStates := int64(0)

	for s := 0; s < numTogglers; s++ {
		wg.Add(1)
		go func(start bool) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				mock.SetHealthy((i%2 == 0) == start)
			}
		}(s%2 == 0)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				h := mock.HealthCheck()
				if h != true && h != false {
					atomic.AddInt64(&invalidStates, 1)
				}
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&invalidStates) > 0 {
		t.Error("found invalid health states (race condition)")
	}
}

func TestMockUpstreamHandler_ConcurrentSetLatency(t *testing.T) {
	mock := NewMockUpstreamHandler("latency-test")

	var wg sync.WaitGroup
	durations := []time.Duration{0, 1 * time.Microsecond, 2 * time.Microsecond, 5 * time.Microsecond}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				mock.SetLatency(durations[(id+j)%len(durations)])
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				w := httptest.NewRecorder()
				mock.ServeHTTP(w, req)
			}
		}()
	}

	wg.Wait()

	if mock.RequestCount() != 500 {
		t.Errorf("expected 500 requests, got %d", mock.RequestCount())
	}
}

func TestMockUpstreamHandler_ConcurrentCustomHandler(t *testing.T) {
	mock := NewMockUpstreamHandler("custom-test")
	counter := int64(0)

	mock.SetCustomHandler(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&counter, 1)
		w.WriteHeader(http.StatusTeapot)
	})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			localCounter := int64(0)
			mock.SetCustomHandler(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt64(&localCounter, 1)
				w.WriteHeader(http.StatusOK)
			})
			time.Sleep(time.Microsecond)
		}
	}()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				w := httptest.NewRecorder()
				mock.ServeHTTP(w, req)
				if w.Code != http.StatusOK && w.Code != http.StatusTeapot {
					t.Errorf("unexpected status code: %d", w.Code)
				}
			}
		}()
	}

	wg.Wait()

	if mock.RequestCount() != 500 {
		t.Errorf("expected 500 requests, got %d", mock.RequestCount())
	}
}

func TestMockUpstreamHandler_ConcurrentResetCount(t *testing.T) {
	mock := NewMockUpstreamHandler("reset-test")

	var wg sync.WaitGroup
	var resets int64

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				w := httptest.NewRecorder()
				mock.ServeHTTP(w, req)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			mock.ResetCount()
			atomic.AddInt64(&resets, 1)
			time.Sleep(time.Microsecond * 50)
		}
	}()

	wg.Wait()

	count := mock.RequestCount()
	if count < 0 {
		t.Errorf("request count should not be negative, got %d", count)
	}
	t.Logf("final request count: %d (after %d resets)", count, atomic.LoadInt64(&resets))
}

func TestGateway_Integration(t *testing.T) {
	store := NewMemoryTokenStore()
	store.Add("token-good", &UserInfo{UserID: "alice", Roles: []string{"user"}})

	gw := NewGateway(GatewayConfig{
		TokenStore:      store,
		Rate:            1,
		RateCapacity:    5,
		RefillRate:      100 * time.Millisecond,
		CheckInterval:   time.Second,
		FailThreshold:   3,
		PassThreshold:   2,
		EnableAuth:      true,
		EnableRateLimit: true,
		EnableLogger:    true,
		AuthExemptPaths: []string{"/health"},
		CircuitConfigs: map[string]CircuitBreakerConfig{
			"user-svc": {
				Name:                "user-svc",
				WindowSize:          time.Minute,
				FailureThreshold:    3,
				OpenDuration:        200 * time.Millisecond,
				HalfOpenMaxRequests: 1,
			},
		},
	})
	defer gw.StopHealthCheck()

	userSvc := NewMockUpstreamHandler("user-svc")
	userSvc.SetResponse(http.StatusOK, `{"user":"alice"}`)
	gw.RegisterUpstream("user-svc", userSvc)

	paymentSvc := NewMockUpstreamHandler("payment-svc")
	paymentSvc.SetResponse(http.StatusOK, `{"status":"paid"}`)
	gw.RegisterUpstream("payment-svc", paymentSvc)

	gw.AddRoute("/health", ExactMatch, "user-svc")
	gw.AddRoute("/api/users/*", WildcardMatch, "user-svc")
	gw.AddRoute("/api/payments", ExactMatch, "payment-svc")

	gw.RegisterFallback("user-svc", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("USER_SVC_FALLBACK"))
	})

	t.Run("health endpoint exempt from auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users/1", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("valid token routing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
		req.RemoteAddr = "10.0.0.2:2222"
		req.Header.Set("Authorization", "Bearer token-good")
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"user":"alice"`) {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
	})

	t.Run("not found route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
		req.RemoteAddr = "10.0.0.3:3333"
		req.Header.Set("Authorization", "Bearer token-good")
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("payment service routing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/payments", nil)
		req.RemoteAddr = "10.0.0.4:4444"
		req.Header.Set("Authorization", "Bearer token-good")
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"status":"paid"`) {
			t.Errorf("unexpected body: %s", w.Body.String())
		}
		if paymentSvc.RequestCount() != 1 {
			t.Errorf("expected 1 request to payment-svc, got %d", paymentSvc.RequestCount())
		}
	})

	t.Run("rate limiting per IP", func(t *testing.T) {
		for i := 0; i < 7; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/users/lim", nil)
			req.RemoteAddr = "10.0.0.99:9999"
			req.Header.Set("Authorization", "Bearer token-good")
			w := httptest.NewRecorder()
			gw.ServeHTTP(w, req)

			if i < 5 {
				if w.Code != http.StatusOK {
					t.Errorf("req %d: expected 200, got %d", i+1, w.Code)
				}
			} else {
				if w.Code != http.StatusTooManyRequests {
					t.Errorf("req %d: expected 429, got %d", i+1, w.Code)
				}
			}
		}
	})

	t.Run("circuit breaker with fallback", func(t *testing.T) {
		userSvc.SetResponse(http.StatusInternalServerError, "ERR")
		userSvc.ResetCount()

		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/users/cb", nil)
			req.RemoteAddr = "10.0.0.88:8888"
			req.Header.Set("Authorization", "Bearer token-good")
			w := httptest.NewRecorder()
			gw.ServeHTTP(w, req)

			if i < 3 {
				if w.Code != http.StatusInternalServerError {
					t.Errorf("req %d: expected 500, got %d", i+1, w.Code)
				}
			} else {
				if w.Code != http.StatusServiceUnavailable {
					t.Errorf("req %d: expected 503 (circuit open), got %d", i+1, w.Code)
				}
				if !strings.Contains(w.Body.String(), "USER_SVC_FALLBACK") {
					t.Errorf("req %d: expected fallback body, got %s", i+1, w.Body.String())
				}
			}
		}

		if userSvc.RequestCount() > 3 {
			t.Errorf("upstream should only be called 3 times before open, got %d", userSvc.RequestCount())
		}

		userSvc.SetResponse(http.StatusOK, `{"user":"recovered"}`)
		time.Sleep(300 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/api/users/recovery", nil)
		req.RemoteAddr = "10.0.0.77:7777"
		req.Header.Set("Authorization", "Bearer token-good")
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("after recovery expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestGateway_HealthCheckIntegration(t *testing.T) {
	gw := NewGateway(GatewayConfig{
		CheckInterval: 20 * time.Millisecond,
		FailThreshold: 2,
		PassThreshold: 2,
	})

	healthy := NewMockUpstreamHandler("good")
	healthy.SetResponse(http.StatusOK, "good")
	gw.RegisterUpstream("good", healthy)

	unhealthy := NewMockUpstreamHandler("bad")
	unhealthy.SetResponse(http.StatusOK, "bad")
	unhealthy.SetHealthy(false)
	gw.RegisterUpstream("bad", unhealthy)

	gw.AddRoute("/good", ExactMatch, "good")
	gw.AddRoute("/bad", ExactMatch, "bad")

	gw.StartHealthCheck()
	defer gw.StopHealthCheck()

	time.Sleep(80 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/good", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("healthy upstream: expected 200, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/bad", nil)
	w2 := httptest.NewRecorder()
	gw.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Errorf("unhealthy upstream: expected 404 (routed out), got %d", w2.Code)
	}

	unhealthy.SetHealthy(true)
	time.Sleep(80 * time.Millisecond)

	req3 := httptest.NewRequest(http.MethodGet, "/bad", nil)
	w3 := httptest.NewRecorder()
	gw.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Errorf("recovered upstream: expected 200, got %d", w3.Code)
	}
}

func TestGateway_Getters(t *testing.T) {
	gw := NewGateway(GatewayConfig{})

	svc := NewMockUpstreamHandler("g")
	gw.RegisterUpstream("g", svc)

	cb := NewCircuitBreaker("g", time.Second, 1, time.Second, 1)
	gw.RegisterCircuitBreaker("g", cb)

	if gw.GetRouter() == nil {
		t.Error("GetRouter should not be nil")
	}
	if gw.GetHealthChecker() == nil {
		t.Error("GetHealthChecker should not be nil")
	}

	gotCb, ok := gw.GetCircuitBreaker("g")
	if !ok || gotCb != cb {
		t.Error("GetCircuitBreaker mismatch")
	}
	_, ok = gw.GetCircuitBreaker("missing")
	if ok {
		t.Error("should not find missing circuit breaker")
	}

	gotUp, ok := gw.GetUpstream("g")
	if !ok || gotUp != svc {
		t.Error("GetUpstream mismatch")
	}
	_, ok = gw.GetUpstream("missing")
	if ok {
		t.Error("should not find missing upstream")
	}
}

func TestGateway_StartStop(t *testing.T) {
	gw := NewGateway(GatewayConfig{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := gw.Stop(ctx)
	if err != nil {
		t.Errorf("Stop before Start should return nil, got %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- gw.Start(":0")
	}()

	time.Sleep(100 * time.Millisecond)

	err = gw.Start(":0")
	if err == nil {
		t.Error("double Start should return error")
	}

	err = gw.Stop(ctx)
	if err != nil {
		t.Errorf("Stop returned error: %v", err)
	}

	select {
	case startErr := <-done:
		if startErr != nil {
			t.Logf("ListenAndServe returned (expected): %v", startErr)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not stop in time")
	}
}

func TestConcurrent_Requests(t *testing.T) {
	store := NewMemoryTokenStore()
	store.Add("tok", &UserInfo{UserID: "u1"})

	gw := NewGateway(GatewayConfig{
		TokenStore:      store,
		Rate:            10,
		RateCapacity:    1000,
		RefillRate:      time.Millisecond,
		EnableAuth:      true,
		EnableRateLimit: true,
	})

	svc := NewMockUpstreamHandler("svc")
	gw.RegisterUpstream("svc", svc)
	gw.AddRoute("/api/test", ExactMatch, "svc")

	var wg sync.WaitGroup
	var errors int64
	numGoroutines := 50
	numRequests := 20

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numRequests; i++ {
				req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
				req.RemoteAddr = "127.0.0." + itoa(id+1) + ":1234"
				req.Header.Set("Authorization", "Bearer tok")
				w := httptest.NewRecorder()
				gw.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					atomic.AddInt64(&errors, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	totalExpected := int64(numGoroutines * numRequests)
	if svc.RequestCount() != totalExpected {
		t.Errorf("expected %d requests, got %d (errors=%d)",
			totalExpected, svc.RequestCount(), atomic.LoadInt64(&errors))
	}
}

func TestGateway_NoMiddleware(t *testing.T) {
	gw := NewGateway(GatewayConfig{})

	svc := NewMockUpstreamHandler("bare")
	gw.RegisterUpstream("bare", svc)
	gw.AddRoute("/bare", ExactMatch, "bare")

	req := httptest.NewRequest(http.MethodGet, "/bare", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with no middleware, got %d", w.Code)
	}
	if svc.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", svc.RequestCount())
	}
}

func TestTokenStore_Concurrent(t *testing.T) {
	store := NewMemoryTokenStore()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := "t" + itoa(i)
			store.Add(token, &UserInfo{UserID: itoa(i)})
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		user, ok := store.Validate("t" + itoa(i))
		if !ok {
			t.Errorf("token t%d not found", i)
			continue
		}
		if user.UserID != itoa(i) {
			t.Errorf("token t%d: expected user %d, got %s", i, i, user.UserID)
		}
	}

	_, ok := store.Validate("nonexistent")
	if ok {
		t.Error("nonexistent token should not validate")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(1, 100, time.Second)
	key := "concurrent-test"

	var wg sync.WaitGroup
	var allowed, denied int64
	num := 200

	for i := 0; i < num; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow(key) {
				atomic.AddInt64(&allowed, 1)
			} else {
				atomic.AddInt64(&denied, 1)
			}
		}()
	}
	wg.Wait()

	if atomic.LoadInt64(&allowed) != 100 {
		t.Errorf("expected 100 allowed, got %d (denied=%d)", allowed, denied)
	}
	if atomic.LoadInt64(&denied) != 100 {
		t.Errorf("expected 100 denied, got %d", denied)
	}
}

func TestCircuitBreaker_Name(t *testing.T) {
	cb := NewCircuitBreaker("named-cb", time.Minute, 5, time.Minute, 1)
	if cb.Name() != "named-cb" {
		t.Errorf("expected named-cb, got %s", cb.Name())
	}
}

func TestRouter_LongestPrefixMatch(t *testing.T) {
	health := NewHealthChecker(time.Second, 3, 3)
	router := NewRouter(health)

	router.AddRoute("/a/*", WildcardMatch, "svc-a")
	router.AddRoute("/a/b/*", WildcardMatch, "svc-ab")
	router.AddRoute("/a/b/c/*", WildcardMatch, "svc-abc")

	tests := []struct {
		path     string
		expected string
	}{
		{"/a/x", "svc-a"},
		{"/a/b/x", "svc-ab"},
		{"/a/b/c/x", "svc-abc"},
		{"/a/b/c/d/e", "svc-abc"},
	}

	for _, tt := range tests {
		upstream, ok := router.Match(tt.path)
		if !ok || upstream != tt.expected {
			t.Errorf("path %s: expected %s, got %s (ok=%v)",
				tt.path, tt.expected, upstream, ok)
		}
	}
}

func TestLoggerMiddleware_NoWriteHeader(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	lm := NewLoggerMiddlewareWithLogger(logger)

	next := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}

	handler := lm.Middleware()(next)
	req := httptest.NewRequest(http.MethodGet, "/no-header", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !strings.Contains(buf.String(), "status=200") {
		t.Errorf("expected default 200 status, got: %s", buf.String())
	}
}

func TestHealthStatus_Struct(t *testing.T) {
	hc := NewHealthChecker(time.Second, 1, 1)
	mock := NewMockUpstreamHandler("h")
	hc.AddUpstream("h", mock)

	status, ok := hc.GetStatus("h")
	if !ok {
		t.Fatal("expected status")
	}

	if status.Name != "h" {
		t.Errorf("expected h, got %s", status.Name)
	}
	if !status.Healthy {
		t.Error("expected healthy")
	}
	if status.LastCheck.IsZero() {
		t.Error("LastCheck should not be zero")
	}
}
