package visual

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/storage"
)

// matchThresholdPercent — diff at or below this is a visual match.
const matchThresholdPercent = 2.0

type baselinesOutput struct {
	Body struct {
		Items []Baseline `json:"items"`
	}
}

type baselineOutput struct {
	Body Baseline
}

type comparisonsOutput struct {
	Body struct {
		Items []Comparison `json:"items"`
	}
}

type compareOutput struct {
	Body Comparison
}

// RegisterRoutes mounts visual regression baselines and comparisons.
func RegisterRoutes(api huma.API, store *Store, attRepo attachment.Repository, store_ storage.Storage, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-visual-baselines",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases/{id}/visual-baselines",
		Summary:     "List visual regression baselines of a test case",
		Tags:        []string{"Visual"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*baselinesOutput, error) {
		if err := guard.EnforceTestCase(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.ListBaselines(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list baselines", err)
		}
		out := &baselinesOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "add-visual-baseline",
		Method:      http.MethodPost,
		Path:        "/api/v1/testcases/{id}/visual-baselines",
		Summary:     "Promote an existing image attachment of this test case to a visual baseline",
		Tags:        []string{"Visual"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			AttachmentID int64  `json:"attachment_id"`
			Label        string `json:"label,omitempty"`
		}
	}) (*baselineOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.ID, true); err != nil {
			return nil, err
		}
		a, err := attRepo.GetByID(ctx, input.Body.AttachmentID)
		if err != nil {
			return nil, huma.Error404NotFound("attachment not found")
		}
		if a.EntityType != "test_case" || a.EntityID != input.ID {
			return nil, huma.Error400BadRequest("attachment does not belong to this test case")
		}
		if !strings.HasPrefix(a.ContentType, "image/") {
			return nil, huma.Error400BadRequest("attachment is not an image")
		}
		b, err := store.AddBaseline(ctx, input.ID, input.Body.AttachmentID, input.Body.Label)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to add baseline", err)
		}
		return &baselineOutput{Body: b}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-visual-baseline",
		Method:      http.MethodDelete,
		Path:        "/api/v1/visual-baselines/{id}",
		Summary:     "Delete a visual baseline",
		Tags:        []string{"Visual"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		caseID, err := store.BaselineCaseID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("baseline not found")
		}
		if err := guard.EnforceTestCase(ctx, caseID, true); err != nil {
			return nil, err
		}
		if err := store.DeleteBaseline(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete baseline", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "visual-compare",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{id}/visual-compare",
		Summary:     "Compare an execution screenshot against the test case's latest (or given) baseline",
		Tags:        []string{"Visual"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			AttachmentID int64 `json:"attachment_id" doc:"Image attachment of this execution (the actual screenshot)"`
			BaselineID   int64 `json:"baseline_id,omitempty" doc:"0 = latest baseline of the test case"`
		}
	}) (*compareOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ID, true); err != nil {
			return nil, err
		}
		caseID, err := store.ExecutionCaseID(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("execution not found")
		}
		baseline, err := store.GetBaseline(ctx, input.Body.BaselineID)
		if input.Body.BaselineID == 0 || errors.Is(err, sql.ErrNoRows) {
			baseline, err = store.LatestBaseline(ctx, caseID)
		}
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c, cerr := store.SaveComparison(ctx, input.ID, 0, input.Body.AttachmentID, 100, "no-baseline")
				if cerr != nil {
					return nil, huma.Error500InternalServerError("failed to record comparison", cerr)
				}
				return &compareOutput{Body: c}, nil
			}
			return nil, huma.Error500InternalServerError("failed to load baseline", err)
		}
		actual, err := attRepo.GetByID(ctx, input.Body.AttachmentID)
		if err != nil {
			return nil, huma.Error404NotFound("attachment not found")
		}
		if !strings.HasPrefix(actual.ContentType, "image/") {
			return nil, huma.Error400BadRequest("attachment is not an image")
		}
		baseAtt, err := attRepo.GetByID(ctx, baseline.AttachmentID)
		if err != nil {
			return nil, huma.Error500InternalServerError("baseline attachment missing", err)
		}
		readAtt := func(a attachment.Attachment) ([]byte, error) {
			rc, err := store_.Open(ctx, a.StoragePath)
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return ReadAll(rc)
		}
		baseBytes, err := readAtt(baseAtt)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read baseline image", err)
		}
		actualBytes, err := readAtt(actual)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read actual image", err)
		}
		diff, err := DiffPercent(baseBytes, actualBytes)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		verdict := "match"
		if diff > matchThresholdPercent {
			verdict = "mismatch"
		}
		c, err := store.SaveComparison(ctx, input.ID, baseline.ID, input.Body.AttachmentID, diff, verdict)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to record comparison", err)
		}
		return &compareOutput{Body: c}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-visual-comparisons",
		Method:      http.MethodGet,
		Path:        "/api/v1/executions/{id}/visual-comparisons",
		Summary:     "List visual comparison results of an execution",
		Tags:        []string{"Visual"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*comparisonsOutput, error) {
		if err := guard.EnforceExecution(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.ListComparisons(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list comparisons", err)
		}
		out := &comparisonsOutput{}
		out.Body.Items = items
		return out, nil
	})
}
