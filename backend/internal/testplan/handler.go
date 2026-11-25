package testplan

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type TestPlan struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	VersionID   int64     `json:"version_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateTestPlanInput struct {
	Body struct {
		ProductID   int64  `json:"product_id"`
		VersionID   int64  `json:"version_id"`
		Name        string `json:"name" minLength:"1" maxLength:"255"`
		Description string `json:"description,omitempty"`
	}
}

type TestPlanOutput struct {
	Body TestPlan
}

type ListTestPlansOutput struct {
	Body struct {
		Items []TestPlan `json:"items"`
	}
}

type AddCasesInput struct {
	ID   int64 `path:"id"`
	Body struct {
		CaseIDs []int64 `json:"case_ids" minItems:"1"`
	}
}

type AddCasesOutput struct {
	Body struct {
		PlanID  int64 `json:"plan_id"`
		Added   int   `json:"added"`
	}
}

// RegisterRoutes registers TestPlan OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo Repository) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testplans",
		Method:      http.MethodGet,
		Path:        "/api/v1/testplans",
		Summary:     "List test plans",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct{}) (*ListTestPlansOutput, error) {
		items, err := repo.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list test plans", err)
		}
		out := &ListTestPlansOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-testplan",
		Method:      http.MethodPost,
		Path:        "/api/v1/testplans",
		Summary:     "Create test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *CreateTestPlanInput) (*TestPlanOutput, error) {
		tp, err := repo.Create(ctx, input.Body.ProductID, input.Body.VersionID, input.Body.Name, input.Body.Description)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test plan", err)
		}
		return &TestPlanOutput{Body: tp}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "add-testplan-cases",
		Method:      http.MethodPost,
		Path:        "/api/v1/testplans/{id}/cases",
		Summary:     "Attach test cases to a test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *AddCasesInput) (*AddCasesOutput, error) {
		if err := repo.AddCases(ctx, input.ID, input.Body.CaseIDs); err != nil {
			return nil, huma.Error400BadRequest("failed to attach cases", err)
		}
		out := &AddCasesOutput{}
		out.Body.PlanID = input.ID
		out.Body.Added = len(input.Body.CaseIDs)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "clone-testplan",
		Method:      http.MethodPost,
		Path:        "/api/v1/testplans/{id}/clone",
		Summary:     "Clone a test plan with all attached test cases",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Name      string `json:"name" minLength:"1" maxLength:"255"`
			VersionID *int64 `json:"version_id,omitempty"`
		}
	}) (*TestPlanOutput, error) {
		tp, err := repo.Clone(ctx, input.ID, input.Body.Name, input.Body.VersionID)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to clone test plan", err)
		}
		return &TestPlanOutput{Body: tp}, nil
	})
}
