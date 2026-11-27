package testcase

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/management"
	"github.com/kiwit/kiwit/internal/platform/db"
)

func TestParseImportCSV(t *testing.T) {
	csvData := "summary,preconditions,format,priority,status,is_automated,script,arguments,steps,gherkin_body\n" +
		"Login works,DB ready,steps,critical,confirmed,true,tests/login.spec.ts,--headed,\"Open /login => form visible ;; Submit creds => dashboard\", \n" +
		"Gherkin case,,gherkin,high,draft,false,,,\"Feature: x\",Feature: Login\n"

	records, err := parseImportCSV([]byte(csvData))
	require.NoError(t, err)
	require.Len(t, records, 2)

	r1 := records[0]
	require.Equal(t, "Login works", r1.Summary)
	require.Equal(t, "steps", r1.Format)
	require.Equal(t, "critical", r1.Priority)
	require.True(t, r1.IsAutomated)
	require.Equal(t, "tests/login.spec.ts", r1.Script)
	require.Len(t, r1.Steps, 2)
	require.Equal(t, 1, r1.Steps[0].Position)
	require.Equal(t, "Open /login", r1.Steps[0].Action)
	require.Equal(t, "form visible", r1.Steps[0].ExpectedResult)
	require.Equal(t, "Submit creds", r1.Steps[1].Action)

	r2 := records[1]
	require.Equal(t, "gherkin", r2.Format)
	require.Equal(t, "Feature: Login", r2.GherkinBody)
	require.False(t, r2.IsAutomated)
	require.Equal(t, "high", r2.Priority)
	require.Equal(t, "draft", r2.Status)
}

func TestParseImportCSVStepsActionOnly(t *testing.T) {
	records, err := parseImportCSV([]byte("summary,steps\nA,\"step one ;; step two => done\"\n"))
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Len(t, records[0].Steps, 2)
	require.Equal(t, "step one", records[0].Steps[0].Action)
	require.Empty(t, records[0].Steps[0].ExpectedResult)
	require.Equal(t, "step two", records[0].Steps[1].Action)
	require.Equal(t, "done", records[0].Steps[1].ExpectedResult)
}

func TestParseImportCSVInvalid(t *testing.T) {
	_, err := parseImportCSV([]byte("priority,category\nhigh,x\n"))
	require.ErrorContains(t, err, "summary column")

	_, err = parseImportCSV([]byte("summary\n"))
	require.ErrorContains(t, err, "at least one data row")

	_, err = parseImportCSV([]byte("summary,format\nA,bogus\n"))
	require.ErrorContains(t, err, "format must be one of")

	_, err = parseImportCSV([]byte("summary\n   \n"))
	require.ErrorContains(t, err, "no records in file")
}

func TestParseImportJSON(t *testing.T) {
	payload := `[{"summary":"A","format":"steps","status":"confirmed","priority":"high",` +
		`"steps":[{"action":"open","expected_result":"form"},{"action":"submit","expected":"dashboard"}]},` +
		`{"summary":"B","format":"gherkin","gherkin_body":"Feature: B"}]`
	records, err := parseImportJSON([]byte(payload))
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Len(t, records[0].Steps, 2)
	require.Equal(t, "form", records[0].Steps[0].ExpectedResult)
	require.Equal(t, "dashboard", records[0].Steps[1].ExpectedResult) // expected alias
	require.Equal(t, 2, records[0].Steps[1].Position)                 // renumbered
	require.Equal(t, "confirmed", records[1].Status)                  // default
	require.Equal(t, "Feature: B", records[1].GherkinBody)
}

func TestParseImportJSONFlatExport(t *testing.T) {
	// Shape produced by GET /api/v1/testcases/export?format=json: every cell
	// is a string, steps is the step count, and id/created_at are present.
	flat := `[{"id":"5","product_id":"1","category_id":"","summary":"A","format":"steps",` +
		`"priority":"high","status":"confirmed","is_automated":"true","steps":"2",` +
		`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]`
	records, err := parseImportJSON([]byte(flat))
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "A", records[0].Summary)
	require.Equal(t, "steps", records[0].Format)
	require.Equal(t, "high", records[0].Priority)
	require.True(t, records[0].IsAutomated)
	require.Empty(t, records[0].Steps) // step count ignored, not an error
}

func TestParseImportJSONInvalid(t *testing.T) {
	_, err := parseImportJSON([]byte(`{"summary":"not an array"}`))
	require.ErrorContains(t, err, "array")

	_, err = parseImportJSON([]byte(`[]`))
	require.ErrorContains(t, err, "no records")

	_, err = parseImportJSON([]byte(`[{"summary":"A","status":"bogus"}]`))
	require.ErrorContains(t, err, "status must be one of")
}

// TestImportRouteHandler exercises the full HTTP path: multipart CSV upload
// through the chi route with an admin user in context.
func TestImportRouteHandler(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prodRepo := management.NewSQLiteRepository(sqlDB)
	prod, err := prodRepo.CreateProduct(ctx, "Kiwit", "")
	require.NoError(t, err)
	// The import handler stamps the context user as author; users.id=1 must
	// exist to satisfy the foreign key.
	_, err = sqlDB.ExecContext(ctx, `INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
		VALUES ('admin@t.local', 'admin', 'x', 'admin', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	repo := NewSQLiteRepository(sqlDB)
	guard := auth.NewAccessGuard(sqlDB, nil)

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(auth.WithUser(r.Context(), auth.User{ID: 1, Role: "admin"}))
			next.ServeHTTP(w, r)
		})
	})
	RegisterImportRoutes(router, repo, guard, nil)

	body := &strings.Builder{}
	body.WriteString("--BOUNDARY\r\nContent-Disposition: form-data; name=\"product_id\"\r\n\r\n")
	body.WriteString(jsonNumber(prod.ID))
	body.WriteString("\r\n--BOUNDARY\r\nContent-Disposition: form-data; name=\"file\"; filename=\"cases.csv\"\r\n")
	body.WriteString("Content-Type: text/csv\r\n\r\n")
	body.WriteString("summary,format,steps\nCSV case,steps,\"one => ok\"\nbad-format,bogus,\n\r\n--BOUNDARY--\r\n")

	// The second row has an invalid format and must be reported per-row...
	// (it is rejected at parse time, so the whole file 400s — see below.)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/testcases/import", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=BOUNDARY")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Parse-level validation rejects the whole file with 400.
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "format must be one of")

	// Now a valid file: one good row, one row with an empty summary is a
	// parse error too; row-level failures happen only on repo errors, which
	// are rare — so verify the happy path response shape instead.
	body.Reset()
	body.WriteString("--BOUNDARY\r\nContent-Disposition: form-data; name=\"product_id\"\r\n\r\n")
	body.WriteString(jsonNumber(prod.ID))
	body.WriteString("\r\n--BOUNDARY\r\nContent-Disposition: form-data; name=\"file\"; filename=\"cases.csv\"\r\n")
	body.WriteString("Content-Type: text/csv\r\n\r\n")
	body.WriteString("summary,format,steps\nCSV case,steps,\"one => ok\"\nSecond case,markdown,\n\r\n--BOUNDARY--\r\n")

	req = httptest.NewRequest(http.MethodPost, "/api/v1/testcases/import", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=BOUNDARY")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Imported int           `json:"imported"`
		Failed   int           `json:"failed"`
		Errors   []importError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Imported)
	require.Equal(t, 0, resp.Failed)

	list, total, err := repo.ListWithFilter(ctx, ListFilter{ProductID: prod.ID, Limit: 50})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
