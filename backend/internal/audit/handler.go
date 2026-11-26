package audit

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

// RegisterRoutes mounts audit log read endpoints.
func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-entity-audit",
		Method:      http.MethodGet,
		Path:        "/api/v1/audit",
		Summary:     "List audit log entries for an entity",
		Tags:        []string{"Audit"},
	}, func(ctx context.Context, input *struct {
		EntityType string `query:"entity_type" enum:"test_case,test_plan,test_run,test_execution"`
		EntityID   int64  `query:"entity_id" minimum:"1"`
		Limit      int    `query:"limit" minimum:"1" maximum:"500" default:"100"`
	}) (*struct {
		Body struct {
			Items []Entry `json:"items"`
		}
	}, error) {
		if err := guard.EnforceEntity(ctx, input.EntityType, input.EntityID, false); err != nil {
			return nil, huma.Error403Forbidden("no access")
		}
		items, err := store.List(ctx, input.EntityType, input.EntityID, input.Limit)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Items []Entry `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-all-audit",
		Method:      http.MethodGet,
		Path:        "/api/v1/audit/all",
		Summary:     "List recent audit log entries across all entities (admin)",
		Tags:        []string{"Audit"},
	}, func(ctx context.Context, input *struct {
		Limit int `query:"limit" minimum:"1" maximum:"500" default:"100"`
	}) (*struct {
		Body struct {
			Items []Entry `json:"items"`
		}
	}, error) {
		u, ok := auth.UserFromContext(ctx)
		if !ok || u.Role != "admin" {
			return nil, huma.Error403Forbidden("admin only")
		}
		items, err := store.ListAll(ctx, input.Limit)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		out := &struct {
			Body struct {
				Items []Entry `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})
}

// RecordChange is a helper for handlers: extracts the actor and records an audit entry.
func RecordChange(ctx context.Context, store *Store, entityType string, entityID int64, action string, oldVals, newVals map[string]any) {
	if store == nil {
		return
	}
	u, ok := auth.UserFromContext(ctx)
	actorID := (*int64)(nil)
	actorName := "system"
	if ok {
		id := u.ID
		actorID = &id
		actorName = u.Username
	}
	changedOld, changedNew, _ := DiffFields(oldVals, newVals)
	if len(changedOld) == 0 && action != "create" && action != "delete" {
		return
	}
	_ = store.Record(ctx, entityType, entityID, action, actorID, actorName, changedOld, changedNew)
}
