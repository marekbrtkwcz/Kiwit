package notify

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailSettings struct {
	EntityType     string   `json:"entity_type"`
	EntityID       int64    `json:"entity_id"`
	NotifyOnUpdate bool     `json:"notify_on_update"`
	CCList         []string `json:"cc_list"`
}

type SettingsStore struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

func NewSettingsStore(db *sql.DB, pgPool *pgxpool.Pool) *SettingsStore {
	return &SettingsStore{db: db, pgPool: pgPool}
}

func (s *SettingsStore) Get(ctx context.Context, entityType string, entityID int64) (EmailSettings, error) {
	out := EmailSettings{EntityType: entityType, EntityID: entityID, NotifyOnUpdate: true, CCList: []string{}}
	var rawCC string
	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx,
			`SELECT notify_on_update, cc_list FROM email_settings WHERE entity_type = $1 AND entity_id = $2`,
			entityType, entityID,
		).Scan(&out.NotifyOnUpdate, &rawCC)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return out, nil
			}
			return out, err
		}
		out.CCList = splitCC(rawCC)
		return out, nil
	}

	var notifyInt int
	err := s.db.QueryRowContext(ctx,
		`SELECT notify_on_update, cc_list FROM email_settings WHERE entity_type = ? AND entity_id = ?`,
		entityType, entityID,
	).Scan(&notifyInt, &rawCC)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, nil
		}
		return out, err
	}
	out.NotifyOnUpdate = notifyInt == 1
	out.CCList = splitCC(rawCC)
	return out, nil
}

func (s *SettingsStore) Upsert(ctx context.Context, in EmailSettings) (EmailSettings, error) {
	rawCC := strings.Join(in.CCList, ",")
	now := time.Now().UTC()
	if s.pgPool != nil {
		_, err := s.pgPool.Exec(ctx, `
			INSERT INTO email_settings (entity_type, entity_id, notify_on_update, cc_list, updated_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (entity_type, entity_id)
			DO UPDATE SET notify_on_update = EXCLUDED.notify_on_update, cc_list = EXCLUDED.cc_list, updated_at = EXCLUDED.updated_at`,
			in.EntityType, in.EntityID, in.NotifyOnUpdate, rawCC, now,
		)
		return in, err
	}

	notifyInt := 0
	if in.NotifyOnUpdate {
		notifyInt = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO email_settings (entity_type, entity_id, notify_on_update, cc_list, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id)
		DO UPDATE SET notify_on_update = excluded.notify_on_update, cc_list = excluded.cc_list, updated_at = excluded.updated_at`,
		in.EntityType, in.EntityID, notifyInt, rawCC, now,
	)
	return in, err
}

func RegisterEmailRoutes(api huma.API, store *SettingsStore, emailClient *EmailClient) {
	huma.Register(api, huma.Operation{
		OperationID: "get-email-settings",
		Method:      http.MethodGet,
		Path:        "/api/v1/email-settings",
		Summary:     "Get email notification settings and CC list for TestCase or TestPlan",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_case,test_plan"`
		EntityID   int64  `query:"entity_id" minimum:"1"`
	}) (*struct{ Body EmailSettings }, error) {
		res, err := store.Get(ctx, input.EntityType, input.EntityID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load email settings", err)
		}
		return &struct{ Body EmailSettings }{Body: res}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "upsert-email-settings",
		Method:      http.MethodPut,
		Path:        "/api/v1/email-settings",
		Summary:     "Save email notification settings and CC list for TestCase or TestPlan",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			EntityType     string   `json:"entity_type" enum:"test_case,test_plan"`
			EntityID       int64    `json:"entity_id" minimum:"1"`
			NotifyOnUpdate bool     `json:"notify_on_update"`
			CCList         []string `json:"cc_list"`
		}
	}) (*struct{ Body EmailSettings }, error) {
		saved, err := store.Upsert(ctx, EmailSettings{
			EntityType:     input.Body.EntityType,
			EntityID:       input.Body.EntityID,
			NotifyOnUpdate: input.Body.NotifyOnUpdate,
			CCList:         input.Body.CCList,
		})
		if err != nil {
			return nil, huma.Error400BadRequest("failed to save email settings", err)
		}
		return &struct{ Body EmailSettings }{Body: saved}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "send-test-email",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/email-test",
		Summary:     "Send a test SMTP email",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			To      []string `json:"to" minItems:"1"`
			Subject string   `json:"subject" minLength:"1"`
			Body    string   `json:"body" minLength:"1"`
		}
	}) (*struct {
		Body struct {
			Sent bool `json:"sent"`
		}
	}, error) {
		if !emailClient.Enabled() {
			return nil, huma.Error400BadRequest("KIWIT_SMTP_HOST is not configured")
		}
		if err := emailClient.Send(input.Body.To, input.Body.Subject, input.Body.Body); err != nil {
			return nil, huma.Error502BadGateway("failed to send email", err)
		}
		out := &struct {
			Body struct {
				Sent bool `json:"sent"`
			}
		}{}
		out.Body.Sent = true
		return out, nil
	})
}

func splitCC(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
