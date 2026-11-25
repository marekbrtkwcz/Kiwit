package testcase

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type Step struct {
	Position       int    `json:"position" minimum:"1"`
	Action         string `json:"action" minLength:"1"`
	ExpectedResult string `json:"expected_result,omitempty"`
}

type TestCase struct {
	ID            int64     `json:"id"`
	ProductID     int64     `json:"product_id"`
	Summary       string    `json:"summary"`
	Preconditions string    `json:"preconditions,omitempty"`
	Format        string    `json:"format" enum:"steps,gherkin"`
	GherkinBody   string    `json:"gherkin_body,omitempty"`
	Priority      string    `json:"priority" enum:"low,medium,high,critical"`
	Status        string    `json:"status" enum:"draft,confirmed,disabled"`
	Steps         []Step    `json:"steps"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UpsertTestCaseInput struct {
	ID   int64 `path:"id" required:"false"`
	Body struct {
		ProductID     int64  `json:"product_id"`
		Summary       string `json:"summary" minLength:"1" maxLength:"255"`
		Preconditions string `json:"preconditions,omitempty"`
		Format        string `json:"format,omitempty" default:"steps" enum:"steps,gherkin"`
		GherkinBody   string `json:"gherkin_body,omitempty"`
		Priority      string `json:"priority,omitempty" default:"medium" enum:"low,medium,high,critical"`
		Status        string `json:"status,omitempty" default:"confirmed" enum:"draft,confirmed,disabled"`
		Steps         []Step `json:"steps,omitempty"`
	}
}

type TestCaseOutput struct {
	Body TestCase
}

type ListTestCasesOutput struct {
	Body struct {
		Items []TestCase `json:"items"`
	}
}

// RegisterRoutes registers TestCase OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo Repository) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testcases",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases",
		Summary:     "List test cases",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct{}) (*ListTestCasesOutput, error) {
		items, err := repo.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list test cases", err)
		}
		out := &ListTestCasesOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-testcase",
		Method:      http.MethodPost,
		Path:        "/api/v1/testcases",
		Summary:     "Create test case with structured steps",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *UpsertTestCaseInput) (*TestCaseOutput, error) {
		tc, err := repo.Create(ctx, UpsertParams{
			ProductID:     input.Body.ProductID,
			Summary:       input.Body.Summary,
			Preconditions: input.Body.Preconditions,
			Format:        input.Body.Format,
			GherkinBody:   input.Body.GherkinBody,
			Priority:      input.Body.Priority,
			Status:        input.Body.Status,
			Steps:         input.Body.Steps,
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
		tc, err := repo.Update(ctx, input.ID, UpsertParams{
			ProductID:     input.Body.ProductID,
			Summary:       input.Body.Summary,
			Preconditions: input.Body.Preconditions,
			Format:        input.Body.Format,
			GherkinBody:   input.Body.GherkinBody,
			Priority:      input.Body.Priority,
			Status:        input.Body.Status,
			Steps:         input.Body.Steps,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test case not found")
			}
			return nil, huma.Error400BadRequest("failed to update test case", err)
		}
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
}
