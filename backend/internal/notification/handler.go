package notification

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes exposes the in-app notification center over REST.
// All endpoints operate only on the authenticated user's own notifications,
// so no product-level access guard is needed.
func RegisterRoutes(api huma.API, store *Store) {
	currentUserID := func(ctx context.Context) (int64, error) {
		u, ok := auth.UserFromContext(ctx)
		if !ok {
			return 0, huma.Error401Unauthorized("unauthorized")
		}
		return u.ID, nil
	}

	huma.Register(api, huma.Operation{
		OperationID: "list-notifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/notifications",
		Summary:     "List the current user's in-app notifications",
		Description: "Returns newest notifications for the authenticated user, the total count and the unread count for the bell badge.",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		Unread bool `query:"unread" doc:"Only return unread notifications"`
		Limit  int  `query:"limit" minimum:"1" maximum:"100" doc:"Page size (default 20)"`
		Offset int  `query:"offset" minimum:"0" doc:"Page offset"`
	}) (*struct {
		Body ListResult
	}, error) {
		userID, err := currentUserID(ctx)
		if err != nil {
			return nil, err
		}
		res, err := store.List(ctx, userID, input.Unread, input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list notifications", err)
		}
		return &struct{ Body ListResult }{Body: res}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "read-notification",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/{id}/read",
		Summary:     "Mark a single notification as read",
		Description: "Marks the notification as read; silently succeeds when it was already read. Returns 404 when the notification does not belong to the caller.",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id" minimum:"1"`
	}) (*struct {
		Body struct {
			Read bool `json:"read"`
		}
	}, error) {
		userID, err := currentUserID(ctx)
		if err != nil {
			return nil, err
		}
		if err := store.MarkRead(ctx, userID, input.ID, time.Now().UTC()); err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil, huma.Error404NotFound("notification not found")
			}
			return nil, huma.Error500InternalServerError("failed to mark notification", err)
		}
		out := &struct {
			Body struct {
				Read bool `json:"read"`
			}
		}{}
		out.Body.Read = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-my-notification-settings",
		Method:      http.MethodGet,
		Path:        "/api/v1/me/notification-settings",
		Summary:     "Get the current user's notification preferences (event x channel matrix)",
		Description: "Returns the notification event catalog with effective in_app/email flags for the authenticated user. Unconfigured events default to enabled.",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body struct {
			Events []EventPreference `json:"events"`
		}
	}, error) {
		userID, err := currentUserID(ctx)
		if err != nil {
			return nil, err
		}
		events, err := store.Preferences(ctx, userID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load notification settings", err)
		}
		out := &struct {
			Body struct {
				Events []EventPreference `json:"events"`
			}
		}{}
		out.Body.Events = events
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "put-my-notification-settings",
		Method:      http.MethodPut,
		Path:        "/api/v1/me/notification-settings",
		Summary:     "Update the current user's notification preferences",
		Description: "Upserts the per-event in_app/email flags and returns the merged event matrix.",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Events []EventPreference `json:"events" minItems:"1"`
		}
	}) (*struct {
		Body struct {
			Events []EventPreference `json:"events"`
		}
	}, error) {
		userID, err := currentUserID(ctx)
		if err != nil {
			return nil, err
		}
		for _, e := range input.Body.Events {
			if !IsValidEventType(e.EventType) {
				return nil, huma.Error400BadRequest("unknown event_type: " + e.EventType)
			}
		}
		events, err := store.UpsertPreferences(ctx, userID, input.Body.Events)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to save notification settings", err)
		}
		out := &struct {
			Body struct {
				Events []EventPreference `json:"events"`
			}
		}{}
		out.Body.Events = events
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "read-all-notifications",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/read-all",
		Summary:     "Mark all of the current user's notifications as read",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body struct {
			Updated int64 `json:"updated"`
		}
	}, error) {
		userID, err := currentUserID(ctx)
		if err != nil {
			return nil, err
		}
		n, err := store.MarkAllRead(ctx, userID, time.Now().UTC())
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to mark notifications", err)
		}
		out := &struct {
			Body struct {
				Updated int64 `json:"updated"`
			}
		}{}
		out.Body.Updated = n
		return out, nil
	})
}
