package exploratory

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

type listOutput struct {
	Body struct {
		Items []Session `json:"items"`
	}
}

type singleOutput struct {
	Body Session
}

// RegisterRoutes mounts exploratory session CRUD.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-exploratory-sessions",
		Method:      http.MethodGet,
		Path:        "/api/v1/exploratory",
		Summary:     "List exploratory testing sessions, optionally per product",
		Tags:        []string{"Exploratory"},
	}, func(ctx context.Context, input *struct {
		ProductID int64 `query:"product_id"`
	}) (*listOutput, error) {
		if input.ProductID != 0 {
			if err := guard.EnforceProduct(ctx, input.ProductID, false); err != nil {
				return nil, err
			}
		}
		items, err := store.List(ctx, input.ProductID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list sessions", err)
		}
		out := &listOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-exploratory-session",
		Method:      http.MethodPost,
		Path:        "/api/v1/exploratory",
		Summary:     "Start an exploratory session (charter + timebox)",
		Tags:        []string{"Exploratory"},
	}, func(ctx context.Context, input *struct {
		Body SessionInput
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		if input.Body.TesterID == nil {
			if u, ok := auth.UserFromContext(ctx); ok {
				input.Body.TesterID = &u.ID
			}
		}
		s, err := store.Create(ctx, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create session", err)
		}
		return &singleOutput{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-exploratory-session",
		Method:      http.MethodGet,
		Path:        "/api/v1/exploratory/{id}",
		Summary:     "Get an exploratory session",
		Tags:        []string{"Exploratory"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*singleOutput, error) {
		pid, err := store.ProductIDOf(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("session not found")
			}
			return nil, huma.Error500InternalServerError("failed to resolve session", err)
		}
		if err := guard.EnforceProduct(ctx, pid, false); err != nil {
			return nil, err
		}
		s, err := store.Get(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to get session", err)
		}
		return &singleOutput{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-exploratory-session",
		Method:      http.MethodPut,
		Path:        "/api/v1/exploratory/{id}",
		Summary:     "Update an exploratory session (notes, findings)",
		Tags:        []string{"Exploratory"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body SessionInput
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		pid, err := store.ProductIDOf(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("session not found")
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		s, err := store.Update(ctx, input.ID, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to update session", err)
		}
		return &singleOutput{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-exploratory-session",
		Method:      http.MethodDelete,
		Path:        "/api/v1/exploratory/{id}",
		Summary:     "Delete an exploratory session",
		Tags:        []string{"Exploratory"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		pid, err := store.ProductIDOf(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("session not found")
		}
		if err := guard.EnforceProduct(ctx, pid, true); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete session", err)
		}
		return nil, nil
	})
}
