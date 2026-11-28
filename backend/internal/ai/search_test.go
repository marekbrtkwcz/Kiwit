package ai

import (
	"strings"
	"testing"
)

func TestSearchPromptCoversAllModelsAndOperators(t *testing.T) {
	p := searchPrompt()
	for model := range fieldDocs {
		if !strings.Contains(p, "- "+model+":") {
			t.Errorf("prompt missing model %q", model)
		}
	}
	for _, op := range []string{"__contains", "__gte", "__in", "__isnull", "__range"} {
		if !strings.Contains(p, op) {
			t.Errorf("prompt missing operator %q", op)
		}
	}
	if !strings.Contains(p, "Respond ONLY with a JSON object") {
		t.Error("prompt should demand JSON-only output")
	}
}

func TestSearchTitle(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]any
		want string
	}{
		{"summary preferred", map[string]any{"id": 5, "summary": "Login test", "name": "x"}, "Login test"},
		{"name fallback", map[string]any{"id": 6, "name": "Plan A"}, "Plan A"},
		{"value fallback", map[string]any{"id": 7, "value": "1.0"}, "1.0"},
		{"username fallback", map[string]any{"id": 8, "username": "alice"}, "alice"},
		{"id fallback", map[string]any{"id": 9}, "#9"},
		{"empty strings skipped", map[string]any{"id": 10, "summary": "", "name": "N"}, "N"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := searchTitle("test_case", tc.row); got != tc.want {
				t.Errorf("searchTitle = %q, want %q", got, tc.want)
			}
		})
	}
}
