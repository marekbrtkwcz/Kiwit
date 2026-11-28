package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestRateLimiterLimitAndRetryAfter(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	h := rl.LimitAuthRoutes(okHandler())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header")
	}

	// A different IP is unaffected
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "203.0.113.11:1234"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("other IP should pass, got %d", rec.Code)
	}

	// Non-auth routes are not limited
	for i := 0; i < 10; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/testcases", nil)
		req.RemoteAddr = "203.0.113.10:1234"
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("non-auth route limited: got %d", rec.Code)
		}
	}
}

func TestRateLimiterXFFSpoofing(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	h := rl.LimitAuthRoutes(okHandler())

	// Untrusted remote sending a spoofed X-Forwarded-For: must be keyed by
	// RemoteAddr, so rotating the fake XFF does not bypass the limit.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "198.51.100.7:9999"
		req.Header.Set("X-Forwarded-For", "10.0.0."+string(rune('1'+i)))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "198.51.100.7:9999"
	req.Header.Set("X-Forwarded-For", "10.0.0.99") // spoof attempt
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF bypassed the limit: got %d", rec.Code)
	}
}

func TestRateLimiterTrustedProxy(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	rl.SetTrustedProxies("10.0.0.0/8")
	h := rl.LimitAuthRoutes(okHandler())

	// Peer is a trusted proxy (loopback): XFF is honored, so different
	// forwarded clients get separate buckets.
	for i, client := range []string{"192.0.2.1", "192.0.2.2"} {
		for j := 0; j < 2; j++ {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
			req.RemoteAddr = "127.0.0.1:5555"
			req.Header.Set("X-Forwarded-For", client)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("client %s request %d: expected 200, got %d", client, j, rec.Code)
			}
			_ = i
		}
	}
	// Third request from the first client is limited
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for repeated forwarded client, got %d", rec.Code)
	}
}

func TestRateLimiterSweep(t *testing.T) {
	rl := NewRateLimiter(100, time.Minute)
	now := time.Now()
	rl.allow("203.0.113.50", now)
	rl.allow("203.0.113.51", now)
	rl.mu.Lock()
	n := len(rl.entries)
	rl.mu.Unlock()
	if n != 2 {
		t.Fatalf("expected 2 entries, got %d", n)
	}
	// Advance beyond one window: the sweep must drop both expired entries.
	rl.allow("203.0.113.52", now.Add(2*time.Hour))
	rl.mu.Lock()
	n = len(rl.entries)
	rl.mu.Unlock()
	if n != 1 { // only 52 remains
		t.Fatalf("expected sweep to remove expired entries (1 left), got %d", n)
	}
}
