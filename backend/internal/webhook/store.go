// Package webhook manages outbound webhook resources: per-event HTTP targets
// whose payloads are signed with an HMAC-SHA256 secret. Webhooks are
// instance-level integration resources managed by admins.
package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("webhook not found")

// Webhook is a configured outbound webhook. Events holds the event types the
// webhook subscribes to (stored as a JSON array). Secret is never serialized
// into API responses (json:"-"); requests still accept it via webhookBody.
type Webhook struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"`
	Events    []string  `json:"events"`
	Active    bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store persists webhooks in SQLite or PostgreSQL (dual-connection convention).
type Store struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

func NewStore(db *sql.DB, pgPool *pgxpool.Pool) *Store {
	return &Store{db: db, pgPool: pgPool}
}

const webhookColumns = `id, name, url, secret, events, is_active, created_at, updated_at`

func (s *Store) queryList(ctx context.Context, q string, args ...any) ([]Webhook, error) {
	out := []Webhook{}
	scan := func(scan func(dest ...any) error) error {
		var w Webhook
		var events string
		var active bool
		if err := scan(&w.ID, &w.Name, &w.URL, &w.Secret, &events, &active, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return err
		}
		w.Active = active
		w.Events = []string{}
		_ = json.Unmarshal([]byte(events), &w.Events)
		out = append(out, w)
		return nil
	}

	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			if err := scan(rows.Scan); err != nil {
				return nil, err
			}
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := scan(rows.Scan); err != nil {
			return nil, err
		}
	}
	return out, rows.Err()
}

// List returns all webhooks (newest last).
func (s *Store) List(ctx context.Context) ([]Webhook, error) {
	return s.queryList(ctx, `SELECT `+webhookColumns+` FROM webhooks ORDER BY id ASC`)
}

// listActive returns the active webhooks for event fan-out.
func (s *Store) listActive(ctx context.Context) ([]Webhook, error) {
	return s.queryList(ctx, `SELECT `+webhookColumns+` FROM webhooks WHERE is_active ORDER BY id ASC`)
}

// Get returns a single webhook by ID.
func (s *Store) Get(ctx context.Context, id int64) (Webhook, error) {
	var items []Webhook
	var err error
	if s.pgPool != nil {
		items, err = s.queryList(ctx, `SELECT `+webhookColumns+` FROM webhooks WHERE id = $1`, id)
	} else {
		items, err = s.queryList(ctx, `SELECT `+webhookColumns+` FROM webhooks WHERE id = ?`, id)
	}
	if err != nil {
		return Webhook{}, err
	}
	if len(items) == 0 {
		return Webhook{}, ErrNotFound
	}
	return items[0], nil
}

func marshalEvents(events []string) string {
	if len(events) == 0 {
		return "[]"
	}
	b, err := json.Marshal(events)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// Create inserts a webhook and returns the stored row.
func (s *Store) Create(ctx context.Context, w Webhook) (Webhook, error) {
	now := time.Now().UTC()
	events := marshalEvents(w.Events)

	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx, `
			INSERT INTO webhooks (name, url, secret, events, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $6)
			RETURNING `+webhookColumns, w.Name, w.URL, w.Secret, events, w.Active, now).Scan(
			&w.ID, &w.Name, &w.URL, &w.Secret, &events, &w.Active, &w.CreatedAt, &w.UpdatedAt)
		if err != nil {
			return Webhook{}, err
		}
		w.Events = []string{}
		_ = json.Unmarshal([]byte(events), &w.Events)
		return w, nil
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO webhooks (name, url, secret, events, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, w.Name, w.URL, w.Secret, events, w.Active, now, now)
	if err != nil {
		return Webhook{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Webhook{}, err
	}
	return s.Get(ctx, id)
}

// Update replaces the mutable fields of a webhook and returns the stored row.
// keepSecret controls the semantics of an empty w.Secret: with keepSecret=true
// the currently stored secret is kept (an update without re-sending the
// secret never disables signing); with keepSecret=false the secret is taken
// as-is, so an empty one disables signing (the secret was explicitly cleared).
func (s *Store) Update(ctx context.Context, id int64, w Webhook, keepSecret bool) (Webhook, error) {
	now := time.Now().UTC()
	events := marshalEvents(w.Events)

	if s.pgPool != nil {
		var query string
		if keepSecret {
			query = `UPDATE webhooks SET name = $1, url = $2, secret = CASE WHEN $3 = '' THEN secret ELSE $3 END, events = $4, is_active = $5, updated_at = $6
			WHERE id = $7`
		} else {
			query = `UPDATE webhooks SET name = $1, url = $2, secret = $3, events = $4, is_active = $5, updated_at = $6
			WHERE id = $7`
		}
		tag, err := s.pgPool.Exec(ctx, query, w.Name, w.URL, w.Secret, events, w.Active, now, id)
		if err != nil {
			return Webhook{}, err
		}
		if tag.RowsAffected() == 0 {
			return Webhook{}, ErrNotFound
		}
		return s.Get(ctx, id)
	}

	var query string
	var args []any
	if keepSecret {
		query = `UPDATE webhooks SET name = ?, url = ?, secret = CASE WHEN ? = '' THEN secret ELSE ? END, events = ?, is_active = ?, updated_at = ?
		WHERE id = ?`
		args = []any{w.Name, w.URL, w.Secret, w.Secret, events, w.Active, now, id}
	} else {
		query = `UPDATE webhooks SET name = ?, url = ?, secret = ?, events = ?, is_active = ?, updated_at = ?
		WHERE id = ?`
		args = []any{w.Name, w.URL, w.Secret, events, w.Active, now, id}
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return Webhook{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Webhook{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete removes a webhook; ErrNotFound when the ID does not exist.
func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pgPool != nil {
		tag, err := s.pgPool.Exec(ctx, `DELETE FROM webhooks WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
