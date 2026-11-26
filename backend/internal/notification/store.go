// Package notification stores and serves per-user in-app notifications.
// Notifications are personal (scoped to the recipient user_id) and are
// delivered synchronously through the REST API defined in handler.go.
package notification

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification is a single in-app notification owned by one user.
type Notification struct {
	ID         int64      `json:"id"`
	EventType  string     `json:"event_type"`
	EntityType string     `json:"entity_type,omitempty"`
	EntityID   int64      `json:"entity_id,omitempty"`
	Title      string     `json:"title"`
	Body       string     `json:"body,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ListResult is the payload returned by GET /api/v1/notifications.
type ListResult struct {
	Items       []Notification `json:"items"`
	Total       int64          `json:"total"`
	UnreadCount int64          `json:"unread_count"`
}

// Store persists notifications in SQLite or PostgreSQL using the same
// dual-connection convention as notify.SettingsStore.
type Store struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

// NewStore creates a notification store backed by the active database engine.
func NewStore(db *sql.DB, pgPool *pgxpool.Pool) *Store {
	return &Store{db: db, pgPool: pgPool}
}

const notificationColumns = `id, event_type, entity_type, entity_id, title, body, read_at, created_at`

func scanNotification(scan func(dest ...any) error) (Notification, error) {
	var n Notification
	var readAt sql.NullTime
	err := scan(&n.ID, &n.EventType, &n.EntityType, &n.EntityID, &n.Title, &n.Body, &readAt, &n.CreatedAt)
	if err != nil {
		return Notification{}, err
	}
	if readAt.Valid {
		n.ReadAt = &readAt.Time
	}
	return n, nil
}

// List returns the user's notifications (newest first), the total number of
// matching rows and the count of all unread rows for the badge.
func (s *Store) List(ctx context.Context, userID int64, unreadOnly bool, limit, offset int) (ListResult, error) {
	out := ListResult{Items: []Notification{}, Total: 0, UnreadCount: 0}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	filter := ""
	if unreadOnly {
		filter = " AND read_at IS NULL"
	}

	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx, `
			SELECT `+notificationColumns+` FROM notifications
			WHERE user_id = $1`+filter+`
			ORDER BY id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
		if err != nil {
			return out, err
		}
		defer rows.Close()
		for rows.Next() {
			n, err := scanNotification(rows.Scan)
			if err != nil {
				return out, err
			}
			out.Items = append(out.Items, n)
		}
		if err := rows.Err(); err != nil {
			return out, err
		}
		if err := s.pgPool.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END), 0)
			FROM notifications WHERE user_id = $1`+filter, userID).Scan(&out.Total, &out.UnreadCount); err != nil {
			return out, err
		}
		return out, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+notificationColumns+` FROM notifications
		WHERE user_id = ?`+filter+`
		ORDER BY id DESC LIMIT ? OFFSET ?`, userID, limit, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		n, err := scanNotification(rows.Scan)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, n)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END), 0)
		FROM notifications WHERE user_id = ?`+filter, userID).Scan(&out.Total, &out.UnreadCount); err != nil {
		return out, err
	}
	return out, nil
}

var ErrNotFound = errors.New("notification not found")

// MarkRead marks a notification as read. Only the owner may do this; returns
// ErrNotFound when the notification does not exist for that user.
func (s *Store) MarkRead(ctx context.Context, userID, notificationID int64, now time.Time) error {
	if s.pgPool != nil {
		tag, err := s.pgPool.Exec(ctx, `
			UPDATE notifications SET read_at = $1
			WHERE id = $2 AND user_id = $3 AND read_at IS NULL`, now, notificationID, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return s.ensureOwned(ctx, userID, notificationID)
		}
		return nil
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET read_at = ?
		WHERE id = ? AND user_id = ? AND read_at IS NULL`, now, notificationID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return s.ensureOwned(ctx, userID, notificationID)
	}
	return nil
}

// ensureOwned distinguishes "already read" (ok) from "not yours / missing".
func (s *Store) ensureOwned(ctx context.Context, userID, notificationID int64) error {
	var one int64
	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx, `SELECT 1 FROM notifications WHERE id = $1 AND user_id = $2`, notificationID, userID).Scan(&one)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		return nil
	}
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM notifications WHERE id = ? AND user_id = ?`, notificationID, userID).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// MarkAllRead marks every unread notification of the user as read and returns
// the number of updated rows.
func (s *Store) MarkAllRead(ctx context.Context, userID int64, now time.Time) (int64, error) {
	if s.pgPool != nil {
		tag, err := s.pgPool.Exec(ctx, `
			UPDATE notifications SET read_at = $1
			WHERE user_id = $2 AND read_at IS NULL`, now, userID)
		if err != nil {
			return 0, err
		}
		return tag.RowsAffected(), nil
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE notifications SET read_at = ?
		WHERE user_id = ? AND read_at IS NULL`, now, userID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Emit inserts one in-app notification per recipient (deduplicated, skipping
// non-positive IDs) unless the recipient disabled the in-app channel for the
// event in their preferences. It is intentionally cheap so handlers can
// fire-and-forget.
func (s *Store) Emit(ctx context.Context, recipients []int64, eventType, entityType string, entityID int64, title, body string) error {
	seen := make(map[int64]bool, len(recipients))
	now := time.Now().UTC()
	for _, uid := range recipients {
		if uid <= 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		if !s.channelEnabled(ctx, uid, eventType, "in_app") {
			continue
		}
		if s.pgPool != nil {
			if _, err := s.pgPool.Exec(ctx, `
				INSERT INTO notifications (user_id, event_type, entity_type, entity_id, title, body, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`, uid, eventType, entityType, entityID, title, body, now); err != nil {
				return err
			}
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO notifications (user_id, event_type, entity_type, entity_id, title, body, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, uid, eventType, entityType, entityID, title, body, now); err != nil {
			return err
		}
	}
	return nil
}
