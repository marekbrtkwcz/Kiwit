package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
)

type Filter struct {
	ProductID  int64
	VersionID  int64
	BuildID    int64
	TestPlanID int64
	From       string // YYYY-MM-DD
	To         string // YYYY-MM-DD
	UserID     *int64
}

type filterInput struct {
	ProductID  int64  `query:"product_id" doc:"Filter by product ID"`
	VersionID  int64  `query:"version_id" doc:"Filter by version ID"`
	BuildID    int64  `query:"build_id" doc:"Filter by build ID"`
	TestPlanID int64  `query:"test_plan_id" doc:"Filter by test plan ID"`
	From       string `query:"from" doc:"Start date inclusive (YYYY-MM-DD)"`
	To         string `query:"to" doc:"End date inclusive (YYYY-MM-DD)"`
}

func (in *filterInput) toFilter(ctx context.Context) Filter {
	return Filter{
		ProductID:  in.ProductID,
		VersionID:  in.VersionID,
		BuildID:    in.BuildID,
		TestPlanID: in.TestPlanID,
		From:       strings.TrimSpace(in.From),
		To:         strings.TrimSpace(in.To),
		UserID:     auth.FilterUserID(ctx),
	}
}

type Summary struct {
	Products   int64 `json:"products"`
	TestCases  int64 `json:"test_cases"`
	TestPlans  int64 `json:"test_plans"`
	TestRuns   int64 `json:"test_runs"`
	Executions struct {
		Total    int64            `json:"total"`
		Passed   int64            `json:"passed"`
		Failed   int64            `json:"failed"`
		Blocked  int64            `json:"blocked"`
		Skipped  int64            `json:"skipped"`
		Idle     int64            `json:"idle"`
		ByStatus map[string]int64 `json:"by_status"`
	} `json:"executions"`
}

type TrendPoint struct {
	Date   string `json:"date"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type MatrixRow struct {
	TestCaseID int64  `json:"test_case_id"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	Count      int64  `json:"count"`
}

