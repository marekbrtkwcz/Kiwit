package testcase

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/export"
)

const exportLimit = 10000

// RegisterExportRoutes mounts unified export endpoints for test cases:
//
//	GET  /api/v1/testcases/export          - filtered list (csv|json|md)
//	GET  /api/v1/testcases/{id}/export     - single case as a Markdown document (or table row)
//	POST /api/v1/testcases/export          - selected IDs ({ids, format})
func RegisterExportRoutes(r chi.Router, repo Repository, guard *auth.AccessGuard) {
	r.Get("/api/v1/testcases/export", func(w http.ResponseWriter, req *http.Request) {
		auto := req.URL.Query().Get("is_automated")
		var isAuto *bool
		if auto == "true" || auto == "false" {
			v := auto == "true"
			isAuto = &v
		}
		items, _, err := repo.ListWithFilter(req.Context(), ListFilter{
			Q:           req.URL.Query().Get("q"),
			ProductID:   export.QueryInt64(req, "product_id"),
			CategoryID:  export.QueryInt64(req, "category_id"),
			ComponentID: export.QueryInt64(req, "component_id"),
			TagID:       export.QueryInt64(req, "tag_id"),
			Priority:    req.URL.Query().Get("priority"),
			Status:      req.URL.Query().Get("status"),
			IsAutomated: isAuto,
			AuthorID:    export.QueryInt64(req, "author_id"),
			UserID:      auth.FilterUserID(req.Context()),
			Limit:       exportLimit,
		})
		if err != nil {
			http.Error(w, `{"detail":"failed to list test cases"}`, http.StatusInternalServerError)
			return
		}
		format := export.Format(req.URL.Query().Get("format"))
		switch format {
		case "md":
			renderCasesMarkdown(w, "testcases", items)
		case "feature":
			renderCasesFeature(w, "testcases", items)
		default:
			export.Render(w, format, "testcases", caseTable(items))
		}
	})

	r.Get("/api/v1/testcases/{id}/export", func(w http.ResponseWriter, req *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, `{"detail":"invalid test case id"}`, http.StatusBadRequest)
			return
		}
		if auth.WriteAccessError(w, guard.EnforceTestCase(req.Context(), id, false)) {
			return
		}
		tc, err := repo.GetByID(req.Context(), id)
		if err != nil {
			http.Error(w, `{"detail":"test case not found"}`, http.StatusNotFound)
			return
		}
		format := export.Format(req.URL.Query().Get("format"))
		switch format {
		case "md":
			renderCasesMarkdown(w, fmt.Sprintf("testcase-%d", id), []TestCase{tc})
		case "feature":
			renderCasesFeature(w, fmt.Sprintf("testcase-%d", id), []TestCase{tc})
		default:
			export.Render(w, format, fmt.Sprintf("testcase-%d", id), caseTable([]TestCase{tc}))
		}
	})

	r.Post("/api/v1/testcases/export", func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			IDs    []int64 `json:"ids"`
			Format string  `json:"format"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil || len(body.IDs) == 0 {
			http.Error(w, `{"detail":"ids required"}`, http.StatusBadRequest)
			return
		}
		if len(body.IDs) > exportLimit {
			http.Error(w, `{"detail":"too many ids"}`, http.StatusBadRequest)
			return
		}
		items := make([]TestCase, 0, len(body.IDs))
		for _, id := range body.IDs {
			if auth.WriteAccessError(w, guard.EnforceTestCase(req.Context(), id, false)) {
				return
			}
			tc, err := repo.GetByID(req.Context(), id)
			if err != nil {
				continue // skip deleted cases, keep the rest exportable
			}
			items = append(items, tc)
		}
		format := export.Format(body.Format)
		switch format {
		case "md":
			renderCasesMarkdown(w, "testcases", items)
		case "feature":
			renderCasesFeature(w, "testcases", items)
		default:
			export.Render(w, format, "testcases", caseTable(items))
		}
	})
}


func caseTable(items []TestCase) export.Table {
	t := export.Table{Columns: []string{
		"id", "product_id", "category_id", "summary", "format", "priority",
		"status", "is_automated", "steps", "created_at", "updated_at",
	}}
	for _, tc := range items {
		t.Rows = append(t.Rows, []string{
			export.I64(tc.ID),
			export.I64(tc.ProductID),
			export.OptI64(tc.CategoryID),
			tc.Summary,
			tc.Format,
			tc.Priority,
			tc.Status,
			strconv.FormatBool(tc.IsAutomated),
			strconv.Itoa(len(tc.Steps)),
			export.Time(tc.CreatedAt),
			export.Time(tc.UpdatedAt),
		})
	}
	return t
}

// renderCasesFeature writes a .feature file with the Gherkin bodies of the
// given cases (non-Gherkin cases are skipped). Cucumber-style tools accept
// several Feature blocks in one file.
func renderCasesFeature(w http.ResponseWriter, name string, items []TestCase) {
	var b strings.Builder
	for _, tc := range items {
		if tc.Format != "gherkin" || strings.TrimSpace(tc.GherkinBody) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.TrimSpace(tc.GherkinBody))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		http.Error(w, `{"detail":"no gherkin test cases in selection"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.feature"`, name))
	_, _ = w.Write([]byte(b.String()))
}

// renderCasesMarkdown writes full Markdown documents for the given cases,
// separated by horizontal rules so several cases can live in one file.
func renderCasesMarkdown(w http.ResponseWriter, name string, items []TestCase) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, name))
	for i, tc := range items {
		if i > 0 {
			fmt.Fprint(w, "\n---\n\n")
		}
		fmt.Fprintf(w, "# %s\n\n", tc.Summary)
		fmt.Fprintf(w, "- **ID:** %d\n", tc.ID)
		fmt.Fprintf(w, "- **Priority:** %s\n", tc.Priority)
		fmt.Fprintf(w, "- **Status:** %s\n", tc.Status)
		fmt.Fprintf(w, "- **Format:** %s\n", tc.Format)
		fmt.Fprintf(w, "- **Automated:** %t\n", tc.IsAutomated)
		fmt.Fprintf(w, "- **Created:** %s\n\n", export.Time(tc.CreatedAt))
		if tc.Preconditions != "" {
			fmt.Fprintf(w, "## Preconditions\n\n%s\n\n", tc.Preconditions)
		}
		switch tc.Format {
		case "gherkin":
			fmt.Fprintf(w, "```gherkin\n%s\n```\n\n", strings.TrimSpace(tc.GherkinBody))
		case "markdown":
			fmt.Fprintf(w, "%s\n\n", strings.TrimSpace(tc.GherkinBody))
		default:
			if len(tc.Steps) > 0 {
				fmt.Fprint(w, "## Steps\n\n")
				for _, s := range tc.Steps {
					fmt.Fprintf(w, "%d. %s\n", s.Position, s.Action)
					if s.ExpectedResult != "" {
						fmt.Fprintf(w, "   - **Expected:** %s\n", s.ExpectedResult)
					}
				}
				fmt.Fprint(w, "\n")
			}
		}
		if tc.Notes != "" {
			fmt.Fprintf(w, "## Notes\n\n%s\n\n", tc.Notes)
		}
	}
}
