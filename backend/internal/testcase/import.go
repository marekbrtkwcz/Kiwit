// Test case import: accepts CSV or JSON files and creates test cases in a
// target product. Columns/keys mirror the export pipeline so a JSON export
// round-trips, and the CSV convention is designed for hand-editing in a
// spreadsheet.
package testcase

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kiwit/kiwit/internal/audit"
	"github.com/kiwit/kiwit/internal/auth"
)

// maxImportBytes caps an uploaded import file (10 MiB).
const maxImportBytes = 10 << 20

// maxImportRecords caps how many cases one file may contain.
const maxImportRecords = 1000

// ImportRecord is one row of an import file, mapped onto UpsertParams.
type ImportRecord struct {
	Summary       string  `json:"summary"`
	Preconditions string  `json:"preconditions"`
	Requirement   string  `json:"requirement"`
	ExtraLink     string  `json:"extra_link"`
	Notes         string  `json:"notes"`
	Format        string  `json:"format"`
	GherkinBody   string  `json:"gherkin_body"`
	Priority      string  `json:"priority"`
	Status        string  `json:"status"`
	IsAutomated   bool    `json:"is_automated"`
	Script        string  `json:"script"`
	Arguments     string  `json:"arguments"`
	Steps         []Step  `json:"steps"`
}

// importStepJSON accepts both expected_result and expected keys.
type importStepJSON struct {
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
	Expected       string `json:"expected"`
}

// importStep accepts a step object; the flat JSON export emits the step
// COUNT as a number, which is tolerated and ignored for round-trip
// compatibility with `GET /api/v1/testcases/export?format=json`.
type importStep struct {
	Obj importStepJSON
}

func (s *importStep) UnmarshalJSON(b []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(b)), "{") {
		return json.Unmarshal(b, &s.Obj)
	}
	// Not an object (e.g. the step count from the flat export): ignore.
	return nil
}

// parseImportStepsJSON accepts an array of step objects or any non-array
// JSON value (ignored) from the flat export.
func parseImportStepsJSON(raw json.RawMessage) ([]Step, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "[") {
		return nil, nil // step count from the flat export
	}
	var steps []importStep
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, errors.New("steps must be an array of {action, expected_result} objects")
	}
	var out []Step
	for _, s := range steps {
		if len(s.Obj.Action) == 0 && s.Obj.ExpectedResult == "" && s.Obj.Expected == "" {
			continue // non-object entry (flat export step count)
		}
		expected := s.Obj.ExpectedResult
		if expected == "" {
			expected = s.Obj.Expected
		}
		out = append(out, Step{Action: s.Obj.Action, ExpectedResult: expected})
	}
	return out, nil
}

// importBool accepts a JSON boolean or its string form ("true"/"false"),
// because the flat export pipeline serializes every cell as a string.
type importBool bool

func (v *importBool) UnmarshalJSON(b []byte) error {
	s := strings.ToLower(strings.Trim(strings.TrimSpace(string(b)), `"`))
	switch s {
	case "true", "1", "yes":
		*v = true
	case "false", "0", "no", "", "null":
		*v = false
	default:
		return errors.New("is_automated must be a boolean")
	}
	return nil
}

// normalizeImportRecord applies defaults and validates enum fields.
func normalizeImportRecord(r *ImportRecord) error {
	r.Summary = strings.TrimSpace(r.Summary)
	if r.Summary == "" {
		return errors.New("summary is required")
	}
	r.Format = strings.TrimSpace(strings.ToLower(r.Format))
	switch r.Format {
	case "":
		r.Format = "steps"
	case "steps", "gherkin", "markdown", "checklist":
	default:
		return errors.New("format must be one of steps, gherkin, markdown, checklist")
	}
	r.Priority = strings.TrimSpace(strings.ToLower(r.Priority))
	if r.Priority == "" {
		r.Priority = "medium"
	}
	r.Status = strings.TrimSpace(strings.ToLower(r.Status))
	switch r.Status {
	case "":
		r.Status = "confirmed"
	case "draft", "confirmed", "disabled":
	default:
		return errors.New("status must be one of draft, confirmed, disabled")
	}
	for i := range r.Steps {
		r.Steps[i].Position = i + 1
		if strings.TrimSpace(r.Steps[i].Action) == "" {
			return errors.New("step " + strconv.Itoa(i+1) + " has an empty action")
		}
	}
	return nil
}

