package testplan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/notify"
)

type TestPlan struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	VersionID   int64     `json:"version_id"`
	ParentID    *int64    `json:"parent_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	PlanType    string    `json:"plan_type"`
	ExtraLink   string    `json:"extra_link,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateTestPlanInput struct {
	Body struct {
		ProductID   int64  `json:"product_id"`
		VersionID   int64  `json:"version_id"`
		ParentID    *int64 `json:"parent_id,omitempty"`
		Name        string `json:"name" minLength:"1" maxLength:"255"`
		Description string `json:"description,omitempty"`
		PlanType    string `json:"plan_type,omitempty" default:"regression"`
		ExtraLink   string `json:"extra_link,omitempty"`
	}
}

type UpdateTestPlanInput struct {
	ID   int64 `path:"id"`
	Body struct {
		VersionID   *int64 `json:"version_id,omitempty"`
		ParentID    *int64 `json:"parent_id,omitempty"`
		Name        string `json:"name" minLength:"1" maxLength:"255"`
		Description string `json:"description,omitempty"`
		PlanType    string `json:"plan_type,omitempty"`
		ExtraLink   string `json:"extra_link,omitempty"`
		IsActive    *bool  `json:"is_active,omitempty"`
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
		PlanID int64 `json:"plan_id"`
		Added  int   `json:"added"`
	}
}

// RegisterRoutes registers TestPlan OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo Repository, emailSettings *notify.SettingsStore, emailClient *notify.EmailClient, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testplans",
		Method:      http.MethodGet,
		Path:        "/api/v1/testplans",
		Summary:     "List test plans with optional filters",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct {
		ProductID int64  `query:"product_id" doc:"Filter by product ID"`
		VersionID int64  `query:"version_id" doc:"Filter by version ID"`
		IsActive  string `query:"is_active" enum:",true,false" doc:"Filter by active flag"`
		TagID     int64  `query:"tag_id" doc:"Filter by tag ID"`
		Q         string `query:"q" doc:"Filter by name substring"`
	}) (*ListTestPlansOutput, error) {
		var isActive *bool
		if input.IsActive == "true" {
			v := true
			isActive = &v
		} else if input.IsActive == "false" {
			v := false
			isActive = &v
		}
		items, err := repo.ListFiltered(ctx, ListFilter{
			ProductID: input.ProductID,
			VersionID: input.VersionID,
			IsActive:  isActive,
			TagID:     input.TagID,
			Q:         input.Q,
			UserID:    auth.FilterUserID(ctx),
		})
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
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		tp, err := repo.CreateWithMeta(ctx, CreateParams{
			ProductID:   input.Body.ProductID,
			VersionID:   input.Body.VersionID,
			ParentID:    input.Body.ParentID,
			Name:        input.Body.Name,
			Description: input.Body.Description,
			PlanType:    input.Body.PlanType,
			ExtraLink:   input.Body.ExtraLink,
		})
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test plan", err)
		}
		return &TestPlanOutput{Body: tp}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-testplan",
		Method:      http.MethodPut,
		Path:        "/api/v1/testplans/{id}",
		Summary:     "Update test plan details",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *UpdateTestPlanInput) (*TestPlanOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		tp, err := repo.Update(ctx, input.ID, UpdateParams{
			VersionID:   input.Body.VersionID,
			ParentID:    input.Body.ParentID,
			Name:        input.Body.Name,
			Description: input.Body.Description,
			PlanType:    input.Body.PlanType,
			ExtraLink:   input.Body.ExtraLink,
			IsActive:    input.Body.IsActive,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test plan not found")
			}
			return nil, huma.Error400BadRequest("failed to update test plan", err)
		}
		emailSettings.NotifyIfEnabled(emailClient, "test_plan", tp.ID,
			fmt.Sprintf("[Kiwit] TestPlan #%d updated", tp.ID),
			fmt.Sprintf("TestPlan #%d (%s) was updated.", tp.ID, tp.Name),
		)
		return &TestPlanOutput{Body: tp}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-testplan-cases",
		Method:      http.MethodGet,
		Path:        "/api/v1/testplans/{id}/cases",
		Summary:     "List test cases attached to a test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Items []PlanCase `json:"items"`
		}
	}, error) {
		if err := guard.EnforceTestPlan(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := repo.ListCases(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list plan cases", err)
		}
		out := &struct {
			Body struct {
				Items []PlanCase `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "add-testplan-cases",
		Method:      http.MethodPost,
		Path:        "/api/v1/testplans/{id}/cases",
		Summary:     "Attach test cases to a test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *AddCasesInput) (*AddCasesOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		for _, cid := range input.Body.CaseIDs {
			if err := guard.EnforceTestCase(ctx, cid, false); err != nil {
				return nil, err
			}
		}
		if err := repo.AddCases(ctx, input.ID, input.Body.CaseIDs); err != nil {
			return nil, huma.Error400BadRequest("failed to attach cases", err)
		}
		emailSettings.NotifyIfEnabled(emailClient, "test_plan", input.ID,
			fmt.Sprintf("[Kiwit] TestPlan #%d updated", input.ID),
			fmt.Sprintf("%d test case(s) attached to TestPlan #%d.", len(input.Body.CaseIDs), input.ID),
		)
		out := &AddCasesOutput{}
		out.Body.PlanID = input.ID
		out.Body.Added = len(input.Body.CaseIDs)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "remove-testplan-case",
		Method:      http.MethodDelete,
		Path:        "/api/v1/testplans/{id}/cases/{case_id}",
		Summary:     "Detach a single test case from a test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct {
		ID     int64 `path:"id"`
		CaseID int64 `path:"case_id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := repo.RemoveCase(ctx, input.ID, input.CaseID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test case not attached to plan")
			}
			return nil, huma.Error500InternalServerError("failed to detach case", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "reorder-testplan-cases",
		Method:      http.MethodPut,
		Path:        "/api/v1/testplans/{id}/cases/order",
		Summary:     "Reorder test cases attached to a test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			CaseIDs []int64 `json:"case_ids" minItems:"1"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := repo.ReorderCases(ctx, input.ID, input.Body.CaseIDs); err != nil {
			return nil, huma.Error400BadRequest("failed to reorder cases", err)
		}
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
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
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		tp, err := repo.Clone(ctx, input.ID, input.Body.Name, input.Body.VersionID)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to clone test plan", err)
		}
		return &TestPlanOutput{Body: tp}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-testplan",
		Method:      http.MethodDelete,
		Path:        "/api/v1/testplans/{id}",
		Summary:     "Delete a test plan",
		Tags:        []string{"TestPlans"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := repo.Delete(ctx, input.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test plan not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete test plan", err)
		}
		return nil, nil
	})
}
