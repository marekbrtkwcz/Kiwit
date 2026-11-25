package testcase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
)

func TestSQLiteTestCaseRepository(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)

	tcRepo := testcase.NewSQLiteRepository(sqlDB)

	created, err := tcRepo.Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID,
		Summary:   "User can log in",
		Format:    "steps",
		Priority:  "high",
		Status:    "confirmed",
		Steps: []testcase.Step{
			{Position: 1, Action: "Open /login", ExpectedResult: "Login form visible"},
			{Position: 2, Action: "Submit credentials", ExpectedResult: "Redirect to dashboard"},
		},
	})
	require.NoError(t, err)
	require.Len(t, created.Steps, 2)

	updated, err := tcRepo.Update(ctx, created.ID, testcase.UpsertParams{
		ProductID: prod.ID,
		Summary:   "User can log in (updated)",
		Format:    "steps",
		Priority:  "critical",
		Status:    "confirmed",
		Steps: []testcase.Step{
			{Position: 1, Action: "Open /login", ExpectedResult: "Login form visible"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "User can log in (updated)", updated.Summary)
	require.Len(t, updated.Steps, 1)

	list, err := tcRepo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Len(t, list[0].Steps, 1)

	revs, err := tcRepo.ListRevisions(ctx, created.ID)
	require.NoError(t, err)
	require.Len(t, revs, 2)
	require.Equal(t, 2, revs[0].Version)
	require.Equal(t, 1, revs[1].Version)
	require.Len(t, revs[1].Steps, 2)
}
