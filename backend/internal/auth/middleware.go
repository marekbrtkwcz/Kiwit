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

// IsAPIToken reports whether the current request was authenticated with a kiwit_* API token.
func IsAPIToken(ctx context.Context) bool {
	m, _ := ctx.Value(authMethodContextKey).(string)
	return m == "api_token"
}

// CheckRole validates that the authenticated user in ctx has one of the allowed roles (admin always passes).
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

// RequireAuth protects /api/v1/* routes (except login and register) using JWT or kiwit_* API tokens.
func (s *Service) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") ||
			r.URL.Path == "/api/v1/auth/login" ||
			r.URL.Path == "/api/v1/auth/register" {
			next.ServeHTTP(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"title":"Unauthorized","status":401,"detail":"missing bearer token"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		u, err := s.AuthenticateBearer(r.Context(), token)
		if err != nil {
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
