package web

import (
	"net"
	"net/http"
	"strings"
)

// RealIP replaces chi's deprecated middleware.RealIP: it overwrites
// r.RemoteAddr with X-Forwarded-For/X-Real-IP ONLY when the direct peer is a
// trusted proxy (loopback/link-local or one of trustedCIDRs). chi's version
// trusts any peer, which lets clients spoof their IP (GHSA-3fxj-6jh8-hvhx).
func RealIP(trustedCIDRs []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil || host == "" {
				host = r.RemoteAddr
			}
			if isTrustedPeer(host, trustedCIDRs) {
				if ip := forwardedIP(r); ip != "" {
					r.RemoteAddr = ip
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ParseTrustedCIDRs parses CIDR strings, skipping invalid entries.
func ParseTrustedCIDRs(cidrs []string) []*net.IPNet {
	var out []*net.IPNet
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}

func isTrustedPeer(host string, trusted []*net.IPNet) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// forwardedIP returns the leftmost X-Forwarded-For entry, else X-Real-IP.
func forwardedIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if parts := strings.Split(xff, ","); len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}
	return strings.TrimSpace(r.Header.Get("X-Real-IP"))
}
