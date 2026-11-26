package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// ErrResetTokenInvalid reports an unknown, expired or already used password
// reset token. It is the single error surfaced publicly so callers cannot
// distinguish the three cases.
var ErrResetTokenInvalid = errors.New("invalid, expired or used password reset token")

// passwordResetTokenTTL is how long a reset token stays valid.
const passwordResetTokenTTL = time.Hour

// EmailSender is the minimal e-mail capability the password reset flow needs;
// *notify.EmailClient satisfies it.
type EmailSender interface {
	Send(to []string, subject, body string) error
	Enabled() bool
}

// hashResetToken returns the hex-encoded SHA-256 of the plain token. Only the
// hash ever reaches the database, so a data leak does not compromise accounts.
func hashResetToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// CreateResetToken generates a one-time password reset token for the user:
// 32 random bytes from crypto/rand, of which only the SHA-256 hash is stored.
// The plain token is returned exactly once, together with the expiry.
func (s *Service) CreateResetToken(ctx context.Context, userID int64, ttl time.Duration) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generate reset token: %w", err)
	}
	plain := hex.EncodeToString(raw)
	tokenHash := hashResetToken(plain)
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	if s.pgPool != nil {
		if _, err := s.pgPool.Exec(ctx, `
			INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, created_at)
			VALUES ($1, $2, $3, $4)`, userID, tokenHash, expiresAt, now); err != nil {
			return "", time.Time{}, fmt.Errorf("insert reset token: %w", err)
		}
		return plain, expiresAt, nil
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?)`, userID, tokenHash, expiresAt, now); err != nil {
		return "", time.Time{}, fmt.Errorf("insert reset token: %w", err)
	}
	return plain, expiresAt, nil
}

// FindValidResetToken resolves a still-valid reset token to its user ID.
// Unknown, expired or already used tokens yield ErrResetTokenInvalid.
func (s *Service) FindValidResetToken(ctx context.Context, plainToken string) (int64, error) {
	tokenHash := hashResetToken(plainToken)
	now := time.Now().UTC()

	if s.pgPool != nil {
		var userID int64
		err := s.pgPool.QueryRow(ctx, `
			SELECT user_id FROM password_reset_tokens
			WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2`,
			tokenHash, now).Scan(&userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, ErrResetTokenInvalid
			}
			return 0, fmt.Errorf("query reset token: %w", err)
		}
		return userID, nil
	}

	var userID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id FROM password_reset_tokens
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		tokenHash, now).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrResetTokenInvalid
		}
		return 0, fmt.Errorf("query reset token: %w", err)
	}
	return userID, nil
}

// MarkResetTokenUsed invalidates the given reset token (idempotent).
func (s *Service) MarkResetTokenUsed(ctx context.Context, plainToken string) error {
	tokenHash := hashResetToken(plainToken)
	now := time.Now().UTC()

	if s.pgPool != nil {
		_, err := s.pgPool.Exec(ctx,
			`UPDATE password_reset_tokens SET used_at = $1 WHERE token_hash = $2`, now, tokenHash)
		if err != nil {
			return fmt.Errorf("mark reset token used: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = ? WHERE token_hash = ?`, now, tokenHash); err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}
	return nil
}

// RequestPasswordReset starts the self-service flow for the given e-mail.
// Unknown e-mails are a silent no-op so the flow cannot enumerate accounts.
// With SMTP configured the one-time link is sent in the background; without
// it, the link is logged on the server (emergency access).
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	userID, userEmail, ok, err := s.findUserByEmail(ctx, email)
	if err != nil || !ok {
		return err // unknown e-mail: nil, caller answers a fixed 200
	}
	s.issuePasswordResetLink(ctx, userID, userEmail)
	return nil
}

// AdminRequestPasswordReset forces a fresh reset token for the target user.
// Creating it invalidates all of the user's previously active reset tokens,
// so only the newest link works at any time. With SMTP configured the link
// is emailed; without SMTP it is returned in the response (self-hosted
// fallback) and logged.
func (s *Service) AdminRequestPasswordReset(ctx context.Context, targetUserID int64) (string, bool, error) {
	var userEmail string
	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, targetUserID).Scan(&userEmail)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return "", false, sql.ErrNoRows
			}
			return "", false, fmt.Errorf("query user: %w", err)
		}
	} else {
		err := s.db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = ?`, targetUserID).Scan(&userEmail)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", false, sql.ErrNoRows
			}
			return "", false, fmt.Errorf("query user: %w", err)
		}
	}

	if err := s.invalidateActiveResetTokens(ctx, targetUserID); err != nil {
		return "", false, err
	}

	if s.emailSender != nil && s.emailSender.Enabled() {
		s.issuePasswordResetLink(ctx, targetUserID, userEmail)
		return "", true, nil
	}
	resetURL := s.buildResetURL(s.createPlainResetToken(ctx, targetUserID))
	slog.Info("password reset requested without SMTP; reset link follows", "user_id", targetUserID, "reset_url", resetURL)
	return resetURL, false, nil
}

// invalidateActiveResetTokens marks every unused reset token of the user as
// used, so issuing a new reset link invalidates all previous ones.
func (s *Service) invalidateActiveResetTokens(ctx context.Context, userID int64) error {
	now := time.Now().UTC()
	if s.pgPool != nil {
		if _, err := s.pgPool.Exec(ctx,
			`UPDATE password_reset_tokens SET used_at = $1 WHERE user_id = $2 AND used_at IS NULL`,
			now, userID); err != nil {
			return fmt.Errorf("invalidate reset tokens: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = ? WHERE user_id = ? AND used_at IS NULL`,
		now, userID); err != nil {
		return fmt.Errorf("invalidate reset tokens: %w", err)
	}
	return nil
}

