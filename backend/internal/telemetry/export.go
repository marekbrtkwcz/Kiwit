package telemetry

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

// RegisterExportRoutes mounts the unified export endpoint for telemetry:
//
//	GET /api/v1/telemetry/export?dataset=summary|trends|matrix|health|breakdown|ux|dashboard
//
// It accepts the same filters as the telemetry API and renders CSV, JSON or
// Markdown via the shared export renderer.
func RegisterExportRoutes(r chi.Router, db *sql.DB, pgPool *pgxpool.Pool) {
	r.Get("/api/v1/telemetry/export", func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		f := Filter{
			ProductID:  export.QueryInt64(req, "product_id"),
			VersionID:  export.QueryInt64(req, "version_id"),
			BuildID:    export.QueryInt64(req, "build_id"),
			TestPlanID: export.QueryInt64(req, "test_plan_id"),
			From:       req.URL.Query().Get("from"),
			To:         req.URL.Query().Get("to"),
			UserID:     auth.FilterUserID(ctx),
		}
		format := req.URL.Query().Get("format")
		dataset := req.URL.Query().Get("dataset")

		table, name, err := telemetryTable(ctx, db, pgPool, f, dataset)
		if err != nil {
			http.Error(w, `{"detail":"failed to query telemetry"}`, http.StatusInternalServerError)
			return
		}
		export.Render(w, format, name, table)
	})
}

func telemetryTable(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, f Filter, dataset string) (export.Table, string, error) {
	switch dataset {
	case "trends":
		items, err := queryTrends(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{"date", "status", "count"}}
		for _, p := range items {
			t.Rows = append(t.Rows, []string{p.Date, p.Status, export.I64(p.Count)})
		}
		return t, "telemetry-trends", nil

	case "matrix":
		items, err := queryMatrix(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{"test_case_id", "summary", "status", "count"}}
		for _, m := range items {
			t.Rows = append(t.Rows, []string{export.I64(m.TestCaseID), m.Summary, m.Status, export.I64(m.Count)})
		}
		return t, "telemetry-status-matrix", nil

	case "health":
		items, err := queryHealth(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{"test_case_id", "summary", "total", "failed", "fail_rate"}}
		for _, h := range items {
			t.Rows = append(t.Rows, []string{export.I64(h.TestCaseID), h.Summary, export.I64(h.Total), export.I64(h.Failed), export.F(h.FailRate)})
		}
		return t, "telemetry-test-case-health", nil

	case "breakdown":
		b, err := queryBreakdown(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{"metric", "value"}}
		t.Rows = append(t.Rows,
			[]string{"automated", export.I64(b.Automated)},
			[]string{"manual", export.I64(b.Manual)},
		)
		for _, c := range b.ByPriority {
			t.Rows = append(t.Rows, []string{"priority:" + c.Name, export.I64(c.Count)})
		}
		for _, c := range b.ByStatus {
			t.Rows = append(t.Rows, []string{"status:" + c.Name, export.I64(c.Count)})
		}
		for _, c := range b.ByCategory {
			t.Rows = append(t.Rows, []string{"category:" + c.Name, export.I64(c.Count)})
		}
		return t, "telemetry-breakdown", nil

	case "ux":
		rows, err := queryUXMetrics(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{"product_id", "product_name", "sessions", "avg_sus", "avg_task_time_sec", "avg_errors"}}
		for _, u := range rows {
			t.Rows = append(t.Rows, []string{export.I64(u.ProductID), u.ProductName, export.I64(u.Sessions), export.F(u.AvgSUS), export.F(u.AvgTaskTimeSec), export.F(u.AvgErrors)})
		}
		return t, "telemetry-ux-metrics", nil

	case "dashboard":
		d, err := queryExecutionDashboard(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{
			"test_run_id", "test_plan_id", "plan_name", "summary", "started_at", "stopped",
			"total", "passed", "failed", "blocked", "skipped", "idle", "progress",
		}}
		for _, r := range d.Runs {
			t.Rows = append(t.Rows, []string{
				export.I64(r.TestRunID), export.I64(r.TestPlanID), r.PlanName, r.Summary,
				r.StartedAt, boolStr(r.Stopped), export.I64(r.Total), export.I64(r.Passed),
				export.I64(r.Failed), export.I64(r.Blocked), export.I64(r.Skipped),
				export.I64(r.Idle), export.F(r.Progress),
			})
		}
		return t, "telemetry-execution-dashboard", nil

	default: // summary
		s, err := querySummary(ctx, db, pgPool, f)
		if err != nil {
			return export.Table{}, "", err
		}
		t := export.Table{Columns: []string{"metric", "value"}}
		t.Rows = append(t.Rows,
			[]string{"products", export.I64(s.Products)},
			[]string{"test_cases", export.I64(s.TestCases)},
			[]string{"test_plans", export.I64(s.TestPlans)},
			[]string{"test_runs", export.I64(s.TestRuns)},
			[]string{"executions:total", export.I64(s.Executions.Total)},
			[]string{"executions:passed", export.I64(s.Executions.Passed)},
			[]string{"executions:failed", export.I64(s.Executions.Failed)},
			[]string{"executions:blocked", export.I64(s.Executions.Blocked)},
			[]string{"executions:skipped", export.I64(s.Executions.Skipped)},
			[]string{"executions:idle", export.I64(s.Executions.Idle)},
		)
		return t, "telemetry-summary", nil
	}
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
