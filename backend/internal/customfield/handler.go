package customfield

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

type defsOutput struct {
	Body struct {
		Items []FieldDef `json:"items"`
	}
}

type defOutput struct {
	Body FieldDef
}

type valuesOutput struct {
	Body struct {
		Items []FieldWithValue `json:"items"`
	}
}

type valueOutput struct {
	Body FieldValue
}

var entityTypes = map[string]func(*auth.AccessGuard, context.Context, int64, bool) error{
	"test_case":      func(g *auth.AccessGuard, c context.Context, id int64, w bool) error { return g.EnforceTestCase(c, id, w) },
	"test_plan":      func(g *auth.AccessGuard, c context.Context, id int64, w bool) error { return g.EnforceTestPlan(c, id, w) },
	"test_run":       func(g *auth.AccessGuard, c context.Context, id int64, w bool) error { return g.EnforceTestRun(c, id, w) },
	"test_execution": func(g *auth.AccessGuard, c context.Context, id int64, w bool) error { return g.EnforceExecution(c, id, w) },
	"requirement":    func(g *auth.AccessGuard, c context.Context, id int64, w bool) error { return g.EnforceRequirement(c, id, w) },
	"milestone":      func(g *auth.AccessGuard, c context.Context, id int64, w bool) error { return g.EnforceMilestone(c, id, w) },
}

// RegisterRoutes mounts custom field definition and value APIs.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-custom-field-defs",
		Method:      http.MethodGet,
		Path:        "/api/v1/custom-fields",
		Summary:     "List custom field definitions, optionally per entity type",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_case,test_plan,test_run,test_execution,requirement,milestone,"`
	}) (*defsOutput, error) {
		items, err := store.ListDefs(ctx, input.EntityType)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list field defs", err)
		}
		out := &defsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-custom-field-def",
		Method:      http.MethodPost,
		Path:        "/api/v1/custom-fields",
		Summary:     "Define a custom field (admin only)",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, input *struct {
		Body DefInput
	}) (*defOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if _, ok := entityTypes[input.Body.EntityType]; !ok {
			return nil, huma.Error400BadRequest("unsupported entity_type")
		}
		d, err := store.CreateDef(ctx, input.Body)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &defOutput{Body: d}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-custom-field-def",
		Method:      http.MethodPut,
		Path:        "/api/v1/custom-fields/{id}",
		Summary:     "Update a custom field definition (admin only)",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body DefInput
	}) (*defOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		d, err := store.UpdateDef(ctx, input.ID, input.Body)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("field def not found")
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &defOutput{Body: d}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-custom-field-def",
		Method:      http.MethodDelete,
		Path:        "/api/v1/custom-fields/{id}",
		Summary:     "Delete a custom field definition and its values (admin only)",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := store.DeleteDef(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete field def", err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-entity-custom-fields",
		Method:      http.MethodGet,
		Path:        "/api/v1/{entity_type}/{id}/custom-fields",
		Summary:     "Get custom field values for an entity",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, input *struct {
		EntityType string `path:"entity_type"`
		ID         int64  `path:"id"`
	}) (*valuesOutput, error) {
		enforce, ok := entityTypes[input.EntityType]
		if !ok {
			return nil, huma.Error400BadRequest("unsupported entity_type")
		}
		if err := enforce(guard, ctx, input.ID, false); err != nil {
			return nil, err
		}
		items, err := store.ValuesForEntity(ctx, input.EntityType, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load custom fields", err)
		}
		out := &valuesOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-entity-custom-field",
		Method:      http.MethodPut,
		Path:        "/api/v1/{entity_type}/{id}/custom-fields/{field_id}",
		Summary:     "Set a custom field value for an entity",
		Tags:        []string{"CustomFields"},
	}, func(ctx context.Context, input *struct {
		EntityType string `path:"entity_type"`
		ID         int64  `path:"id"`
		FieldID    int64  `path:"field_id"`
		Body       struct {
			Value string `json:"value"`
		}
	}) (*valueOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		enforce, ok := entityTypes[input.EntityType]
		if !ok {
			return nil, huma.Error400BadRequest("unsupported entity_type")
		}
		if err := enforce(guard, ctx, input.ID, true); err != nil {
			return nil, err
		}
		v, err := store.SetValue(ctx, input.FieldID, input.EntityType, input.ID, input.Body.Value)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("field def not found")
			}
			return nil, huma.Error400BadRequest(err.Error())
		}
		return &valueOutput{Body: v}, nil
	})
}
