package property

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes mounts entity properties and TestRun CC endpoints.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	// List properties: GET /api/v1/properties?entity_type=...&entity_id=...
	huma.Register(api, huma.Operation{
		OperationID: "list-properties",
		Method:      http.MethodGet,
		Path:        "/api/v1/properties",
		Summary:     "List key=value properties of an entity (test_case, test_run, environment)",
		Tags:        []string{"Properties"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_case,test_run,environment"`
		EntityID   int64  `query:"entity_id" minimum:"1"`
	}) (*struct {
		Body struct {
			Items []Property `json:"items"`
		}
	}, error) {
		if input.EntityType == "environment" { if _, ok := auth.UserFromContext(ctx); !ok { return nil, huma.Error401Unauthorized("unauthorized") } } else if err := guard.EnforceEntity(ctx, input.EntityType, input.EntityID, false); err != nil {
			return nil, huma.Error403Forbidden("no access")
		}
		items, err := store.List(ctx, input.EntityType, input.EntityID)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Items []Property `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	// Set property: POST /api/v1/properties
	huma.Register(api, huma.Operation{
		OperationID: "set-property",
		Method:      http.MethodPost,
		Path:        "/api/v1/properties",
		Summary:     "Set (upsert) a key=value property on an entity",
		Tags:        []string{"Properties"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			EntityType string `json:"entity_type" enum:"test_case,test_run,environment"`
			EntityID   int64  `json:"entity_id" minimum:"1"`
			Name       string `json:"name" minLength:"1" maxLength:"200"`
			Value      string `json:"value" maxLength:"4000"`
		}
	}) (*struct{ Body Property }, error) {
		if input.Body.EntityType == "environment" { if _, ok := auth.UserFromContext(ctx); !ok { return nil, huma.Error401Unauthorized("unauthorized") } } else if err := guard.EnforceEntity(ctx, input.Body.EntityType, input.Body.EntityID, true); err != nil {
			return nil, huma.Error403Forbidden("no write access")
		}
		p, err := store.Set(ctx, input.Body.EntityType, input.Body.EntityID, input.Body.Name, input.Body.Value)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body Property }{Body: p}, nil
	})

	// Delete property: DELETE /api/v1/properties?entity_type=...&entity_id=...&name=...
	huma.Register(api, huma.Operation{
		OperationID: "delete-property",
		Method:      http.MethodDelete,
		Path:        "/api/v1/properties",
		Summary:     "Delete a property from an entity",
		Tags:        []string{"Properties"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_case,test_run,environment"`
		EntityID   int64  `query:"entity_id" minimum:"1"`
		Name       string `query:"name" minLength:"1"`
	}) (*struct{}, error) {
		if input.EntityType == "environment" { if _, ok := auth.UserFromContext(ctx); !ok { return nil, huma.Error401Unauthorized("unauthorized") } } else if err := guard.EnforceEntity(ctx, input.EntityType, input.EntityID, true); err != nil {
			return nil, huma.Error403Forbidden("no write access")
		}
		if err := store.Delete(ctx, input.EntityType, input.EntityID, input.Name); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete property", err)
		}
		return nil, nil
	})

	// TestRun CC: GET /api/v1/testruns/{id}/cc
	huma.Register(api, huma.Operation{
		OperationID: "list-run-cc",
		Method:      http.MethodGet,
		Path:        "/api/v1/testruns/{id}/cc",
		Summary:     "List CC users of a test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Items []int64 `json:"items"`
		}
	}, error) {
		if err := guard.EnforceEntity(ctx, "test_run", input.ID, false); err != nil {
			return nil, huma.Error403Forbidden("no access")
		}
		items, err := store.ListRunCC(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list cc", err)
		}
		out := &struct {
			Body struct {
				Items []int64 `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	// Add CC: POST /api/v1/testruns/{id}/cc
	huma.Register(api, huma.Operation{
		OperationID: "add-run-cc",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/{id}/cc",
		Summary:     "Add a CC user to a test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			UserID int64 `json:"user_id" minimum:"1"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := guard.EnforceEntity(ctx, "test_run", input.ID, true); err != nil {
			return nil, huma.Error403Forbidden("no write access")
		}
		if err := store.AddRunCC(ctx, input.ID, input.Body.UserID); err != nil {
			return nil, huma.Error500InternalServerError("failed to add cc", err)
		}
		return &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{Body: struct {
			OK bool `json:"ok"`
		}{OK: true}}, nil
	})

	// Remove CC: DELETE /api/v1/testruns/{id}/cc/{user_id}
	huma.Register(api, huma.Operation{
		OperationID: "remove-run-cc",
		Method:      http.MethodDelete,
		Path:        "/api/v1/testruns/{id}/cc/{user_id}",
		Summary:     "Remove a CC user from a test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID     int64 `path:"id"`
		UserID int64 `path:"user_id"`
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := guard.EnforceEntity(ctx, "test_run", input.ID, true); err != nil {
			return nil, huma.Error403Forbidden("no write access")
		}
		if err := store.RemoveRunCC(ctx, input.ID, input.UserID); err != nil {
			return nil, huma.Error500InternalServerError("failed to remove cc", err)
		}
		return &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{Body: struct {
			OK bool `json:"ok"`
		}{OK: true}}, nil
	})
}
