package testrun

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts unified export endpoints for test runs:
//
//	GET  /api/v1/testruns/{id}/export   - executions of one run (csv|html|json|md|junit)
//	GET  /api/v1/testruns/export        - filtered list of runs (csv|json|md)
//	POST /api/v1/testruns/export        - selected run IDs ({ids, format})
func RegisterExportRoutes(r chi.Router, repo Repository, guard *auth.AccessGuard) {
	r.Get("/api/v1/testruns/{id}/export", func(w http.ResponseWriter, req *http.Request) {
		runID, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err != nil || runID <= 0 {
			http.Error(w, `{"detail":"invalid testrun id"}`, http.StatusBadRequest)
			return
		}
		if auth.WriteAccessError(w, guard.EnforceTestRun(req.Context(), runID, false)) {
			return
		}

		items, err := repo.ListExecutions(req.Context(), runID)
		if err != nil {
			http.Error(w, `{"detail":"failed to load executions"}`, http.StatusInternalServerError)
			return
		}

		name := fmt.Sprintf("testrun-%d", runID)
		switch req.URL.Query().Get("format") {
		case "html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, "<!doctype html><html><head><meta charset='utf-8'><title>TestRun #%d</title></head><body><h1>TestRun #%d</h1><table border='1' cellpadding='6' cellspacing='0'><tr><th>ID</th><th>TestCase</th><th>Status</th><th>Comment</th></tr>", runID, runID)
			for _, ex := range items {
				_, _ = fmt.Fprintf(w, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>",
					ex.ID, html.EscapeString(ex.Summary), html.EscapeString(ex.Status), html.EscapeString(ex.Comment))
			}
			_, _ = fmt.Fprint(w, "</table></body></html>")
		case "junit":
			cases := make([]export.JUnitCase, 0, len(items))
			for _, ex := range items {
				cases = append(cases, export.JUnitCase{
					Name:      ex.Summary,
					ClassName: name,
					Status:    export.JUnitStatus(ex.Status),
					Message:   ex.Comment,
				})
			}
			export.RenderJUnit(w, name, cases)
		default:
			format := export.Format(req.URL.Query().Get("format"))
			if format == "csv" {
				// keep the original single-run CSV layout (execution-centric)
				w.Header().Set("Content-Type", "text/csv; charset=utf-8")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, name))
				_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
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
				return
			}
			export.Render(w, format, name, executionTable(items))
		}
	})

	r.Get("/api/v1/testruns/export", func(w http.ResponseWriter, req *http.Request) {
		items, _, err := repo.ListFiltered(req.Context(), ListFilter{
			ProductID:  export.QueryInt64(req, "product_id"),
			TestPlanID: export.QueryInt64(req, "test_plan_id"),
			BuildID:    export.QueryInt64(req, "build_id"),
			Status:     req.URL.Query().Get("status"),
			Q:          req.URL.Query().Get("q"),
			UserID:     auth.FilterUserID(req.Context()),
			Limit:      10000,
		})
		if err != nil {
			http.Error(w, `{"detail":"failed to list test runs"}`, http.StatusInternalServerError)
			return
		}
		export.Render(w, req.URL.Query().Get("format"), "testruns", runTable(items))
	})

	r.Post("/api/v1/testruns/export", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			IDs    []int64 `json:"ids"`
			Format string  `json:"format"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
			http.Error(w, `{"detail":"ids required"}`, http.StatusBadRequest)
			return
		}
		if len(body.IDs) > 10000 {
			http.Error(w, `{"detail":"too many ids"}`, http.StatusBadRequest)
			return
		}
		items := make([]TestRun, 0, len(body.IDs))
		for _, id := range body.IDs {
			if auth.WriteAccessError(w, guard.EnforceTestRun(req.Context(), id, false)) {
				return
			}
			run, err := repo.GetByID(req.Context(), id)
			if err != nil {
				continue // skip deleted runs, keep the rest exportable
			}
			items = append(items, run)
		}
		export.Render(w, body.Format, "testruns", runTable(items))
	})
}


func runStatus(r TestRun) string {
	if r.StoppedAt != nil {
		return "stopped"
	}
	return "running"
}

func runTable(items []TestRun) export.Table {
	t := export.Table{Columns: []string{
		"id", "test_plan_id", "build_id", "environment_id", "manager_id",
		"summary", "status", "planned_start", "planned_stop", "started_at", "stopped_at",
	}}
	for _, r := range items {
		t.Rows = append(t.Rows, []string{
			export.I64(r.ID),
			export.I64(r.TestPlanID),
			export.I64(r.BuildID),
			export.OptI64(r.EnvironmentID),
			export.OptI64(r.ManagerID),
			r.Summary,
			runStatus(r),
			export.OptTime(r.PlannedStart),
			export.OptTime(r.PlannedStop),
			export.Time(r.StartedAt),
			export.OptTime(r.StoppedAt),
		})
	}
	return t
}

func executionTable(items []ExecutionItem) export.Table {
	t := export.Table{Columns: []string{
		"execution_id", "test_case_id", "summary", "status", "comment",
		"assignee_id", "tested_by_id", "environment_id", "started_at",
	}}
	for _, ex := range items {
		t.Rows = append(t.Rows, []string{
			export.I64(ex.ID),
			export.I64(ex.TestCaseID),
			ex.Summary,
			ex.Status,
			ex.Comment,
			export.OptI64(ex.AssigneeID),
			export.OptI64(ex.TestedByID),
			export.OptI64(ex.EnvironmentID),
			export.OptTime(ex.StartedAt),
		})
	}
	return t
}
