package ai

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/report"
)

// RegisterReportRoutes mounts POST /api/v1/ai/report-summary — an LLM-written
// executive summary of the multi-milestone release report, aimed at release
// management rather than testers.
func RegisterReportRoutes(api huma.API, servers *llm.ServerStore, repStore *report.Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "ai-report-summary",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/report-summary",
		Summary:     "LLM-written executive summary of a multi-milestone release report",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			MilestoneIDs []int64 `json:"milestone_ids"`
			ServerID     int64   `json:"server_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			Summary string `json:"summary"`
			Model   string `json:"model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		ids := input.Body.MilestoneIDs
		if len(ids) == 0 {
			return nil, huma.Error400BadRequest("milestone_ids required")
		}
		if len(ids) > 20 {
			ids = ids[:20]
		}
		for _, id := range ids {
			if err := guard.EnforceMilestone(ctx, id, false); err != nil {
				return nil, err
			}
		}
		rep, err := repStore.Build(ctx, ids)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to build report", err)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Release report over %d milestone(s). Totals: %d executions, %d passed, %d failed, %d blocked, %d skipped, %d open bugs.\n\n",
			len(rep.Milestones), rep.Totals.Total, rep.Totals.Passed, rep.Totals.Failed, rep.Totals.Blocked, rep.Totals.Skipped, rep.Totals.OpenBugs)
		for _, m := range rep.Milestones {
			signoff := "none"
			if m.SignoffDecision != nil && *m.SignoffDecision != "" {
				signoff = *m.SignoffDecision
			}
			due := "no due date"
			if m.DueDate != nil {
				due = *m.DueDate
			}
			fmt.Fprintf(&b, "- %s (%s, product %s, status %s, due %s, signoff %s): %d executions — %d passed, %d failed, %d blocked, %d skipped; %d open bugs.\n",
				m.Name, m.Version, m.ProductName, m.Status, due, signoff, m.Total, m.Passed, m.Failed, m.Blocked, m.Skipped, m.OpenBugs)
		}
		sys := `You are a release manager writing the executive summary of a test report for stakeholders. 3-6 sentences, plain text, no markdown. Cover: overall pass rate and whether the release is on track, milestones at risk (high failure or blocked counts, missing signoff), open bug exposure, and a clear go / no-go / conditional-go recommendation with the main condition. Be concrete and use the numbers given; do not invent data. Answer in the user's language (the request is usually Polish).`
		out, err := cfg.Chat(ctx, sys, b.String())
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return &struct {
			Body struct {
				Summary string `json:"summary"`
				Model   string `json:"model"`
			}
		}{Body: struct {
			Summary string `json:"summary"`
			Model   string `json:"model"`
		}{Summary: strings.TrimSpace(out), Model: cfg.Model}}, nil
	})
}
