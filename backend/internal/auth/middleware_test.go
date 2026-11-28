package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func newTestService(t *testing.T) *auth.Service {
	t.Helper()
	sqlDB, err := db.OpenSQLite(context.Background(), "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return auth.NewService(sqlDB, "test-jwt-secret")
}

// recorder returns a handler that captures the request context user.
func recorder(roleSeen *string, code *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if role, ok := auth.RoleFromContext(r.Context()); ok {
			*roleSeen = role
		}
		w.WriteHeader(http.StatusOK)
		*code = http.StatusOK
	})
}

func TestRequireAuthPublicPaths(t *testing.T) {
	svc := newTestService(t)
	public := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodPost, "/api/v1/auth/register"},
		{http.MethodGet, "/api/v1/auth/captcha"},
		{http.MethodGet, "/api/v1/auth/methods"},
		{http.MethodPost, "/api/v1/auth/ldap-login"},
		{http.MethodGet, "/api/v1/auth/sso/login"},
		{http.MethodGet, "/api/v1/auth/sso/callback"},
		{http.MethodGet, "/api/v1/auth/sso/config"},
		{http.MethodPost, "/api/v1/auth/password-reset"},
		{http.MethodGet, "/api/v1/menu"},
		{http.MethodGet, "/api/v1/settings"},
		{http.MethodGet, "/api/v1/settings/domain"},
		{http.MethodGet, "/api/v1/settings/some_key"},
	}
	for _, p := range public {
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			var role string
			var code int
			h := svc.RequireAuth(recorder(&role, &code))
			req := httptest.NewRequest(p.method, p.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, "public path must bypass auth, body: %s", rec.Body.String())
		})
	}
}

func TestRequireAuthProtectedPaths(t *testing.T) {
	svc := newTestService(t)

	// No token → 401
	var role string
	var code int
	h := svc.RequireAuth(recorder(&role, &code))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/testcases", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Valid token → passes, role readable via the typed context key
	// (the user must exist in the DB — AuthenticateBearer reloads it by sub)
	u, err := svc.Register(context.Background(), "t@kiwit.local", "tester", "supersecret123")
	require.NoError(t, err)
	tok, err := svc.IssueJWT(u)
	require.NoError(t, err)
	role, code = "", 0
	req = httptest.NewRequest(http.MethodGet, "/api/v1/testcases", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, u.Role, role)

	// Non-API path passes through untouched
	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAuthAnonymousAccess(t *testing.T) {
	svc := newTestService(t)
	svc.SetAnonymousAccess(true)

	var role string
	var code int
	h := svc.RequireAuth(recorder(&role, &code))

	// GET without token → anonymous read-only user
	req := httptest.NewRequest(http.MethodGet, "/api/v1/testcases", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "anonymous", role)

	// POST without token → still 401 (anonymous is read-only)
	role, code = "", 0
	req = httptest.NewRequest(http.MethodPost, "/api/v1/testcases", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
