package testcase

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderCasesFeature(t *testing.T) {
	items := []TestCase{
		{ID: 1, Format: "gherkin", GherkinBody: "Feature: Login\n  Scenario: Successful login"},
		{ID: 2, Format: "steps", GherkinBody: "ignored"}, // non-gherkin: skipped
		{ID: 3, Format: "gherkin", GherkinBody: "   "},   // empty body: skipped
		{ID: 4, Format: "gherkin", GherkinBody: "Feature: Logout"},
	}
	rec := httptest.NewRecorder()
	renderCasesFeature(rec, "testcases", items)
	body := rec.Body.String()
	if !strings.HasPrefix(body, "Feature: Login\n") {
		t.Fatalf("unexpected start: %q", body)
	}
	if !strings.Contains(body, "\n\nFeature: Logout\n") {
		t.Fatalf("features not separated: %q", body)
	}
	if strings.Contains(body, "ignored") {
		t.Fatal("non-gherkin case leaked into feature file")
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="testcases.feature"` {
		t.Fatalf("unexpected disposition: %s", cd)
	}
}

func TestRenderCasesFeatureEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	renderCasesFeature(rec, "testcases", []TestCase{{ID: 1, Format: "steps"}})
	if rec.Code != 404 {
		t.Fatalf("expected 404 for no gherkin cases, got %d", rec.Code)
	}
}
