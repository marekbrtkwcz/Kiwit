package auth_test

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/platform/db"
)

// newResetFixture opens an in-memory SQLite database, registers one user
// (the first user becomes admin) and returns the service, the user and the
// raw DB handle (for asserting on stored hashes).
func newResetFixture(t *testing.T) (*auth.Service, auth.User, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	svc := auth.NewService(sqlDB, "test-jwt-secret")
	u, err := svc.Register(ctx, "reset@kiwit.local", "resetter", "supersecret123")
	require.NoError(t, err)
	return svc, u, sqlDB
}

// TestResetTokenCreateAndLookup: a freshly created token resolves back to the
// owning user and the expiry honours the TTL.
func TestResetTokenCreateAndLookup(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)

	plain, expiry, err := svc.CreateResetToken(ctx, u.ID, time.Hour)
	require.NoError(t, err)
	require.Len(t, plain, 64, "32 random bytes hex-encoded")
	require.WithinDuration(t, time.Now().UTC().Add(time.Hour), expiry, time.Minute)

	gotUserID, err := svc.FindValidResetToken(ctx, plain)
	require.NoError(t, err)
	require.Equal(t, u.ID, gotUserID)
}

// TestResetTokenMarkUsedInvalidates: using a token invalidates it.
func TestResetTokenMarkUsedInvalidates(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)

	plain, _, err := svc.CreateResetToken(ctx, u.ID, time.Hour)
	require.NoError(t, err)
	require.NoError(t, svc.MarkResetTokenUsed(ctx, plain))

	_, err = svc.FindValidResetToken(ctx, plain)
	require.ErrorIs(t, err, auth.ErrResetTokenInvalid)
}

// TestResetTokenNotFound: an unknown token is rejected.
func TestResetTokenNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newResetFixture(t)

	_, err := svc.FindValidResetToken(ctx, "0000000000000000000000000000000000000000000000000000000000000000")
	require.ErrorIs(t, err, auth.ErrResetTokenInvalid)
}

// TestResetTokenExpired: a token past its TTL is rejected.
func TestResetTokenExpired(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)

	plain, _, err := svc.CreateResetToken(ctx, u.ID, -time.Minute)
	require.NoError(t, err)

	_, err = svc.FindValidResetToken(ctx, plain)
	require.ErrorIs(t, err, auth.ErrResetTokenInvalid)
}

// TestResetTokenStoredHashed: the database stores only the SHA-256 hash,
// never the plain token.
func TestResetTokenStoredHashed(t *testing.T) {
	ctx := context.Background()
	svc, u, sqlDB := newResetFixture(t)

	plain, _, err := svc.CreateResetToken(ctx, u.ID, time.Hour)
	require.NoError(t, err)

	var storedHash string
	require.NoError(t, sqlDB.QueryRow(`SELECT token_hash FROM password_reset_tokens WHERE user_id = ?`, u.ID).Scan(&storedHash))
	require.NotEqual(t, plain, storedHash)
	require.Len(t, storedHash, 64, "hex-encoded SHA-256")
}

// TestPasswordResetServiceFlow covers the Phase 2 service contract on top of
// the SQLite store.
func TestPasswordResetServiceFlow(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)

	// Unknown e-mail: silent no-op (no enumeration), never an error.
	require.NoError(t, svc.RequestPasswordReset(ctx, "ghost@kiwit.local"))

	// Admin-requested reset without SMTP returns the reset link with the token.
	resetURL, emailed, err := svc.AdminRequestPasswordReset(ctx, u.ID)
	require.NoError(t, err)
	require.False(t, emailed)
	token := strings.TrimPrefix(resetURL, "http://localhost:8080/reset-password?token=")
	require.Len(t, token, 64, "reset URL must carry the one-time token")

	// Self-service request also succeeds (log-only without SMTP).
	require.NoError(t, svc.RequestPasswordReset(ctx, u.Email))

	// Confirm changes the password hash; the old password stops working.
	require.NoError(t, svc.ConfirmPasswordReset(ctx, token, "brandnewpass123"))
	_, _, err = svc.Login(ctx, u.Email, "supersecret123")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
	_, _, err = svc.Login(ctx, u.Email, "brandnewpass123")
	require.NoError(t, err)

	// The token is one-time: reuse is rejected.
	require.ErrorIs(t, svc.ConfirmPasswordReset(ctx, token, "yetanotherpass123"), auth.ErrResetTokenInvalid)
}

