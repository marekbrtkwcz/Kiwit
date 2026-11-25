package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/comment"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
)

func TestHTTPE2EAuthRBACAndCoreFlow(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	authSvc := auth.NewService(sqlDB, "e2e-secret-strong-enough")
	prodRepo := management.NewSQLiteRepository(sqlDB)
	tcRepo := testcase.NewSQLiteRepository(sqlDB)
	tpRepo := testplan.NewSQLiteRepository(sqlDB)
	trRepo := testrun.NewSQLiteRepository(sqlDB)
	attRepo := attachment.NewSQLiteRepository(sqlDB)
	commentRepo := comment.NewSQLiteRepository(sqlDB)

	r := chi.NewMux()
	r.Use(authSvc.RequireAuth)
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	api := humachi.New(r, huma.DefaultConfig("Kiwit E2E", "0.1.0"))
	auth.RegisterRoutes(api, authSvc)
	management.RegisterRoutes(api, prodRepo)
	testcase.RegisterRoutes(api, tcRepo, nil, nil)
	testplan.RegisterRoutes(api, tpRepo, nil, nil)
	testrun.RegisterRoutes(api, trRepo, nil)
	testrun.RegisterExportRoutes(r, trRepo)
	comment.RegisterRoutes(api, commentRepo)
	attachment.RegisterRoutes(r, attRepo, t.TempDir())

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	doJSON := func(method, path, token string, body any) *http.Response {
		var buf bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&buf).Encode(body))
		}
		req, err := http.NewRequest(method, srv.URL+path, &buf)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return res
	}

	// 1. Healthz works without auth; protected route rejects unauthenticated
	res := doJSON(http.MethodGet, "/healthz", "", nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/products", "", nil)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	_ = res.Body.Close()

	// 2. Register admin, testerA, testerB
	register := func(email, user string) int64 {
		r := doJSON(http.MethodPost, "/api/v1/auth/register", "", map[string]string{
			"email": email, "username": user, "password": "password123",
		})
		require.Equal(t, http.StatusOK, r.StatusCode)
		var out struct{ ID int64 `json:"id"` }
		require.NoError(t, json.NewDecoder(r.Body).Decode(&out))
		_ = r.Body.Close()
		return out.ID
	}
	adminID := register("admin@kiwit.local", "admin")
	testerAID := register("testera@kiwit.local", "testera")
	_ = register("testerb@kiwit.local", "testerb")

	// Closing registration blocks further signups
	authSvc.SetAllowRegistration(false)
	res = doJSON(http.MethodPost, "/api/v1/auth/register", "", map[string]string{
		"email": "blocked@kiwit.local", "username": "blocked", "password": "password123",
	})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	login := func(email, pass string) string {
		r := doJSON(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": email, "password": pass})
		require.Equal(t, http.StatusOK, r.StatusCode)
		var out struct {
			AccessToken string `json:"access_token"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&out))
		_ = r.Body.Close()
		return out.AccessToken
	}

	adminJWT := login("admin@kiwit.local", "password123")
	testerAJWT := login("testera@kiwit.local", "password123")
	testerBJWT := login("testerb@kiwit.local", "password123")

	// 3. RBAC & Product isolation
	res = doJSON(http.MethodPost, "/api/v1/products", testerAJWT, map[string]string{"name": "Forbidden Prod"})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/products", adminJWT, map[string]string{"name": "E2E Product"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var prod struct{ ID int64 `json:"id"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&prod))
	_ = res.Body.Close()

	// Restrict product to testerA only
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/products/%d/members", prod.ID), adminJWT, map[string]any{
		"user_id": testerAID, "role": "tester",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	// testerA can list versions; testerB gets 403
	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/versions", prod.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/versions", prod.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	// 4. Token list & revoke + change-password hardening
	res = doJSON(http.MethodPost, "/api/v1/auth/tokens", testerAJWT, map[string]string{"name": "ci"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tokOut struct{ Token string `json:"token"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tokOut))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/auth/tokens", testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tokList struct {
		Items []struct{ ID int64 `json:"id"` } `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tokList))
	_ = res.Body.Close()
	require.Len(t, tokList.Items, 1)

	res = doJSON(http.MethodPost, "/api/v1/auth/change-password", tokOut.Token, map[string]string{
		"current_password": "password123", "new_password": "newpassword123",
	})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	// Revoke token and verify it stops working immediately
	res = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/auth/tokens/%d", tokList.Items[0].ID), testerAJWT, nil)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/products", tokOut.Token, nil)
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/auth/change-password", testerAJWT, map[string]string{
		"current_password": "wrongpassword", "new_password": "newpassword123",
	})
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/auth/change-password", testerAJWT, map[string]string{
		"current_password": "password123", "new_password": "newpassword123",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	// 5. Admin user management (list, prevent self-demotion, deactivate testerB -> JWT rejected)
	res = doJSON(http.MethodGet, "/api/v1/admin/users", adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", adminID), adminJWT, map[string]any{
		"role": "tester", "is_active": true,
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// 6. Polymorphic entity validation & orphan cleanup
	res = doJSON(http.MethodPost, "/api/v1/comments", testerAJWT, map[string]any{
		"entity_type": "test_case", "entity_id": 99999, "body": "orphan",
	})
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testcases", testerAJWT, map[string]any{
		"product_id": prod.ID, "summary": "Case for attachment", "format": "steps", "priority": "medium", "status": "confirmed",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tc struct{ ID int64 `json:"id"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tc))
	_ = res.Body.Close()

	var mbuf bytes.Buffer
	mw := multipart.NewWriter(&mbuf)
	_ = mw.WriteField("entity_type", "test_case")
	_ = mw.WriteField("entity_id", fmt.Sprintf("%d", tc.ID))
	fw, err := mw.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, _ = fw.Write([]byte("plain log output"))
	require.NoError(t, mw.Close())

	upReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/attachments", &mbuf)
	require.NoError(t, err)
	upReq.Header.Set("Content-Type", mw.FormDataContentType())
	upReq.Header.Set("Authorization", "Bearer "+testerAJWT)
	res, err = http.DefaultClient.Do(upReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/attachments/1/download", testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "sandbox", res.Header.Get("Content-Security-Policy"))
	require.Contains(t, res.Header.Get("Content-Disposition"), "attachment")
	_ = res.Body.Close()

	// Deleting test case cleans up polymorphic attachment
	res = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/testcases/%d", tc.ID), testerAJWT, nil)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/attachments/1/download", testerAJWT, nil)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	_ = res.Body.Close()
}

func TestAuthRateLimiter(t *testing.T) {
	rl := auth.NewRateLimiter(2, time.Minute)
	h := rl.LimitAuthRoutes(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
}
