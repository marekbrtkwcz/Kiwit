package web

import (
	"net/http"
	"strings"
)

// SecurityHeaders sets baseline security response headers on every request.
// The CSP is a baseline for the embedded SPA: no remote scripts, framing
// denied; style-src allows 'unsafe-inline' because the React build emits
// inline styles; img-src allows blob: for the screenshot gallery
// (URL.createObjectURL). The interactive OpenAPI UI under /docs is exempt
// from CSP because it loads its script from a CDN and uses inline scripts —
// all other headers still apply there.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if !strings.HasPrefix(r.URL.Path, "/docs") {
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}
