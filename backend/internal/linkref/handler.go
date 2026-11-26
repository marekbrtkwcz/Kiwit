package linkref

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes mounts link reference CRUD endpoints.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-links",
		Method:      http.MethodGet,
		Path:        "/api/v1/links",
		Summary:     "List external links attached to a test run or execution",
		Tags:        []string{"Links"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_run,test_execution"`
		EntityID   int64  `query:"entity_id" minimum:"1"`
	}) (*struct {
		Body struct {
			Items []Link `json:"items"`
		}
	}, error) {
		if err := guard.EnforceEntity(ctx, input.EntityType, input.EntityID, false); err != nil {
			return nil, huma.Error403Forbidden("no access")
		}
		items, err := store.List(ctx, input.EntityType, input.EntityID)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Items []Link `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "add-link",
		Method:      http.MethodPost,
		Path:        "/api/v1/links",
		Summary:     "Attach an external link (name + URL) to a test run or execution",
		Tags:        []string{"Links"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			EntityType string `json:"entity_type" enum:"test_run,test_execution"`
			EntityID   int64  `json:"entity_id" minimum:"1"`
			Name       string `json:"name" minLength:"1" maxLength:"200"`
			URL        string `json:"url" minLength:"1" maxLength:"2000" format:"uri"`
		}
	}) (*struct{ Body Link }, error) {
		if err := guard.EnforceEntity(ctx, input.Body.EntityType, input.Body.EntityID, true); err != nil {
			return nil, huma.Error403Forbidden("no write access")
		}
		l, err := store.Add(ctx, input.Body.EntityType, input.Body.EntityID, input.Body.Name, input.Body.URL)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &struct{ Body Link }{Body: l}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-link",
		Method:      http.MethodDelete,
		Path:        "/api/v1/links/{id}",
		Summary:     "Delete an external link",
		Tags:        []string{"Links"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete link", err)
		}
		return nil, nil
	})
}
