package telemetry_test

import (
	"context"
	"encoding/json"
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
	tp, err := tpRepo.Create(ctx, prod.ID, ver.ID, "Plan", "")
	require.NoError(t, err)
	require.NoError(t, tpRepo.AddCases(ctx, tp.ID, []int64{tc.ID}))

	trRepo := testrun.NewSQLiteRepository(sqlDB)
	run, err := trRepo.Create(ctx, tp.ID, bld.ID, "Run", "")
	require.NoError(t, err)
	execs, err := trRepo.ListExecutions(ctx, run.ID)
	require.NoError(t, err)
	_, err = trRepo.UpdateExecution(ctx, execs[0].ID, "failed", "timeout")
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
}
