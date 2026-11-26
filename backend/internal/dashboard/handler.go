package dashboard

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes mounts the personalized dashboard endpoint.
func RegisterRoutes(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "get-dashboard",
		Method:      http.MethodGet,
		Path:        "/api/v1/dashboard",
		Summary:     "Personalized dashboard: my test runs and recent activity",
		Tags:        []string{"Dashboard"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body Snapshot
	}, error) {
		u, ok := auth.UserFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("authentication required")
		}
		snap, err := store.Get(ctx, u.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load dashboard", err)
		}
		return &struct {
			Body Snapshot
		}{Body: snap}, nil
	})
}