// importCaseJSON is the wire shape of one imported case; steps accept both
// expected_result and expected keys.
type importCaseJSON struct {
	Summary       string            `json:"summary"`
	Preconditions string            `json:"preconditions"`
	Requirement   string            `json:"requirement"`
	ExtraLink     string            `json:"extra_link"`
	Notes         string            `json:"notes"`
	Format        string            `json:"format"`
	GherkinBody   string            `json:"gherkin_body"`
	Priority      string            `json:"priority"`
	Status        string            `json:"status"`
	IsAutomated   importBool        `json:"is_automated"`
	Script        string            `json:"script"`
	Arguments     string            `json:"arguments"`
	Steps         json.RawMessage   `json:"steps"`
}

// parseImportJSON decodes an array of case objects (same shape as the API
// body / JSON export, with steps as {action, expected_result} objects).
func parseImportJSON(data []byte) ([]ImportRecord, error) {
	var rows []importCaseJSON
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, errors.New("invalid JSON: expected an array of test case objects")
	}
	if len(rows) == 0 {
		return nil, errors.New("no records in file")
	}
	records := make([]ImportRecord, 0, len(rows))
	for i, row := range rows {
		rec := ImportRecord{
			Summary:       row.Summary,
			Preconditions: row.Preconditions,
			Requirement:   row.Requirement,
			ExtraLink:     row.ExtraLink,
			Notes:         row.Notes,
			Format:        row.Format,
			GherkinBody:   row.GherkinBody,
			Priority:      row.Priority,
			Status:        row.Status,
			IsAutomated:   bool(row.IsAutomated),
			Script:        row.Script,
			Arguments:     row.Arguments,
		}
		steps, err := parseImportStepsJSON(row.Steps)
		if err != nil {
			return nil, errors.New("record " + strconv.Itoa(i+1) + ": " + err.Error())
		}
		rec.Steps = steps
		if err := normalizeImportRecord(&rec); err != nil {
			return nil, errors.New("record " + strconv.Itoa(i+1) + ": " + err.Error())
		}
		records = append(records, rec)
	}
	return records, nil
}

// parseImportCSV decodes a CSV file with a header row. Column names are
// case-insensitive; unknown columns are ignored. The steps column uses
// " ;; " between steps and " => " between action and expected result.
func parseImportCSV(data []byte) ([]ImportRecord, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, errors.New("invalid CSV: " + err.Error())
	}
	if len(rows) < 2 {
		return nil, errors.New("CSV needs a header row and at least one data row")
	}
	colIndex := map[string]int{}
	for i, name := range rows[0] {
		colIndex[strings.ToLower(strings.TrimSpace(name))] = i
	}
	if _, ok := colIndex["summary"]; !ok {
		return nil, errors.New("CSV is missing the required summary column")
	}
	cell := func(row []string, name string) string {
		if i, ok := colIndex[name]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	var records []ImportRecord
	for _, row := range rows[1:] {
		isAllEmpty := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				isAllEmpty = false
				break
			}
		}
		if isAllEmpty {
			continue
		}
		auto := false
		switch strings.ToLower(cell(row, "is_automated")) {
		case "true", "1", "yes", "y", "t":
			auto = true
		}
		records = append(records, ImportRecord{
			Summary:       cell(row, "summary"),
			Preconditions: cell(row, "preconditions"),
			Requirement:   cell(row, "requirement"),
			ExtraLink:     cell(row, "extra_link"),
			Notes:         cell(row, "notes"),
			Format:        cell(row, "format"),
			GherkinBody:   cell(row, "gherkin_body"),
			Priority:      cell(row, "priority"),
			Status:        cell(row, "status"),
			IsAutomated:   auto,
			Script:        cell(row, "script"),
			Arguments:     cell(row, "arguments"),
			Steps:         parseImportSteps(cell(row, "steps")),
		})
	}
	if len(records) == 0 {
		return nil, errors.New("no records in file")
	}
	for i := range records {
		if err := normalizeImportRecord(&records[i]); err != nil {
			return nil, errors.New("record " + strconv.Itoa(i+1) + ": " + err.Error())
		}
	}
	return records, nil
}

