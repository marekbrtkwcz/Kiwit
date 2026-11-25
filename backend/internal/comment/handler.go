package comment

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

func RegisterRoutes(api huma.API, repo Repository) {
	huma.Register(api, huma.Operation{
		OperationID: "list-comments",
		Method:      http.MethodGet,
		Path:        "/api/v1/comments",
		Summary:     "List comments for a TestCase or TestExecution",
		Tags:        []string{"Comments"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_case,test_execution"`
		EntityID   int64  `query:"entity_id" minimum:"1"`
	}) (*struct {
		Body struct {
			Items []Comment `json:"items"`
		}
	}, error) {
		items, err := repo.List(ctx, input.EntityType, input.EntityID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list comments", err)
		}
		out := &struct {
			Body struct {
				Items []Comment `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-comment",
		Method:      http.MethodPost,
		Path:        "/api/v1/comments",
		Summary:     "Add a comment to a TestCase or TestExecution",
		Tags:        []string{"Comments"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			EntityType string `json:"entity_type" enum:"test_case,test_execution"`
			EntityID   int64  `json:"entity_id" minimum:"1"`
			Body       string `json:"body" minLength:"1"`
		}
	}) (*struct{ Body Comment }, error) {
		var authorID *int64
		if u, ok := auth.UserFromContext(ctx); ok {
			authorID = &u.ID
		}
		c, err := repo.Create(ctx, input.Body.EntityType, input.Body.EntityID, authorID, input.Body.Body)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create comment", err)
		}
		return &struct{ Body Comment }{Body: c}, nil
	})
}
