// Package export provides a unified data-export pipeline: every entity maps
// its rows to a Table, and Render serializes that table to CSV, JSON or
// Markdown with consistent headers and filenames.
package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Table is the unified export data model: named columns and string rows.
type Table struct {
	Columns []string
	Rows    [][]string
}

// Format returns the canonical format for a query parameter value.
// Accepted values: "csv" (default), "json", "md" (alias "markdown"),
// "xlsx" and "feature" (the latter is interpreted by the test case routes).
func Format(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "json":
		return "json"
	case "md", "markdown":
		return "md"
	case "xlsx":
		return "xlsx"
	case "feature":
		return "feature"
	default:
		return "csv"
	}
}

// Render writes the table to w in the given format ("csv", "json" or "md").
func Render(w http.ResponseWriter, format, name string, t Table) {
	switch Format(format) {
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, name))
		rows := make([]map[string]string, 0, len(t.Rows))
		for _, r := range t.Rows {
			m := make(map[string]string, len(t.Columns))
			for i, c := range t.Columns {
				if i < len(r) {
					m[c] = r[i]
				}
			}
			rows = append(rows, m)
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rows)
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, name))
		writeMarkdownTable(w, t)
	case "xlsx":
		renderXLSX(w, name, t)
	default:
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, name))
		// UTF-8 BOM so Excel opens the file with correct encoding.
		_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
		cw := csv.NewWriter(w)
		_ = cw.Write(t.Columns)
		for _, r := range t.Rows {
			_ = cw.Write(r)
		}
		cw.Flush()
	}
}

func writeMarkdownTable(w http.ResponseWriter, t Table) {
	var b strings.Builder
	sep := func() {
		b.WriteString("|")
		for range t.Columns {
			b.WriteString(" --- |")
		}
		b.WriteString("\n")
	}
	b.WriteString("|")
	for _, c := range t.Columns {
		b.WriteString(" " + mdEscape(c) + " |")
	}
	b.WriteString("\n")
	sep()
	for _, r := range t.Rows {
		b.WriteString("|")
		for i := range t.Columns {
			v := ""
			if i < len(r) {
				v = r[i]
			}
			b.WriteString(" " + mdEscape(v) + " |")
		}
		b.WriteString("\n")
	}
	_, _ = w.Write([]byte(b.String()))
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

// renderXLSX writes the table as a single-sheet Excel workbook.
func renderXLSX(w http.ResponseWriter, name string, t Table) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := "Data"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		http.Error(w, `{"detail":"failed to build workbook"}`, http.StatusInternalServerError)
		return
	}
	for i, c := range t.Columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, c)
	}
	if styleID, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}}); err == nil {
		lastCell, _ := excelize.CoordinatesToCellName(len(t.Columns), 1)
		_ = f.SetCellStyle(sheet, "A1", lastCell, styleID)
	}
	for r, row := range t.Rows {
		for i := range t.Columns {
			v := ""
			if i < len(row) {
				v = row[i]
			}
			cell, _ := excelize.CoordinatesToCellName(i+1, r+2)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
	if err := f.Write(w); err != nil {
		// headers are already sent; nothing more we can do
		_ = err
	}
}

// JUnitCase is a single test result for the JUnit XML renderer.
type JUnitCase struct {
	Name      string
	ClassName string
	// Status must be one of "passed", "failed", "error", "skipped".
	Status string
	Message string
}

// JUnitStatus maps a Kiwit execution status to a JUnit outcome.
func JUnitStatus(status string) string {
	switch strings.ToLower(status) {
	case "passed", "pass":
		return "passed"
	case "failed", "fail":
		return "failed"
	case "blocked", "error":
		return "error"
	default: // skipped, idle, unknown
		return "skipped"
	}
}

// RenderJUnit writes cases as a JUnit XML document.
func RenderJUnit(w http.ResponseWriter, name string, cases []JUnitCase) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xml"`, name))
	failures, errors, skipped := 0, 0, 0
	for _, c := range cases {
		switch c.Status {
		case "failed":
			failures++
		case "error":
			errors++
		case "skipped":
			skipped++
		}
	}
	fmt.Fprintf(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	fmt.Fprintf(w, "<testsuites name=%q tests=\"%d\" failures=\"%d\" errors=\"%d\" skipped=\"%d\">\n",
		name, len(cases), failures, errors, skipped)
	fmt.Fprintf(w, "  <testsuite name=%q tests=\"%d\" failures=\"%d\" errors=\"%d\" skipped=\"%d\">\n",
		name, len(cases), failures, errors, skipped)
	for _, c := range cases {
		fmt.Fprintf(w, "    <testcase name=%q classname=%q", c.Name, c.ClassName)
		switch c.Status {
		case "failed":
			fmt.Fprintf(w, ">\n      <failure message=%q/>\n    </testcase>\n", c.Message)
		case "error":
			fmt.Fprintf(w, ">\n      <error message=%q/>\n    </testcase>\n", c.Message)
		case "skipped":
			fmt.Fprintf(w, ">\n      <skipped/>\n    </testcase>\n")
		default:
			fmt.Fprintf(w, "/>\n")
		}
	}
	fmt.Fprint(w, "  </testsuite>\n</testsuites>\n")
}

// --- request helpers shared by export routes ---

// QueryInt64 returns a positive int64 query parameter or 0.
func QueryInt64(req *http.Request, name string) int64 {
	v, err := strconv.ParseInt(req.URL.Query().Get(name), 10, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

// --- cell helpers shared by entity mappers ---

// I64 formats an int64 cell.
func I64(v int64) string { return strconv.FormatInt(v, 10) }

// OptI64 formats a nullable int64 cell ("" for nil).
func OptI64(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// Time formats a timestamp cell.
func Time(v time.Time) string { return v.UTC().Format(time.RFC3339) }

// OptTime formats a nullable timestamp cell ("" for nil).
func OptTime(v *time.Time) string {
	if v == nil {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

// F formats a float cell.
func F(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
