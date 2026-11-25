package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestHTTPE2EAuthAndProducts(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	authSvc := auth.NewService(sqlDB, "e2e-secret")
	prodRepo := management.NewSQLiteRepository(sqlDB)

	r := chi.NewMux()
	r.Use(authSvc.RequireAuth)
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	api := humachi.New(r, huma.DefaultConfig("Kiwit E2E", "0.1.0"))
	auth.RegisterRoutes(api, authSvc)
	management.RegisterRoutes(api, prodRepo)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// 1. Healthz works without auth
	res, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	// 2. Protected route rejects unauthenticated requests
	res, err = http.Get(srv.URL + "/api/v1/products")
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)

	// 3. Register + Login
	regBody, _ := json.Marshal(map[string]string{
		"email":    "e2e@kiwit.local",
		"username": "e2e",
		"password": "password123",
	})
	res, err = http.Post(srv.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "e2e@kiwit.local",
		"password": "password123",
	})
	res, err = http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	require.NoError(t, err)
	var loginOut struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&loginOut))
	_ = res.Body.Close()
	require.NotEmpty(t, loginOut.AccessToken)

	// 4. Create Product with Bearer JWT
	prodBody, _ := json.Marshal(map[string]string{"name": "E2E Product"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/products", bytes.NewReader(prodBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginOut.AccessToken)
	res, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
}
