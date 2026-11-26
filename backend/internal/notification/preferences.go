package notification

import (
	"context"
	"time"

	"github.com/kiwit/kiwit/internal/notify"
)

// EventTypes is the catalog of emittable events the user can configure.
// Every event defaults to enabled on both channels; a user's row in
// user_notification_settings overrides the defaults.
var EventTypes = []string{"run.stopped", "execution.failed", "execution.assigned"}

// EventPreference is the per-event channel matrix exposed over REST.
type EventPreference struct {
	EventType string `json:"event_type"`
	InApp     bool   `json:"in_app"`
	Email     bool   `json:"email"`
}

func IsValidEventType(eventType string) bool {
	for _, e := range EventTypes {
		if e == eventType {
			return true
		}
	}
	return false
}

// Preferences returns the full event matrix for the user: catalog defaults
// merged with stored overrides.
func (s *Store) Preferences(ctx context.Context, userID int64) ([]EventPreference, error) {
	byEvent := make(map[string]EventPreference, len(EventTypes))
	for _, e := range EventTypes {
		byEvent[e] = EventPreference{EventType: e, InApp: true, Email: true}
	}

	scan := func(eventType string, inApp, email bool) {
		p := byEvent[eventType] // default when unknown event stored
		p.EventType = eventType
		p.InApp = inApp
		p.Email = email
		byEvent[eventType] = p
	}

	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx,
			`SELECT event_type, in_app, email FROM user_notification_settings WHERE user_id = $1`, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var e string
			var inApp, email bool
			if err := rows.Scan(&e, &inApp, &email); err != nil {
				return nil, err
			}
			scan(e, inApp, email)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	} else {
		rows, err := s.db.QueryContext(ctx,
			`SELECT event_type, in_app, email FROM user_notification_settings WHERE user_id = ?`, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var e string
			var inAppInt, emailInt int
			if err := rows.Scan(&e, &inAppInt, &emailInt); err != nil {
				return nil, err
			}
			scan(e, inAppInt == 1, emailInt == 1)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	out := make([]EventPreference, 0, len(EventTypes))
	for _, e := range EventTypes {
		if p, ok := byEvent[e]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

// UpsertPreferences saves the given event rows and returns the merged matrix.
func (s *Store) UpsertPreferences(ctx context.Context, userID int64, prefs []EventPreference) ([]EventPreference, error) {
	now := time.Now().UTC()
	for _, p := range prefs {
		if s.pgPool != nil {
			if _, err := s.pgPool.Exec(ctx, `
				INSERT INTO user_notification_settings (user_id, event_type, in_app, email, updated_at)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (user_id, event_type)
				DO UPDATE SET in_app = EXCLUDED.in_app, email = EXCLUDED.email, updated_at = EXCLUDED.updated_at`,
				userID, p.EventType, p.InApp, p.Email, now); err != nil {
				return nil, err
			}
			continue
		}
		inAppInt, emailInt := 0, 0
		if p.InApp {
			inAppInt = 1
		}
		if p.Email {
			emailInt = 1
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO user_notification_settings (user_id, event_type, in_app, email, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(user_id, event_type)
			DO UPDATE SET in_app = excluded.in_app, email = excluded.email, updated_at = excluded.updated_at`,
			userID, p.EventType, inAppInt, emailInt, now); err != nil {
			return nil, err
		}
	}
	return s.Preferences(ctx, userID)
}

// channelEnabled reports whether the user wants the given channel for the
// event. Missing rows default to true; errors fail open (notify anyway).
// The channel name is interpolated into SQL, so only the two known channels
// may reach the query; anything else fails open without touching the database.
func (s *Store) channelEnabled(ctx context.Context, userID int64, eventType, channel string) bool {
	if channel != "in_app" && channel != "email" {
		return true
	}
	if s.pgPool != nil {
		var enabled bool
		err := s.pgPool.QueryRow(ctx,
			`SELECT `+channel+` FROM user_notification_settings WHERE user_id = $1 AND event_type = $2`,
			userID, eventType).Scan(&enabled)
		if err != nil {
			return true
		}
		return enabled
	}
	var enabledInt int
	err := s.db.QueryRowContext(ctx,
		`SELECT `+channel+` FROM user_notification_settings WHERE user_id = ? AND event_type = ?`,
		userID, eventType).Scan(&enabledInt)
	if err != nil {
		return true
	}
	return enabledInt == 1
}

// EmitEmail sends the event over SMTP to recipients who enabled the email
// channel. It is fire-and-forget: SMTP latency must never block a request.
func (s *Store) EmitEmail(_ context.Context, client *notify.EmailClient, recipients []int64, eventType, subject, body string) {
	if s == nil || client == nil || !client.Enabled() || len(recipients) == 0 {
		return
	}
	seen := make(map[int64]bool, len(recipients))
	filtered := make([]int64, 0, len(recipients))
	for _, uid := range recipients {
		if uid <= 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		filtered = append(filtered, uid)
	}
	if len(filtered) == 0 {
		return
	}

	go func(userIDs []int64) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, uid := range userIDs {
			if !s.channelEnabled(ctx, uid, eventType, "email") {
				continue
			}
			var email string
			var err error
			if s.pgPool != nil {
				err = s.pgPool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, uid).Scan(&email)
			} else {
				err = s.db.QueryRowContext(ctx, `SELECT email FROM users WHERE id = ?`, uid).Scan(&email)
			}
			if err != nil {
				continue
			}
			_ = client.Send([]string{email}, subject, body)
		}
	}(filtered)
}
