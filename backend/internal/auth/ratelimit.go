package auth

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter implements a simple per-IP sliding-window limiter for sensitive auth routes.
type RateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = 20
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string][]time.Time),
	}
}

func (rl *RateLimiter) allow(ip string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := now.Add(-rl.window)
	times := rl.entries[ip]
	kept := times[:0]
	for _, ts := range times {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= rl.limit {
		rl.entries[ip] = kept
		return false
	}
	rl.entries[ip] = append(kept, now)
	return true
}

// LimitAuthRoutes applies rate limiting to sensitive unauthenticated routes:
// /api/v1/auth/login, /api/v1/auth/register and the password-reset pair.
func (rl *RateLimiter) LimitAuthRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost &&
			(r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/register" ||
				r.URL.Path == "/api/v1/auth/password-reset" || r.URL.Path == "/api/v1/auth/password-reset/confirm") {
			ip := clientIP(r)
			if !rl.allow(ip, time.Now()) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
				http.Error(w, `{"title":"Too Many Requests","status":429,"detail":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if parts := strings.Split(xff, ","); len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
