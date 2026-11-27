package bug

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts the unified export endpoint for bugs:
//
//	GET /api/v1/bugs/export - filtered list (csv|json|md)
func RegisterExportRoutes(r chi.Router, store *Store) {
	r.Get("/api/v1/bugs/export", func(w http.ResponseWriter, req *http.Request) {
		items, err := store.List(req.Context(), Filter{
			ProductID:       export.QueryInt64(req, "product_id"),
			Status:          req.URL.Query().Get("status"),
			Severity:        req.URL.Query().Get("severity"),
			TestExecutionID: export.QueryInt64(req, "test_execution_id"),
			Q:               req.URL.Query().Get("q"),
			UserID:          auth.FilterUserID(req.Context()),
		})
		if err != nil {
			http.Error(w, `{"detail":"failed to list bugs"}`, http.StatusInternalServerError)
			return
		}
		t := export.Table{Columns: []string{
			"id", "product_id", "build_id", "test_execution_id", "summary",
			"status", "severity", "reporter_id", "assignee_id", "created_at", "updated_at",
		}}
		for _, b := range items {
			t.Rows = append(t.Rows, []string{
				export.I64(b.ID),
				export.I64(b.ProductID),
				export.OptI64(b.BuildID),
				export.OptI64(b.TestExecutionID),
				b.Summary,
				b.Status,
				b.Severity,
				export.OptI64(b.ReporterID),
				export.OptI64(b.AssigneeID),
				export.Time(b.CreatedAt),
				export.Time(b.UpdatedAt),
			})
		}
		export.Render(w, req.URL.Query().Get("format"), "bugs", t)
	})
}
