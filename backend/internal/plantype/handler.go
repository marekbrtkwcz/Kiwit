package plantype

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes mounts CRUD for test plan types (admin only for writes).
func RegisterRoutes(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "list-plan-types",
		Method:      http.MethodGet,
		Path:        "/api/v1/plantypes",
		Summary:     "List test plan types",
		Tags:        []string{"Dictionaries"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []PlanType `json:"items"`
		}
	}, error) {
		items, err := store.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list plan types", err)
		}
		out := &struct {
			Body struct {
				Items []PlanType `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-plan-type",
		Method:      http.MethodPost,
		Path:        "/api/v1/plantypes",
		Summary:     "Create a test plan type (admin only)",
		Tags:        []string{"Dictionaries"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name string `json:"name" minLength:"1" maxLength:"100"`
		}
	}) (*struct{ Body PlanType }, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		t, err := store.Create(ctx, input.Body.Name)
		if err != nil {
			return nil, huma.Error409Conflict("plan type already exists", err)
		}
		return &struct{ Body PlanType }{Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-plan-type",
		Method:      http.MethodDelete,
		Path:        "/api/v1/plantypes/{id}",
		Summary:     "Delete a test plan type (admin only; plans keep NULL type)",
		Tags:        []string{"Dictionaries"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete plan type", err)
		}
		return nil, nil
	})
}