// TestAdminResetInvalidatesPreviousTokens: a second admin reset invalidates
// the first, still-unused token.
func TestAdminResetInvalidatesPreviousTokens(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)

	firstURL, _, err := svc.AdminRequestPasswordReset(ctx, u.ID)
	require.NoError(t, err)
	firstToken := strings.TrimPrefix(firstURL, "http://localhost:8080/reset-password?token=")
	_, err = svc.FindValidResetToken(ctx, firstToken)
	require.NoError(t, err)

	secondURL, _, err := svc.AdminRequestPasswordReset(ctx, u.ID)
	require.NoError(t, err)
	secondToken := strings.TrimPrefix(secondURL, "http://localhost:8080/reset-password?token=")
	require.NotEqual(t, firstToken, secondToken)

	// The first token no longer resolves; the newest one does.
	_, err = svc.FindValidResetToken(ctx, firstToken)
	require.ErrorIs(t, err, auth.ErrResetTokenInvalid)
	_, err = svc.FindValidResetToken(ctx, secondToken)
	require.NoError(t, err)
}

// captureSender is a fake EmailSender that records one delivery on a channel.
type captureSender struct {
	mail chan capturedMail
}

type capturedMail struct {
	to      []string
	subject string
	body    string
}

func (c *captureSender) Enabled() bool { return true }

func (c *captureSender) Send(to []string, subject, body string) error {
	c.mail <- capturedMail{to: to, subject: subject, body: body}
	return nil
}

// TestAdminResetWithSMTPEnabled: with an e-mail sender configured, the reset
// link goes out via e-mail only — never in the API response.
func TestAdminResetWithSMTPEnabled(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)
	sender := &captureSender{mail: make(chan capturedMail, 1)}
	svc.SetEmailSender(sender)

	resetURL, emailed, err := svc.AdminRequestPasswordReset(ctx, u.ID)
	require.NoError(t, err)
	require.True(t, emailed)
	require.Empty(t, resetURL, "link must never leak into the API response when SMTP is enabled")

	select {
	case mail := <-sender.mail:
		require.Equal(t, []string{"reset@kiwit.local"}, mail.to)
		require.Contains(t, mail.body, "/reset-password?token=")
		token := regexp.MustCompile(`[0-9a-f]{64}`).FindString(mail.body)
		require.Len(t, token, 64)
		userID, err := svc.FindValidResetToken(ctx, token)
		require.NoError(t, err, "e-mailed link must carry a valid one-time token")
		require.Equal(t, u.ID, userID)
	case <-time.After(3 * time.Second):
		t.Fatal("e-mail was not delivered by the background sender")
	}
}

// TestAdminResetWithDisabledSender: a configured but disabled sender behaves
// like no sender at all (self-hosted fallback without SMTP).
func TestAdminResetWithDisabledSender(t *testing.T) {
	ctx := context.Background()
	svc, u, _ := newResetFixture(t)
	svc.SetEmailSender(&deadSender{})

	resetURL, emailed, err := svc.AdminRequestPasswordReset(ctx, u.ID)
	require.NoError(t, err)
	require.False(t, emailed)
	require.NotEmpty(t, resetURL)
}

type deadSender struct{}

func (d *deadSender) Enabled() bool                          { return false }
func (d *deadSender) Send(to []string, subj, b string) error { return nil }

// TestAdminResetUnknownUser reports a missing target user without side effects.
func TestAdminResetUnknownUser(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newResetFixture(t)

	_, _, err := svc.AdminRequestPasswordReset(ctx, 9999)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
