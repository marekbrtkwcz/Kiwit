package telemetry

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Summary struct {
	Products   int64 `json:"products"`
	TestCases  int64 `json:"test_cases"`
	TestPlans  int64 `json:"test_plans"`
	TestRuns   int64 `json:"test_runs"`
	Executions struct {
		Total   int64 `json:"total"`
		Passed  int64 `json:"passed"`
		Failed  int64 `json:"failed"`
		Blocked int64 `json:"blocked"`
		Skipped int64 `json:"skipped"`
		Idle    int64 `json:"idle"`
	} `json:"executions"`
}

func RegisterRoutes(api huma.API, db *sql.DB, pgPool *pgxpool.Pool) {
	huma.Register(api, huma.Operation{
		OperationID: "telemetry-summary",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/summary",
		Summary:     "Get TCMS telemetry summary and Pass/Fail breakdown",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *struct{}) (*struct{ Body Summary }, error) {
		var s Summary
		if pgPool != nil {
			_ = pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&s.Products)
			_ = pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM test_cases`).Scan(&s.TestCases)
			_ = pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM test_plans`).Scan(&s.TestPlans)
			_ = pgPool.QueryRow(ctx, `SELECT COUNT(*) FROM test_runs`).Scan(&s.TestRuns)

			rows, err := pgPool.Query(ctx, `SELECT status, COUNT(*) FROM test_executions GROUP BY status`)
			if err != nil {
				return nil, huma.Error500InternalServerError("failed to query telemetry", err)
			}
			defer rows.Close()
			for rows.Next() {
				var status string
				var cnt int64
				if err := rows.Scan(&status, &cnt); err != nil {
					return nil, huma.Error500InternalServerError("failed to scan telemetry", err)
				}
				accumulateStatus(&s, status, cnt)
			}
			return &struct{ Body Summary }{Body: s}, nil
		}

		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products`).Scan(&s.Products)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_cases`).Scan(&s.TestCases)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_plans`).Scan(&s.TestPlans)
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_runs`).Scan(&s.TestRuns)

		rows, err := db.QueryContext(ctx, `SELECT status, COUNT(*) FROM test_executions GROUP BY status`)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query telemetry", err)
		}
		defer func() { _ = rows.Close() }()

		for rows.Next() {
			var status string
			var cnt int64
			if err := rows.Scan(&status, &cnt); err != nil {
				return nil, huma.Error500InternalServerError("failed to scan telemetry", err)
			}
			accumulateStatus(&s, status, cnt)
		}

		return &struct{ Body Summary }{Body: s}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "telemetry-execution-trends",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/execution-trends",
		Summary:     "Get daily execution trends by status",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []TrendPoint `json:"items"`
		}
	}, error) {
		items, err := queryTrends(ctx, db, pgPool)
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
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []MatrixRow `json:"items"`
		}
	}, error) {
		items, err := queryMatrix(ctx, db, pgPool)
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
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []CaseHealth `json:"items"`
		}
	}, error) {
		items, err := queryHealth(ctx, db, pgPool)
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

func queryTrends(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool) ([]TrendPoint, error) {
	items := make([]TrendPoint, 0)
	if pgPool != nil {
		rows, err := pgPool.Query(ctx, `SELECT TO_CHAR(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS d, status, COUNT(*) FROM test_executions GROUP BY d, status ORDER BY d ASC`)
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
	rows, err := db.QueryContext(ctx, `SELECT SUBSTR(CAST(created_at AS TEXT), 1, 10) AS d, status, COUNT(*) FROM test_executions GROUP BY d, status ORDER BY d ASC`)
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

func queryMatrix(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool) ([]MatrixRow, error) {
	items := make([]MatrixRow, 0)
	q := `SELECT e.test_case_id, tc.summary, e.status, COUNT(*) FROM test_executions e JOIN test_cases tc ON tc.id = e.test_case_id GROUP BY e.test_case_id, tc.summary, e.status ORDER BY e.test_case_id ASC`
	if pgPool != nil {
		rows, err := pgPool.Query(ctx, q)
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
	rows, err := db.QueryContext(ctx, q)
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

func queryHealth(ctx context.Context, db *sql.DB, pgPool *pgxpool.Pool) ([]CaseHealth, error) {
	items := make([]CaseHealth, 0)
	q := `
		SELECT e.test_case_id, tc.summary,
		       COUNT(*) AS total,
		       SUM(CASE WHEN e.status = 'failed' THEN 1 ELSE 0 END) AS failed
		FROM test_executions e
		JOIN test_cases tc ON tc.id = e.test_case_id
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
	if pgPool != nil {
		rows, err := pgPool.Query(ctx, q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return items, scanRows(rows)
	}
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return items, scanRows(rows)
}

func accumulateStatus(s *Summary, status string, cnt int64) {
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
