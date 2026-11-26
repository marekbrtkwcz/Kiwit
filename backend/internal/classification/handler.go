package classification

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes mounts CRUD for product classifications (admin only for writes).
func RegisterRoutes(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "list-classifications",
		Method:      http.MethodGet,
		Path:        "/api/v1/classifications",
		Summary:     "List product classifications",
		Tags:        []string{"Dictionaries"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Classification `json:"items"`
		}
	}, error) {
		items, err := store.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list classifications", err)
		}
		out := &struct {
			Body struct {
				Items []Classification `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-classification",
		Method:      http.MethodPost,
		Path:        "/api/v1/classifications",
		Summary:     "Create a product classification (admin only)",
		Tags:        []string{"Dictionaries"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name string `json:"name" minLength:"1" maxLength:"100"`
		}
	}) (*struct{ Body Classification }, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		c, err := store.Create(ctx, input.Body.Name)
		if err != nil {
			return nil, huma.Error409Conflict("classification already exists", err)
		}
		return &struct{ Body Classification }{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-classification",
		Method:      http.MethodDelete,
		Path:        "/api/v1/classifications/{id}",
		Summary:     "Delete a classification (admin only; products keep NULL)",
		Tags:        []string{"Dictionaries"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete classification", err)
		}
		return nil, nil
	})
}
