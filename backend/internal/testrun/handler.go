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
	ID              int64      `json:"id"`
	TestPlanID      int64      `json:"test_plan_id"`
	BuildID         int64      `json:"build_id"`
	EnvironmentID   *int64     `json:"environment_id,omitempty"`
	ManagerID       *int64     `json:"manager_id,omitempty"`
	DefaultTesterID *int64     `json:"default_tester_id,omitempty"`
	Summary         string     `json:"summary"`
	Notes           string     `json:"notes,omitempty"`
	PlannedStart    *time.Time `json:"planned_start,omitempty"`
	PlannedStop     *time.Time `json:"planned_stop,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	StoppedAt       *time.Time `json:"stopped_at,omitempty"`
}

type CreateTestRunInput struct {
	Body struct {
		TestPlanID      int64      `json:"test_plan_id"`
		BuildID         int64      `json:"build_id"`
		EnvironmentID   *int64     `json:"environment_id,omitempty"`
		ManagerID       *int64     `json:"manager_id,omitempty"`
		DefaultTesterID *int64     `json:"default_tester_id,omitempty"`
		Summary         string     `json:"summary" minLength:"1" maxLength:"255"`
		Notes           string     `json:"notes,omitempty"`
		PlannedStart    *time.Time `json:"planned_start,omitempty"`
		PlannedStop     *time.Time `json:"planned_stop,omitempty"`
	}
}

type UpdateTestRunInput struct {
	ID   int64 `path:"id"`
	Body struct {
		BuildID         *int64     `json:"build_id,omitempty"`
		EnvironmentID   *int64     `json:"environment_id,omitempty"`
		ManagerID       *int64     `json:"manager_id,omitempty"`
		DefaultTesterID *int64     `json:"default_tester_id,omitempty"`
		Summary         string     `json:"summary" minLength:"1" maxLength:"255"`
		Notes           string     `json:"notes,omitempty"`
		PlannedStart    *time.Time `json:"planned_start,omitempty"`
		PlannedStop     *time.Time `json:"planned_stop,omitempty"`
	}
}

type TestRunOutput struct {
	Body TestRun
}

type UpdateExecutionInput struct {
	ID   int64 `path:"id"`
	Body struct {
		Status        string `json:"status,omitempty" maxLength:"64"`
		Comment       string `json:"comment,omitempty"`
		AssigneeID    *int64 `json:"assignee_id,omitempty"`
		TestedByID    *int64 `json:"tested_by_id,omitempty"`
		EnvironmentID *int64 `json:"environment_id,omitempty"`
	}
}

type ExecutionOutput struct {
	Body ExecutionResult
}

type ExecutionLink struct {
	ID              int64     `json:"id"`
	TestExecutionID int64     `json:"test_execution_id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	CreatedAt       time.Time `json:"created_at"`
}

