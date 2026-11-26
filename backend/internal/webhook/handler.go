package webhook

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/notification"
)

// RegisterRoutes exposes webhook resources over REST. All endpoints are
// admin-only: webhooks are instance-level integration points.
func RegisterRoutes(api huma.API, store *Store) {
	requireAdmin := func(ctx context.Context) error {
		return auth.CheckRole(ctx, "admin")
	}

	type webhooksBody struct {
		Items []Webhook `json:"items"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "list-webhooks",
		Method:      http.MethodGet,
		Path:        "/api/v1/webhooks",
		Summary:     "List configured webhooks",
		Tags:        []string{"Webhooks"},
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body webhooksBody
	}, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		items, err := store.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list webhooks", err)
		}
		return &struct{ Body webhooksBody }{Body: webhooksBody{Items: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-webhook",
		Method:      http.MethodPost,
		Path:        "/api/v1/webhooks",
		Summary:     "Create a webhook",
		Tags:        []string{"Webhooks"},
	}, func(ctx context.Context, input *struct {
		Body webhookBody
	}) (*struct {
		Body Webhook
	}, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		w, err := buildWebhook(input.Body, true)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		created, err := store.Create(ctx, w)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create webhook", err)
		}
		return &struct{ Body Webhook }{Body: created}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-webhook",
		Method:      http.MethodPut,
		Path:        "/api/v1/webhooks/{id}",
		Summary:     "Update a webhook",
		Description: "Update a webhook. An omitted or empty secret keeps the currently stored secret, so an update never silently disables signing.",
		Tags:        []string{"Webhooks"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id" minimum:"1"`
		Body webhookBody
	}) (*struct {
		Body Webhook
	}, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		w, err := buildWebhook(input.Body, false)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		updated, err := store.Update(ctx, input.ID, w)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound("webhook not found")
			}
			return nil, huma.Error500InternalServerError("failed to update webhook", err)
		}
		return &struct{ Body Webhook }{Body: updated}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-webhook",
		Method:      http.MethodDelete,
		Path:        "/api/v1/webhooks/{id}",
		Summary:     "Delete a webhook",
		Tags:        []string{"Webhooks"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id" minimum:"1"`
	}) (*struct {
		Body struct {
			Deleted bool `json:"deleted"`
		}
	}, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound("webhook not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete webhook", err)
		}
		out := &struct {
			Body struct {
				Deleted bool `json:"deleted"`
			}
		}{}
		out.Body.Deleted = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "test-webhook",
		Method:      http.MethodPost,
		Path:        "/api/v1/webhooks/{id}/test",
		Summary:     "Send a signed test delivery to a webhook",
		Tags:        []string{"Webhooks"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id" minimum:"1"`
	}) (*struct {
		Body struct {
			Delivered  bool   `json:"delivered"`
			StatusCode int    `json:"status_code,omitempty"`
			Error      string `json:"error,omitempty"`
		}
	}, error) {
		if err := requireAdmin(ctx); err != nil {
			return nil, err
		}
		w, err := store.Get(ctx, input.ID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound("webhook not found")
			}
			return nil, huma.Error500InternalServerError("failed to load webhook", err)
		}
		status, err := Deliver(ctx, w, Payload{
			Event:      "webhook.test",
			EntityType: "webhook",
			EntityID:   w.ID,
			Title:      "Test delivery from Kiwit TCMS",
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		})
		if err != nil {
			slog.Default().Error("webhook test delivery", "err", err)
			return nil, huma.Error502BadGateway("webhook delivery failed; check server logs")
		}
		out := &struct {
			Body struct {
				Delivered  bool   `json:"delivered"`
				StatusCode int    `json:"status_code,omitempty"`
				Error      string `json:"error,omitempty"`
			}
		}{}
		out.Body.Delivered = true
		out.Body.StatusCode = status
		return out, nil
	})
}

// webhookBody is the request body shared by create and update.
// is_active is optional and defaults to true on create.
type webhookBody struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Secret string   `json:"secret,omitempty"`
	Events []string `json:"events"`
	Active *bool    `json:"is_active,omitempty"`
}

// buildWebhook validates the request body shared by create and update.
func buildWebhook(b webhookBody, requireEvents bool) (Webhook, error) {
	w := Webhook{
		Name:   strings.TrimSpace(b.Name),
		URL:    strings.TrimSpace(b.URL),
		Secret: b.Secret,
		Events: b.Events,
		Active: true,
	}
	if b.Active != nil {
		w.Active = *b.Active
	}
	if w.Name == "" {
		return Webhook{}, errors.New("name is required")
	}
	u, err := url.Parse(w.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Webhook{}, errors.New("url must be a valid http(s) URL")
	}
	if requireEvents && len(w.Events) == 0 {
		return Webhook{}, errors.New("at least one event is required")
	}
	for _, e := range w.Events {
		if !notification.IsValidEventType(e) {
			return Webhook{}, errors.New("unknown event type: " + e)
		}
	}
	w.Events = dedupeEvents(w.Events)
	return w, nil
}

func dedupeEvents(events []string) []string {
	seen := make(map[string]bool, len(events))
	out := make([]string, 0, len(events))
	for _, e := range events {
		if seen[e] {
			continue
		}
		seen[e] = true
		out = append(out, e)
	}
	return out
}
