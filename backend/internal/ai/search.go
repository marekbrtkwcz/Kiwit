package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/xmlrpc"
)

// fieldDocs documents the filterable models and their fields for the LLM.
var fieldDocs = map[string][]string{
	"test_case":   {"id", "summary", "category_id", "is_automated", "priority (low|medium|high|critical)", "status (draft|ready|review|deprecated)", "product_id", "author__username", "product__name", "category__name"},
	"test_run":    {"id", "summary", "test_plan_id", "build_id", "stopped_at", "manager__username", "plan__name", "build__name"},
	"test_plan":   {"id", "name", "product_id", "version_id", "is_active", "author__username", "product__name", "version__value"},
	"product":     {"id", "name", "description", "classification__name"},
	"build":       {"id", "name", "version_id", "version__value"},
	"version":     {"id", "value", "product_id", "product__name"},
	"component":   {"id", "name", "product_id", "product__name", "description"},
	"tag":         {"id", "name"},
	"environment": {"id", "name", "description"},
	"user":        {"id", "username", "email", "role", "is_active"},
}

// searchPrompt builds the system prompt describing models, fields and lookup
// operators the LLM may emit.
func searchPrompt() string {
	var b strings.Builder
	b.WriteString("You translate a natural-language search request into a structured query for a test-management system. ")
	b.WriteString("Respond ONLY with a JSON object: {\"model\": \"<one of the models below>\", \"filters\": {<field-lookup: value>}, \"explanation\": \"<one short sentence in the user's language>\"}.\n")
	b.WriteString("Lookup operators (Django style, appended to field names with double underscore): __exact, __startswith, __endswith, __contains, __icontains, __gt, __gte, __lt, __lte, __in (array value), __isnull (bool), __range (2-element array). Omit __exact for plain equality.\n")
	b.WriteString("Dates use ISO 8601 (YYYY-MM-DD). If the request mentions a date range use __range or __gte/__lte on the date field. If no filter is derivable, return an empty filters object. Never invent fields.\n")
	b.WriteString("Models and their filterable fields:\n")
	keys := make([]string, 0, len(fieldDocs))
	for k := range fieldDocs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "- %s: %s\n", k, strings.Join(fieldDocs[k], ", "))
	}
	return b.String()
}

type searchPayload struct {
	Query    string `json:"query"`
	ServerID int64  `json:"server_id,omitempty"`
}

type searchResultItem struct {
	Type  string         `json:"type"`
	Title string         `json:"title"`
	Row   map[string]any `json:"row"`
}

// RegisterSearchRoutes mounts POST /api/v1/ai/search — natural-language
// search translated by an LLM into field lookups and executed safely.
func RegisterSearchRoutes(api huma.API, servers *llm.ServerStore, db *sql.DB, pg *pgxpool.Pool) {
	huma.Register(api, huma.Operation{
		OperationID: "ai-search",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/search",
		Summary:     "Natural-language search: an LLM translates the query into field lookups and executes them",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body searchPayload
	}) (*struct {
		Body struct {
			Model       string             `json:"model"`
			Filters     map[string]any     `json:"filters"`
			Explanation string             `json:"explanation"`
			Items       []searchResultItem `json:"items"`
			LLMModel    string             `json:"llm_model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		q := strings.TrimSpace(input.Body.Query)
		if q == "" {
			return nil, huma.Error400BadRequest("query required")
		}
		if len(q) > 500 {
			q = q[:500]
		}
		raw, err := cfg.Chat(ctx, searchPrompt(), q)
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			Model       string         `json:"model"`
			Filters     map[string]any `json:"filters"`
			Explanation string         `json:"explanation"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		if _, ok := xmlrpc.FilterModels[parsed.Model]; !ok {
			// model not whitelisted — fall back to test_case with plain contains
			parsed.Model = "test_case"
			if len(parsed.Filters) == 0 {
				parsed.Filters = map[string]any{}
			}
		}
		items := []searchResultItem{}
		rows, err := xmlrpc.FilterModel(ctx, db, pg, parsed.Model, parsed.Filters, 50)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to execute search", err)
		}
		for _, r := range rows {
			title := searchTitle(parsed.Model, r)
			items = append(items, searchResultItem{Type: parsed.Model, Title: title, Row: r})
		}
		out := &struct {
			Body struct {
				Model       string             `json:"model"`
				Filters     map[string]any     `json:"filters"`
				Explanation string             `json:"explanation"`
				Items       []searchResultItem `json:"items"`
				LLMModel    string             `json:"llm_model"`
			}
		}{}
		out.Body.Model = parsed.Model
		out.Body.Filters = parsed.Filters
		out.Body.Explanation = parsed.Explanation
		out.Body.Items = items
		out.Body.LLMModel = cfg.Model
		return out, nil
	})
}

// searchTitle picks the most human-readable column for a hit.
func searchTitle(model string, row map[string]any) string {
	preferred := []string{"summary", "name", "value", "username", "title"}
	for _, k := range preferred {
		if v, ok := row[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return fmt.Sprintf("#%v", row["id"])
}
