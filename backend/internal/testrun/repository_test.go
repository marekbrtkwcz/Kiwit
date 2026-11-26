package testrun_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
)

func TestSQLiteTestRunAndExecutionFlow(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)

	// Seed version & build
	res, err := sqlDB.ExecContext(ctx, `INSERT INTO versions (product_id, value) VALUES (?, '1.0.0')`, prod.ID)
	require.NoError(t, err)
	versionID, err := res.LastInsertId()
	require.NoError(t, err)

	res, err = sqlDB.ExecContext(ctx, `INSERT INTO builds (version_id, name) VALUES (?, 'build-42')`, versionID)
	require.NoError(t, err)
	buildID, err := res.LastInsertId()
	require.NoError(t, err)

	tcRepo := testcase.NewSQLiteRepository(sqlDB)
	tc, err := tcRepo.Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID,
		Summary:   "Checkout works",
		Format:    "steps",
		Priority:  "high",
		Status:    "confirmed",
		Steps: []testcase.Step{
			{Position: 1, Action: "Add item", ExpectedResult: "Item in cart"},
		},
	})
	require.NoError(t, err)

	tpRepo := testplan.NewSQLiteRepository(sqlDB)
	tp, err := tpRepo.Create(ctx, prod.ID, versionID, nil, "Release 1.0", "")
	require.NoError(t, err)
	require.NoError(t, tpRepo.AddCases(ctx, tp.ID, []int64{tc.ID}))

	trRepo := testrun.NewSQLiteRepository(sqlDB)
	run, err := trRepo.Create(ctx, tp.ID, buildID, "Smoke run", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), run.ID)

	// Verify execution was created with steps_snapshot and can be updated
	var execID int64
	var snapshot string
	err = sqlDB.QueryRowContext(ctx, `SELECT id, steps_snapshot FROM test_executions WHERE test_run_id = ?`, run.ID).Scan(&execID, &snapshot)
	require.NoError(t, err)
	require.Contains(t, snapshot, "Add item")

	updated, err := trRepo.UpdateExecution(ctx, execID, "passed", "Verified on build-42")
	require.NoError(t, err)
	require.Equal(t, "passed", updated.Status)
}

func newStopTestRun(t *testing.T) (*testrun.SQLiteRepository, int64) {
	t.Helper()
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)

	res, err := sqlDB.ExecContext(ctx, `INSERT INTO versions (product_id, value) VALUES (?, '1.0.0')`, prod.ID)
	require.NoError(t, err)
	versionID, err := res.LastInsertId()
	require.NoError(t, err)

	res, err = sqlDB.ExecContext(ctx, `INSERT INTO builds (version_id, name) VALUES (?, 'stop-build')`, versionID)
	require.NoError(t, err)
	buildID, err := res.LastInsertId()
	require.NoError(t, err)

	tpRepo := testplan.NewSQLiteRepository(sqlDB)
	tp, err := tpRepo.Create(ctx, prod.ID, versionID, nil, "Stop Plan", "")
	require.NoError(t, err)

	trRepo := testrun.NewSQLiteRepository(sqlDB)
	run, err := trRepo.Create(ctx, tp.ID, buildID, "Stop run", "")
	require.NoError(t, err)
	return trRepo, run.ID
}

func TestSQLiteStopRunReportsNewlyStopped(t *testing.T) {
	ctx := context.Background()
	trRepo, runID := newStopTestRun(t)

	stoppedNow, err := trRepo.Stop(ctx, runID)
	require.NoError(t, err)
	require.True(t, stoppedNow, "stopping a running run reports the transition")

	run, err := trRepo.GetByID(ctx, runID)
	require.NoError(t, err)
	require.NotNil(t, run.StoppedAt)
}

func TestSQLiteStopRunSecondCallIsIdempotentNoOp(t *testing.T) {
	ctx := context.Background()
	trRepo, runID := newStopTestRun(t)

	_, err := trRepo.Stop(ctx, runID)
	require.NoError(t, err)

	stoppedNow, err := trRepo.Stop(ctx, runID)
	require.NoError(t, err, "re-stopping an already stopped run is a no-op, not an error")
	require.False(t, stoppedNow, "re-stopping an already stopped run reports no transition")
}

func TestSQLiteStopRunMissingRunReturnsErrNoRows(t *testing.T) {
	ctx := context.Background()
	trRepo, _ := newStopTestRun(t)

	_, err := trRepo.Stop(ctx, 99999)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
