package dataset

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
		Items []Dataset `json:"items"`
	}
}

type singleOutput struct {
	Body Dataset
}

// RegisterRoutes mounts dataset CRUD under a test case plus run expansion.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-case-datasets",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases/{id}/datasets",
		Summary:     "List data-driven datasets of a test case",
		Tags:        []string{"Datasets"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*listOutput, error) {
		if err := guard.EnforceTestCase(ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.ListForCase(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list datasets", err)
		}
		out := &listOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-dataset",
		Method:      http.MethodPost,
		Path:        "/api/v1/testcases/{id}/datasets",
		Summary:     "Add a dataset to a test case",
		Tags:        []string{"Datasets"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Name   string `json:"name" minLength:"1" maxLength:"120"`
			Params string `json:"params,omitempty" doc:"JSON object, e.g. {\"user\":\"admin\",\"role\":\"editor\"}"`
		}
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.ID, true); err != nil {
			return nil, err
		}
		d, err := store.Create(ctx, input.ID, input.Body.Name, input.Body.Params)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &singleOutput{Body: d}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-dataset",
		Method:      http.MethodPut,
		Path:        "/api/v1/datasets/{id}",
		Summary:     "Update a dataset",
		Tags:        []string{"Datasets"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Name   string `json:"name" minLength:"1" maxLength:"120"`
			Params string `json:"params,omitempty"`
		}
	}) (*singleOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		d0, err := store.Get(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("dataset not found")
			}
			return nil, huma.Error500InternalServerError("failed to get dataset", err)
		}
		if err := guard.EnforceTestCase(ctx, d0.TestCaseID, true); err != nil {
			return nil, err
		}
		d, err := store.Update(ctx, input.ID, input.Body.Name, input.Body.Params)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &singleOutput{Body: d}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-dataset",
		Method:      http.MethodDelete,
		Path:        "/api/v1/datasets/{id}",
		Summary:     "Delete a dataset (executions keep NULL dataset)",
		Tags:        []string{"Datasets"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		d0, err := store.Get(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("dataset not found")
			}
			return nil, huma.Error500InternalServerError("failed to get dataset", err)
		}
		if err := guard.EnforceTestCase(ctx, d0.TestCaseID, true); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete dataset", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "expand-datasets",
		Method:      http.MethodPost,
		Path:        "/api/v1/testruns/{id}/cases/{case_id}/expand-datasets",
		Summary:     "Create one execution per dataset of a test case in this run",
		Tags:        []string{"Datasets"},
	}, func(ctx context.Context, input *struct {
		ID     int64 `path:"id"`
		CaseID int64 `path:"case_id"`
	}) (*struct {
		Body struct {
			Created int `json:"created"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestRun(ctx, input.ID, true); err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.CaseID, true); err != nil {
			return nil, err
		}
		n, err := store.ExpandDatasets(ctx, input.ID, input.CaseID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to expand datasets", err)
		}
		out := &struct {
			Body struct {
				Created int `json:"created"`
			}
		}{}
		out.Body.Created = n
		return out, nil
	})
}
