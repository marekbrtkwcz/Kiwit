package testrun

import (
	"encoding/csv"
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// RegisterExportRoutes mounts CSV and HTML export endpoints for a TestRun.
func RegisterExportRoutes(r chi.Router, repo Repository) {
	r.Get("/api/v1/testruns/{id}/export", func(w http.ResponseWriter, req *http.Request) {
		runID, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err != nil || runID <= 0 {
			http.Error(w, `{"detail":"invalid testrun id"}`, http.StatusBadRequest)
			return
		}

		items, err := repo.ListExecutions(req.Context(), runID)
		if err != nil {
			http.Error(w, `{"detail":"failed to load executions"}`, http.StatusInternalServerError)
			return
		}

		format := req.URL.Query().Get("format")
		if format == "html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, "<!doctype html><html><head><meta charset='utf-8'><title>TestRun #%d</title></head><body><h1>TestRun #%d</h1><table border='1' cellpadding='6' cellspacing='0'><tr><th>ID</th><th>TestCase</th><th>Status</th><th>Comment</th></tr>", runID, runID)
			for _, ex := range items {
				_, _ = fmt.Fprintf(w, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
					ex.ID, html.EscapeString(ex.Summary), html.EscapeString(ex.Status), html.EscapeString(ex.Comment))
			}
			_, _ = fmt.Fprint(w, "</table></body></html>")
			return
		}

		// Default: CSV
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="testrun-%d.csv"`, runID))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"execution_id", "test_case_id", "summary", "status", "comment"})
		for _, ex := range items {
			_ = cw.Write([]string{
				strconv.FormatInt(ex.ID, 10),
				strconv.FormatInt(ex.TestCaseID, 10),
				ex.Summary,
				ex.Status,
				ex.Comment,
			})
		}
		cw.Flush()
	})
}
