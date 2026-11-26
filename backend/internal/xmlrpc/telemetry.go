package xmlrpc

import (
	"fmt"
	"net/http"
	"time"
)

// Telemetry methods — lightweight aggregates over test executions (SQLite path).

func (h *Handler) telemetryBreakdown(r *http.Request, _ []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	var total, automated int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT COUNT(*), COALESCE(SUM(is_automated),0) FROM test_cases`).Scan(&total, &automated)
	var passed, failed, blocked, skipped int64
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT es.name, COUNT(*) FROM test_executions te
		JOIN execution_statuses es ON es.name = te.status
		GROUP BY es.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	statuses := map[string]int64{}
	for rows.Next() {
		var name string
		var c int64
		if err := rows.Scan(&name, &c); err == nil {
			statuses[name] = c
			switch name {
			case "passed", "pass":
				passed = c
			case "failed", "fail":
				failed = c
			case "blocked":
				blocked = c
			case "skipped", "skip":
				skipped = c
			}
		}
	}
	return map[string]any{
		"total_cases": total,
		"automated":   automated,
		"manual":      total - automated,
		"passed":      passed,
		"failed":      failed,
		"blocked":     blocked,
		"skipped":     skipped,
		"statuses":    statuses,
	}, nil
}

func (h *Handler) telemetryMatrix(r *http.Request, _ []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT tc.id, tc.summary, es.name, COUNT(te.id)
		FROM test_cases tc
		LEFT JOIN test_executions te ON te.case_id = tc.id
		LEFT JOIN execution_statuses es ON es.name = te.status
		GROUP BY tc.id, tc.summary, es.name ORDER BY tc.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		CaseID  int64             `json:"case_id"`
		Summary string            `json:"summary"`
		Counts  map[string]int64  `json:"counts"`
	}
	m := map[int64]*row{}
	var order []int64
	for rows.Next() {
		var caseID int64
		var summary, status string
		var c int64
		if err := rows.Scan(&caseID, &summary, &status, &c); err != nil {
			continue
		}
		if _, ok := m[caseID]; !ok {
			m[caseID] = &row{CaseID: caseID, Summary: summary, Counts: map[string]int64{}}
			order = append(order, caseID)
		}
		if status != "" {
			m[caseID].Counts[status] = c
		}
	}
	out := make([]map[string]any, 0, len(order))
	for _, id := range order {
		rw := m[id]
		out = append(out, map[string]any{"case_id": rw.CaseID, "summary": rw.Summary, "counts": rw.Counts})
	}
	return out, nil
}

func (h *Handler) telemetryTrends(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "query")
	from, _ := m["from"].(string)
	to, _ := m["to"].(string)
	q := `SELECT DATE(te.executed_at), es.name, COUNT(*) FROM test_executions te
		 JOIN execution_statuses es ON es.name = te.status WHERE 1=1`
	var params []any
	if from != "" {
		q += ` AND DATE(te.executed_at) >= ?`
		params = append(params, from)
	}
	if to != "" {
		q += ` AND DATE(te.executed_at) <= ?`
		params = append(params, to)
	}
	q += ` GROUP BY DATE(te.executed_at), es.name ORDER BY 1`
	rows, err := h.db.QueryContext(r.Context(), q, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type day struct {
		Date   string           `json:"date"`
		Counts map[string]int64 `json:"counts"`
	}
	days := map[string]*day{}
	var order []string
	for rows.Next() {
		var d, status string
		var c int64
		if err := rows.Scan(&d, &status, &c); err != nil {
			continue
		}
		if _, ok := days[d]; !ok {
			days[d] = &day{Date: d, Counts: map[string]int64{}}
			order = append(order, d)
		}
		days[d].Counts[status] = c
	}
	out := make([]map[string]any, 0, len(order))
	for _, d := range order {
		out = append(out, map[string]any{"date": days[d].Date, "counts": days[d].Counts})
	}
	return out, nil
}

func (h *Handler) telemetryHealth(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "query")
	limit := 50
	if l, ok := m["limit"]; ok {
		limit = int(argInt(l))
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT tc.id, tc.summary,
			COUNT(te.id) AS total,
			COALESCE(SUM(CASE WHEN es.name IN ('failed','fail') THEN 1 ELSE 0 END),0) AS failed,
			COALESCE(SUM(CASE WHEN es.name IN ('passed','pass') THEN 1 ELSE 0 END),0) AS passed
		FROM test_cases tc LEFT JOIN test_executions te ON te.case_id = tc.id
		LEFT JOIN execution_statuses es ON es.name = te.status
		GROUP BY tc.id, tc.summary HAVING total > 0 ORDER BY failed DESC, total DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, total, failed, passed int64
		var summary string
		if err := rows.Scan(&id, &summary, &total, &failed, &passed); err != nil {
			continue
		}
		state := "healthy"
		if total > 0 && failed*2 >= total {
			state = "failing"
		} else if failed > 0 && passed > 0 {
			state = "flaky"
		}
		out = append(out, map[string]any{
			"case_id": id, "summary": summary,
			"total": total, "failed": failed, "passed": passed,
			"state": state, "checked_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

var _ = fmt.Sprintf
