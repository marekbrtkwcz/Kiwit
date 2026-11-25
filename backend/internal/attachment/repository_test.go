package attachment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestSQLiteAttachmentRepository(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := attachment.NewSQLiteRepository(sqlDB)

	created, err := repo.Create(ctx, attachment.Attachment{
		EntityType:  "test_execution",
		EntityID:    42,
		Filename:    "failure.png",
		ContentType: "image/png",
		SizeBytes:   1024,
		StoragePath: "./uploads/abc_failure.png",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)

	items, err := repo.ListByEntity(ctx, "test_execution", 42)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "failure.png", items[0].Filename)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "./uploads/abc_failure.png", got.StoragePath)
}
