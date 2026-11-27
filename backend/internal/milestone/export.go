package milestone

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts unified export endpoints for milestones:
//
//	GET  /api/v1/milestones/export - list, optionally filtered by version (csv|json|md)
//	POST /api/v1/milestones/export - selected IDs ({ids, format})
func RegisterExportRoutes(r chi.Router, store *Store, guard *auth.AccessGuard) {
	r.Get("/api/v1/milestones/export", func(w http.ResponseWriter, req *http.Request) {
		versionID := export.QueryInt64(req, "version_id")
		if versionID != 0 {
			if auth.WriteAccessError(w, guard.EnforceVersion(req.Context(), versionID, false)) {
				return
			}
		}
		items, err := store.List(req.Context(), versionID)
		if err != nil {
			http.Error(w, `{"detail":"failed to list milestones"}`, http.StatusInternalServerError)
			return
		}
		export.Render(w, req.URL.Query().Get("format"), "milestones", milestoneTable(items))
	})

	r.Post("/api/v1/milestones/export", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			IDs    []int64 `json:"ids"`
			Format string  `json:"format"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
			http.Error(w, `{"detail":"ids required"}`, http.StatusBadRequest)
			return
		}
		items := make([]Milestone, 0, len(body.IDs))
		for _, id := range body.IDs {
			if auth.WriteAccessError(w, guard.EnforceMilestone(req.Context(), id, false)) {
				return
			}
			m, err := store.Get(req.Context(), id)
			if err != nil {
				continue // skip deleted milestones, keep the rest exportable
			}
			items = append(items, m)
		}
		export.Render(w, body.Format, "milestones", milestoneTable(items))
	})
}

func milestoneTable(items []Milestone) export.Table {
	t := export.Table{Columns: []string{
		"id", "version_id", "name", "description", "due_date", "status",
		"signoff_decision", "signoff_note", "created_at", "updated_at",
	}}
	for _, m := range items {
		t.Rows = append(t.Rows, []string{
			export.I64(m.ID),
			export.I64(m.VersionID),
			m.Name,
			m.Description,
			optStr(m.DueDate),
			m.Status,
			optStr(m.SignoffDecision),
			m.SignoffNote,
			export.Time(m.CreatedAt),
			export.Time(m.UpdatedAt),
		})
	}
	return t
}

func optStr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
