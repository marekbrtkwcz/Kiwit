package requirement

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts the unified export endpoint for requirements:
//
//	GET /api/v1/requirements/export - filtered list (csv|json|md)
func RegisterExportRoutes(r chi.Router, store *Store) {
	r.Get("/api/v1/requirements/export", func(w http.ResponseWriter, req *http.Request) {
		items, _, err := store.List(req.Context(), filter{
			q:         req.URL.Query().Get("q"),
			productID: export.QueryInt64(req, "product_id"),
			status:    req.URL.Query().Get("status"),
			priority:  req.URL.Query().Get("priority"),
			category:  req.URL.Query().Get("category"),
		}, 10000, 0)
		if err != nil {
			http.Error(w, `{"detail":"failed to list requirements"}`, http.StatusInternalServerError)
			return
		}
		t := export.Table{Columns: []string{
			"id", "product_id", "key", "name", "version", "status",
			"priority", "category", "ai_quality", "created_at", "updated_at",
		}}
		for _, r := range items {
			t.Rows = append(t.Rows, []string{
				export.I64(r.ID),
				export.I64(r.ProductID),
				r.Key,
				r.Name,
				r.Version,
				r.Status,
				r.Priority,
				r.Category,
				r.AIQuality,
				export.Time(r.CreatedAt),
				export.Time(r.UpdatedAt),
			})
		}
		export.Render(w, req.URL.Query().Get("format"), "requirements", t)
	})
}
