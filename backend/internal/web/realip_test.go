package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealIPOnlyFromTrustedProxy(t *testing.T) {
	trusted := ParseTrustedCIDRs([]string{"10.0.0.0/8"})
	seen := ""
	h := RealIP(trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.RemoteAddr
		w.WriteHeader(http.StatusOK)
	}))

	// Untrusted direct peer with spoofed XFF — RemoteAddr must stay.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.9:1111"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "203.0.113.9:1111" {
		t.Errorf("untrusted peer: RemoteAddr overwritten: %q", seen)
	}

	// Trusted proxy (loopback) — XFF wins.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:2222"
	req.Header.Set("X-Forwarded-For", "198.51.100.23, 10.0.0.1")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "198.51.100.23" {
		t.Errorf("trusted loopback peer: expected XFF ip, got %q", seen)
	}

	// Configured CIDR proxy — XFF wins.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:3333"
	req.Header.Set("X-Real-IP", "192.0.2.55")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "192.0.2.55" {
		t.Errorf("trusted CIDR peer: expected X-Real-IP, got %q", seen)
	}

	// Invalid CIDR entries are skipped, not fatal.
	if got := ParseTrustedCIDRs([]string{"", "bogus", "10.0.0.0/8"}); len(got) != 1 {
		t.Errorf("expected 1 valid CIDR, got %d", len(got))
	}
}
