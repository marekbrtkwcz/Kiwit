package milestone

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
		Items []Milestone `json:"items"`
	}
}

type singleOutput struct {
	Body Milestone
}

type progressOutput struct {
	Body Progress
}

type runsOutput struct {
	Body struct {
		Items []int64 `json:"items"`
	}
}

// RegisterRoutes mounts the Milestones (release) REST API with go/no-go signoff.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-milestones",
		Method:      http.MethodGet,
		Path:        "/api/v1/milestones",
		Summary:     "List milestones (releases), optionally filtered by version",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		VersionID int64 `query:"version_id" doc:"Filter by version (0 = all)"`
	}) (*listOutput, error) {
		if input.VersionID != 0 {
			if err := guard.EnforceVersion(ctx, input.VersionID, false); err != nil {
				return nil, err
			}
		}
		items, err := store.List(ctx, input.VersionID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list milestones", err)
		}
		out := &listOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-milestone",
		Method:      http.MethodPost,
		Path:        "/api/v1/milestones",
		Summary:     "Create a milestone (release) for a version",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		Body MilestoneInput
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceVersion(ctx, input.Body.VersionID, true); err != nil {
			return nil, err
		}
		m, err := store.Create(ctx, input.Body)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to create milestone", err)
		}
		return &singleOutput{Body: m}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-milestone",
		Method:      http.MethodGet,
		Path:        "/api/v1/milestones/{id}",
		Summary:     "Get a milestone by ID",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*singleOutput, error) {
		if err := guard.EnforceMilestone(ctx, input.ID, false); err != nil {
			return nil, err
		}
		m, err := store.Get(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("milestone not found")
			}
			return nil, huma.Error500InternalServerError("failed to get milestone", err)
		}
		return &singleOutput{Body: m}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-milestone",
		Method:      http.MethodPut,
		Path:        "/api/v1/milestones/{id}",
		Summary:     "Update a milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body MilestoneInput
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceMilestone(ctx, input.ID, true); err != nil {
			return nil, err
		}
		m, err := store.Update(ctx, input.ID, input.Body)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("milestone not found")
			}
			return nil, huma.Error500InternalServerError("failed to update milestone", err)
		}
		return &singleOutput{Body: m}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-milestone",
		Method:      http.MethodDelete,
		Path:        "/api/v1/milestones/{id}",
		Summary:     "Delete a milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceMilestone(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete milestone", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "milestone-signoff",
		Method:      http.MethodPost,
		Path:        "/api/v1/milestones/{id}/signoff",
		Summary:     "Record a go/no-go decision on a milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Decision string `json:"decision" enum:"go,no-go"`
			Note     string `json:"note,omitempty"`
		}
	}) (*singleOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := guard.EnforceMilestone(ctx, input.ID, true); err != nil {
			return nil, err
		}
		u, ok := auth.UserFromContext(ctx)
		if !ok {
			return nil, huma.Error401Unauthorized("unauthorized")
		}
		m, err := store.Signoff(ctx, input.ID, u.ID, input.Body.Decision, input.Body.Note)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("milestone not found")
			}
			return nil, huma.Error500InternalServerError("failed to record signoff", err)
		}
		return &singleOutput{Body: m}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "milestone-progress",
		Method:      http.MethodGet,
		Path:        "/api/v1/milestones/{id}/progress",
		Summary:     "Aggregated execution results of all runs linked to this milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*progressOutput, error) {
		if err := guard.EnforceMilestone(ctx, input.ID, false); err != nil {
			return nil, err
		}
		p, err := store.Progress(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to compute progress", err)
		}
		return &progressOutput{Body: p}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "milestone-link-run",
		Method:      http.MethodPost,
		Path:        "/api/v1/milestones/{id}/link-run",
		Summary:     "Link a test run to this milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			TestRunID int64 `json:"test_run_id"`
		}
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceMilestone(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestRun(ctx, input.Body.TestRunID, true); err != nil {
			return nil, err
		}
		if err := store.LinkRun(ctx, input.ID, input.Body.TestRunID); err != nil {
			return nil, huma.Error500InternalServerError("failed to link run", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "milestone-unlink-run",
		Method:      http.MethodPost,
		Path:        "/api/v1/milestones/{id}/unlink-run",
		Summary:     "Unlink a test run from this milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			TestRunID int64 `json:"test_run_id"`
		}
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceMilestone(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := store.UnlinkRun(ctx, input.ID, input.Body.TestRunID); err != nil {
			return nil, huma.Error500InternalServerError("failed to unlink run", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "milestone-runs",
		Method:      http.MethodGet,
		Path:        "/api/v1/milestones/{id}/runs",
		Summary:     "List test run IDs linked to this milestone",
		Tags:        []string{"Milestones"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*runsOutput, error) {
		if err := guard.EnforceMilestone(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.Runs(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list runs", err)
		}
		out := &runsOutput{}
		out.Body.Items = items
		return out, nil
	})
}
