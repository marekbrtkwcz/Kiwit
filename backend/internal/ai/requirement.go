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
	"github.com/kiwit/kiwit/internal/requirement"
)

// RetestEntry is one test case the LLM recommends retesting.
type RetestEntry struct {
	CaseID   int64  `json:"case_id"`
	Summary  string `json:"summary"`
	Reason   string `json:"reason"`
	Priority string `json:"priority" enum:"low,medium,high"`
}

type regressionOutput struct {
	Body struct {
		Summary string        `json:"summary"`
		Retest  []RetestEntry `json:"retest"`
		Risks   []string      `json:"risks"`
		Gaps    []string      `json:"gaps"`
		Model   string        `json:"model"`
	}
}

// RegisterRequirementRoutes mounts AI helpers for requirements: quality
// analysis (is this requirement testable?) and regression scoping (what to
// retest when it changes).
func RegisterRequirementRoutes(api huma.API, servers *llm.ServerStore, reqStore *requirement.Store, guard *auth.AccessGuard) {
	// POST /api/v1/ai/analyze-requirement — quality critique of a requirement.
	huma.Register(api, huma.Operation{
		OperationID: "ai-analyze-requirement",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/analyze-requirement",
		Summary:     "LLM critiques a requirement: clarity, testability, missing acceptance criteria",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			RequirementID int64 `json:"requirement_id"`
			ServerID      int64 `json:"server_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			QualityScore int    `json:"quality_score"`
			Verdict      string `json:"verdict" enum:"good,needs-work,poor"`
			Summary      string `json:"summary"`
			Issues       []struct {
				Type        string `json:"type"`
				Severity    string `json:"severity"`
				Description string `json:"description"`
			} `json:"issues"`
			Suggestions []string `json:"suggestions"`
			Model       string   `json:"model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		if err := guard.EnforceRequirement(ctx, input.Body.RequirementID, false); err != nil {
			return nil, err
		}
		r, err := reqStore.Get(ctx, input.Body.RequirementID)
		if err != nil {
			return nil, huma.Error404NotFound("requirement not found")
		}
		sys := `You are a requirements engineer doing a quality gate review. Analyze the requirement for: clarity (unambiguous language), testability (can a tester verify it), completeness (missing acceptance criteria, edge cases, error handling), scope (too large or compound), and consistency. Respond ONLY with JSON: {"quality_score": 0-100, "verdict": "good"|"needs-work"|"poor", "summary": "one sentence", "issues": [{"type": "clarity"|"testability"|"completeness"|"scope"|"consistency", "severity": "low"|"medium"|"high", "description": "..."}], "suggestions": ["concrete improvement", "..."]}. Answer in the requirement's language. Be specific, quote the problematic fragment when useful.`
		user := fmt.Sprintf("Requirement %s (%s), priority %s, version %s:\nName: %s\nDescription:\n%s", r.Key, r.Status, r.Priority, r.Version, r.Name, r.Description)
		raw, err := cfg.Chat(ctx, sys, user)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			QualityScore int    `json:"quality_score"`
			Verdict      string `json:"verdict"`
			Summary      string `json:"summary"`
			Issues       []struct {
				Type        string `json:"type"`
				Severity    string `json:"severity"`
				Description string `json:"description"`
			} `json:"issues"`
			Suggestions []string `json:"suggestions"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		parsed.Verdict = strings.ToLower(strings.TrimSpace(parsed.Verdict))
		switch parsed.Verdict {
		case "good", "needs-work", "poor":
		default:
			parsed.Verdict = "needs-work"
		}
		if parsed.QualityScore < 0 {
			parsed.QualityScore = 0
		}
		if parsed.QualityScore > 100 {
			parsed.QualityScore = 100
		}
		if parsed.Issues == nil {
			parsed.Issues = []struct {
				Type        string `json:"type"`
				Severity    string `json:"severity"`
				Description string `json:"description"`
			}{}
		}
		if parsed.Suggestions == nil {
			parsed.Suggestions = []string{}
		}
		stored, _ := json.Marshal(parsed)
		if err := reqStore.UpdateAIQuality(ctx, r.ID, string(stored)); err != nil {
			return nil, huma.Error500InternalServerError("failed to persist analysis", err)
		}
		return &struct {
			Body struct {
				QualityScore int    `json:"quality_score"`
				Verdict      string `json:"verdict" enum:"good,needs-work,poor"`
				Summary      string `json:"summary"`
				Issues       []struct {
					Type        string `json:"type"`
					Severity    string `json:"severity"`
					Description string `json:"description"`
				} `json:"issues"`
				Suggestions []string `json:"suggestions"`
				Model       string   `json:"model"`
			}
		}{Body: struct {
			QualityScore int    `json:"quality_score"`
			Verdict      string `json:"verdict" enum:"good,needs-work,poor"`
			Summary      string `json:"summary"`
			Issues       []struct {
				Type        string `json:"type"`
				Severity    string `json:"severity"`
				Description string `json:"description"`
			} `json:"issues"`
			Suggestions []string `json:"suggestions"`
			Model       string   `json:"model"`
		}{QualityScore: parsed.QualityScore, Verdict: parsed.Verdict, Summary: parsed.Summary, Issues: parsed.Issues, Suggestions: parsed.Suggestions, Model: cfg.Model}}, nil
	})

	// POST /api/v1/ai/regression-scope — which linked cases to retest when the requirement changes.
	huma.Register(api, huma.Operation{
		OperationID: "ai-regression-scope",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/regression-scope",
		Summary:     "LLM ranks the requirement's covered test cases by regression risk when the requirement changes",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			RequirementID int64 `json:"requirement_id"`
			ServerID      int64 `json:"server_id,omitempty"`
		}
	}) (*regressionOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		if err := guard.EnforceRequirement(ctx, input.Body.RequirementID, false); err != nil {
			return nil, err
		}
		r, err := reqStore.Get(ctx, input.Body.RequirementID)
		if err != nil {
			return nil, huma.Error404NotFound("requirement not found")
		}
		coverage, err := reqStore.Coverage(ctx, input.Body.RequirementID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load coverage", err)
		}
		if len(coverage) == 0 {
			return nil, huma.Error400BadRequest("requirement has no test case coverage — link cases first")
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Requirement %s: %s\n%s\n\nLinked test cases:\n", r.Key, r.Name, r.Description)
		for _, c := range coverage {
			exec := c.LatestExecStatus
			if exec == "" {
				exec = "never-executed"
			}
			fmt.Fprintf(&b, "- #%d [%s | case status: %s | last execution: %s] %s\n", c.TestCaseID, c.Status, c.Status, exec, c.Summary)
		}
		sys := `You are a QA lead planning regression testing after a requirement changed. Given the requirement and its linked test cases, decide which cases MUST be retested (directly exercise the changed behavior), which look stale (draft/deprecated, low automation), and what coverage gaps exist (aspects of the requirement no linked case covers). Respond ONLY with JSON: {"summary": "one or two sentences", "retest": [{"case_id": <only IDs from the list>, "summary": "<case summary>", "reason": "why retest", "priority": "high"|"medium"|"low"}], "risks": ["risk of the change", "..."], "gaps": ["missing coverage", "..."]}. Order retest entries by priority (high first). Answer in the requirement's language.`
		raw, err := cfg.Chat(ctx, sys, b.String())
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			Summary string        `json:"summary"`
			Retest  []RetestEntry `json:"retest"`
			Risks   []string      `json:"risks"`
			Gaps    []string      `json:"gaps"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		// Keep only case IDs that actually belong to this requirement's coverage.
		valid := map[int64]bool{}
		for _, c := range coverage {
			valid[c.TestCaseID] = true
		}
		kept := []RetestEntry{}
		for _, rt := range parsed.Retest {
			if !valid[rt.CaseID] {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(rt.Priority)) {
			case "high", "medium", "low":
			default:
				rt.Priority = "medium"
			}
			kept = append(kept, rt)
		}
		if parsed.Risks == nil {
			parsed.Risks = []string{}
		}
		if parsed.Gaps == nil {
			parsed.Gaps = []string{}
		}
		out := &regressionOutput{}
		out.Body.Summary = parsed.Summary
		out.Body.Retest = kept
		out.Body.Risks = parsed.Risks
		out.Body.Gaps = parsed.Gaps
		out.Body.Model = cfg.Model
		return out, nil
	})
}
