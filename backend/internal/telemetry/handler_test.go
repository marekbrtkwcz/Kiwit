package telemetry_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/telemetry"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
)

func TestTelemetryEndpoints(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	mgmt := management.NewSQLiteRepository(sqlDB)
	prod, err := mgmt.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	ver, err := mgmt.CreateVersion(ctx, prod.ID, "1.0")
	require.NoError(t, err)
	bld, err := mgmt.CreateBuild(ctx, ver.ID, "b1")
	require.NoError(t, err)

	tc, err := testcase.NewSQLiteRepository(sqlDB).Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID, Summary: "Flaky login", Format: "steps", Priority: "high", Status: "confirmed",
	})
	require.NoError(t, err)

	tpRepo := testplan.NewSQLiteRepository(sqlDB)
	tp, err := tpRepo.Create(ctx, prod.ID, ver.ID, nil, "Plan", "")
	require.NoError(t, err)
	require.NoError(t, tpRepo.AddCases(ctx, tp.ID, []int64{tc.ID}))

	trRepo := testrun.NewSQLiteRepository(sqlDB)
	run, err := trRepo.Create(ctx, tp.ID, bld.ID, "Run", "")
	require.NoError(t, err)
	execs, err := trRepo.ListExecutions(ctx, run.ID)
	require.NoError(t, err)

	// A tester user who evaluates the sole execution.
	res, err := sqlDB.ExecContext(ctx,
		`INSERT INTO users (email, username, password_hash) VALUES ('tester@kiwit.local', 'tester', 'x')`)
	require.NoError(t, err)
	testerID, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = trRepo.UpdateExecutionFull(ctx, execs[0].ID, testrun.ExecutionUpdateParams{
		Status:     "failed",
		Comment:    "timeout",
		TestedByID: &testerID,
	})
	require.NoError(t, err)

	r := chi.NewMux()
	api := humachi.New(r, huma.DefaultConfig("Telemetry Test", "0.1.0"))
	telemetry.RegisterRoutes(api, sqlDB, nil)

	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	for _, path := range []string{
		"/api/v1/telemetry/execution-trends",
		"/api/v1/telemetry/status-matrix",
		"/api/v1/telemetry/test-case-health",
	} {
		res, err := http.Get(srv.URL + path)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)
		var out struct {
			Items []map[string]any `json:"items"`
		}
		require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
		_ = res.Body.Close()
		require.NotEmpty(t, out.Items)
	}

	// Verify breakdown endpoint
	res2, err := http.Get(srv.URL + "/api/v1/telemetry/breakdown")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res2.StatusCode)
	var bd telemetry.Breakdown
	require.NoError(t, json.NewDecoder(res2.Body).Decode(&bd))
	_ = res2.Body.Close()
	require.Equal(t, int64(1), bd.Manual)
	require.NotEmpty(t, bd.ByPriority)

	// Verify execution dashboard
	res3, err := http.Get(srv.URL + "/api/v1/telemetry/execution-dashboard")
	require.NoError(t, err)
	if res3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res3.Body)
		_ = res3.Body.Close()
		t.Fatalf("execution-dashboard status %d: %s", res3.StatusCode, body)
	}
	var dash telemetry.ExecutionDashboard
	require.NoError(t, json.NewDecoder(res3.Body).Decode(&dash))
	_ = res3.Body.Close()

	require.Len(t, dash.Runs, 1)
	rp := dash.Runs[0]
	require.Equal(t, run.ID, rp.TestRunID)
	require.Equal(t, "Plan", rp.PlanName)
	require.False(t, rp.Stopped)
	require.Equal(t, int64(1), rp.Total)
	require.Equal(t, int64(1), rp.Failed)
	require.Equal(t, int64(0), rp.Idle)
	require.InDelta(t, 1.0, rp.Progress, 0.0001)
	require.NotEmpty(t, rp.StartedAt)

	require.Len(t, dash.Recent, 1)
	re := dash.Recent[0]
	require.Equal(t, execs[0].ID, re.ExecutionID)
	require.Equal(t, tc.ID, re.TestCaseID)
	require.Equal(t, run.ID, re.TestRunID)
	require.Equal(t, "failed", re.Status)
	require.Equal(t, "tester", re.Tester)

	require.Len(t, dash.Testers, 1)
	tl := dash.Testers[0]
	require.Equal(t, "tester", tl.Username)
	require.Equal(t, int64(1), tl.Total)
	require.Equal(t, int64(1), tl.Failed)
	require.Equal(t, float64(0), tl.PassRate)
}
