package gateway

import (
	"bufio"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type LoggerMiddleware struct {
	logger *log.Logger
}

func NewLoggerMiddleware() *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: log.New(os.Stdout, "[GATEWAY] ", log.LstdFlags),
	}
}

func NewLoggerMiddlewareWithLogger(l *log.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{logger: l}
}

func (l *LoggerMiddleware) Middleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next(rw, r)
			duration := time.Since(start)
			l.logger.Printf("method=%s path=%s status=%d duration=%s",
				r.Method, r.URL.Path, rw.statusCode, duration)
		}
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	written     bool
	hijacker    http.Hijacker
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	if !l.written {
		l.statusCode = code
		l.written = true
	}
	l.ResponseWriter.WriteHeader(code)
}

func (l *loggingResponseWriter) Write(b []byte) (int, error) {
	if !l.written {
		l.written = true
	}
	return l.ResponseWriter.Write(b)
}

func (l *loggingResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := l.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("hijack not supported")
	}
	return h.Hijack()
}

func (l *loggingResponseWriter) Flush() {
	if f, ok := l.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type RequestLog struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
	IP       string
	UserID   string
}

type LogCollector struct {
	logs   []RequestLog
	logsMu sync.Mutex
}

func NewLogCollector() *LogCollector {
	return &LogCollector{logs: make([]RequestLog, 0)}
}

func (c *LogCollector) Add(log RequestLog) {
	c.logsMu.Lock()
	defer c.logsMu.Unlock()
	c.logs = append(c.logs, log)
}

func (c *LogCollector) GetLogs() []RequestLog {
	c.logsMu.Lock()
	defer c.logsMu.Unlock()
	result := make([]RequestLog, len(c.logs))
	copy(result, c.logs)
	return result
}

func (c *LogCollector) Clear() {
	c.logsMu.Lock()
	defer c.logsMu.Unlock()
	c.logs = c.logs[:0]
}

func (c *LogCollector) Middleware() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next(rw, r)
			duration := time.Since(start)

			userID := ""
			if user, ok := UserFromContext(r.Context()); ok {
				userID = user.UserID
			}

			c.Add(RequestLog{
				Method:   r.Method,
				Path:     r.URL.Path,
				Status:   rw.statusCode,
				Duration: duration,
				IP:       extractIP(r),
				UserID:   userID,
			})
		}
	}
}
