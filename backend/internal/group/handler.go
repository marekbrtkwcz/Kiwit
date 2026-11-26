package group

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes registers admin-only group management endpoints.
func RegisterRoutes(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-list-groups",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/groups",
		Summary:     "List user groups with members (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Group `json:"items"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		groups, err := store.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list groups", err)
		}
		for i := range groups {
			members, err := store.Members(ctx, groups[i].ID)
			if err == nil {
				groups[i].Members = members
			}
		}
		out := &struct {
			Body struct {
				Items []Group `json:"items"`
			}
		}{}
		out.Body.Items = groups
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-create-group",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/groups",
		Summary:     "Create a user group (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name string `json:"name" minLength:"1" maxLength:"100"`
		}
	}) (*struct{ Body Group }, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		g, err := store.Create(ctx, input.Body.Name)
		if err != nil {
			return nil, huma.Error409Conflict("group already exists or failed to create", err)
		}
		return &struct{ Body Group }{Body: g}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-group",
		Method:      http.MethodDelete,
		Path:        "/api/v1/admin/groups/{id}",
		Summary:     "Delete a user group (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete group", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-add-group-member",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/groups/{id}/members",
		Summary:     "Add a user to a group (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			UserID int64 `json:"user_id"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.AddMember(ctx, input.ID, input.Body.UserID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("group or user not found")
			}
			return nil, huma.Error500InternalServerError("failed to add member", err)
		}
		return &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{Body: struct {
			OK bool `json:"ok"`
		}{OK: true}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-list-group-product-perms",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/groups/{id}/product-perms",
		Summary:     "List per-product roles granted to a group (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Items []GroupProductPerm `json:"items"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		items, err := store.ListProductPerms(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list product perms", err)
		}
		out := &struct {
			Body struct {
				Items []GroupProductPerm `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-set-group-product-perm",
		Method:      http.MethodPut,
		Path:        "/api/v1/admin/groups/{id}/product-perms/{product_id}",
		Summary:     "Grant a group a role on a product (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID        int64  `path:"id"`
		ProductID int64  `path:"product_id"`
		Body      struct {
			Role string `json:"role" enum:"viewer,tester,maintainer"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.SetProductPerm(ctx, input.ID, input.ProductID, input.Body.Role); err != nil {
			return nil, huma.Error500InternalServerError("failed to set product perm", err)
		}
		return &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{Body: struct {
			OK bool `json:"ok"`
		}{OK: true}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-delete-group-product-perm",
		Method:      http.MethodDelete,
		Path:        "/api/v1/admin/groups/{id}/product-perms/{product_id}",
		Summary:     "Revoke a group's role on a product (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID        int64 `path:"id"`
		ProductID int64 `path:"product_id"`
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.DeleteProductPerm(ctx, input.ID, input.ProductID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete product perm", err)
		}
		return &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{Body: struct {
			OK bool `json:"ok"`
		}{OK: true}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "admin-remove-group-member",
		Method:      http.MethodDelete,
		Path:        "/api/v1/admin/groups/{id}/members/{user_id}",
		Summary:     "Remove a user from a group (admin only)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		ID     int64 `path:"id"`
		UserID int64 `path:"user_id"`
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.RemoveMember(ctx, input.ID, input.UserID); err != nil {
			return nil, huma.Error500InternalServerError("failed to remove member", err)
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
