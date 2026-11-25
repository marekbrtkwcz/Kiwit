package notify_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/notify"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestEmailSettingsStore(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	store := notify.NewSettingsStore(sqlDB, nil)

	// Default when not configured
	def, err := store.Get(ctx, "test_plan", 7)
	require.NoError(t, err)
	require.True(t, def.NotifyOnUpdate)
	require.Empty(t, def.CCList)

	// Save CC list
	_, err = store.Upsert(ctx, notify.EmailSettings{
		EntityType:     "test_plan",
		EntityID:       7,
		NotifyOnUpdate: true,
		CCList:         []string{"qa@kiwit.local", "lead@kiwit.local"},
	})
	require.NoError(t, err)

	got, err := store.Get(ctx, "test_plan", 7)
	require.NoError(t, err)
	require.Equal(t, []string{"qa@kiwit.local", "lead@kiwit.local"}, got.CCList)
}
