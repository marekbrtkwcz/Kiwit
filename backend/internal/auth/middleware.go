package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

type contextKey string

const (
	userContextKey       contextKey = "auth_user"
	authMethodContextKey contextKey = "auth_method"
)

// UserFromContext returns the authenticated user stored by RequireAuth, if any.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userContextKey).(User)
	return u, ok
}

// WithUser returns a context carrying the given user (used by XML-RPC session auth).
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

// RoleFromContext returns the role of the authenticated user in ctx, if any.
func RoleFromContext(ctx context.Context) (string, bool) {
	u, ok := UserFromContext(ctx)
	if !ok {
		return "", false
	}
	return u.Role, true
}

// IsAPIToken reports whether the current request was authenticated with a kiwit_* API token.
func IsAPIToken(ctx context.Context) bool {
	m, _ := ctx.Value(authMethodContextKey).(string)
	return m == "api_token"
}

// CheckRole validates that the authenticated user in ctx has one of the allowed roles (admin always passes).
// The "anonymous" role is allowed only when explicitly listed — it grants read-only access.
// Returns nil when no auth context is present only if RequireAuth is not active? No: fails closed when missing.
func CheckRole(ctx context.Context, allowed ...string) error {
	u, ok := UserFromContext(ctx)
	if !ok {
		return huma.Error401Unauthorized("unauthorized")
	}
	if u.Role == "admin" {
		return nil
	}
	for _, role := range allowed {
		if u.Role == role {
			return nil
		}
	}
	return huma.Error403Forbidden("insufficient role")
}

// CheckWriteRole validates that the user can perform write operations.
// Anonymous users are always rejected for writes (read-only access).
func CheckWriteRole(ctx context.Context, allowed ...string) error {
	u, ok := UserFromContext(ctx)
	if !ok {
		return huma.Error401Unauthorized("unauthorized")
	}
	if u.Role == "anonymous" {
		return huma.Error403Forbidden("anonymous users have read-only access")
	}
	return CheckRole(ctx, allowed...)
}

// IsAnonymous reports whether the context user is the anonymous read-only user.
func IsAnonymous(ctx context.Context) bool {
	u, ok := UserFromContext(ctx)
	return ok && u.Role == "anonymous"
}

// AnonymousUser is the synthetic read-only user injected when anonymous access is enabled.
var AnonymousUser = User{
	ID:       0,
	Email:    "",
	Username: "AnonymousUser",
	Role:     "anonymous",
}

// publicAuthPaths are /api/v1/* routes reachable without a bearer token:
// everything needed by the pre-login UI (captcha, auth method discovery,
// LDAP/SSO login, menu, public settings). The password-reset pair is handled
// separately: it is public only when no bearer is sent, so API tokens are
// still detected and rejected by the handlers.
var publicAuthPaths = map[string]bool{
	"/api/v1/auth/login":       true,
	"/api/v1/auth/register":    true,
	"/api/v1/auth/captcha":     true,
	"/api/v1/auth/methods":     true,
	"/api/v1/auth/ldap-login":  true,
	"/api/v1/menu":             true,
	"/api/v1/auth/sso/login":   true,
	"/api/v1/auth/sso/callback": true,
	"/api/v1/auth/sso/config":  true,
}

// isPublicSettingsPath reports whether a GET on a settings route is public.
// GET /api/v1/settings and GET /api/v1/settings/{key} are public — the
// handler itself only exposes keys marked Public and enforces admin for the
// rest (fail-closed via UserFromContext).
func isPublicSettingsPath(method, path string) bool {
	if method != http.MethodGet {
		return false
	}
	return path == "/api/v1/settings" || path == "/api/v1/settings/domain" ||
		strings.HasPrefix(path, "/api/v1/settings/")
}

// RequireAuth protects /api/v1/* routes using JWT or kiwit_* API tokens.
// Public (bypass) paths: login, register, captcha, auth methods, LDAP/SSO
// login, menu, public settings and the password-reset pair — the reset paths
// still authenticate when a valid bearer is provided, so API tokens can be
// detected and rejected by the handlers.
// When anonymous access is enabled (SetAnonymousAccess(true)), requests
// without a bearer token are allowed through with the AnonymousUser in
// context — handlers must still enforce read-only via CheckRole.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") ||
			publicAuthPaths[r.URL.Path] ||
			isPublicSettingsPath(r.Method, r.URL.Path) ||
			((r.URL.Path == "/api/v1/auth/password-reset" ||
				r.URL.Path == "/api/v1/auth/password-reset/confirm") && header == "") {
			next.ServeHTTP(w, r)
			return
		}

		if !strings.HasPrefix(header, "Bearer ") {
			if s.anonymousAccess && r.Method == http.MethodGet {
				// Anonymous read-only access: inject AnonymousUser into context
				ctx := context.WithValue(r.Context(), userContextKey, AnonymousUser)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"title":"Unauthorized","status":401,"detail":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		u, err := s.AuthenticateBearer(r.Context(), token)
		if err != nil {
			if s.anonymousAccess && r.Method == http.MethodGet {
				ctx := context.WithValue(r.Context(), userContextKey, AnonymousUser)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"title":"Unauthorized","status":401,"detail":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		method := "jwt"
		if strings.HasPrefix(token, "kiwit_") {
			method = "api_token"
		}
		ctx := context.WithValue(r.Context(), userContextKey, u)
		ctx = context.WithValue(ctx, authMethodContextKey, method)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole enforces that the authenticated user has one of the allowed roles (admin always passes).
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"title":"Unauthorized","status":401}`, http.StatusUnauthorized)
				return
			}
			if u.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			for _, role := range allowed {
				if u.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"title":"Forbidden","status":403,"detail":"insufficient role"}`, http.StatusForbidden)
		})
	}
}
