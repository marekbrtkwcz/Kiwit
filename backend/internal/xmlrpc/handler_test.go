package xmlrpc

import (
	"database/sql"
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
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, username TEXT, password_hash TEXT, role TEXT, is_active INTEGER, created_at TEXT, updated_at TEXT)`,
		`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (1, 'admin@example.com', 'admin', 'x', 'admin', 1)`,
		`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (2, 'tester@example.com', 'tester', 'x', 'tester', 1)`,
		`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (3, 'viewer@example.com', 'viewer', 'x', 'viewer', 1)`,
		`CREATE TABLE product_members (product_id INTEGER, user_id INTEGER, role TEXT, PRIMARY KEY (product_id, user_id))`,
		`CREATE TABLE user_groups (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE user_group_members (group_id INTEGER, user_id INTEGER, PRIMARY KEY (group_id, user_id))`,
		`CREATE TABLE group_product_permissions (group_id INTEGER, product_id INTEGER, role TEXT, PRIMARY KEY (group_id, product_id))`,
		`CREATE TABLE test_runs (id INTEGER PRIMARY KEY, summary TEXT, plan_id INTEGER, build_id INTEGER, stopped_at TEXT)`,
		`CREATE TABLE test_cases (id INTEGER PRIMARY KEY, summary TEXT, category_id INTEGER, is_automated INTEGER, deleted_at DATETIME NULL, product_id INTEGER, priority TEXT, status TEXT, preconditions TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE test_plans (id INTEGER PRIMARY KEY, name TEXT, product_id INTEGER, version_id INTEGER, description TEXT DEFAULT '', is_active INTEGER DEFAULT 1, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE test_plan_cases (test_plan_id INTEGER, test_case_id INTEGER, position INTEGER DEFAULT 0, PRIMARY KEY (test_plan_id, test_case_id))`,
		`CREATE TABLE versions (id INTEGER PRIMARY KEY, value TEXT, product_id INTEGER)`,
		`CREATE TABLE builds (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER)`,
		`CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, description TEXT, classification_id INTEGER)`,
		`INSERT INTO products (id, name, description) VALUES (1, 'Demo', 'Test product')`,
		`INSERT INTO test_plans (id, name, product_id) VALUES (1, 'Plan A', 1)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

// adminToken issues a JWT for the seeded admin user (id 1).
func adminToken(t *testing.T, svc *auth.Service) string {
	t.Helper()
	return userToken(t, svc, 1, "admin")
}

// userToken issues a JWT for a seeded test user (AuthenticateBearer reloads
// the user from the DB by sub, so the row must exist in setupTestDB).
func userToken(t *testing.T, svc *auth.Service, id int64, role string) string {
	t.Helper()
	tok, err := svc.IssueJWT(auth.User{ID: id, Username: role, Email: role + "@example.com", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestVersion(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")

	r := chi.NewRouter()
	h.Register(r)

	body := `<?xml version="1.0"?><methodCall><methodName>KiwiTCMS.version</methodName><params></params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "0.1.0") {
		t.Fatalf("expected version in response, got: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Kiwit TCMS") {
		t.Fatalf("expected name in response, got: %s", rec.Body.String())
	}
}

func TestProductFilter(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")

	r := chi.NewRouter()
	h.Register(r)

	body := `<?xml version="1.0"?><methodCall><methodName>Product.filter</methodName><params><param><value><struct></struct></value></param></params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "Demo") {
		t.Fatalf("expected product name in response, got: %s", rec.Body.String())
	}
}

func TestPlanCreateAndSortkeys(t *testing.T) {
	db := setupTestDB(t)
	db.Exec(`INSERT INTO test_cases (id, summary, category_id, is_automated) VALUES (1, 'Login test', 0, 0), (2, 'Logout test', 0, 0)`)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	call := func(name, params string) string {
		body := `<?xml version="1.0"?><methodCall><methodName>` + name + `</methodName><params>` + params + `</params></methodCall>`
		req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	// Create plan
	out := call("TestPlan.create", `<param><value><struct><member><name>name</name><value><string>XML Plan</string></value></member><member><name>product_id</name><value><int>1</int></value></member><member><name>version_id</name><value><int>1</int></value></member></struct></value></param>`)
	if !strings.Contains(out, "XML Plan") {
		t.Fatalf("expected plan name, got: %s", out)
	}
	// Attach cases and read sortkeys
	call("TestPlan.add_case", `<param><value><int>1</int></value></param><param><value><int>1</int></value></param>`)
	call("TestPlan.add_case", `<param><value><int>1</int></value></param><param><value><int>2</int></value></param>`)
	sk := call("TestPlan.sortkeys", `<param><value><int>1</int></value></param>`)
	if !strings.Contains(sk, "sortkey") || !strings.Contains(sk, "case_id") {
		t.Fatalf("expected sortkeys, got: %s", sk)
	}
}

func TestMarkdownRender(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)
	body := `<?xml version="1.0"?><methodCall><methodName>Markdown.render</methodName><params><param><value><struct><member><name>text</name><value><string>**hi**</string></value></member></struct></value></param></params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "&lt;strong&gt;hi&lt;/strong&gt;") && !strings.Contains(rec.Body.String(), "<strong>hi</strong>") {
		t.Fatalf("expected rendered html, got: %s", rec.Body.String())
	}
}

func TestUnknownMethod(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")

	r := chi.NewRouter()
	h.Register(r)

	body := `<?xml version="1.0"?><methodCall><methodName>Nope.nope</methodName><params></params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "fault") {
		t.Fatalf("expected fault, got: %s", rec.Body.String())
	}
}
