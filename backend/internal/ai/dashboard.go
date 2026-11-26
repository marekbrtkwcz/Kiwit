package ai

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/telemetry"
)

type dashboardTldrOutput struct {
	Body struct {
		Summary string `json:"summary"`
		Model   string `json:"model"`
	}
}

// RegisterDashboardRoutes mounts POST /api/v1/ai/dashboard-tldr — the LLM
// writes a short daily TL;DR of the execution dashboard: overall health,
// biggest risk, workload and what to focus on today.
func RegisterDashboardRoutes(api huma.API, servers *llm.ServerStore, db *sql.DB, pg *pgxpool.Pool, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "ai-dashboard-tldr",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/dashboard-tldr",
		Summary:     "LLM writes a short daily TL;DR of the test execution dashboard",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			ProductID int64  `json:"product_id,omitempty"`
			From      string `json:"from,omitempty" doc:"Start date inclusive (YYYY-MM-DD)"`
			To        string `json:"to,omitempty" doc:"End date inclusive (YYYY-MM-DD)"`
			ServerID  int64  `json:"server_id,omitempty"`
		}
	}) (*dashboardTldrOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		if input.Body.ProductID != 0 {
			if err := guard.EnforceProduct(ctx, input.Body.ProductID, false); err != nil {
				return nil, err
			}
		}
		f := telemetry.Filter{
			ProductID: input.Body.ProductID,
			From:      strings.TrimSpace(input.Body.From),
			To:        strings.TrimSpace(input.Body.To),
			UserID:    auth.FilterUserID(ctx),
		}
		data, err := telemetry.QueryTLDRData(ctx, db, pg, f)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to query telemetry", err)
		}
		s := data.Summary
		var b strings.Builder
		fmt.Fprintf(&b, "Inventory: %d products, %d test cases, %d test plans, %d test runs.\n", s.Products, s.TestCases, s.TestPlans, s.TestRuns)
		fmt.Fprintf(&b, "Executions: %d total — %d passed, %d failed, %d blocked, %d skipped, %d idle.\n",
			s.Executions.Total, s.Executions.Passed, s.Executions.Failed, s.Executions.Blocked, s.Executions.Skipped, s.Executions.Idle)
		fmt.Fprintf(&b, "Automation split: %d automated, %d manual cases.\n", data.Breakdown.Automated, data.Breakdown.Manual)
		if len(data.Exec.Runs) > 0 {
			b.WriteString("Test runs:\n")
			for _, r := range data.Exec.Runs {
				state := "running"
				if r.Stopped {
					state = "stopped"
				}
				fmt.Fprintf(&b, "- #%d %s (%s, %d%% complete, %d passed / %d failed / %d blocked / %d idle)\n",
					r.TestRunID, r.Summary, state, int(r.Progress*100), r.Passed, r.Failed, r.Blocked, r.Idle)
			}
		}
		if len(data.Exec.Testers) > 0 {
			b.WriteString("Tester workload (total executions / pass rate):\n")
			for _, w := range data.Exec.Testers {
				fmt.Fprintf(&b, "- %s: %d executions, %d%% pass rate\n", w.Username, w.Total, int(w.PassRate*100))
			}
		}
		if len(data.Health) > 0 {
			b.WriteString("Least healthy test cases (failed/total, fail rate):\n")
			for i, h := range data.Health {
				if i >= 5 {
					break
				}
				fmt.Fprintf(&b, "- #%d %s: %d/%d failed (%d%%)\n", h.TestCaseID, h.Summary, h.Failed, h.Total, int(h.FailRate*100))
			}
		}
		sys := `You are a QA lead writing the daily TL;DR of the team's test status dashboard. 2-4 sentences, plain text, no markdown, no lists. Cover: overall execution health (use the pass rate), the biggest risk (failures, blocked work, unhealthy cases, stalled runs), workload distribution, and the single thing the team should focus on today. Be concrete and use the given numbers; do not invent data. Answer in the user's language (the request is usually Polish).`
		out, err := cfg.Chat(ctx, sys, b.String())
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return &dashboardTldrOutput{Body: struct {
			Summary string `json:"summary"`
			Model   string `json:"model"`
		}{Summary: strings.TrimSpace(out), Model: cfg.Model}}, nil
	})
}