// parseImportSteps splits "a1 => e1 ;; a2 => e2" into structured steps.
func parseImportSteps(s string) []Step {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var steps []Step
	for i, chunk := range strings.Split(s, ";;") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		step := Step{Position: i + 1}
		if action, expected, found := strings.Cut(chunk, "=>"); found {
			step.Action = strings.TrimSpace(action)
			step.ExpectedResult = strings.TrimSpace(expected)
		} else {
			step.Action = chunk
		}
		steps = append(steps, step)
	}
	return steps
}

// importError describes a single rejected record.
type importError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// RegisterImportRoutes mounts POST /api/v1/testcases/import (multipart:
// file + product_id; format detected from the file extension or a format field).
func RegisterImportRoutes(r chi.Router, repo Repository, guard *auth.AccessGuard, auditStore *audit.Store) {
	r.With(auth.RequireRole("tester")).Post("/api/v1/testcases/import", func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxImportBytes)
		if err := req.ParseMultipartForm(maxImportBytes); err != nil {
			http.Error(w, `{"detail":"file too large or invalid multipart form"}`, http.StatusBadRequest)
			return
		}
		productID, err := strconv.ParseInt(req.FormValue("product_id"), 10, 64)
		if err != nil || productID <= 0 {
			http.Error(w, `{"detail":"invalid product_id"}`, http.StatusBadRequest)
			return
		}
		if auth.WriteAccessError(w, guard.EnforceProduct(req.Context(), productID, true)) {
			return
		}
		file, header, err := req.FormFile("file")
		if err != nil {
			http.Error(w, `{"detail":"missing file field"}`, http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()
		data, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, `{"detail":"failed to read file"}`, http.StatusBadRequest)
			return
		}
		format := strings.ToLower(strings.TrimSpace(req.FormValue("format")))
		if format == "" && header != nil {
			switch {
			case strings.HasSuffix(strings.ToLower(header.Filename), ".json"):
				format = "json"
			case strings.HasSuffix(strings.ToLower(header.Filename), ".csv"):
				format = "csv"
			}
		}
		var records []ImportRecord
		var parseErr error
		switch format {
		case "json":
			records, parseErr = parseImportJSON(data)
		case "csv":
			records, parseErr = parseImportCSV(data)
		default:
			http.Error(w, `{"detail":"format must be csv or json (set the format field or use a .csv/.json filename)"}`, http.StatusBadRequest)
			return
		}
		if parseErr != nil {
			http.Error(w, `{"detail":"`+parseErr.Error()+`"}`, http.StatusBadRequest)
			return
		}
		if len(records) > maxImportRecords {
			http.Error(w, `{"detail":"too many records (max 1000 per file)"}`, http.StatusBadRequest)
			return
		}

		var authorID *int64
		if u, ok := auth.UserFromContext(req.Context()); ok {
			uid := u.ID
			authorID = &uid
		}
		var errs []importError
		imported := 0
		for i, rec := range records {
			tc, err := repo.Create(req.Context(), UpsertParams{
				ProductID:     productID,
				AuthorID:      authorID,
				Summary:       rec.Summary,
				Preconditions: rec.Preconditions,
				Requirement:   rec.Requirement,
				ExtraLink:     rec.ExtraLink,
				Notes:         rec.Notes,
				Format:        rec.Format,
				GherkinBody:   rec.GherkinBody,
				Priority:      rec.Priority,
				Status:        rec.Status,
				IsAutomated:   rec.IsAutomated,
				Script:        rec.Script,
				Arguments:     rec.Arguments,
				Steps:         rec.Steps,
			})
			if err != nil {
				errs = append(errs, importError{Row: i + 1, Error: err.Error()})
				continue
			}
			imported++
			audit.RecordChange(req.Context(), auditStore, "test_case", tc.ID, "create", nil, map[string]any{
				"summary": tc.Summary, "status": tc.Status, "priority": tc.Priority, "product_id": productID, "imported": true,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"imported": imported,
			"failed":   len(errs),
			"errors":   errs,
		})
	})
}
