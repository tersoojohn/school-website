package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Limiter is a small in-memory sliding-window limiter, suitable for a single server process.
type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{hits: make(map[string][]time.Time), limit: limit, window: window}
}

func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.hits) > 10_000 {
		for k, v := range l.hits {
			if len(v) == 0 || v[len(v)-1].Before(cutoff) {
				delete(l.hits, k)
			}
		}
	}
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	return true
}

// Middleware limits POST requests per client IP.
func (l *Limiter) Middleware(trustProxy bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && !l.Allow(ClientIP(r, trustProxy)) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Too many attempts. Wait a minute and try again.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIP returns the peer address. X-Real-IP is honoured only when the server is
// deliberately configured to sit behind a trusted reverse proxy.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
