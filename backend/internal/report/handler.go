package report

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
)

func parseIDs(r *http.Request) []int64 {
	raw := r.URL.Query().Get("milestone_ids")
	if raw == "" {
		raw = r.URL.Query().Get("ids")
	}
	var out []int64
	for _, p := range strings.Split(raw, ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

// RegisterRoutes mounts the multi-milestone (multi-campaign) report as JSON and printable HTML.
func RegisterRoutes(api huma.API, r chi.Router, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "multi-milestone-report",
		Method:      http.MethodGet,
		Path:        "/api/v1/reports/multi",
		Summary:     "Aggregated report across multiple milestones (campaigns) as JSON",
		Tags:        []string{"Reports"},
	}, func(ctx context.Context, input *struct {
		MilestoneIDs string `query:"milestone_ids" doc:"Comma-separated milestone IDs"`
	}) (*struct {
		Body MultiReport
	}, error) {
		ids := parseIDsFromStr(input.MilestoneIDs)
		if len(ids) == 0 {
			return nil, huma.Error400BadRequest("milestone_ids required")
		}
		for _, id := range ids {
			if err := guard.EnforceMilestone(ctx, id, false); err != nil {
				return nil, err
			}
		}
		rep, err := store.Build(ctx, ids)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build report", err)
		}
		rep.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
		return &struct {
			Body MultiReport
		}{Body: rep}, nil
	})

	// Printable HTML variant (browser Print → PDF).
	r.Get("/api/v1/reports/multi/html", func(w http.ResponseWriter, req *http.Request) {
		if _, ok := auth.UserFromContext(req.Context()); !ok {
			http.Error(w, `{"detail":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ids := parseIDs(req)
		if len(ids) == 0 {
			http.Error(w, `{"detail":"milestone_ids required"}`, http.StatusBadRequest)
			return
		}
		for _, id := range ids {
			if err := guard.EnforceMilestone(req.Context(), id, false); err != nil {
				http.Error(w, `{"detail":"forbidden"}`, http.StatusForbidden)
				return
			}
		}
		rep, err := store.Build(req.Context(), ids)
		if err != nil {
			http.Error(w, `{"detail":"failed to build report"}`, http.StatusInternalServerError)
			return
		}
		rep.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderHTML(rep)))
	})
}

func parseIDsFromStr(s string) []int64 {
	var out []int64
	for _, p := range strings.Split(s, ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func pct(part, total int64) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", float64(part)/float64(total)*100)
}

func renderHTML(rep MultiReport) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html lang=en><head><meta charset=utf-8><title>Kiwit Multi-Milestone Report</title>")
	b.WriteString("<style>body{font-family:system-ui,sans-serif;margin:2rem;color:#111}table{border-collapse:collapse;width:100%;margin:1rem 0}th,td{border:1px solid #ccc;padding:6px 10px;text-align:left;font-size:14px}th{background:#f3f4f6}h1{font-size:22px}h2{font-size:17px;margin-top:1.6rem}.badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:12px}.go{background:#dcfce7}.nogo{background:#fee2e2}.muted{color:#666}@media print{button{display:none}}</style>")
	b.WriteString("</head><body>")
	b.WriteString("<h1>Kiwit — Multi-Milestone Report</h1>")
	b.WriteString(fmt.Sprintf("<p class=muted>Generated: %s</p>", html.EscapeString(rep.GeneratedAt)))

	t := rep.Totals
	b.WriteString("<h2>Totals</h2><table><tr><th>Executions</th><th>Passed</th><th>Failed</th><th>Blocked</th><th>Skipped</th><th>Idle/Running</th><th>Open bugs</th></tr>")
	b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d (%s)</td><td>%d (%s)</td><td>%d</td><td>%d</td><td>%d/%d</td><td>%d</td></tr>",
		t.Total, t.Passed, pct(t.Passed, t.Total), t.Failed, pct(t.Failed, t.Total), t.Blocked, t.Skipped, t.Idle, t.Running, t.OpenBugs))
	b.WriteString("</table>")

	for _, m := range rep.Milestones {
		b.WriteString("<h2>")
		b.WriteString(html.EscapeString(fmt.Sprintf("#%d %s — %s %s (%s)", m.ID, m.Name, m.ProductName, m.Version, m.Status)))
		b.WriteString("</h2>")
		if m.SignoffDecision != nil {
			cls := "go"
			if *m.SignoffDecision == "no-go" {
				cls = "nogo"
			}
			b.WriteString(fmt.Sprintf("<p>Signoff: <span class='badge %s'>%s</span> %s</p>", cls, html.EscapeString(*m.SignoffDecision), html.EscapeString(deref(m.SignoffAt))))
		}
		b.WriteString("<table><tr><th>Executions</th><th>Passed</th><th>Failed</th><th>Blocked</th><th>Skipped</th><th>Idle/Running</th><th>Open bugs</th><th>Runs</th></tr>")
		runs := make([]string, len(m.Runs))
		for i, r := range m.Runs {
			runs[i] = fmt.Sprintf("#%d", r)
		}
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d (%s)</td><td>%d (%s)</td><td>%d</td><td>%d</td><td>%d/%d</td><td>%d</td><td>%s</td></tr>",
			m.Total, m.Passed, pct(m.Passed, m.Total), m.Failed, pct(m.Failed, m.Total), m.Blocked, m.Skipped, m.Idle, m.Running, m.OpenBugs, html.EscapeString(strings.Join(runs, ", "))))
		b.WriteString("</table>")
	}
	b.WriteString("</body></html>")
	return b.String()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
