package notification_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/notification"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestSQLiteNotificationStoreFlow(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	seed := func(email string) int64 {
		res, err := sqlDB.ExecContext(ctx,
			`INSERT INTO users (email, username, password_hash) VALUES (?, ?, 'x')`,
			email, email)
		require.NoError(t, err)
		id, err := res.LastInsertId()
		require.NoError(t, err)
		return id
	}
	alice := seed("alice@example.com")
	bob := seed("bob@example.com")
	now := func() time.Time { return time.Now().UTC() }

	store := notification.NewStore(sqlDB, nil)

	// Emit deduplicates recipients and skips invalid ids.
	require.NoError(t, store.Emit(ctx, []int64{alice, alice, 0, bob},
		"run.stopped", "test_run", 42, "Test run #42 stopped", "Regression suite"))
	require.NoError(t, store.Emit(ctx, []int64{bob},
		"execution.failed", "test_execution", 7, "Execution #7 failed", "checkout broken"))

	// Alice sees exactly one notification with a correct unread badge count.
	aliceRes, err := store.List(ctx, alice, false, 20, 0)
	require.NoError(t, err)
	require.Len(t, aliceRes.Items, 1)
	require.Equal(t, int64(1), aliceRes.Total)
	require.Equal(t, int64(1), aliceRes.UnreadCount)
	require.Equal(t, "run.stopped", aliceRes.Items[0].EventType)
	require.Equal(t, "test_run", aliceRes.Items[0].EntityType)
	require.Equal(t, int64(42), aliceRes.Items[0].EntityID)
	require.Nil(t, aliceRes.Items[0].ReadAt)

	// Unread filter and per-user scoping hold for bob.
	bobRes, err := store.List(ctx, bob, true, 20, 0)
	require.NoError(t, err)
	require.Len(t, bobRes.Items, 2)

	// Marking someone else's notification fails as not found.
	require.ErrorIs(t, store.MarkRead(ctx, bob, aliceRes.Items[0].ID, now()), notification.ErrNotFound)

	// Owner marks it read; unread count drops; repeated call stays idempotent.
	require.NoError(t, store.MarkRead(ctx, alice, aliceRes.Items[0].ID, now()))
	require.NoError(t, store.MarkRead(ctx, alice, aliceRes.Items[0].ID, now()))
	aliceRes, err = store.List(ctx, alice, true, 20, 0)
	require.NoError(t, err)
	require.Len(t, aliceRes.Items, 0)
	require.Equal(t, int64(0), aliceRes.UnreadCount)

	// Pagination: limit 1 on bob's list returns one item but full totals.
	bobRes, err = store.List(ctx, bob, false, 1, 0)
	require.NoError(t, err)
	require.Len(t, bobRes.Items, 1)
	require.Equal(t, int64(2), bobRes.Total)

	// MarkAllRead marks everything for bob only.
	n, err := store.MarkAllRead(ctx, bob, now())
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
	bobRes, err = store.List(ctx, bob, true, 20, 0)
	require.NoError(t, err)
	require.Len(t, bobRes.Items, 0)
}

func TestSQLiteNotificationPreferencesFlow(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	seed := func(email string) int64 {
		res, err := sqlDB.ExecContext(ctx,
			`INSERT INTO users (email, username, password_hash) VALUES (?, ?, 'x')`,
			email, email)
		require.NoError(t, err)
		id, err := res.LastInsertId()
		require.NoError(t, err)
		return id
	}
	alice := seed("alice2@example.com")
	bob := seed("bob2@example.com")

	store := notification.NewStore(sqlDB, nil)

	// Defaults: full catalog, both channels enabled.
	prefs, err := store.Preferences(ctx, alice)
	require.NoError(t, err)
	require.Len(t, prefs, 3)
	for _, p := range prefs {
		require.True(t, p.InApp)
		require.True(t, p.Email)
	}
	require.True(t, notification.IsValidEventType("run.stopped"))
	require.False(t, notification.IsValidEventType("made.up"))

	// Disable both channels for run.stopped only for alice.
	saved, err := store.UpsertPreferences(ctx, alice, []notification.EventPreference{
		{EventType: "run.stopped", InApp: false, Email: false},
	})
	require.NoError(t, err)
	var runStoppedFound, others int
	for _, p := range saved {
		if p.EventType == "run.stopped" {
			runStoppedFound++
			require.False(t, p.InApp)
			require.False(t, p.Email)
		} else {
			others++
			require.True(t, p.InApp)
			require.True(t, p.Email)
		}
	}
	require.Equal(t, 1, runStoppedFound)
	require.Equal(t, 2, others)

	// Emit respects the disabled in-app channel for alice but still reaches bob.
	require.NoError(t, store.Emit(ctx, []int64{alice, bob},
		"run.stopped", "test_run", 1, "Test run #1 stopped", "suite"))
	aliceRes, err := store.List(ctx, alice, false, 20, 0)
	require.NoError(t, err)
	require.Len(t, aliceRes.Items, 0)
	bobRes, err := store.List(ctx, bob, false, 20, 0)
	require.NoError(t, err)
	require.Len(t, bobRes.Items, 1)

	// A different event still reaches alice (preference is per event).
	require.NoError(t, store.Emit(ctx, []int64{alice},
		"execution.failed", "test_execution", 2, "Execution #2 failed", ""))
	aliceRes, err = store.List(ctx, alice, false, 20, 0)
	require.NoError(t, err)
	require.Len(t, aliceRes.Items, 1)
	require.Equal(t, "execution.failed", aliceRes.Items[0].EventType)

	// EmitEmail is a safe no-op without a configured client.
	store.EmitEmail(ctx, nil, []int64{alice, bob}, "run.stopped", "subject", "body")
}
