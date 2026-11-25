package telemetry

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
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

func RegisterRoutes(api huma.API, db *sql.DB) {
	huma.Register(api, huma.Operation{
		OperationID: "telemetry-summary",
		Method:      http.MethodGet,
		Path:        "/api/v1/telemetry/summary",
		Summary:     "Get TCMS telemetry summary and Pass/Fail breakdown",
		Tags:        []string{"Telemetry"},
	}, func(ctx context.Context, input *struct{}) (*struct{ Body Summary }, error) {
		var s Summary
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

		return &struct{ Body Summary }{Body: s}, nil
	})
}
