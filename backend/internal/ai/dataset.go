package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/customfield"
	"github.com/kiwit/kiwit/internal/dataset"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/testcase"
)

// DatasetProposal is one LLM-suggested dataset (params JSON object as string).
type DatasetProposal struct {
	Name      string `json:"name"`
	Params    string `json:"params"`
	Rationale string `json:"rationale"`
}

type datasetOutput struct {
	Body struct {
		Datasets []DatasetProposal `json:"datasets"`
		Model    string            `json:"model"`
	}
}

// RegisterDatasetRoutes mounts POST /api/v1/ai/generate-datasets — the LLM
// proposes parameter datasets (boundary values, invalid input, distinct
// equivalence classes) for a test case, ready to save as data-driven datasets.
func RegisterDatasetRoutes(api huma.API, servers *llm.ServerStore, tcRepo testcase.Repository, dsStore *dataset.Store, cfStore *customfield.Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "ai-generate-datasets",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/generate-datasets",
		Summary:     "LLM proposes data-driven datasets (boundary values, equivalence classes) for a test case",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			TestCaseID int64 `json:"test_case_id"`
			Count      int   `json:"count,omitempty" default:"5" doc:"How many datasets to propose (1-20)"`
			ServerID   int64 `json:"server_id,omitempty"`
		}
	}) (*datasetOutput, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		if err := guard.EnforceTestCase(ctx, input.Body.TestCaseID, true); err != nil {
			return nil, err
		}
		tc, err := tcRepo.GetByID(ctx, input.Body.TestCaseID)
		if err != nil {
			return nil, huma.Error404NotFound("test case not found")
		}
		count := input.Body.Count
		if count <= 0 || count > 20 {
			count = 5
		}
		existing, err := dsStore.ListForCase(ctx, tc.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load existing datasets", err)
		}
		defs, err := cfStore.ListDefs(ctx, "test_case")
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to load field definitions", err)
		}

		var b strings.Builder
		fmt.Fprintf(&b, "Test case #%d: %s\n", tc.ID, tc.Summary)
		if tc.Preconditions != "" {
			fmt.Fprintf(&b, "Preconditions: %s\n", tc.Preconditions)
		}
		switch tc.Format {
		case "gherkin":
			fmt.Fprintf(&b, "Gherkin body:\n%s\n", tc.GherkinBody)
		case "checklist", "markdown":
			if tc.Notes != "" {
				fmt.Fprintf(&b, "Notes:\n%s\n", tc.Notes)
			}
		default:
			for i, st := range tc.Steps {
				fmt.Fprintf(&b, "Step %d: %s", i+1, st.Action)
				if st.ExpectedResult != "" {
					fmt.Fprintf(&b, " → %s", st.ExpectedResult)
				}
				b.WriteString("\n")
			}
		}
		if tc.Arguments != "" {
			fmt.Fprintf(&b, "Automation arguments: %s\n", tc.Arguments)
		}
		for _, d := range defs {
			opts := d.Options
			if opts == "" || opts == "[]" {
				opts = "any value"
			}
			fmt.Fprintf(&b, "Parameter field %q (type %s, options %s)\n", d.Name, d.FieldType, opts)
		}
		if len(existing) > 0 {
			b.WriteString("Existing datasets (do NOT duplicate these):\n")
			for _, d := range existing {
				fmt.Fprintf(&b, "- %s: %s\n", d.Name, d.Params)
			}
		}

		sys := fmt.Sprintf(`You are a test data designer. Given a test case, propose %d parameter datasets for data-driven testing. Cover: one happy path, boundary values (zero, negative, max/long, empty string, unicode where relevant), invalid input, and distinct equivalence classes. If the case uses Gherkin <param> placeholders or named arguments, the params keys MUST be exactly those names; otherwise invent short snake_case keys matching the inputs the case exercises. Values must be JSON types (numbers as numbers, booleans as booleans). Respond ONLY with JSON: {"datasets": [{"name": "short human label", "params": {param: value, ...}, "rationale": "what this dataset exercises"}]}. Do not duplicate existing datasets. Answer labels and rationales in the test case's language (usually Polish).`, count)
		raw, err := cfg.Chat(ctx, sys, b.String())
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			Datasets []struct {
				Name      string         `json:"name"`
				Params    map[string]any `json:"params"`
				Rationale string         `json:"rationale"`
			} `json:"datasets"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		out := &datasetOutput{Body: struct {
			Datasets []DatasetProposal `json:"datasets"`
			Model    string            `json:"model"`
		}{Datasets: []DatasetProposal{}}}
		for _, d := range parsed.Datasets {
			name := strings.TrimSpace(d.Name)
			if name == "" || d.Params == nil {
				continue
			}
			if len(name) > 120 {
				name = name[:120]
			}
			pb, err := json.Marshal(d.Params)
			if err != nil {
				continue
			}
			out.Body.Datasets = append(out.Body.Datasets, DatasetProposal{Name: name, Params: string(pb), Rationale: strings.TrimSpace(d.Rationale)})
			if len(out.Body.Datasets) >= count {
				break
			}
		}
		if len(out.Body.Datasets) == 0 {
			return nil, huma.Error502BadGateway("LLM returned no usable datasets")
		}
		out.Body.Model = cfg.Model
		return out, nil
	})
}
