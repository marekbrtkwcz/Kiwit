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
	mu           sync.Mutex
	limit        int
	window       time.Duration
	entries      map[string][]time.Time
	trustedCIDRs []*net.IPNet // proxies whose X-Forwarded-For we trust
	lastSweep    time.Time
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

// SetTrustedProxies registers additional CIDRs (e.g. "10.0.0.0/8") whose
// X-Forwarded-For header is trusted. Loopback/link-local are always trusted.
func (rl *RateLimiter) SetTrustedProxies(cidrs ...string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			rl.trustedCIDRs = append(rl.trustedCIDRs, n)
		}
	}
}

func (rl *RateLimiter) allow(ip string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Periodically drop entries whose timestamps all fell out of the window,
	// so the map does not grow unboundedly (memory leak).
	if now.Sub(rl.lastSweep) >= rl.window {
		rl.sweepLocked(now)
	}

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

// sweepLocked removes expired entries. Caller must hold rl.mu.
func (rl *RateLimiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-rl.window)
	for ip, times := range rl.entries {
		if len(times) == 0 || times[len(times)-1].Before(cutoff) {
			delete(rl.entries, ip)
		}
	}
	rl.lastSweep = now
}

// LimitAuthRoutes applies rate limiting to sensitive unauthenticated routes:
// /api/v1/auth/login, /api/v1/auth/register, /api/v1/auth/ldap-login and the
// password-reset pair.
func (rl *RateLimiter) LimitAuthRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost &&
			(r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/register" ||
				r.URL.Path == "/api/v1/auth/ldap-login" ||
				r.URL.Path == "/api/v1/auth/password-reset" || r.URL.Path == "/api/v1/auth/password-reset/confirm") {
			ip := rl.clientIP(r)
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

// clientIP returns the client IP. X-Forwarded-For is honored only when the
// direct peer is a trusted proxy (loopback/link-local or a configured CIDR);
// otherwise the header is trivially spoofable and would bypass the limit.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	if rl.isTrustedProxy(host) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if parts := strings.Split(xff, ","); len(parts) > 0 {
				if ip := strings.TrimSpace(parts[0]); ip != "" {
					return ip
				}
			}
		}
	}
	return host
}

func (rl *RateLimiter) isTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for _, cidr := range rl.trustedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
