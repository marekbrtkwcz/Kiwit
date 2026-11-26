package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/exploratory"
	"github.com/kiwit/kiwit/internal/llm"
)

// RegisterExploratoryRoutes mounts AI helpers for exploratory sessions:
// charter drafting and turning raw notes into structured findings.
func RegisterExploratoryRoutes(api huma.API, servers *llm.ServerStore, exStore *exploratory.Store, guard *auth.AccessGuard) {
	// POST /api/v1/ai/generate-charter — objective → session-based test charter.
	huma.Register(api, huma.Operation{
		OperationID: "ai-generate-charter",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/generate-charter",
		Summary:     "LLM drafts a session-based exploratory test charter from an objective",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Objective string `json:"objective"`
			ServerID  int64  `json:"server_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			Charter string `json:"charter"`
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
		objective := strings.TrimSpace(input.Body.Objective)
		if objective == "" {
			return nil, huma.Error400BadRequest("objective required")
		}
		sys := `You are an exploratory testing coach writing a session-based test charter (SBTM). Use the classic pattern: "Explore <area> with <resources/tools> to discover <information/risks>". Follow with 3-5 bullet points of what to cover (heuristics, edge cases, failure modes worth probing). Plain text, no markdown headers, max 120 words. Answer in the user's language.`
		out, err := cfg.Chat(ctx, sys, objective)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		return &struct {
			Body struct {
				Charter string `json:"charter"`
				Model   string `json:"model"`
			}
		}{Body: struct {
			Charter string `json:"charter"`
			Model   string `json:"model"`
		}{Charter: strings.TrimSpace(out), Model: cfg.Model}}, nil
	})

	// POST /api/v1/ai/notes-to-findings — raw session notes → structured findings.
	huma.Register(api, huma.Operation{
		OperationID: "ai-notes-to-findings",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/notes-to-findings",
		Summary:     "LLM turns exploratory session notes into a structured findings list",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			SessionID int64 `json:"session_id"`
			ServerID  int64 `json:"server_id,omitempty"`
		}
	}) (*struct {
		Body struct {
			Findings []string `json:"findings"`
			Model    string   `json:"model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		s, err := exStore.Get(ctx, input.Body.SessionID)
		if err != nil {
			return nil, huma.Error404NotFound("session not found")
		}
		if err := guard.EnforceProduct(ctx, s.ProductID, false); err != nil {
			return nil, err
		}
		if strings.TrimSpace(s.Notes) == "" {
			return nil, huma.Error400BadRequest("session has no notes to analyze")
		}
		sys := `You are a QA analyst reviewing notes from an exploratory testing session. Extract concrete findings: bugs (prefix "bug:"), risks ("risk:"), open questions ("question:"), and coverage notes ("note:"). Each finding is one short sentence in the user's language. Drop noise and duplicates. Respond ONLY with a JSON object: {"findings": ["...", "..."]}. If nothing useful can be extracted, return an empty array.`
		user := fmt.Sprintf("Charter:\n%s\n\nSession notes:\n%s", s.Charter, s.Notes)
		raw, err := cfg.Chat(ctx, sys, user)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			Findings []string `json:"findings"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		if parsed.Findings == nil {
			parsed.Findings = []string{}
		}
		if len(parsed.Findings) > 30 {
			parsed.Findings = parsed.Findings[:30]
		}
		return &struct {
			Body struct {
				Findings []string `json:"findings"`
				Model    string   `json:"model"`
			}
		}{Body: struct {
			Findings []string `json:"findings"`
			Model    string   `json:"model"`
		}{Findings: parsed.Findings, Model: cfg.Model}}, nil
	})
}
