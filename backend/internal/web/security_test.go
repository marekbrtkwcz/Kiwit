package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/menu", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %s: expected %q, got %q", k, v, got)
		}
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" || !strings.Contains(got, "frame-ancestors 'none'") {
		t.Errorf("expected baseline CSP with frame-ancestors 'none', got %q", got)
	}
	if got := rec.Header().Get("Permissions-Policy"); got == "" {
		t.Error("expected Permissions-Policy header")
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "blob:") {
		t.Errorf("CSP img-src must allow blob: for the screenshot gallery, got %q", got)
	}
}

func TestSecurityHeadersDocsExemptFromCSP(t *testing.T) {
	h := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))
	if got := rec.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("/docs must be CSP-exempt (Swagger UI loads CDN script), got %q", got)
	}
	// Other hardening headers still apply on /docs
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("/docs must still send X-Frame-Options, got %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("/docs must still send nosniff, got %q", got)
	}
}
