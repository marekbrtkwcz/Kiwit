package initdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, username TEXT, password_hash TEXT, role TEXT, is_active INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE api_tokens (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE priorities (id INTEGER PRIMARY KEY, value TEXT, is_active INTEGER)`,
		`CREATE TABLE execution_statuses (id INTEGER PRIMARY KEY, name TEXT, color TEXT, weight INTEGER)`,
		`CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE versions (id INTEGER PRIMARY KEY, value TEXT, product_id INTEGER)`,
		`CREATE TABLE builds (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER)`,
		`CREATE TABLE test_plans (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE test_cases (id INTEGER PRIMARY KEY, summary TEXT)`,
		`CREATE TABLE test_case_steps (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_case_revisions (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_plan_cases (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_runs (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_executions (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE bugs (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE internal_bugs (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE attachments (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE comments (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE notifications (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE user_notification_settings (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE webhooks (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE password_reset_tokens (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE user_groups (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE user_group_members (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE entity_properties (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE link_references (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE group_product_permissions (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_execution_tags (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_run_tags (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_plan_tags (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_case_tags (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_case_components (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE environments (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE templates (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE issue_trackers (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE audit_log (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE components (id INTEGER PRIMARY KEY, name TEXT, product_id INTEGER, description TEXT DEFAULT '')`,
		`CREATE TABLE tags (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE test_case_statuses (id INTEGER PRIMARY KEY, name TEXT, description TEXT DEFAULT '')`,
		`CREATE TABLE classifications (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE test_plan_types (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE product_members (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE email_settings (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE execution_links (id INTEGER PRIMARY KEY)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	return db
}

func setupRouter(db *sql.DB) chi.Router {
	r := chi.NewRouter()
	Register(r, NewStore(db, nil))
	return r
}

func TestInitDBStatusEmpty(t *testing.T) {
	db := setupTestDB(t)
	r := setupRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/init-db/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Initialized {
		t.Error("expected initialized=false for empty DB")
	}
}

func TestInitDBPost(t *testing.T) {
	db := setupTestDB(t)
	r := setupRouter(db)

	body := `{"admin_username": "boss", "admin_password": "secret123", "admin_email": "boss@test.com"}`
	req := httptest.NewRequest(http.MethodPost, "/init-db/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp initResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Admin != "boss" {
		t.Errorf("unexpected response: %+v", resp)
	}

	// Verify user was created
	var role string
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
	_ = db.QueryRow(`SELECT role FROM users WHERE username = 'boss'`).Scan(&role)
	if role != "admin" {
		t.Errorf("expected admin role, got %s", role)
	}

	// Verify dictionaries seeded
	_ = db.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&count)
	if count == 0 {
		t.Error("expected categories to be seeded")
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM priorities`).Scan(&count)
	if count == 0 {
		t.Error("expected priorities to be seeded")
	}
}

func TestInitDBPostConflict(t *testing.T) {
	db := setupTestDB(t)
	_, _ = db.Exec(`INSERT INTO users (email, username, password_hash, role, is_active) VALUES ('a@b.c', 'existing', 'x', 'admin', 1)`)
	r := setupRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/init-db/", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestInitDBStatusRedirect(t *testing.T) {
	db := setupTestDB(t)
	_, _ = db.Exec(`INSERT INTO users (email, username, password_hash, role, is_active) VALUES ('a@b.c', 'u', 'x', 'admin', 1)`)
	r := setupRouter(db)

	req := httptest.NewRequest(http.MethodGet, "/init-db/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
}

func TestResetDB(t *testing.T) {
	db := setupTestDB(t)
	_, _ = db.Exec(`INSERT INTO users (email, username, password_hash, role, is_active) VALUES ('a@b.c', 'u', 'x', 'admin', 1)`)
	_, _ = db.Exec(`INSERT INTO products (id, name) VALUES (1, 'Test')`)
	r := setupRouter(db)

	// Simulate admin context
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reset-db", nil)
	req = req.WithContext(contextWithRole(req, "admin"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Verify data wiped
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 users after reset, got %d", count)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 products after reset, got %d", count)
	}
}

func TestResetDBForbidden(t *testing.T) {
	db := setupTestDB(t)
	r := setupRouter(db)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reset-db", nil)
	req = req.WithContext(contextWithRole(req, "tester"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// contextWithRole is a test helper to inject a role into the request context.
func contextWithRole(r *http.Request, role string) (ctx context.Context) {
	return auth.WithUser(r.Context(), auth.User{Role: role})
}

// Fail-closed: a request without an authenticated user in context must NOT
// be able to wipe the database (regression: plain-string context key made
// the role check fail open).
func TestResetDBNoContextForbidden(t *testing.T) {
	db := setupTestDB(t)
	r := setupRouter(db)
	// Seed a row that a successful wipe would delete
	if _, err := db.Exec(`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (1, 'a@b.c', 'admin', 'x', 'admin', 1)`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reset-db", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if count == 0 {
		t.Error("database was wiped without an authenticated admin in context")
	}
}
