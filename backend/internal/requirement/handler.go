package requirement

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
		Items []Requirement `json:"items"`
		Total int64         `json:"total"`
	}
}

type singleOutput struct {
	Body Requirement
}

type coverageOutput struct {
	Body struct {
		Items []CoverageEntry `json:"items"`
	}
}

type caseRequirementsOutput struct {
	Body struct {
		Items []Requirement `json:"items"`
	}
}

// RegisterRoutes mounts the Requirements REST API with coverage linking.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-requirements",
		Method:      http.MethodGet,
		Path:        "/api/v1/requirements",
		Summary:     "List requirements with optional filters and pagination",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		Q         string `query:"q" doc:"Search in key, name, description"`
		ProductID int64  `query:"product_id"`
		Status    string `query:"status" enum:"draft,approved,obsolete"`
		Priority  string `query:"priority"`
		Category  string `query:"category"`
		Limit     int    `query:"limit" default:"100"`
		Offset    int    `query:"offset" default:"0"`
	}) (*listOutput, error) {
		if input.ProductID != 0 {
			if err := guard.EnforceProduct(ctx, input.ProductID, false); err != nil {
				return nil, err
			}
		}
		items, total, err := store.List(ctx, filter{
			q:         input.Q,
			productID: input.ProductID,
			status:    input.Status,
			priority:  input.Priority,
			category:  input.Category,
		}, input.Limit, input.Offset)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list requirements", err)
		}
		out := &listOutput{}
		out.Body.Items = items
		out.Body.Total = total
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-requirement",
		Method:      http.MethodPost,
		Path:        "/api/v1/requirements",
		Summary:     "Create a requirement",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		Body RequirementInput
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		if input.Body.AuthorID == nil {
			if u, ok := auth.UserFromContext(ctx); ok {
				input.Body.AuthorID = &u.ID
			}
		}
		r, err := store.Create(ctx, input.Body)
		if err != nil {
			return nil, huma.Error409Conflict("requirement key already exists for this product", err)
		}
		return &singleOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-requirement",
		Method:      http.MethodGet,
		Path:        "/api/v1/requirements/{id}",
		Summary:     "Get a requirement by ID",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*singleOutput, error) {
		if err := guard.EnforceRequirement(ctx, input.ID, false); err != nil {
			return nil, err
		}
		r, err := store.Get(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("requirement not found")
			}
			return nil, huma.Error500InternalServerError("failed to get requirement", err)
		}
		return &singleOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-requirement",
		Method:      http.MethodPut,
		Path:        "/api/v1/requirements/{id}",
		Summary:     "Update a requirement",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body RequirementInput
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceRequirement(ctx, input.ID, true); err != nil {
			return nil, err
		}
		r, err := store.Update(ctx, input.ID, input.Body)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("requirement not found")
			}
			return nil, huma.Error409Conflict("requirement key already exists for this product", err)
		}
		return &singleOutput{Body: r}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-requirement",
		Method:      http.MethodDelete,
		Path:        "/api/v1/requirements/{id}",
		Summary:     "Delete a requirement (coverage links cascade)",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceRequirement(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete requirement", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "link-requirement-case",
		Method:      http.MethodPost,
		Path:        "/api/v1/requirements/{id}/link-case",
		Summary:     "Link a test case to a requirement (coverage)",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			TestCaseID int64 `json:"test_case_id"`
		}
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceRequirement(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.Body.TestCaseID, true); err != nil {
			return nil, err
		}
		if err := store.LinkCase(ctx, input.ID, input.Body.TestCaseID); err != nil {
			return nil, huma.Error500InternalServerError("failed to link test case", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unlink-requirement-case",
		Method:      http.MethodPost,
		Path:        "/api/v1/requirements/{id}/unlink-case",
		Summary:     "Unlink a test case from a requirement",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			TestCaseID int64 `json:"test_case_id"`
		}
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceRequirement(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := store.UnlinkCase(ctx, input.ID, input.Body.TestCaseID); err != nil {
			return nil, huma.Error500InternalServerError("failed to unlink test case", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "requirement-coverage",
		Method:      http.MethodGet,
		Path:        "/api/v1/requirements/{id}/coverage",
		Summary:     "List test cases covering this requirement with latest execution status",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*coverageOutput, error) {
		if err := guard.EnforceRequirement(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.Coverage(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load coverage", err)
		}
		out := &coverageOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "testcase-requirements",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases/{id}/requirements",
		Summary:     "List requirements covered by a test case",
		Tags:        []string{"Requirements"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*caseRequirementsOutput, error) {
		if err := guard.EnforceTestCase(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.RequirementsForCase(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load requirements", err)
		}
		out := &caseRequirementsOutput{}
		out.Body.Items = items
		return out, nil
	})
}
