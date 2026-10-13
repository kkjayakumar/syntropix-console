package httpserver

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type ipRateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	max     int
	window  time.Duration
}

func newIPRateLimiter(max int, window time.Duration) *ipRateLimiter {
	if max <= 0 {
		max = 120
	}
	if window <= 0 {
		window = time.Minute
	}
	return &ipRateLimiter{hits: make(map[string][]time.Time), max: max, window: window}
}

func (l *ipRateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	var kept []time.Time
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) rateLimitWebhooks(next http.Handler) http.Handler {
	limiter := newIPRateLimiter(s.cfg.WebhookRateLimit, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