// RegisterRoutes registers TestRun and TestExecution OpenAPI endpoints.
func RegisterRoutes(api huma.API, repo Repository, notifier *notify.WebhookClient, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-testruns",
		Method:      http.MethodGet,
		Path:        "/api/v1/testruns",
		Summary:     "List test runs with optional filtering and pagination",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ProductID  int64  `query:"product_id" doc:"Filter by product ID"`
		TestPlanID int64  `query:"test_plan_id" doc:"Filter by test plan ID"`
		BuildID    int64  `query:"build_id" doc:"Filter by build ID"`
		Status     string `query:"status" enum:",running,stopped" doc:"Filter by run status"`
		Q          string `query:"q" doc:"Filter by summary substring"`
		Limit      int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Offset     int    `query:"offset" default:"0" minimum:"0"`
	}) (*struct {
		Body struct {
			Items []TestRun `json:"items"`
			Total int64     `json:"total"`
		}
	}, error) {
		items, total, err := repo.ListFiltered(ctx, ListFilter{
			ProductID:  input.ProductID,
			TestPlanID: input.TestPlanID,
			BuildID:    input.BuildID,
			Status:     input.Status,
			Q:          input.Q,
			UserID:     auth.FilterUserID(ctx),
			Limit:      input.Limit,
			Offset:     input.Offset,
		})
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
		OperationID: "update-testrun",
		Method:      http.MethodPut,
		Path:        "/api/v1/testruns/{id}",
		Summary:     "Update test run metadata",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *UpdateTestRunInput) (*TestRunOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
			return nil, err
		}
		tr, err := repo.Update(ctx, input.ID, UpdateParams{
			BuildID:         input.Body.BuildID,
			EnvironmentID:   input.Body.EnvironmentID,
			ManagerID:       input.Body.ManagerID,
			DefaultTesterID: input.Body.DefaultTesterID,
			Summary:         input.Body.Summary,
			Notes:           input.Body.Notes,
			PlannedStart:    input.Body.PlannedStart,
			PlannedStop:     input.Body.PlannedStop,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("test run not found")
			}
			return nil, huma.Error400BadRequest("failed to update test run", err)
		}
		return &TestRunOutput{Body: tr}, nil
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
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
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
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
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
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
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
		cloned, err := repo.CreateWithMeta(ctx, CreateParams{
			TestPlanID:      r.TestPlanID,
			BuildID:         r.BuildID,
			EnvironmentID:   r.EnvironmentID,
			ManagerID:       r.ManagerID,
			DefaultTesterID: r.DefaultTesterID,
			Summary:         sum,
			Notes:           r.Notes,
			PlannedStart:    r.PlannedStart,
			PlannedStop:     r.PlannedStop,
		})
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
		if err := guard.EnforceTestRun(ctx, input.ID, false); err != nil {
			return nil, err
		}
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
		OperationID: "add-testrun-cases",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/{id}/cases",
		Summary:     "Add individual test cases to an existing test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			CaseIDs       []int64 `json:"case_ids" minItems:"1"`
			EnvironmentID *int64  `json:"environment_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			Added int `json:"added"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
			return nil, err
		}
		for _, cid := range input.Body.CaseIDs {
			if err := guard.EnforceTestCase(ctx, cid, false); err != nil {
				return nil, err
			}
		}
		n, err := repo.AddCases(ctx, input.ID, input.Body.CaseIDs, input.Body.EnvironmentID)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to add cases to test run", err)
		}
		out := &struct {
			Body struct {
				Added int `json:"added"`
			}
		}{}
		out.Body.Added = n
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-testrun-execution",
		Method:      http.MethodDelete,
		Path:        "/api/v1/testruns/{id}/executions/{exec_id}",
		Summary:     "Remove a single execution from a test run",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID     int64 `path:"id"`
		ExecID int64 `path:"exec_id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := repo.DeleteExecution(ctx, input.ID, input.ExecID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("execution not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete execution", err)
		}
		return nil, nil
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
		if err := guard.EnforceTestPlan(ctx, input.Body.TestPlanID, true); err != nil {
			return nil, err
		}
		managerID := input.Body.ManagerID
		if managerID == nil {
			if u, ok := auth.UserFromContext(ctx); ok {
				uid := u.ID
				managerID = &uid
			}
		}
		tr, err := repo.CreateWithMeta(ctx, CreateParams{
			TestPlanID:      input.Body.TestPlanID,
			BuildID:         input.Body.BuildID,
			EnvironmentID:   input.Body.EnvironmentID,
			ManagerID:       managerID,
			DefaultTesterID: input.Body.DefaultTesterID,
			Summary:         input.Body.Summary,
			Notes:           input.Body.Notes,
			PlannedStart:    input.Body.PlannedStart,
			PlannedStop:     input.Body.PlannedStop,
		})
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create test run", err)
		}
		return &TestRunOutput{Body: tr}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "import-junit-xml",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/import/junit",
		Summary:     "Import JUnit XML results from CI/CD (with optional auto-creation of missing test cases)",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			TestPlanID      int64  `json:"test_plan_id"`
			BuildID         int64  `json:"build_id"`
			Summary         string `json:"summary" minLength:"1"`
			XML             string `json:"xml" minLength:"1"`
			AutoCreateCases *bool  `json:"auto_create_cases,omitempty" doc:"Automatically create missing TestCases and attach them to the TestPlan (default: true)"`
		}
	}) (*struct {
		Body struct {
			TestRun      TestRun            `json:"test_run"`
			Results      []ParsedCaseResult `json:"results"`
			CreatedCases int                `json:"created_cases"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestPlan(ctx, input.Body.TestPlanID, true); err != nil {
			return nil, err
		}
		results, err := ParseJUnitXML([]byte(input.Body.XML))
		if err != nil {
			return nil, huma.Error400BadRequest("invalid JUnit XML", err)
		}
		autoCreate := true
		if input.Body.AutoCreateCases != nil {
			autoCreate = *input.Body.AutoCreateCases
		}
		tr, createdCases, err := repo.ImportJUnitResults(ctx, input.Body.TestPlanID, input.Body.BuildID, input.Body.Summary, results, autoCreate)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to import JUnit results", err)
		}
		out := &struct {
			Body struct {
				TestRun      TestRun            `json:"test_run"`
				Results      []ParsedCaseResult `json:"results"`
				CreatedCases int                `json:"created_cases"`
			}
		}{}
		out.Body.TestRun = tr
		out.Body.Results = results
		out.Body.CreatedCases = createdCases
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "bulk-update-executions",
		Method:      http.MethodPatch,
		Path:        "/api/v1/executions/bulk",
		Summary:     "Bulk update status or assignee for multiple test executions",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			IDs        []int64 `json:"ids" minItems:"1"`
			Status     string  `json:"status,omitempty" maxLength:"64"`
			AssigneeID *int64  `json:"assignee_id,omitempty"`
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
			if err := guard.EnforceExecution(ctx, id, true); err != nil {
				return nil, err
			}
		}
		var testedBy *int64
		if input.Body.Status != "" {
			if u, ok := auth.UserFromContext(ctx); ok {
				uid := u.ID
				testedBy = &uid
			}
		}
		n, err := repo.BulkUpdateExecutions(ctx, input.Body.IDs, input.Body.Status, input.Body.AssigneeID, testedBy)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to bulk update executions", err)
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
		OperationID: "update-test-execution",
		Method:      http.MethodPatch,
		Path:        "/api/v1/executions/{id}",
		Summary:     "Update test execution status, assignee, environment, or comment",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *UpdateExecutionInput) (*ExecutionOutput, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ID, true); err != nil {
			return nil, err
		}
		testedBy := input.Body.TestedByID
		if testedBy == nil && input.Body.Status != "" {
			if u, ok := auth.UserFromContext(ctx); ok {
				uid := u.ID
				testedBy = &uid
			}
		}
		res, err := repo.UpdateExecutionFull(ctx, input.ID, ExecutionUpdateParams{
			Status:        input.Body.Status,
			Comment:       input.Body.Comment,
			AssigneeID:    input.Body.AssigneeID,
			TestedByID:    testedBy,
			EnvironmentID: input.Body.EnvironmentID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("execution not found")
			}
			return nil, huma.Error400BadRequest("failed to update execution", err)
		}
		return &ExecutionOutput{Body: res}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sync-execution-case",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{id}/sync-case",
		Summary:     "Refresh execution steps snapshot to the latest test case revision",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{ Body ExecutionItem }, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ID, true); err != nil {
			return nil, err
		}
		item, err := repo.SyncExecutionCase(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("execution not found")
			}
			return nil, huma.Error500InternalServerError("failed to sync execution snapshot", err)
		}
		return &struct{ Body ExecutionItem }{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-execution-links",
		Method:      http.MethodGet,
		Path:        "/api/v1/executions/{id}/links",
		Summary:     "List CI/log link references attached to a test execution",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct {
		Body struct {
			Items []ExecutionLink `json:"items"`
		}
	}, error) {
		if err := guard.EnforceExecution(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := repo.ListLinks(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list links", err)
		}
		out := &struct {
			Body struct {
				Items []ExecutionLink `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "add-execution-link",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{id}/links",
		Summary:     "Attach a CI/log link reference to a test execution",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Name string `json:"name" minLength:"1" maxLength:"200"`
			URL  string `json:"url" minLength:"1" maxLength:"1000"`
		}
	}) (*struct{ Body ExecutionLink }, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ID, true); err != nil {
			return nil, err
		}
		link, err := repo.AddLink(ctx, input.ID, input.Body.Name, input.Body.URL)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to add link", err)
		}
		return &struct{ Body ExecutionLink }{Body: link}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-execution-link",
		Method:      http.MethodDelete,
		Path:        "/api/v1/executions/{id}/links/{link_id}",
		Summary:     "Delete a link reference from a test execution",
		Tags:        []string{"TestRuns"},
	}, func(ctx context.Context, input *struct {
		ID     int64 `path:"id"`
		LinkID int64 `path:"link_id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := repo.DeleteLink(ctx, input.ID, input.LinkID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("link not found")
			}
			return nil, huma.Error500InternalServerError("failed to delete link", err)
		}
		return nil, nil
	})
}
