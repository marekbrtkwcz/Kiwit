package testrun

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type TestRun struct {
	ID         int64     `json:"id"`
	TestPlanID int64     `json:"test_plan_id"`
	BuildID    int64     `json:"build_id"`
	Summary    string    `json:"summary"`
	Notes      string    `json:"notes,omitempty"`
	StartedAt  time.Time `json:"started_at"`
}

type CreateTestRunInput struct {
	Body struct {
		TestPlanID int64  `json:"test_plan_id"`
		BuildID    int64  `json:"build_id"`
		Summary    string `json:"summary" minLength:"1" maxLength:"255"`
		Notes      string `json:"notes,omitempty"`
	}
}

type TestRunOutput struct {
	Body TestRun
}

type UpdateExecutionInput struct {
	ID   int64 `path:"id"`
	Body struct {
		Status  string `json:"status" enum:"idle,running,passed,failed,blocked,skipped"`
		Comment string `json:"comment,omitempty"`
	}
}

type ExecutionOutput struct {
	Body struct {
		ID      int64  `json:"id"`
		Status  string `json:"status"`
		Comment string `json:"comment,omitempty"`
	}
}

// RegisterRoutes registers TestRun and TestExecution OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo Repository) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testruns",
		Method:      http.MethodGet,
		Path:        "/api/v1/testruns",
		Summary:     "List test runs",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []TestRun `json:"items"`
		}
	}, error) {
		items, err := repo.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list test runs", err)
		}
		out := &struct {
			Body struct {
				Items []TestRun `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-testrun-executions",
		Method:      http.MethodGet,
		Path:        "/api/v1/testruns/{id}/executions",
		Summary:     "List executions for a test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Items []ExecutionItem `json:"items"`
		}
	}, error) {
		items, err := repo.ListExecutions(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list executions", err)
		}
		out := &struct {
			Body struct {
				Items []ExecutionItem `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "create-testrun",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns",
		Summary:     "Create test run and snapshot test case steps",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *CreateTestRunInput) (*TestRunOutput, error) {
		tr, err := repo.Create(ctx, input.Body.TestPlanID, input.Body.BuildID, input.Body.Summary, input.Body.Notes)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test run", err)
		}
		return &TestRunOutput{Body: tr}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "import-junit-xml",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/import/junit",
		Summary:     "Import JUnit XML results from CI/CD",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			TestPlanID int64  `json:"test_plan_id"`
			BuildID    int64  `json:"build_id"`
			Summary    string `json:"summary" minLength:"1"`
			XML        string `json:"xml" minLength:"1"`
		}
	}) (*struct {
		Body struct {
			TestRun TestRun            `json:"test_run"`
			Results []ParsedCaseResult `json:"results"`
		}
	}, error) {
		results, err := ParseJUnitXML([]byte(input.Body.XML))
		if err != nil {
			return nil, huma.Error400BadRequest("invalid JUnit XML", err)
		}
		tr, err := repo.Create(ctx, input.Body.TestPlanID, input.Body.BuildID, input.Body.Summary, "Imported from JUnit XML")
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test run", err)
		}
		out := &struct {
			Body struct {
				TestRun TestRun            `json:"test_run"`
				Results []ParsedCaseResult `json:"results"`
			}
		}{}
		out.Body.TestRun = tr
		out.Body.Results = results
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-test-execution",
		Method:      http.MethodPatch,
		Path:        "/api/v1/executions/{id}",
		Summary:     "Update test execution status",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *UpdateExecutionInput) (*ExecutionOutput, error) {
		res, err := repo.UpdateExecution(ctx, input.ID, input.Body.Status, input.Body.Comment)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("execution not found")
			}
			return nil, huma.Error400BadRequest("failed to update execution", err)
		}
		out := &ExecutionOutput{}
		out.Body.ID = res.ID
		out.Body.Status = res.Status
		out.Body.Comment = res.Comment
		return out, nil
	})
}
