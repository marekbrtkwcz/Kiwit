package testplan_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
	"github.com/kiwit/kiwit/internal/testplan"
)

func TestSQLiteTestPlanHierarchyAndClone(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	ver, err := prodRepo.CreateVersion(ctx, prod.ID, "1.0.0")
	require.NoError(t, err)

	tcRepo := testcase.NewSQLiteRepository(sqlDB)
	tc, err := tcRepo.Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID,
		Summary:   "Smoke case",
		Format:    "steps",
		Priority:  "high",
		Status:    "confirmed",
	})
	require.NoError(t, err)

	tpRepo := testplan.NewSQLiteRepository(sqlDB)
	root, err := tpRepo.Create(ctx, prod.ID, ver.ID, nil, "Root Plan", "Main")
	require.NoError(t, err)
	require.Nil(t, root.ParentID)

	child, err := tpRepo.Create(ctx, prod.ID, ver.ID, &root.ID, "Child Plan", "Sub")
	require.NoError(t, err)
	require.NotNil(t, child.ParentID)
	require.Equal(t, root.ID, *child.ParentID)

	require.NoError(t, tpRepo.AddCases(ctx, child.ID, []int64{tc.ID}))

	attached, err := tpRepo.ListCases(ctx, child.ID)
	require.NoError(t, err)
	require.Len(t, attached, 1)
	require.Equal(t, tc.ID, attached[0].TestCaseID)
	require.Equal(t, "Smoke case", attached[0].Summary)

	cloned, err := tpRepo.Clone(ctx, child.ID, "Child Plan Copy", nil)
	require.NoError(t, err)
	require.NotNil(t, cloned.ParentID)
	require.Equal(t, root.ID, *cloned.ParentID)

	all, err := tpRepo.List(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)

	require.NoError(t, tpRepo.Delete(ctx, cloned.ID))
	require.ErrorIs(t, tpRepo.Delete(ctx, cloned.ID), sql.ErrNoRows)
}
