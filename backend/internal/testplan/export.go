package testplan

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts the unified export endpoint for test plans:
//
//	GET /api/v1/testplans/export - filtered list (csv|json|md)
func RegisterExportRoutes(r chi.Router, repo Repository) {
	r.Get("/api/v1/testplans/export", func(w http.ResponseWriter, req *http.Request) {
		var isActive *bool
		switch req.URL.Query().Get("is_active") {
		case "true":
			v := true
			isActive = &v
		case "false":
			v := false
			isActive = &v
		}
		items, err := repo.ListFiltered(req.Context(), ListFilter{
			ProductID: export.QueryInt64(req, "product_id"),
			VersionID: export.QueryInt64(req, "version_id"),
			IsActive:  isActive,
			TagID:     export.QueryInt64(req, "tag_id"),
			Q:         req.URL.Query().Get("q"),
			UserID:    auth.FilterUserID(req.Context()),
		})
		if err != nil {
			http.Error(w, `{"detail":"failed to list test plans"}`, http.StatusInternalServerError)
			return
		}
		t := export.Table{Columns: []string{
			"id", "product_id", "version_id", "parent_id", "name",
			"plan_type", "is_active", "extra_link", "created_at",
		}}
		for _, p := range items {
			t.Rows = append(t.Rows, []string{
				export.I64(p.ID),
				export.I64(p.ProductID),
				export.I64(p.VersionID),
				export.OptI64(p.ParentID),
				p.Name,
				p.PlanType,
				boolStr(p.IsActive),
				p.ExtraLink,
				export.Time(p.CreatedAt),
			})
		}
		export.Render(w, req.URL.Query().Get("format"), "testplans", t)
	})
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
