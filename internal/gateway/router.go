package gateway

import (
	"net/http"
	"strings"
)

func NewRouter(health *HealthChecker) *Router {
	return &Router{
		routes:    make(map[string]string),
		wildcards: make([]wildcardRoute, 0),
		health:    health,
	}
}

func (r *Router) AddRoute(path string, routeType RouteType, upstream string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch routeType {
	case ExactMatch:
		r.routes[path] = upstream
	case WildcardMatch:
		prefix := strings.TrimSuffix(path, "*")
		r.wildcards = append(r.wildcards, wildcardRoute{
			Prefix:   prefix,
			Upstream: upstream,
		})
	}
}

func (r *Router) Match(path string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if upstream, ok := r.routes[path]; ok {
		if r.isUpstreamHealthy(upstream) {
			return upstream, true
		}
	}

	bestMatch := ""
	bestLen := -1
	for _, wc := range r.wildcards {
		if strings.HasPrefix(path, wc.Prefix) {
			if len(wc.Prefix) > bestLen {
				bestLen = len(wc.Prefix)
				bestMatch = wc.Upstream
			}
		}
	}

	if bestMatch != "" && r.isUpstreamHealthy(bestMatch) {
		return bestMatch, true
	}

	return "", false
}

func (r *Router) isUpstreamHealthy(upstream string) bool {
	if r.health == nil {
		return true
	}
	status, ok := r.health.GetStatus(upstream)
	if !ok {
		return true
	}
	return status.Healthy
}

func (r *Router) Handler(upstreams map[string]UpstreamHandler, circuits map[string]*CircuitBreaker, fallbacks map[string]HandlerFunc) HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		upstreamName, ok := r.Match(req.URL.Path)
		if !ok {
			http.NotFound(w, req)
			return
		}

		handler, exists := upstreams[upstreamName]
		if !exists {
			http.NotFound(w, req)
			return
		}

		cb, hasCb := circuits[upstreamName]
		if hasCb {
			allowed, shouldRecord := cb.Allow()
			if !allowed {
				if fb, ok := fallbacks[upstreamName]; ok {
					fb(w, req)
				} else {
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte("Service Unavailable: Circuit Breaker Open"))
				}
				return
			}

			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			handler.ServeHTTP(rw, req)

			if shouldRecord {
				if rw.statusCode >= 500 {
					cb.RecordFailure()
				} else {
					cb.RecordSuccess()
				}
			}
			return
		}

		handler.ServeHTTP(w, req)
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}
