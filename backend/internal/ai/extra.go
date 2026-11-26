package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/testrun"
)

// RegisterExtraRoutes mounts AI helpers: run summary, bug severity suggestion,
// Gherkin → automation code.
func RegisterExtraRoutes(api huma.API, servers *llm.ServerStore, runRepo testrun.Repository, guard *auth.AccessGuard) {
	// POST /api/v1/ai/summarize-run — narrative summary of a test run.
	huma.Register(api, huma.Operation{
		OperationID: "ai-summarize-run",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/summarize-run",
		Summary:     "LLM-written narrative summary of a test run's results",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			TestRunID int64 `json:"test_run_id"`
			ServerID  int64 `json:"server_id,omitempty"`
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
		if err := guard.EnforceTestRun(ctx, input.Body.TestRunID, false); err != nil {
			return nil, err
		}
		run, err := runRepo.GetByID(ctx, input.Body.TestRunID)
		if err != nil {
			return nil, huma.Error404NotFound("test run not found")
		}
		execs, err := runRepo.ListExecutions(ctx, input.Body.TestRunID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list executions", err)
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Test run #%d: %s\n", run.ID, run.Summary))
		counts := map[string]int{}
		for _, e := range execs {
			counts[e.Status]++
			line := fmt.Sprintf("- #%d [%s] %s", e.TestCaseID, e.Status, e.Summary)
			if e.Comment != "" {
				line += " — " + e.Comment
			}
			b.WriteString(line + "\n")
		}
		b.WriteString(fmt.Sprintf("\nTotals: %d executions (%s)\n", len(execs), summarizeCounts(counts)))
		sys := "You are a QA lead writing a short release-note style summary of a test run. 3-6 sentences, plain text, no markdown headers. Mention pass rate, notable failures, and a go/no-go hint."
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
		}{Summary: out, Model: cfg.Model}}, nil
	})

	// POST /api/v1/ai/suggest-bug — severity + short title proposal for a bug report.
	huma.Register(api, huma.Operation{
		OperationID: "ai-suggest-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/suggest-bug",
		Summary:     "LLM suggests severity and a concise title for a bug report",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Summary     string `json:"summary"`
			Description string `json:"description,omitempty"`
			ServerID    int64  `json:"server_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			Severity string `json:"severity"`
			Title    string `json:"title"`
			Model    string `json:"model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Body.Summary) == "" {
			return nil, huma.Error400BadRequest("summary required")
		}
		sys := `You are a QA triager. Given a bug report, respond ONLY with a JSON object: {"severity": one of low|medium|high|critical, "title": a concise one-line title max 100 chars}. No prose.`
		user := "Bug summary: " + input.Body.Summary + "\nDescription: " + input.Body.Description
		raw, err := cfg.Chat(ctx, sys, user)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		raw = llm.ExtractJSON(raw)
		var parsed struct {
			Severity string `json:"severity"`
			Title    string `json:"title"`
		}
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		parsed.Severity = strings.ToLower(strings.TrimSpace(parsed.Severity))
		switch parsed.Severity {
		case "low", "medium", "high", "critical":
		default:
			parsed.Severity = "medium"
		}
		return &struct {
			Body struct {
				Severity string `json:"severity"`
				Title    string `json:"title"`
				Model    string `json:"model"`
			}
		}{Body: struct {
			Severity string `json:"severity"`
			Title    string `json:"title"`
			Model    string `json:"model"`
		}{Severity: parsed.Severity, Title: parsed.Title, Model: cfg.Model}}, nil
	})

	// POST /api/v1/ai/generate-steps — Gherkin → Playwright TS code.
	huma.Register(api, huma.Operation{
		OperationID: "ai-generate-steps",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/generate-steps",
		Summary:     "LLM turns a Gherkin scenario into Playwright TypeScript test code",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Gherkin  string `json:"gherkin"`
			ServerID int64  `json:"server_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			Code  string `json:"code"`
			Model string `json:"model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(input.Body.Gherkin) == "" {
			return nil, huma.Error400BadRequest("gherkin required")
		}
		sys := `You are a test automation engineer. Convert the Gherkin scenario into a single Playwright Test (TypeScript) file using @playwright/test.
Rules: output ONLY a fenced code block with language "ts". Use page.getByRole/getByLabel locators, expect() assertions, no hardcoded waits. One test() per scenario step group.`
		raw, err := cfg.Chat(ctx, sys, input.Body.Gherkin)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		code := llm.ExtractCode(raw)
		return &struct {
			Body struct {
				Code  string `json:"code"`
				Model string `json:"model"`
			}
		}{Body: struct {
			Code  string `json:"code"`
			Model string `json:"model"`
		}{Code: code, Model: cfg.Model}}, nil
	})
}

func summarizeCounts(counts map[string]int) string {
	parts := []string{}
	for _, s := range []string{"passed", "failed", "blocked", "skipped", "running", "idle"} {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", s, counts[s]))
		}
	}
	return strings.Join(parts, ", ")
}
