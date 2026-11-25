package issuetracker_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/issuetracker"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
	"github.com/kiwit/kiwit/internal/testrun"
)

func TestSQLiteIssueTrackerAndBugs(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prod, err := management.NewSQLiteRepository(sqlDB).CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	ver, err := management.NewSQLiteRepository(sqlDB).CreateVersion(ctx, prod.ID, "1.0")
	require.NoError(t, err)
	bld, err := management.NewSQLiteRepository(sqlDB).CreateBuild(ctx, ver.ID, "b1")
	require.NoError(t, err)

	tc, err := testcase.NewSQLiteRepository(sqlDB).Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID, Summary: "Login", Format: "steps", Priority: "high", Status: "confirmed",
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
	require.Len(t, execs, 1)

	itRepo := issuetracker.NewSQLiteRepository(sqlDB)
	tracker, err := itRepo.CreateTracker(ctx, "GitHub", "github", "https://github.com/org/repo")
	require.NoError(t, err)

	bug, err := itRepo.LinkBug(ctx, execs[0].ID, &tracker.ID, "#42", "https://github.com/org/repo/issues/42", "500 on login")
	require.NoError(t, err)
	require.Equal(t, "#42", bug.ExternalKey)

	bugs, err := itRepo.ListBugsByExecution(ctx, execs[0].ID)
	require.NoError(t, err)
	require.Len(t, bugs, 1)
}
