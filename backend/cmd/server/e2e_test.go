package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/bug"
	"github.com/kiwit/kiwit/internal/comment"
	"github.com/kiwit/kiwit/internal/issuetracker"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/notification"
	"github.com/kiwit/kiwit/internal/notify"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/telemetry"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
	"github.com/kiwit/kiwit/internal/webhook"
)

func TestHTTPE2E_SQLite(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	runE2ESuite(t, sqlDB, nil)
}

func TestHTTPE2E_Postgres(t *testing.T) {
	dsn := os.Getenv("KIWIT_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("KIWIT_TEST_POSTGRES_URL is not set; skipping PostgreSQL contract E2E suite")
	}
	ctx := context.Background()
	pgPool, err := db.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pgPool.Close)

	// Clean all domain tables so the contract suite is idempotent on a shared Postgres instance.
	_, err = pgPool.Exec(ctx, `
		TRUNCATE TABLE
			internal_bugs, execution_links, bugs, issue_trackers, attachments, comments, email_settings,
			webhooks, user_notification_settings, notifications,
			test_executions, test_runs, test_plan_cases, test_plans,
			test_case_revisions, test_case_steps, test_case_components,
			test_case_tags, test_plan_tags, test_run_tags, test_cases,
			product_members, components, builds, versions, products,
			password_reset_tokens, categories, tags, api_tokens, users
		RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	runE2ESuite(t, nil, pgPool)
}

func runE2ESuite(t *testing.T, sqlDB *sql.DB, pgPool *pgxpool.Pool) {
	t.Helper()

	// Webhook receivers in sections 7-8 are local httptest servers (127.0.0.1),
	// so private targets must be dial-able for the duration of the suite. The
	// safe default is restored afterwards.
	webhook.AllowPrivateTargets = true
	t.Cleanup(func() { webhook.AllowPrivateTargets = false })

	var authSvc *auth.Service
	var prodRepo management.ProductRepository
	var tcRepo testcase.Repository
	var tpRepo testplan.Repository
	var trRepo testrun.Repository
	var attRepo attachment.Repository
	var commentRepo comment.Repository
	var issueRepo issuetracker.Repository

	if pgPool != nil {
		authSvc = auth.NewPostgresService(pgPool, "e2e-secret-strong-enough")
		prodRepo = management.NewPostgresRepository(pgPool)
		tcRepo = testcase.NewPostgresRepository(pgPool)
		tpRepo = testplan.NewPostgresRepository(pgPool)
		trRepo = testrun.NewPostgresRepository(pgPool)
		attRepo = attachment.NewPostgresRepository(pgPool)
		commentRepo = comment.NewPostgresRepository(pgPool)
		issueRepo = issuetracker.NewPostgresRepository(pgPool)
	} else {
		authSvc = auth.NewService(sqlDB, "e2e-secret-strong-enough")
		prodRepo = management.NewSQLiteRepository(sqlDB)
		tcRepo = testcase.NewSQLiteRepository(sqlDB)
		tpRepo = testplan.NewSQLiteRepository(sqlDB)
		trRepo = testrun.NewSQLiteRepository(sqlDB)
		attRepo = attachment.NewSQLiteRepository(sqlDB)
		commentRepo = comment.NewSQLiteRepository(sqlDB)
		issueRepo = issuetracker.NewSQLiteRepository(sqlDB)
	}

	guard := auth.NewAccessGuard(sqlDB, pgPool)
	emailSettings := notify.NewSettingsStore(sqlDB, pgPool)

	r := chi.NewMux()
	r.Use(authSvc.RequireAuth)
	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	api := humachi.New(r, huma.DefaultConfig("Kiwit E2E", "0.1.0"))
	auth.RegisterRoutes(api, authSvc)
	management.RegisterRoutes(api, prodRepo, guard)
	management.RegisterRelationRoutes(api, management.NewRelationStore(sqlDB, pgPool), guard)
	testcase.RegisterRoutes(api, tcRepo, emailSettings, nil, guard)
	testplan.RegisterRoutes(api, tpRepo, emailSettings, nil, guard)
	testrun.RegisterRoutes(api, trRepo, nil, notification.NewStore(sqlDB, pgPool), nil, webhook.NewStore(sqlDB, pgPool), guard)
	testrun.RegisterExportRoutes(r, trRepo, guard)
	issuetracker.RegisterRoutes(api, issueRepo, guard)
	bug.RegisterRoutes(api, bug.NewStore(sqlDB, pgPool), guard)
	telemetry.RegisterRoutes(api, sqlDB, pgPool)
	comment.RegisterRoutes(api, commentRepo, guard)
	notify.RegisterEmailRoutes(api, emailSettings, nil, guard)
	notification.RegisterRoutes(api, notification.NewStore(sqlDB, pgPool))
	webhook.RegisterRoutes(api, webhook.NewStore(sqlDB, pgPool))
	attachment.RegisterRoutes(r, attRepo, t.TempDir(), guard)

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
		var out struct {
			ID int64 `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&out))
		_ = r.Body.Close()
		return out.ID
	}
	adminID := register("admin@kiwit.local", "admin")
	testerAID := register("testera@kiwit.local", "testera")
	testerBID := register("testerb@kiwit.local", "testerb")

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
	var prod struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&prod))
	_ = res.Body.Close()

	// Restrict product to testerA only
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/products/%d/members", prod.ID), adminJWT, map[string]any{
		"user_id": testerAID, "role": "tester",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	// Create version & build as testerA; testerB gets 403 on version/build/plan/case/run
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/products/%d/versions", prod.ID), testerAJWT, map[string]string{"value": "1.0"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ver struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ver))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/versions", prod.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/versions/%d/builds", ver.ID), testerAJWT, map[string]string{"name": "b1"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var bld struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&bld))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/versions/%d/builds", ver.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	// 4. Token list & revoke + change-password hardening
	res = doJSON(http.MethodPost, "/api/v1/auth/tokens", testerAJWT, map[string]string{"name": "ci"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tokOut struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tokOut))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/auth/tokens", testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tokList struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tokList))
	_ = res.Body.Close()
	require.Len(t, tokList.Items, 1)

	res = doJSON(http.MethodPost, "/api/v1/auth/change-password", tokOut.Token, map[string]string{
		"current_password": "password123", "new_password": "newpassword123",
	})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

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

	// 5. Admin user management
	res = doJSON(http.MethodGet, "/api/v1/admin/users", adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/admin/users/%d", adminID), adminJWT, map[string]any{
		"role": "tester", "is_active": true,
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// 6. Full Core TCMS flow + Cross-Product Isolation + SQL Filters + Telemetry Breakdown
	res = doJSON(http.MethodPost, "/api/v1/comments", testerAJWT, map[string]any{
		"entity_type": "test_case", "entity_id": 99999, "body": "orphan",
	})
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testcases", testerBJWT, map[string]any{
		"product_id": prod.ID, "summary": "Should be forbidden", "format": "steps", "priority": "high", "status": "confirmed",
	})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testcases", testerAJWT, map[string]any{
		"product_id": prod.ID, "summary": "Case for attachment", "format": "steps", "priority": "high", "status": "confirmed", "is_automated": true,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tc struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tc))
	_ = res.Body.Close()

	// testerB cannot see testerA's testcase in list, get, revisions, comments, or delete
	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testcases?product_id=%d&is_automated=true", prod.ID), testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tcListB struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tcListB))
	_ = res.Body.Close()
	require.Empty(t, tcListB.Items)
	require.Zero(t, tcListB.Total)

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testcases/%d", tc.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	// testerA sees it with filters
	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testcases?product_id=%d&priority=high&status=confirmed&is_automated=true", prod.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tcListA struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tcListA))
	_ = res.Body.Close()
	require.Len(t, tcListA.Items, 1)
	require.Equal(t, int64(1), tcListA.Total)

	// TestPlan & TestRun + Execution + Telemetry scoping
	res = doJSON(http.MethodPost, "/api/v1/testplans", testerAJWT, map[string]any{
		"product_id": prod.ID, "version_id": ver.ID, "name": "Regression Plan", "plan_type": "smoke",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tp struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tp))
	_ = res.Body.Close()

	// Phase 3: update TestPlan
	res = doJSON(http.MethodPut, fmt.Sprintf("/api/v1/testplans/%d", tp.ID), testerAJWT, map[string]any{
		"name": "Regression Plan Updated", "plan_type": "regression",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testplans/%d/cases", tp.ID), testerAJWT, map[string]any{
		"case_ids": []int64{tc.ID},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPut, fmt.Sprintf("/api/v1/testplans/%d/cases/order", tp.ID), testerAJWT, map[string]any{
		"case_ids": []int64{tc.ID},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testruns", testerAJWT, map[string]any{
		"test_plan_id": tp.ID, "build_id": bld.ID, "summary": "Nightly Run", "default_tester_id": testerAID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tr struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tr))
	_ = res.Body.Close()

	// Phase 3: execution links, bulk update, sync-case
	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testruns/%d/executions", tr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var exList struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&exList))
	_ = res.Body.Close()
	require.Len(t, exList.Items, 1)
	execID := exList.Items[0].ID

	res = doJSON(http.MethodPatch, "/api/v1/executions/bulk", testerAJWT, map[string]any{
		"ids": []int64{execID}, "status": "passed", "assignee_id": testerAID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/executions/%d/links", execID), testerAJWT, map[string]string{
		"name": "CI Log", "url": "https://ci.example.com/job/123",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var linkOut struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&linkOut))
	_ = res.Body.Close()

	res = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/executions/%d/links/%d", execID, linkOut.ID), testerAJWT, nil)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/executions/%d/sync-case", execID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	// Phase 4: built-in Bug Tracker with product_members isolation + JUnit auto_create_cases
	res = doJSON(http.MethodPost, "/api/v1/bugs", testerAJWT, map[string]any{
		"product_id": prod.ID, "test_execution_id": execID, "summary": "Login 500 error", "severity": "high",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ibug struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ibug))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/bugs", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var bugListB struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&bugListB))
	_ = res.Body.Close()
	require.Empty(t, bugListB.Items)

	res = doJSON(http.MethodPost, "/api/v1/testruns/import/junit", testerAJWT, map[string]any{
		"test_plan_id": tp.ID,
		"build_id":     bld.ID,
		"summary":      "CI Auto Import",
		"xml":          `<testsuite name="ci"><testcase name="Auto Created Case"/></testsuite>`,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var junitOut struct {
		CreatedCases int `json:"created_cases"`
		TestRun      struct {
			ID int64 `json:"id"`
		} `json:"test_run"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&junitOut))
	_ = res.Body.Close()
	require.Equal(t, 1, junitOut.CreatedCases)
	_ = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/testruns/%d", junitOut.TestRun.ID), testerAJWT, nil)

	// testerB cannot see testplans, testruns, executions, or export
	res = doJSON(http.MethodGet, "/api/v1/testplans", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tpListB struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tpListB))
	_ = res.Body.Close()
	require.Empty(t, tpListB.Items)

	res = doJSON(http.MethodGet, "/api/v1/testruns?status=running", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var trListB struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&trListB))
	_ = res.Body.Close()
	require.Empty(t, trListB.Items)

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testruns/%d/executions", tr.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testruns/%d/export?format=csv", tr.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	// Telemetry is scoped by product_members: testerB sees 0, testerA sees 1
	res = doJSON(http.MethodGet, "/api/v1/telemetry/summary", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var telB telemetry.Summary
	require.NoError(t, json.NewDecoder(res.Body).Decode(&telB))
	_ = res.Body.Close()
	require.Zero(t, telB.TestCases)
	require.Zero(t, telB.TestRuns)

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/telemetry/breakdown?product_id=%d", prod.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var bd telemetry.Breakdown
	require.NoError(t, json.NewDecoder(res.Body).Decode(&bd))
	_ = res.Body.Close()
	require.Equal(t, int64(2), bd.Automated)
	require.Equal(t, int64(0), bd.Manual)

	// Upload attachment as testerA; testerB gets 403 on download
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
	var att struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&att))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d/download", att.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d/download", att.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "sandbox", res.Header.Get("Content-Security-Policy"))
	require.Contains(t, res.Header.Get("Content-Disposition"), "attachment")
	_ = res.Body.Close()

	// Deleting test run & test case cleans up polymorphic attachment
	res = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/testruns/%d", tr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/testcases/%d", tc.ID), testerAJWT, nil)
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/attachments/%d/download", att.ID), testerAJWT, nil)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	_ = res.Body.Close()

	// In-app notification center: run stopped & execution failed emitted over REST
	res = doJSON(http.MethodPost, "/api/v1/testcases", testerAJWT, map[string]any{
		"product_id": prod.ID, "summary": "Notify case", "format": "steps", "priority": "low", "status": "confirmed",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ntc struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ntc))
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testplans", testerAJWT, map[string]any{
		"product_id": prod.ID, "version_id": ver.ID, "name": "Notify Plan", "plan_type": "smoke",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ntp struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ntp))
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testplans/%d/cases", ntp.ID), testerAJWT, map[string]any{
		"case_ids": []int64{ntc.ID},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testruns", testerAJWT, map[string]any{
		"test_plan_id": ntp.ID, "build_id": bld.ID, "summary": "Notify Run",
		"manager_id": testerAID, "default_tester_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ntr struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ntr))
	_ = res.Body.Close()

	// Manager (testerA) stops the run → default tester (testerB) gets notified, manager does not self-notify
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testruns/%d/stop", ntr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/notifications?unread=true", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var nbList struct {
		Items []struct {
			ID         int64  `json:"id"`
			EventType  string `json:"event_type"`
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
		} `json:"items"`
		Total       int64 `json:"total"`
		UnreadCount int64 `json:"unread_count"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&nbList))
	_ = res.Body.Close()
	require.Len(t, nbList.Items, 1)
	require.Equal(t, int64(1), nbList.UnreadCount)
	require.Equal(t, int64(1), nbList.Total)
	firstNotif := nbList.Items[0]
	require.Equal(t, "run.stopped", firstNotif.EventType)
	require.Equal(t, "test_run", firstNotif.EntityType)
	require.Equal(t, ntr.ID, firstNotif.EntityID)

	res = doJSON(http.MethodGet, "/api/v1/notifications?unread=true", testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var nbListA struct {
		UnreadCount int64 `json:"unread_count"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&nbListA))
	_ = res.Body.Close()
	require.Zero(t, nbListA.UnreadCount)

	// Reading someone else's notification is 404; owner read is idempotent
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%d/read", firstNotif.ID), testerAJWT, nil)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%d/read", firstNotif.ID), testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/notifications/%d/read", firstNotif.ID), testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	// Failing the execution (default_tester + assignee = testerB) triggers execution.failed
	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testruns/%d/executions", ntr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var nExList struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&nExList))
	_ = res.Body.Close()
	require.Len(t, nExList.Items, 1)
	nExecID := nExList.Items[0].ID

	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/executions/%d", nExecID), testerAJWT, map[string]any{
		"status": "failed", "assignee_id": testerBID, "comment": "checkout broken",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/notifications?unread=true&limit=5", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var nbList2 struct {
		Items []struct {
			EventType string `json:"event_type"`
		} `json:"items"`
		UnreadCount int64 `json:"unread_count"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&nbList2))
	_ = res.Body.Close()
	require.Equal(t, int64(1), nbList2.UnreadCount)
	require.Len(t, nbList2.Items, 1)
	require.Equal(t, "execution.failed", nbList2.Items[0].EventType)

	// read-all zeroes the badge
	res = doJSON(http.MethodPost, "/api/v1/notifications/read-all", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ra struct {
		Updated int64 `json:"updated"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ra))
	_ = res.Body.Close()
	require.Equal(t, int64(1), ra.Updated)

	// Notification preferences (event x channel matrix) over /api/v1/me
	res = doJSON(http.MethodGet, "/api/v1/me/notification-settings", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var prefList struct {
		Events []struct {
			EventType string `json:"event_type"`
			InApp     bool   `json:"in_app"`
			Email     bool   `json:"email"`
		} `json:"events"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&prefList))
	_ = res.Body.Close()
	require.Len(t, prefList.Events, 3)
	for _, e := range prefList.Events {
		require.True(t, e.InApp)
		require.True(t, e.Email)
	}

	// Unknown event_type is rejected
	res = doJSON(http.MethodPut, "/api/v1/me/notification-settings", testerBJWT, map[string]any{
		"events": []map[string]any{{"event_type": "made.up", "in_app": true, "email": true}},
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// testerB disables both channels for run.stopped
	res = doJSON(http.MethodPut, "/api/v1/me/notification-settings", testerBJWT, map[string]any{
		"events": []map[string]any{{"event_type": "run.stopped", "in_app": false, "email": false}},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&prefList))
	_ = res.Body.Close()
	for _, e := range prefList.Events {
		if e.EventType == "run.stopped" {
			require.False(t, e.InApp)
			require.False(t, e.Email)
		}
	}

	// Stopping another run no longer creates an in-app notification for testerB
	res = doJSON(http.MethodPost, "/api/v1/testruns", testerAJWT, map[string]any{
		"test_plan_id": ntp.ID, "build_id": bld.ID, "summary": "Muted Run",
		"manager_id": testerAID, "default_tester_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tr3 struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tr3))
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testruns/%d/stop", tr3.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/notifications?unread=true", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var mutedList struct {
		UnreadCount int64 `json:"unread_count"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&mutedList))
	_ = res.Body.Close()
	require.Zero(t, mutedList.UnreadCount)

	// Re-enabling restores the channel
	res = doJSON(http.MethodPut, "/api/v1/me/notification-settings", testerBJWT, map[string]any{
		"events": []map[string]any{{"event_type": "run.stopped", "in_app": true, "email": true}},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&prefList))
	_ = res.Body.Close()
	for _, e := range prefList.Events {
		if e.EventType == "run.stopped" {
			require.True(t, e.InApp)
			require.True(t, e.Email)
		}
	}

	// 7. Webhook CRUD is admin-only; test delivery is HMAC-signed
	res = doJSON(http.MethodGet, "/api/v1/webhooks", testerAJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/webhooks", testerAJWT, map[string]any{
		"name": "forbidden", "url": "http://127.0.0.1:9/hook", "secret": "s", "events": []string{"run.stopped"},
	})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	var receivedBody []byte
	var receivedEvent, receivedSig, receivedCT string
	hookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		receivedBody = body
		receivedEvent = r.Header.Get("X-Kiwit-Event")
		receivedSig = r.Header.Get("X-Kiwit-Signature")
		receivedCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hookSrv.Close)

	type hookOut struct {
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		URL    string   `json:"url"`
		Events []string `json:"events"`
		Active bool     `json:"is_active"`
	}
	res = doJSON(http.MethodPost, "/api/v1/webhooks", adminJWT, map[string]any{
		"name": "ci-hook", "url": hookSrv.URL, "secret": "topsecret",
		"events": []string{"run.stopped", "execution.failed"},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var hook hookOut
	require.NoError(t, json.NewDecoder(res.Body).Decode(&hook))
	_ = res.Body.Close()
	require.Equal(t, "ci-hook", hook.Name)
	require.Equal(t, []string{"run.stopped", "execution.failed"}, hook.Events)
	require.True(t, hook.Active)

	// Validation: bad URL and unknown events are rejected
	res = doJSON(http.MethodPost, "/api/v1/webhooks", adminJWT, map[string]any{
		"name": "bad", "url": "ftp://example.com", "events": []string{"run.stopped"},
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/webhooks", adminJWT, map[string]any{
		"name": "bad", "url": hookSrv.URL, "events": []string{"made.up"},
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// Test delivery reaches the target with a verifiable HMAC signature
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", hook.ID), adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var testOut struct {
		Delivered  bool `json:"delivered"`
		StatusCode int  `json:"status_code"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&testOut))
	_ = res.Body.Close()
	require.True(t, testOut.Delivered)
	require.Equal(t, http.StatusOK, testOut.StatusCode)

	require.Equal(t, "webhook.test", receivedEvent)
	require.Equal(t, "application/json", receivedCT)
	verifyHookSignature(t, "topsecret", receivedSig, receivedBody)

	var hookPayload webhook.Payload
	require.NoError(t, json.Unmarshal(receivedBody, &hookPayload))
	require.Equal(t, "webhook.test", hookPayload.Event)
	require.Equal(t, "webhook", hookPayload.EntityType)
	require.Equal(t, hook.ID, hookPayload.EntityID)

	// Update: rename and deactivate
	res = doJSON(http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", hook.ID), adminJWT, map[string]any{
		"name": "ci-hook-off", "url": hookSrv.URL, "secret": "topsecret",
		"events":    []string{"run.stopped"},
		"is_active": false,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var updated hookOut
	require.NoError(t, json.NewDecoder(res.Body).Decode(&updated))
	_ = res.Body.Close()
	require.Equal(t, "ci-hook-off", updated.Name)
	require.False(t, updated.Active)
	require.Equal(t, []string{"run.stopped"}, updated.Events)

	// Delete removes the resource; further operations 404
	res = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/webhooks/%d", hook.ID), adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", hook.ID), adminJWT, nil)
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	_ = res.Body.Close()

	// 8. Idempotency guards for event emission + consolidated bulk notifications + t=/v1= webhook signature
	type guardNotifList struct {
		Items []struct {
			ID         int64  `json:"id"`
			EventType  string `json:"event_type"`
			EntityType string `json:"entity_type"`
			EntityID   int64  `json:"entity_id"`
			Title      string `json:"title"`
		} `json:"items"`
		Total       int64 `json:"total"`
		UnreadCount int64 `json:"unread_count"`
	}
	fetchUnread := func() guardNotifList {
		r := doJSON(http.MethodGet, "/api/v1/notifications?unread=true&limit=50", testerBJWT, nil)
		require.Equal(t, http.StatusOK, r.StatusCode)
		var list guardNotifList
		require.NoError(t, json.NewDecoder(r.Body).Decode(&list))
		_ = r.Body.Close()
		return list
	}
	hasEvent := func(list guardNotifList, event string) bool {
		for _, item := range list.Items {
			if item.EventType == event {
				return true
			}
		}
		return false
	}

	// Run by testerA (manager) with testerB as default tester; needs three executions.
	mkGuardCase := func(name string) int64 {
		r := doJSON(http.MethodPost, "/api/v1/testcases", testerAJWT, map[string]any{
			"product_id": prod.ID, "summary": name, "format": "steps", "priority": "medium", "status": "confirmed",
		})
		require.Equal(t, http.StatusOK, r.StatusCode)
		var out struct {
			ID int64 `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&out))
		_ = r.Body.Close()
		return out.ID
	}
	guardCaseIDs := []int64{mkGuardCase("Emission guard case 1"), mkGuardCase("Emission guard case 2"), mkGuardCase("Emission guard case 3")}

	res = doJSON(http.MethodPost, "/api/v1/testplans", testerAJWT, map[string]any{
		"product_id": prod.ID, "version_id": ver.ID, "name": "Emission Guard Plan", "plan_type": "smoke",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var gtp struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&gtp))
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testplans/%d/cases", gtp.ID), testerAJWT, map[string]any{
		"case_ids": guardCaseIDs,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/testruns", testerAJWT, map[string]any{
		"test_plan_id": gtp.ID, "build_id": bld.ID, "summary": "Emission Guard Run",
		"manager_id": testerAID, "default_tester_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var gtr struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&gtr))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testruns/%d/executions", gtr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var gExecList struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&gExecList))
	_ = res.Body.Close()
	require.Len(t, gExecList.Items, 3)
	guardExecIDs := []int64{gExecList.Items[0].ID, gExecList.Items[1].ID, gExecList.Items[2].ID}

	// Assigning an execution without a status change notifies the assignee.
	unreadBeforeAssigned := fetchUnread().UnreadCount
	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/executions/%d", guardExecIDs[0]), testerAJWT, map[string]any{
		"assignee_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	assignedList := fetchUnread()
	require.True(t, hasEvent(assignedList, "execution.assigned"))
	require.Equal(t, unreadBeforeAssigned+1, assignedList.UnreadCount)

	// Re-assigning the same assignee may emit again (documented as non-critical).
	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/executions/%d", guardExecIDs[0]), testerAJWT, map[string]any{
		"assignee_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()
	unreadBeforeStop := fetchUnread().UnreadCount

	// Stopping the run notifies stakeholders; stopping again emits nothing new.
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testruns/%d/stop", gtr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	stoppedList := fetchUnread()
	require.True(t, hasEvent(stoppedList, "run.stopped"))
	require.Equal(t, unreadBeforeStop+1, stoppedList.UnreadCount)

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/testruns/%d/stop", gtr.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()
	require.Equal(t, stoppedList.UnreadCount, fetchUnread().UnreadCount)

	// Failing an execution (padded/mixed-case status is normalized to "failed")
	// notifies stakeholders; re-failing the same execution does not.
	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/executions/%d", guardExecIDs[0]), testerAJWT, map[string]any{
		"status": "  Failed  ",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var normPatch struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&normPatch))
	_ = res.Body.Close()
	require.Equal(t, "failed", normPatch.Status, "padded/mixed-case status must be normalized to lowercase trimmed form")

	failedList := fetchUnread()
	require.Equal(t, stoppedList.UnreadCount+1, failedList.UnreadCount)
	require.Equal(t, "execution.failed", failedList.Items[0].EventType)
	require.Equal(t, "test_execution", failedList.Items[0].EntityType)

	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/executions/%d", guardExecIDs[0]), testerAJWT, map[string]any{
		"status": "failed",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()
	require.Equal(t, failedList.UnreadCount, fetchUnread().UnreadCount)

	// Bulk-fail also notifies the assignee of each failing execution: assign
	// one of the two remaining executions to testerB (also a run stakeholder)
	// and check the notification is still consolidated into a single one.
	unreadBeforeBulkAssign := fetchUnread().UnreadCount
	res = doJSON(http.MethodPatch, fmt.Sprintf("/api/v1/executions/%d", guardExecIDs[1]), testerAJWT, map[string]any{
		"assignee_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()
	bulkAssignList := fetchUnread()
	require.Equal(t, unreadBeforeBulkAssign+1, bulkAssignList.UnreadCount)

	// Bulk-failing the remaining two executions (one of them assigned to
	// testerB) emits ONE consolidated notification per run.
	res = doJSON(http.MethodPatch, "/api/v1/executions/bulk", testerAJWT, map[string]any{
		"ids": guardExecIDs[1:], "status": "failed",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var bulkUpdated struct {
		Updated int64 `json:"updated"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&bulkUpdated))
	_ = res.Body.Close()
	require.Equal(t, int64(2), bulkUpdated.Updated)

	bulkList := fetchUnread()
	require.Equal(t, bulkAssignList.UnreadCount+1, bulkList.UnreadCount)
	require.Equal(t, "execution.failed", bulkList.Items[0].EventType)
	require.Equal(t, "test_run", bulkList.Items[0].EntityType)
	require.Equal(t, gtr.ID, bulkList.Items[0].EntityID)
	require.Equal(t, fmt.Sprintf("2 executions failed (run #%d)", gtr.ID), bulkList.Items[0].Title)

	res = doJSON(http.MethodPatch, "/api/v1/executions/bulk", testerAJWT, map[string]any{
		"ids": guardExecIDs[1:], "status": "failed",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()
	require.Equal(t, bulkList.UnreadCount, fetchUnread().UnreadCount)

	// Webhook with the t=/v1= signature: bulk-failing a fresh run delivers a signed POST.
	type capturedHookReq struct {
		Event string
		Sig   string
		Body  []byte
	}
	var hookMu sync.Mutex
	var hookLog []capturedHookReq
	hookSrv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err == nil {
			hookMu.Lock()
			hookLog = append(hookLog, capturedHookReq{Event: r.Header.Get("X-Kiwit-Event"), Sig: r.Header.Get("X-Kiwit-Signature"), Body: body})
			hookMu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hookSrv2.Close)

	res = doJSON(http.MethodPost, "/api/v1/webhooks", adminJWT, map[string]any{
		"name": "sig-hook", "url": hookSrv2.URL, "secret": "topsecret2", "events": []string{"execution.failed"},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var rawHook map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&rawHook))
	_ = res.Body.Close()
	_, hasSecret := rawHook["secret"]
	require.False(t, hasSecret, "create webhook response must not expose the secret")
	sigHookID := int64(rawHook["id"].(float64))

	res = doJSON(http.MethodPost, "/api/v1/testruns", testerAJWT, map[string]any{
		"test_plan_id": gtp.ID, "build_id": bld.ID, "summary": "Webhook Bulk Run",
		"manager_id": testerAID, "default_tester_id": testerBID,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var hRun struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&hRun))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/testruns/%d/executions", hRun.ID), testerAJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var hExecList struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&hExecList))
	_ = res.Body.Close()
	require.Len(t, hExecList.Items, 3)
	hookExecIDs := make([]int64, 0, 3)
	for _, item := range hExecList.Items {
		hookExecIDs = append(hookExecIDs, item.ID)
	}

	res = doJSON(http.MethodPatch, "/api/v1/executions/bulk", testerAJWT, map[string]any{
		"ids": hookExecIDs, "status": "failed",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	deadline := time.Now().Add(3 * time.Second)
	for {
		hookMu.Lock()
		delivered := len(hookLog)
		hookMu.Unlock()
		if delivered > 0 || !time.Now().Before(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	hookMu.Lock()
	deliveries := append([]capturedHookReq(nil), hookLog...)
	hookMu.Unlock()
	require.NotEmpty(t, deliveries)
	require.Equal(t, "execution.failed", deliveries[0].Event)
	verifyHookSignature(t, "topsecret2", deliveries[0].Sig, deliveries[0].Body)

	var bulkPayload webhook.Payload
	require.NoError(t, json.Unmarshal(deliveries[0].Body, &bulkPayload))
	require.Equal(t, "execution.failed", bulkPayload.Event)
	require.Equal(t, "test_run", bulkPayload.EntityType)
	require.Equal(t, hRun.ID, bulkPayload.EntityID)
	require.Equal(t, fmt.Sprintf("3 executions failed (run #%d)", hRun.ID), bulkPayload.Title)

	// PUT without a secret keeps the stored secret: test delivery stays signed.
	res = doJSON(http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", sigHookID), adminJWT, map[string]any{
		"name": "sig-hook-2", "url": hookSrv2.URL, "events": []string{"execution.failed"}, "is_active": true,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	rawHook = nil
	require.NoError(t, json.NewDecoder(res.Body).Decode(&rawHook))
	_ = res.Body.Close()
	_, hasSecret = rawHook["secret"]
	require.False(t, hasSecret, "update webhook response must not expose the secret")

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", sigHookID), adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var sigTestOut struct {
		Delivered  bool `json:"delivered"`
		StatusCode int  `json:"status_code"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&sigTestOut))
	_ = res.Body.Close()
	require.True(t, sigTestOut.Delivered)

	hookMu.Lock()
	deliveries = append([]capturedHookReq(nil), hookLog...)
	hookMu.Unlock()
	require.NotEmpty(t, deliveries)
	lastDelivery := deliveries[len(deliveries)-1]
	verifyHookSignature(t, "topsecret2", lastDelivery.Sig, lastDelivery.Body)

	// PUT with secret_clear=true removes the stored secret: the test delivery
	// arrives without the X-Kiwit-Signature header (signing disabled).
	res = doJSON(http.MethodPut, fmt.Sprintf("/api/v1/webhooks/%d", sigHookID), adminJWT, map[string]any{
		"name": "sig-hook-3", "url": hookSrv2.URL, "events": []string{"execution.failed"}, "is_active": true,
		"secret_clear": true,
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	hookMu.Lock()
	deliveriesBeforeClear := len(deliveries)
	hookMu.Unlock()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/webhooks/%d/test", sigHookID), adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var clearTestOut struct {
		Delivered  bool `json:"delivered"`
		StatusCode int  `json:"status_code"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&clearTestOut))
	_ = res.Body.Close()
	require.True(t, clearTestOut.Delivered)

	deadline = time.Now().Add(3 * time.Second)
	for {
		hookMu.Lock()
		delivered := len(hookLog)
		hookMu.Unlock()
		if delivered > deliveriesBeforeClear || !time.Now().Before(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	hookMu.Lock()
	deliveries = append([]capturedHookReq(nil), hookLog...)
	hookMu.Unlock()
	require.Greater(t, len(deliveries), deliveriesBeforeClear, "cleared-secret test delivery must reach the receiver")
	clearedDelivery := deliveries[len(deliveries)-1]
	require.Empty(t, clearedDelivery.Sig, "delivery after secret_clear must not carry an X-Kiwit-Signature header")
	require.Equal(t, "webhook.test", clearedDelivery.Event)

	// 9. Password reset: public request without enumeration, one-time token confirm, admin reset
	// Requesting a reset for a non-existent e-mail still returns 200 (no enumeration).
	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset", "", map[string]string{
		"email": "ghost@kiwit.local",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ghostReset struct {
		OK       bool   `json:"ok"`
		ResetURL string `json:"reset_url"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ghostReset))
	_ = res.Body.Close()
	require.True(t, ghostReset.OK)
	require.Empty(t, ghostReset.ResetURL, "public request must never return a reset link")

	// Requesting a reset for testerB also returns a generic 200 (no SMTP: link is logged, not returned).
	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset", "", map[string]string{
		"email": "testerb@kiwit.local",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	// Confirming with a made-up token is rejected.
	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset/confirm", "", map[string]string{
		"token": "not-a-real-token", "new_password": "anotherpass123",
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// Only admins may force a reset for another user.
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/users/%d/password-reset", testerBID), testerAJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/users/%d/password-reset", testerBID), adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var adminReset struct {
		Emailed  bool   `json:"emailed"`
		ResetURL string `json:"reset_url"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&adminReset))
	_ = res.Body.Close()
	require.False(t, adminReset.Emailed)
	require.Contains(t, adminReset.ResetURL, "/reset-password?token=")
	resetToken := strings.SplitN(adminReset.ResetURL, "token=", 2)[1]
	require.Len(t, resetToken, 64)

	// A second admin reset supersedes the first link: the older token dies.
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/users/%d/password-reset", testerBID), adminJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var adminReset2 struct {
		Emailed  bool   `json:"emailed"`
		ResetURL string `json:"reset_url"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&adminReset2))
	_ = res.Body.Close()
	resetToken2 := strings.SplitN(adminReset2.ResetURL, "token=", 2)[1]
	require.NotEqual(t, resetToken, resetToken2)

	// Confirming with the superseded (first) token is rejected ...
	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset/confirm", "", map[string]string{
		"token": resetToken, "new_password": "brandnewpass789",
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// ... and the freshly issued token changes the password; the old one stops working.
	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset/confirm", "", map[string]string{
		"token": resetToken2, "new_password": "brandnewpass123",
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	_ = res.Body.Close()

	testerBJWT2 := login("testerb@kiwit.local", "brandnewpass123")
	require.NotEmpty(t, testerBJWT2)

	res = doJSON(http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "testerb@kiwit.local", "password": "password123",
	})
	require.Equal(t, http.StatusUnauthorized, res.StatusCode)
	_ = res.Body.Close()

	// The token is one-time: reuse is rejected.
	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset/confirm", "", map[string]string{
		"token": resetToken2, "new_password": "brandnewpass456",
	})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
	_ = res.Body.Close()

	// An API token may not reset passwords (same rule as change-password).
	res = doJSON(http.MethodPost, "/api/v1/auth/tokens", testerBJWT2, map[string]string{"name": "reset-ci"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var abToken struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&abToken))
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, "/api/v1/auth/password-reset/confirm", abToken.Token, map[string]string{
		"token": "whatever-token", "new_password": "whateverpass1",
	})
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()
}

// verifyHookSignature independently recomputes the GitHub-style HMAC over
// "<t>.<body>" and compares it with the "t=<unix>,v1=<hex>" header value.
func verifyHookSignature(t *testing.T, secret, sigHeader string, body []byte) {
	t.Helper()
	require.NotEmpty(t, sigHeader)
	parts := strings.SplitN(sigHeader, ",", 2)
	require.Len(t, parts, 2)
	ts, err := strconv.ParseInt(strings.TrimPrefix(parts[0], "t="), 10, 64)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, string(body))))
	require.Equal(t, "v1="+hex.EncodeToString(mac.Sum(nil)), parts[1])
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

	// Password reset routes are rate limited as well; each path counts
	// against the same per-IP budget, so use one IP per path here.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", nil)
		req.RemoteAddr = "203.0.113.11:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset", nil)
	req.RemoteAddr = "203.0.113.11:12345"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", nil)
		req.RemoteAddr = "203.0.113.12:12345"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", nil)
	req.RemoteAddr = "203.0.113.12:12345"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}
