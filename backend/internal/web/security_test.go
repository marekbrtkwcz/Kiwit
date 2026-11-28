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
}
