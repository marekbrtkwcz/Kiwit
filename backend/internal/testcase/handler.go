package testcase

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

type Step struct {
	Position       int    `json:"position" minimum:"1"`
	Action         string `json:"action" minLength:"1"`
	ExpectedResult string `json:"expected_result,omitempty"`
}

type TestCase struct {
	ID              int64     `json:"id"`
	ProductID       int64     `json:"product_id"`
	CategoryID      *int64    `json:"category_id,omitempty"`
	AuthorID        *int64    `json:"author_id,omitempty"`
	DefaultTesterID *int64    `json:"default_tester_id,omitempty"`
	ReviewerID      *int64    `json:"reviewer_id,omitempty"`
	Summary         string    `json:"summary"`
	Preconditions   string    `json:"preconditions,omitempty"`
	Requirement     string    `json:"requirement,omitempty"`
	ExtraLink       string    `json:"extra_link,omitempty"`
	Notes           string    `json:"notes,omitempty"`
	Format          string    `json:"format" enum:"steps,gherkin"`
	GherkinBody     string    `json:"gherkin_body,omitempty"`
	Priority        string    `json:"priority"`
	Status          string    `json:"status" enum:"draft,confirmed,disabled"`
	IsAutomated     bool      `json:"is_automated"`
	Script          string    `json:"script,omitempty"`
	Arguments       string    `json:"arguments,omitempty"`
	Steps           []Step    `json:"steps"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type TestCaseBody struct {
	ProductID       int64  `json:"product_id"`
	CategoryID      *int64 `json:"category_id,omitempty"`
	DefaultTesterID *int64 `json:"default_tester_id,omitempty"`
	ReviewerID      *int64 `json:"reviewer_id,omitempty"`
	Summary         string `json:"summary" minLength:"1" maxLength:"255"`
	Preconditions   string `json:"preconditions,omitempty"`
	Requirement     string `json:"requirement,omitempty"`
	ExtraLink       string `json:"extra_link,omitempty"`
	Notes           string `json:"notes,omitempty"`
	Format          string `json:"format,omitempty" default:"steps" enum:"steps,gherkin"`
	GherkinBody     string `json:"gherkin_body,omitempty"`
	Priority        string `json:"priority,omitempty" default:"medium" minLength:"1" maxLength:"64"`
	Status          string `json:"status,omitempty" default:"confirmed" enum:"draft,confirmed,disabled"`
	IsAutomated     bool   `json:"is_automated,omitempty"`
	Script          string `json:"script,omitempty"`
	Arguments       string `json:"arguments,omitempty"`
	Steps           []Step `json:"steps,omitempty"`
}

type CreateTestCaseInput struct {
	Body TestCaseBody
}

type UpsertTestCaseInput struct {
	ID   int64 `path:"id"`
	Body TestCaseBody
}

type TestCaseOutput struct {
	Body TestCase
}

type ListTestCasesOutput struct {
	Body struct {
		Items []TestCase `json:"items"`
		Total int64      `json:"total"`
	}
}

// RegisterRoutes registers TestCase OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo Repository, emailSettings *notify.SettingsStore, emailClient *notify.EmailClient, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testcases",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases",
		Summary:     "List test cases with optional filtering and pagination",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		Q           string `query:"q" doc:"Filter by summary substring"`
		ProductID   int64  `query:"product_id" doc:"Filter by product ID"`
		CategoryID  int64  `query:"category_id" doc:"Filter by category ID"`
		ComponentID int64  `query:"component_id" doc:"Filter by component ID"`
		TagID       int64  `query:"tag_id" doc:"Filter by tag ID"`
		Priority    string `query:"priority" doc:"Filter by priority"`
		Status      string `query:"status" doc:"Filter by status"`
		IsAutomated string `query:"is_automated" enum:",true,false" doc:"Filter by automation flag"`
		AuthorID    int64  `query:"author_id" doc:"Filter by author ID"`
		Limit       int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Offset      int    `query:"offset" default:"0" minimum:"0"`
	}) (*ListTestCasesOutput, error) {
		var isAuto *bool
		if input.IsAutomated == "true" {
			v := true
			isAuto = &v
		} else if input.IsAutomated == "false" {
			v := false
			isAuto = &v
		}
		items, total, err := repo.ListWithFilter(ctx, ListFilter{
			Q:           input.Q,
			ProductID:   input.ProductID,
			CategoryID:  input.CategoryID,
			ComponentID: input.ComponentID,
			TagID:       input.TagID,
			Priority:    input.Priority,
			Status:      input.Status,
			IsAutomated: isAuto,
			AuthorID:    input.AuthorID,
			UserID:      auth.FilterUserID(ctx),
			Limit:       input.Limit,
			Offset:      input.Offset,
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list test cases", err)
		}
		out := &ListTestCasesOutput{}
		out.Body.Items = items
		out.Body.Total = total
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-testcase",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases/{id}",
		Summary:     "Get a single test case by ID",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*TestCaseOutput, error) {
		if err := guard.EnforceTestCase(ctx, input.ID, false); err != nil {
			return nil, err
		}
		tc, err := repo.GetByID(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test case not found")
			}
			return nil, huma.Error500InternalServerError("failed to load test case", err)
		}
		return &TestCaseOutput{Body: tc}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-testcase",
		Method:      http.MethodPost,
		Path:        "/api/v1/testcases",
		Summary:     "Create test case with structured steps",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *CreateTestCaseInput) (*TestCaseOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		var authorID *int64
		if u, ok := auth.UserFromContext(ctx); ok {
			uid := u.ID
			authorID = &uid
		}
		tc, err := repo.Create(ctx, UpsertParams{
			ProductID:       input.Body.ProductID,
			CategoryID:      input.Body.CategoryID,
			AuthorID:        authorID,
			DefaultTesterID: input.Body.DefaultTesterID,
			ReviewerID:      input.Body.ReviewerID,
			Summary:         input.Body.Summary,
			Preconditions:   input.Body.Preconditions,
			Requirement:     input.Body.Requirement,
			ExtraLink:       input.Body.ExtraLink,
			Notes:           input.Body.Notes,
			Format:          input.Body.Format,
			GherkinBody:     input.Body.GherkinBody,
			Priority:        input.Body.Priority,
			Status:          input.Body.Status,
			IsAutomated:     input.Body.IsAutomated,
			Script:          input.Body.Script,
			Arguments:       input.Body.Arguments,
			Steps:           input.Body.Steps,
		})
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test case", err)
		}
		return &TestCaseOutput{Body: tc}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-testcase",
		Method:      http.MethodPut,
		Path:        "/api/v1/testcases/{id}",
		Summary:     "Update test case and replace steps atomically",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *UpsertTestCaseInput) (*TestCaseOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		tc, err := repo.Update(ctx, input.ID, UpsertParams{
			ProductID:       input.Body.ProductID,
			CategoryID:      input.Body.CategoryID,
			DefaultTesterID: input.Body.DefaultTesterID,
			ReviewerID:      input.Body.ReviewerID,
			Summary:         input.Body.Summary,
			Preconditions:   input.Body.Preconditions,
			Requirement:     input.Body.Requirement,
			ExtraLink:       input.Body.ExtraLink,
			Notes:           input.Body.Notes,
			Format:          input.Body.Format,
			GherkinBody:     input.Body.GherkinBody,
			Priority:        input.Body.Priority,
			Status:          input.Body.Status,
			IsAutomated:     input.Body.IsAutomated,
			Script:          input.Body.Script,
			Arguments:       input.Body.Arguments,
			Steps:           input.Body.Steps,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test case not found")
			}
			return nil, huma.Error400BadRequest("failed to update test case", err)
		}
		emailSettings.NotifyIfEnabled(emailClient, "test_case", tc.ID,
			fmt.Sprintf("[Kiwit] TestCase #%d updated", tc.ID),
			fmt.Sprintf("TestCase #%d (%s) was updated.", tc.ID, tc.Summary),
		)
		return &TestCaseOutput{Body: tc}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-testcase-revisions",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases/{id}/revisions",
		Summary:     "List revision history for a test case",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Items []Revision `json:"items"`
		}
	}, error) {
		if err := guard.EnforceTestCase(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := repo.ListRevisions(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list revisions", err)
		}
		out := &struct {
			Body struct {
				Items []Revision `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "bulk-update-testcases",
		Method:      http.MethodPatch,
		Path:        "/api/v1/testcases/bulk",
		Summary:     "Bulk update status or priority for multiple test cases",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			IDs      []int64 `json:"ids" minItems:"1"`
			Status   string  `json:"status,omitempty" enum:",draft,confirmed,disabled"`
			Priority string  `json:"priority,omitempty" maxLength:"64"`
		}
	}) (*struct {
		Body struct {
			Updated int64 `json:"updated"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		for _, id := range input.Body.IDs {
			if err := guard.EnforceTestCase(ctx, id, true); err != nil {
				return nil, err
			}
		}
		n, err := repo.BulkUpdate(ctx, input.Body.IDs, input.Body.Status, input.Body.Priority)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to bulk update test cases", err)
		}
		out := &struct {
			Body struct {
				Updated int64 `json:"updated"`
			}
		}{}
		out.Body.Updated = n
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-testcase",
		Method:      http.MethodDelete,
		Path:        "/api/v1/testcases/{id}",
		Summary:     "Delete a test case",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := repo.Delete(ctx, input.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test case not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete test case", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "clone-testcase",
		Method:      http.MethodPost,
		Path:        "/api/v1/testcases/{id}/clone",
		Summary:     "Clone a test case with steps and automation fields",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Summary string `json:"summary,omitempty"`
		}
	}) (*TestCaseOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.ID, true); err != nil {
			return nil, err
		}
		tc, err := repo.Clone(ctx, input.ID, input.Body.Summary)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test case not found")
			}
			return nil, huma.Error500InternalServerError("failed to clone test case", err)
		}
		return &TestCaseOutput{Body: tc}, nil
	})
}