type CaseHealth struct {
	TestCaseID int64   `json:"test_case_id"`
	Summary    string  `json:"summary"`
	Total      int64   `json:"total"`
	Failed     int64   `json:"failed"`
	FailRate   float64 `json:"fail_rate"`
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type Breakdown struct {
	Automated  int64        `json:"automated"`
	Manual     int64        `json:"manual"`
	ByPriority []NamedCount `json:"by_priority"`
	ByStatus   []NamedCount `json:"by_status"`
	ByCategory []NamedCount `json:"by_category"`
}

func RegisterRoutes(api huma.API, db *sql.DB, pgPool *pgxpool.Pool) {
	huma.Register(api, huma.Operation{
		OperationID: "telemetry-summary",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/summary",
		Summary:     "Get TCMS telemetry summary and Pass/Fail breakdown",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *filterInput) (*struct{ Body Summary }, error) {
		s, err := querySummary(ctx, db, pgPool, input.toFilter(ctx))
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query telemetry", err)
		}
		return &struct{ Body Summary }{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "telemetry-execution-trends",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/execution-trends",
		Summary:     "Get daily execution trends by status",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *filterInput) (*struct {
		Body struct {
			Items []TrendPoint `json:"items"`
		}
	}, error) {
		items, err := queryTrends(ctx, db, pgPool, input.toFilter(ctx))
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query execution trends", err)
		}
		out := &struct {
			Body struct {
				Items []TrendPoint `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "telemetry-status-matrix",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/status-matrix",
		Summary:     "Get status matrix per test case",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *filterInput) (*struct {
		Body struct {
			Items []MatrixRow `json:"items"`
		}
	}, error) {
		items, err := queryMatrix(ctx, db, pgPool, input.toFilter(ctx))
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query status matrix", err)
		}
		out := &struct {
			Body struct {
				Items []MatrixRow `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "telemetry-test-case-health",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/test-case-health",
		Summary:     "Identify flaky/failing test cases sorted by fail rate",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *filterInput) (*struct {
		Body struct {
			Items []CaseHealth `json:"items"`
		}
	}, error) {
		items, err := queryHealth(ctx, db, pgPool, input.toFilter(ctx))
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query test case health", err)
		}
		out := &struct {
			Body struct {
				Items []CaseHealth `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "telemetry-breakdown",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/breakdown",
		Summary:     "Get test case breakdown (automated vs manual, priority, status, category)",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *filterInput) (*struct{ Body Breakdown }, error) {
		b, err := queryBreakdown(ctx, db, pgPool, input.toFilter(ctx))
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query telemetry breakdown", err)
		}
		return &struct{ Body Breakdown }{Body: b}, nil
	})
}

type sqlBuilder struct {
	pg   bool
	cond []string
	args []any
}

func (b *sqlBuilder) ph() string {
	if b.pg {
		return fmt.Sprintf("$%d", len(b.args)+1)
	}
	return "?"
}

func (b *sqlBuilder) add(expr string, val any) {
	b.cond = append(b.cond, fmt.Sprintf(expr, b.ph()))
	b.args = append(b.args, val)
}

func (b *sqlBuilder) addProductScope(prodCol string, userID *int64) {
	if userID == nil {
		return
	}
	b.cond = append(b.cond, fmt.Sprintf(
		"(NOT EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = %s) OR EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = %s AND pm.user_id = %s))",
		prodCol, prodCol, b.ph(),
	))
	b.args = append(b.args, *userID)
}

func (b *sqlBuilder) where() string {
	if len(b.cond) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(b.cond, " AND ")
}

func execFilterBuilder(pg bool, f Filter) *sqlBuilder {
	b := &sqlBuilder{pg: pg}
	if f.ProductID > 0 {
		b.add("tp.product_id = %s", f.ProductID)
	}
	if f.VersionID > 0 {
		b.add("tp.version_id = %s", f.VersionID)
	}
	if f.BuildID > 0 {
		b.add("tr.build_id = %s", f.BuildID)
	}
	if f.TestPlanID > 0 {
		b.add("tr.test_plan_id = %s", f.TestPlanID)
	}
	if f.From != "" {
		if pg {
			b.add("TO_CHAR(e.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') >= %s", f.From)
		} else {
			b.add("SUBSTR(CAST(e.created_at AS TEXT), 1, 10) >= %s", f.From)
		}
	}
	if f.To != "" {
		if pg {
			b.add("TO_CHAR(e.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') <= %s", f.To)
		} else {
			b.add("SUBSTR(CAST(e.created_at AS TEXT), 1, 10) <= %s", f.To)
		}
	}
	b.addProductScope("tp.product_id", f.UserID)
	return b
}

func queryCount(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, q string, args ...any) (int64, error) {
	var n int64
	if pgPool != nil {
		err := pgPool.QueryRow(ctx, q, args...).Scan(&n)
		return n, err
	}
	err := db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func querySummary(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, f Filter) (Summary, error) {
	var s Summary
	pg := pgPool != nil

	// Products count
	pb := &sqlBuilder{pg: pg}
	if f.ProductID > 0 {
		pb.add("p.id = %s", f.ProductID)
	}
	pb.addProductScope("p.id", f.UserID)
	s.Products, _ = queryCount(ctx, db, pgPool, "SELECT COUNT(*) FROM products p"+pb.where(), pb.args...)

	// TestCases count
	cb := &sqlBuilder{pg: pg}
	if f.ProductID > 0 {
		cb.add("tc.product_id = %s", f.ProductID)
	}
	if f.TestPlanID > 0 {
		cb.add("EXISTS (SELECT 1 FROM test_plan_cases tpc WHERE tpc.test_case_id = tc.id AND tpc.test_plan_id = %s)", f.TestPlanID)
	}
	cb.addProductScope("tc.product_id", f.UserID)
	s.TestCases, _ = queryCount(ctx, db, pgPool, "SELECT COUNT(*) FROM test_cases tc"+cb.where(), cb.args...)

	// TestPlans count
	tpb := &sqlBuilder{pg: pg}
	if f.ProductID > 0 {
		tpb.add("tp.product_id = %s", f.ProductID)
	}
	if f.VersionID > 0 {
		tpb.add("tp.version_id = %s", f.VersionID)
	}
	if f.TestPlanID > 0 {
		tpb.add("tp.id = %s", f.TestPlanID)
	}
	tpb.addProductScope("tp.product_id", f.UserID)
	s.TestPlans, _ = queryCount(ctx, db, pgPool, "SELECT COUNT(*) FROM test_plans tp"+tpb.where(), tpb.args...)

	// TestRuns count
	trb := &sqlBuilder{pg: pg}
	if f.ProductID > 0 {
		trb.add("tp.product_id = %s", f.ProductID)
	}
	if f.VersionID > 0 {
		trb.add("tp.version_id = %s", f.VersionID)
	}
	if f.BuildID > 0 {
		trb.add("tr.build_id = %s", f.BuildID)
	}
	if f.TestPlanID > 0 {
		trb.add("tr.test_plan_id = %s", f.TestPlanID)
	}
	trb.addProductScope("tp.product_id", f.UserID)
	s.TestRuns, _ = queryCount(ctx, db, pgPool,
		"SELECT COUNT(*) FROM test_runs tr JOIN test_plans tp ON tp.id = tr.test_plan_id"+trb.where(),
		trb.args...,
	)

	// Executions breakdown
	eb := execFilterBuilder(pg, f)
	eq := `SELECT e.status, COUNT(*)
		FROM test_executions e
		JOIN test_runs tr ON tr.id = e.test_run_id
		JOIN test_plans tp ON tp.id = tr.test_plan_id` + eb.where() + ` GROUP BY e.status`

	if pg {
		rows, err := pgPool.Query(ctx, eq, eb.args...)
		if err != nil {
			return s, err
		}
		defer rows.Close()
		for rows.Next() {
			var status string
			var cnt int64
			if err := rows.Scan(&status, &cnt); err != nil {
				return s, err
			}
			accumulateStatus(&s, status, cnt)
		}
		return s, rows.Err()
	}
	rows, err := db.QueryContext(ctx, eq, eb.args...)
	if err != nil {
		return s, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			return s, err
		}
		accumulateStatus(&s, status, cnt)
	}
	return s, rows.Err()
}

func queryTrends(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, f Filter) ([]TrendPoint, error) {
	items := make([]TrendPoint, 0)
	pg := pgPool != nil
	eb := execFilterBuilder(pg, f)
	dateExpr := `SUBSTR(CAST(e.created_at AS TEXT), 1, 10)`
	if pg {
		dateExpr = `TO_CHAR(e.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')`
	}
	q := `SELECT ` + dateExpr + ` AS d, e.status, COUNT(*)
		FROM test_executions e
		JOIN test_runs tr ON tr.id = e.test_run_id
		JOIN test_plans tp ON tp.id = tr.test_plan_id` + eb.where() + ` GROUP BY d, e.status ORDER BY d ASC`

	if pg {
		rows, err := pgPool.Query(ctx, q, eb.args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var t TrendPoint
			if err := rows.Scan(&t.Date, &t.Status, &t.Count); err != nil {
				return nil, err
			}
			items = append(items, t)
		}
		return items, rows.Err()
	}
	rows, err := db.QueryContext(ctx, q, eb.args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t TrendPoint
		if err := rows.Scan(&t.Date, &t.Status, &t.Count); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func queryMatrix(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, f Filter) ([]MatrixRow, error) {
	items := make([]MatrixRow, 0)
	pg := pgPool != nil
	eb := execFilterBuilder(pg, f)
	q := `SELECT e.test_case_id, tc.summary, e.status, COUNT(*)
		FROM test_executions e
		JOIN test_cases tc ON tc.id = e.test_case_id
		JOIN test_runs tr ON tr.id = e.test_run_id
		JOIN test_plans tp ON tp.id = tr.test_plan_id` + eb.where() + `
		GROUP BY e.test_case_id, tc.summary, e.status
		ORDER BY e.test_case_id ASC`

	if pg {
		rows, err := pgPool.Query(ctx, q, eb.args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var m MatrixRow
			if err := rows.Scan(&m.TestCaseID, &m.Summary, &m.Status, &m.Count); err != nil {
				return nil, err
			}
			items = append(items, m)
		}
		return items, rows.Err()
	}
	rows, err := db.QueryContext(ctx, q, eb.args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m MatrixRow
		if err := rows.Scan(&m.TestCaseID, &m.Summary, &m.Status, &m.Count); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func queryHealth(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, f Filter) ([]CaseHealth, error) {
	items := make([]CaseHealth, 0)
	pg := pgPool != nil
	eb := execFilterBuilder(pg, f)
	q := `
		SELECT e.test_case_id, tc.summary,
		       COUNT(*) AS total,
		       SUM(CASE WHEN e.status = 'failed' THEN 1 ELSE 0 END) AS failed
		FROM test_executions e
		JOIN test_cases tc ON tc.id = e.test_case_id
		JOIN test_runs tr ON tr.id = e.test_run_id
		JOIN test_plans tp ON tp.id = tr.test_plan_id` + eb.where() + `
		GROUP BY e.test_case_id, tc.summary
		ORDER BY failed DESC, total DESC`

	scanRows := func(rows interface {
		Next() bool
		Scan(...any) error
		Err() error
	}) error {
		for rows.Next() {
			var h CaseHealth
			if err := rows.Scan(&h.TestCaseID, &h.Summary, &h.Total, &h.Failed); err != nil {
				return err
			}
			if h.Total > 0 {
				h.FailRate = float64(h.Failed) / float64(h.Total)
			}
			items = append(items, h)
		}
		return rows.Err()
	}
	if pg {
		rows, err := pgPool.Query(ctx, q, eb.args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return items, scanRows(rows)
	}
	rows, err := db.QueryContext(ctx, q, eb.args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return items, scanRows(rows)
}

func queryBreakdown(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool, f Filter) (Breakdown, error) {
	out := Breakdown{
		ByPriority: make([]NamedCount, 0),
		ByStatus:   make([]NamedCount, 0),
		ByCategory: make([]NamedCount, 0),
	}
	pg := pgPool != nil
	cb := &sqlBuilder{pg: pg}
	if f.ProductID > 0 {
		cb.add("tc.product_id = %s", f.ProductID)
	}
	if f.TestPlanID > 0 {
		cb.add("EXISTS (SELECT 1 FROM test_plan_cases tpc WHERE tpc.test_case_id = tc.id AND tpc.test_plan_id = %s)", f.TestPlanID)
	}
	cb.addProductScope("tc.product_id", f.UserID)
	where := cb.where()

	// Automated vs Manual
	autoExpr := "SUM(CASE WHEN tc.is_automated = 1 THEN 1 ELSE 0 END)"
	manualExpr := "SUM(CASE WHEN tc.is_automated = 0 THEN 1 ELSE 0 END)"
	if pg {
		autoExpr = "SUM(CASE WHEN tc.is_automated THEN 1 ELSE 0 END)"
		manualExpr = "SUM(CASE WHEN NOT tc.is_automated THEN 1 ELSE 0 END)"
	}
	autoQ := "SELECT COALESCE(" + autoExpr + ", 0), COALESCE(" + manualExpr + ", 0) FROM test_cases tc" + where
	if pg {
		if err := pgPool.QueryRow(ctx, autoQ, cb.args...).Scan(&out.Automated, &out.Manual); err != nil {
			return out, err
		}
	} else {
		if err := db.QueryRowContext(ctx, autoQ, cb.args...).Scan(&out.Automated, &out.Manual); err != nil {
			return out, err
		}
	}

	queryGrouped := func(q string) ([]NamedCount, error) {
		res := make([]NamedCount, 0)
		if pg {
			rows, err := pgPool.Query(ctx, q, cb.args...)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			for rows.Next() {
				var nc NamedCount
				if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
					return nil, err
				}
				res = append(res, nc)
			}
			return res, rows.Err()
		}
		rows, err := db.QueryContext(ctx, q, cb.args...)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var nc NamedCount
			if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			res = append(res, nc)
		}
		return res, rows.Err()
	}

	var err error
	if out.ByPriority, err = queryGrouped("SELECT tc.priority, COUNT(*) FROM test_cases tc" + where + " GROUP BY tc.priority ORDER BY COUNT(*) DESC"); err != nil {
		return out, err
	}
	if out.ByStatus, err = queryGrouped("SELECT tc.status, COUNT(*) FROM test_cases tc" + where + " GROUP BY tc.status ORDER BY COUNT(*) DESC"); err != nil {
		return out, err
	}
	if out.ByCategory, err = queryGrouped("SELECT COALESCE(c.name, 'Uncategorized'), COUNT(*) FROM test_cases tc LEFT JOIN categories c ON c.id = tc.category_id" + where + " GROUP BY COALESCE(c.name, 'Uncategorized') ORDER BY COUNT(*) DESC"); err != nil {
		return out, err
	}
	return out, nil
}

func accumulateStatus(s *Summary, status string, cnt int64) {
	if s.Executions.ByStatus == nil {
		s.Executions.ByStatus = make(map[string]int64)
	}
	s.Executions.ByStatus[status] += cnt
	s.Executions.Total += cnt
	switch status {
	case "passed":
		s.Executions.Passed = cnt
	case "failed":
		s.Executions.Failed = cnt
	case "blocked":
		s.Executions.Blocked = cnt
	case "skipped":
		s.Executions.Skipped = cnt
	default:
		s.Executions.Idle += cnt
	}
}
