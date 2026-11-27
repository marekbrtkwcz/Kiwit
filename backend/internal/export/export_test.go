package export

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRenderCSV(t *testing.T) {
	rec := httptest.NewRecorder()
	Render(rec, "csv", "cases", Table{
		Columns: []string{"id", "summary"},
		Rows:    [][]string{{"1", "Login, with;pipe|and \"quote\""}, {"2", "line\nbreak"}},
	})
	body := rec.Body.String()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatalf("expected UTF-8 BOM, got %q", body[:10])
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="cases.csv"` {
		t.Fatalf("unexpected disposition: %s", cd)
	}
	if !strings.Contains(body, "\"Login, with;pipe|and \"\"quote\"\"\"") {
		t.Fatalf("CSV quoting broken: %s", body)
	}
}

func TestRenderJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	Render(rec, "json", "cases", Table{
		Columns: []string{"id", "summary"},
		Rows:    [][]string{{"1", "a"}, {"2", "b"}},
	})
	body := rec.Body.String()
	if !strings.Contains(body, `"summary": "a"`) || !strings.Contains(body, `"summary": "b"`) {
		t.Fatalf("unexpected JSON: %s", body)
	}
	if !strings.HasPrefix(strings.TrimSpace(body), "[") {
		t.Fatalf("expected JSON array: %s", body)
	}
}

func TestRenderMarkdown(t *testing.T) {
	rec := httptest.NewRecorder()
	Render(rec, "markdown", "cases", Table{
		Columns: []string{"id", "summary"},
		Rows:    [][]string{{"1", "pipe|and\nnewline"}, {"2", ""}},
	})
	body := rec.Body.String()
	if !strings.Contains(body, `pipe\|and<br>newline`) {
		t.Fatalf("markdown escaping broken: %s", body)
	}
	if !strings.Contains(body, "| --- | --- |") {
		t.Fatalf("missing separator row: %s", body)
	}
}

func TestRenderEmptyTable(t *testing.T) {
	rec := httptest.NewRecorder()
	Render(rec, "csv", "cases", Table{Columns: []string{"id"}})
	if !strings.Contains(rec.Body.String(), "id") {
		t.Fatal("header row missing for empty table")
	}
}

func TestJUnitStatus(t *testing.T) {
	cases := map[string]string{
		"passed": "passed", "pass": "passed", "failed": "failed", "FAIL": "failed",
		"blocked": "error", "error": "error", "skipped": "skipped",
		"idle": "skipped", "unknown": "skipped",
	}
	for in, want := range cases {
		if got := JUnitStatus(in); got != want {
			t.Fatalf("JUnitStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRenderJUnit(t *testing.T) {
	rec := httptest.NewRecorder()
	RenderJUnit(rec, "run-1", []JUnitCase{
		{Name: "Login", ClassName: "TestRun #1", Status: "passed"},
		{Name: "Logout", ClassName: "TestRun #1", Status: "failed", Message: "boom"},
	})
	body := rec.Body.String()
	for _, want := range []string{
		"<testsuites", `tests="2" failures="1"`, `<testcase name="Login" classname="TestRun #1"/>`,
		`<failure message="boom"/>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("JUnit XML missing %q:\n%s", want, body)
		}
	}
}

func TestRenderXLSX(t *testing.T) {
	rec := httptest.NewRecorder()
	Render(rec, "xlsx", "cases", Table{
		Columns: []string{"id", "summary"},
		Rows:    [][]string{{"1", "Login"}},
	})
	if ct := rec.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd != `attachment; filename="cases.xlsx"` {
		t.Fatalf("unexpected disposition: %s", cd)
	}
	body := rec.Body.Bytes()
	if len(body) < 4 || string(body[:2]) != "PK" {
		t.Fatal("expected a zip (xlsx) container")
	}
	f, err := excelize.OpenReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("workbook not readable: %v", err)
	}
	defer func() { _ = f.Close() }()
	if v, _ := f.GetCellValue("Data", "A1"); v != "id" {
		t.Fatalf("unexpected A1: %q", v)
	}
	if v, _ := f.GetCellValue("Data", "B2"); v != "Login" {
		t.Fatalf("unexpected B2: %q", v)
	}
}
