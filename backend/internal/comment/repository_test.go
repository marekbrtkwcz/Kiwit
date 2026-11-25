package comment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/comment"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestSQLiteCommentRepository(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := comment.NewSQLiteRepository(sqlDB)
	c, err := repo.Create(ctx, "test_case", 1, nil, "Looks good, please add edge case")
	require.NoError(t, err)
	require.Equal(t, int64(1), c.ID)

	items, err := repo.List(ctx, "test_case", 1)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Looks good, please add edge case", items[0].Body)
}
