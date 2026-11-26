package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/bug"
	"github.com/kiwit/kiwit/internal/llm"
)

// maxTriageCandidates caps how many existing bugs are fed to the LLM.
const maxTriageCandidates = 60

type duplicateEntry struct {
	BugID      int64  `json:"bug_id"`
	Confidence int    `json:"confidence"`
	Reason     string `json:"reason"`
}

type triageOutput struct {
	Body struct {
		Duplicates        []duplicateEntry `json:"duplicates"`
		SuggestedSeverity string           `json:"suggested_severity" enum:"low,medium,high,critical"`
		SuggestedStatus   string           `json:"suggested_status" enum:"open,in_progress,resolved,closed"`
		TriageNote        string           `json:"triage_note"`
		Model             string           `json:"model"`
	}
}

// RegisterBugRoutes mounts POST /api/v1/ai/triage-bug — the LLM compares a
// bug against other bugs in the same product and proposes duplicates and a
// corrected severity, cutting manual triage work.
func RegisterBugRoutes(api huma.API, servers *llm.ServerStore, bugStore *bug.Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "ai-triage-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/triage-bug",
		Summary:     "LLM triages a bug: possible duplicates in the same product and a severity/status proposal",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			BugID    int64 `json:"bug_id"`
			ServerID int64 `json:"server_id,omitempty"`
		}
	}) (*triageOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		b, err := bugStore.GetByID(ctx, input.Body.BugID)
		if err != nil {
			return nil, huma.Error404NotFound("bug not found")
		}
		if err := guard.EnforceProduct(ctx, b.ProductID, false); err != nil {
			return nil, err
		}
		all, err := bugStore.List(ctx, bug.Filter{ProductID: b.ProductID})
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list candidate bugs", err)
		}
		candidates := make([]bug.InternalBug, 0, maxTriageCandidates)
		for _, c := range all {
			if c.ID == b.ID || c.Status == "closed" || c.Status == "resolved" {
				continue
			}
			candidates = append(candidates, c)
			if len(candidates) >= maxTriageCandidates {
				break
			}
		}
		trunc := func(s string, n int) string {
			s = strings.TrimSpace(s)
			if len(s) > n {
				return s[:n] + "…"
			}
			return s
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Bug #%d to triage\nSummary: %s\nDescription: %s\nCurrent severity: %s, status: %s\n\nOpen bugs in the same product:\n",
			b.ID, b.Summary, trunc(b.Description, 1500), b.Severity, b.Status)
		if len(candidates) == 0 {
			sb.WriteString("(none)\n")
		}
		for _, c := range candidates {
			fmt.Fprintf(&sb, "- #%d [%s | %s] %s — %s\n", c.ID, c.Status, c.Severity, c.Summary, trunc(c.Description, 200))
		}
		sys := `You are a QA triager. Given one bug and a list of other open bugs in the same product, decide: (1) which existing bugs describe the SAME underlying defect (duplicates) — compare symptoms, not wording; (2) the correct severity of the target bug; (3) a one-sentence triage note for the team. Respond ONLY with JSON: {"duplicates": [{"bug_id": <only IDs from the list, not the target's ID>, "confidence": 0-100, "reason": "why it is the same defect"}], "suggested_severity": "low"|"medium"|"high"|"critical", "suggested_status": "open"|"in_progress", "triage_note": "..."}. If there are no duplicates, return an empty array. Answer in the bug's language.`
		raw, err := cfg.Chat(ctx, sys, sb.String())
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			Duplicates        []duplicateEntry `json:"duplicates"`
			SuggestedSeverity string           `json:"suggested_severity"`
			SuggestedStatus   string           `json:"suggested_status"`
			TriageNote        string           `json:"triage_note"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		// validate: only real candidate IDs, no self-duplicates
		valid := map[int64]bool{}
		for _, c := range candidates {
			valid[c.ID] = true
		}
		kept := []duplicateEntry{}
		for _, d := range parsed.Duplicates {
			if !valid[d.BugID] {
				continue
			}
			if d.Confidence < 0 {
				d.Confidence = 0
			}
			if d.Confidence > 100 {
				d.Confidence = 100
			}
			kept = append(kept, d)
		}
		switch strings.ToLower(strings.TrimSpace(parsed.SuggestedSeverity)) {
		case "low", "medium", "high", "critical":
		default:
			parsed.SuggestedSeverity = b.Severity
		}
		switch strings.ToLower(strings.TrimSpace(parsed.SuggestedStatus)) {
		case "open", "in_progress":
		default:
			parsed.SuggestedStatus = b.Status
		}
		out := &triageOutput{}
		out.Body.Duplicates = kept
		out.Body.SuggestedSeverity = strings.ToLower(parsed.SuggestedSeverity)
		out.Body.SuggestedStatus = strings.ToLower(parsed.SuggestedStatus)
		out.Body.TriageNote = parsed.TriageNote
		out.Body.Model = cfg.Model
		return out, nil
	})
}
