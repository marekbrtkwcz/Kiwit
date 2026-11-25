package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/kiwit/kiwit/internal/notify"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/telemetry"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
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
			test_executions, test_runs, test_plan_cases, test_plans,
			test_case_revisions, test_case_steps, test_case_components,
			test_case_tags, test_plan_tags, test_run_tags, test_cases,
			product_members, components, builds, versions, products,
			categories, tags, api_tokens, users
		RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	runE2ESuite(t, nil, pgPool)
}

func runE2ESuite(t *testing.T, sqlDB *sql.DB, pgPool *pgxpool.Pool) {
	t.Helper()

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
	testrun.RegisterRoutes(api, trRepo, nil, guard)
	testrun.RegisterExportRoutes(r, trRepo, guard)
	issuetracker.RegisterRoutes(api, issueRepo, guard)
	bug.RegisterRoutes(api, bug.NewStore(sqlDB, pgPool), guard)
	telemetry.RegisterRoutes(api, sqlDB, pgPool)
	comment.RegisterRoutes(api, commentRepo, guard)
	notify.RegisterEmailRoutes(api, emailSettings, nil, guard)
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

	// Create version & build as testerA; testerB gets 403 on version/build/plan/case/run
	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/products/%d/versions", prod.ID), testerAJWT, map[string]string{"value": "1.0"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var ver struct{ ID int64 `json:"id"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ver))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/products/%d/versions", prod.ID), testerBJWT, nil)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
	_ = res.Body.Close()

	res = doJSON(http.MethodPost, fmt.Sprintf("/api/v1/versions/%d/builds", ver.ID), testerAJWT, map[string]string{"name": "b1"})
	require.Equal(t, http.StatusOK, res.StatusCode)
	var bld struct{ ID int64 `json:"id"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&bld))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, fmt.Sprintf("/api/v1/versions/%d/builds", ver.ID), testerBJWT, nil)
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
	var tc struct{ ID int64 `json:"id"` }
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
	var tp struct{ ID int64 `json:"id"` }
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
	var tr struct{ ID int64 `json:"id"` }
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
	var linkOut struct{ ID int64 `json:"id"` }
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
	var ibug struct{ ID int64 `json:"id"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&ibug))
	_ = res.Body.Close()

	res = doJSON(http.MethodGet, "/api/v1/bugs", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var bugListB struct{ Items []map[string]any `json:"items"` }
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
		TestRun      struct{ ID int64 `json:"id"` } `json:"test_run"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&junitOut))
	_ = res.Body.Close()
	require.Equal(t, 1, junitOut.CreatedCases)
	_ = doJSON(http.MethodDelete, fmt.Sprintf("/api/v1/testruns/%d", junitOut.TestRun.ID), testerAJWT, nil)

	// testerB cannot see testplans, testruns, executions, or export
	res = doJSON(http.MethodGet, "/api/v1/testplans", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var tpListB struct{ Items []map[string]any `json:"items"` }
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tpListB))
	_ = res.Body.Close()
	require.Empty(t, tpListB.Items)

	res = doJSON(http.MethodGet, "/api/v1/testruns?status=running", testerBJWT, nil)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var trListB struct{ Items []map[string]any `json:"items"` }
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
	var att struct{ ID int64 `json:"id"` }
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