// ConfirmPasswordReset completes the flow atomically: validates the one-time
// token, stores the new bcrypt hash on the user and marks the token used.
func (s *Service) ConfirmPasswordReset(ctx context.Context, plainToken, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	tokenHash := hashResetToken(plainToken)
	now := time.Now().UTC()

	if s.pgPool != nil {
		tx, err := s.pgPool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer func() { _ = tx.Rollback(ctx) }()

		var userID int64
		err = tx.QueryRow(ctx, `
			SELECT user_id FROM password_reset_tokens
			WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2`,
			tokenHash, now).Scan(&userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrResetTokenInvalid
			}
			return fmt.Errorf("query reset token: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3`,
			string(hash), now, userID); err != nil {
			return fmt.Errorf("update user password: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE password_reset_tokens SET used_at = $1 WHERE token_hash = $2`,
			now, tokenHash); err != nil {
			return fmt.Errorf("mark reset token used: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit tx: %w", err)
		}
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var userID int64
	err = tx.QueryRowContext(ctx, `
		SELECT user_id FROM password_reset_tokens
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		tokenHash, now).Scan(&userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrResetTokenInvalid
		}
		return fmt.Errorf("query reset token: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		string(hash), now, userID); err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = ? WHERE token_hash = ?`,
		now, tokenHash); err != nil {
		return fmt.Errorf("mark reset token used: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// findUserByEmail resolves an e-mail to (userID, email, exists).
func (s *Service) findUserByEmail(ctx context.Context, email string) (int64, string, bool, error) {
	if s.pgPool != nil {
		var id int64
		var userEmail string
		err := s.pgPool.QueryRow(ctx, `SELECT id, email FROM users WHERE email = $1`, email).Scan(&id, &userEmail)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, "", false, nil
			}
			return 0, "", false, fmt.Errorf("query user: %w", err)
		}
		return id, userEmail, true, nil
	}
	var id int64
	var userEmail string
	err := s.db.QueryRowContext(ctx, `SELECT id, email FROM users WHERE email = ?`, email).Scan(&id, &userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", false, nil
		}
		return 0, "", false, fmt.Errorf("query user: %w", err)
	}
	return id, userEmail, true, nil
}

// createPlainResetToken creates a fresh reset token and returns the plain
// value once (TTL passwordResetTokenTTL).
func (s *Service) createPlainResetToken(ctx context.Context, userID int64) string {
	plain, _, err := s.CreateResetToken(ctx, userID, passwordResetTokenTTL)
	if err != nil {
		// Callers treat an empty token as "no link"; the error is logged.
		slog.Error("failed to create password reset token", "user_id", userID, "err", err)
		return ""
	}
	return plain
}

// buildResetURL renders the user-facing reset link.
func (s *Service) buildResetURL(plainToken string) string {
	base := strings.TrimRight(s.baseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/reset-password?token=" + plainToken
}

// issuePasswordResetLink creates a fresh token and delivers the link: fired
// in the background via SMTP when configured, otherwise logged server-side.
func (s *Service) issuePasswordResetLink(ctx context.Context, userID int64, userEmail string) {
	plain := s.createPlainResetToken(ctx, userID)
	if plain == "" {
		return
	}
	resetURL := s.buildResetURL(plain)
	if s.emailSender != nil && s.emailSender.Enabled() {
		go func(email, link string) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("password reset e-mail delivery panic", "err", r)
				}
			}()
			body := "Someone requested a password reset for your Kiwit account.\n\n" +
				"Open the link below to choose a new password. The link is valid for one hour " +
				"and can be used only once:\n\n" + link + "\n\n" +
				"If you did not request this, you can safely ignore this message."
			if err := s.emailSender.Send([]string{email}, "[Kiwit] Password reset", body); err != nil {
				slog.Error("password reset e-mail delivery failed", "err", err)
			}
		}(userEmail, resetURL)
		return
	}
	slog.Info("password reset requested without SMTP; reset link follows", "user_id", userID, "reset_url", resetURL)
}
