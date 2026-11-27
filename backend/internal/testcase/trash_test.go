package testcase_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/testcase"
)

func TestSoftDeleteCycle(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	repo := testcase.NewSQLiteRepository(sqlDB)

	created, err := repo.Create(ctx, testcase.UpsertParams{
		ProductID: prod.ID,
		Summary:   "To be deleted",
		Format:    "steps",
		Priority:  "medium",
		Status:    "confirmed",
		Steps:     []testcase.Step{{Position: 1, Action: "step one", ExpectedResult: "ok"}},
	})
	require.NoError(t, err)

	// Soft delete: row disappears from live list and GetByID, appears in trash.
	require.NoError(t, repo.Delete(ctx, created.ID))

	_, total, err := repo.ListWithFilter(ctx, testcase.ListFilter{ProductID: prod.ID})
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	_, err = repo.GetByID(ctx, created.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)

	trash, trashTotal, err := repo.ListWithFilter(ctx, testcase.ListFilter{ProductID: prod.ID, Trash: true})
	require.NoError(t, err)
	require.Equal(t, int64(1), trashTotal)
	require.Len(t, trash, 1)
	require.Equal(t, created.ID, trash[0].ID)
	require.Equal(t, "To be deleted", trash[0].Summary)
	require.Len(t, trash[0].Steps, 1) // steps survive soft delete

	// Double soft delete: nothing left to soft-delete.
	require.ErrorIs(t, repo.Delete(ctx, created.ID), sql.ErrNoRows)

	// Restore: back in the live list, out of the trash.
	require.NoError(t, repo.Restore(ctx, created.ID))
	_, total, err = repo.ListWithFilter(ctx, testcase.ListFilter{ProductID: prod.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	_, trashTotal, err = repo.ListWithFilter(ctx, testcase.ListFilter{ProductID: prod.ID, Trash: true})
	require.NoError(t, err)
	require.Equal(t, int64(0), trashTotal)

	// Restore of a live case fails.
	require.ErrorIs(t, repo.Restore(ctx, created.ID), sql.ErrNoRows)

	// Purge: gone for good, including from the trash.
	require.NoError(t, repo.Delete(ctx, created.ID))
	require.NoError(t, repo.Purge(ctx, created.ID))
	_, trashTotal, err = repo.ListWithFilter(ctx, testcase.ListFilter{ProductID: prod.ID, Trash: true})
	require.NoError(t, err)
	require.Equal(t, int64(0), trashTotal)
	require.ErrorIs(t, repo.Purge(ctx, created.ID), sql.ErrNoRows)
}

func TestBulkUpdateSkipsTrash(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	repo := testcase.NewSQLiteRepository(sqlDB)

	live, err := repo.Create(ctx, testcase.UpsertParams{ProductID: prod.ID, Summary: "Live", Format: "steps", Priority: "medium", Status: "confirmed"})
	require.NoError(t, err)
	dead, err := repo.Create(ctx, testcase.UpsertParams{ProductID: prod.ID, Summary: "Dead", Format: "steps", Priority: "medium", Status: "confirmed"})
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, dead.ID))

	updated, err := repo.BulkUpdate(ctx, []int64{live.ID, dead.ID}, "draft", "critical")
	require.NoError(t, err)
	require.Equal(t, int64(1), updated) // only the live case was touched

	got, err := repo.GetByID(ctx, dead.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
	_ = got
}
