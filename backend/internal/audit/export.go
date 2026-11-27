package audit

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts the unified export endpoint for the audit log:
//
//	GET /api/v1/audit/export - recent entries across all entities (admin only, csv|json|md)
func RegisterExportRoutes(r chi.Router, store *Store) {
	r.Get("/api/v1/audit/export", func(w http.ResponseWriter, req *http.Request) {
		u, ok := auth.UserFromContext(req.Context())
		if !ok || u.Role != "admin" {
			http.Error(w, `{"detail":"admin only"}`, http.StatusForbidden)
			return
		}
		limit := int(export.QueryInt64(req, "limit"))
		if limit <= 0 || limit > 2000 {
			limit = 1000
		}
		items, err := store.ListAll(req.Context(), limit)
		if err != nil {
			http.Error(w, `{"detail":"failed to list audit entries"}`, http.StatusInternalServerError)
			return
		}
		t := export.Table{Columns: []string{
			"id", "created_at", "entity_type", "entity_id", "action", "actor_name", "old_values", "new_values",
		}}
		for _, e := range items {
			t.Rows = append(t.Rows, []string{
				export.I64(e.ID),
				export.Time(e.CreatedAt),
				e.EntityType,
				export.I64(e.EntityID),
				e.Action,
				e.ActorName,
				jsonStr(e.OldValues),
				jsonStr(e.NewValues),
			})
		}
		export.Render(w, req.URL.Query().Get("format"), "audit", t)
	})
}

func jsonStr(v map[string]any) string {
	if len(v) == 0 {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
