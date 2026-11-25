package testrun

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
func RegisterRoutes(api huma.API, repo Repository, notifier *notify.WebhookClient) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testruns",
		Method:      http.MethodGet,
		Path:        "/api/v1/testruns",
		Summary:     "List test runs with pagination",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		Limit  int `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Offset int `query:"offset" default:"0" minimum:"0"`
	}) (*struct {
		Body struct {
			Items []TestRun `json:"items"`
			Total int64     `json:"total"`
		}
	}, error) {
		items, total, err := repo.ListPaginated(ctx, input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list test runs", err)
		}
		out := &struct {
			Body struct {
				Items []TestRun `json:"items"`
				Total int64     `json:"total"`
			}
		}{}
		out.Body.Items = items
		out.Body.Total = total
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "stop-testrun",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/{id}/stop",
		Summary:     "Mark a test run as stopped",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Stopped bool `json:"stopped"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := repo.Stop(ctx, input.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test run not found")
			}
			return nil, huma.Error500InternalServerError("failed to stop test run", err)
		}
		if notifier != nil && notifier.Enabled() {
			go func(id int64) {
				_ = notifier.Send(context.Background(), fmt.Sprintf("TestRun #%d stopped", id))
			}(input.ID)
		}
		out := &struct {
			Body struct {
				Stopped bool `json:"stopped"`
			}
		}{}
		out.Body.Stopped = true
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-testrun",
		Method:      http.MethodDelete,
		Path:        "/api/v1/testruns/{id}",
		Summary:     "Delete a test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := repo.Delete(ctx, input.ID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test run not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete test run", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "clone-testrun",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/{id}/clone",
		Summary:     "Clone a test run (creates a new run from the same plan and build)",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Summary string `json:"summary,omitempty"`
		}
	}) (*TestRunOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		r, err := repo.GetByID(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test run not found")
			}
			return nil, huma.Error500InternalServerError("failed to load test run", err)
		}
		sum := input.Body.Summary
		if sum == "" {
			sum = r.Summary + " (copy)"
		}
		cloned, err := repo.Create(ctx, r.TestPlanID, r.BuildID, sum, r.Notes)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to clone test run", err)
		}
		return &TestRunOutput{Body: cloned}, nil
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
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
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
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		results, err := ParseJUnitXML([]byte(input.Body.XML))
		if err != nil {
			return nil, huma.Error400BadRequest("invalid JUnit XML", err)
		}
		tr, err := repo.Create(ctx, input.Body.TestPlanID, input.Body.BuildID, input.Body.Summary, "Imported from JUnit XML")
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test run", err)
		}
		if execs, err := repo.ListExecutions(ctx, tr.ID); err == nil {
			byName := make(map[string]ParsedCaseResult, len(results))
			for _, r := range results {
				byName[r.Name] = r
			}
			for _, ex := range execs {
				if r, ok := byName[ex.Summary]; ok {
					_, _ = repo.UpdateExecution(ctx, ex.ID, r.Status, r.Comment)
				}
			}
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
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
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
