package permissions

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE user_groups (id INTEGER PRIMARY KEY, name TEXT UNIQUE, created_at TEXT)`,
		`CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE group_product_permissions (group_id INTEGER, product_id INTEGER, role TEXT, PRIMARY KEY (group_id, product_id))`,
		`INSERT INTO products (id, name) VALUES (1, 'Product A'), (2, 'Product B')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestRefreshCreatesGroups(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)

	res, err := s.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GroupsCreated) != 2 {
		t.Errorf("expected 2 groups created, got %v", res.GroupsCreated)
	}
	if !res.OK {
		t.Error("expected OK=true")
	}

	// Verify groups in DB
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM user_groups`).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 groups in DB, got %d", count)
	}

	// Verify Tester assigned to all products
	_ = db.QueryRow(`SELECT COUNT(*) FROM group_product_permissions gpp JOIN user_groups g ON g.id = gpp.group_id WHERE g.name = 'Tester'`).Scan(&count)
	if count != 2 {
		t.Errorf("expected Tester assigned to 2 products, got %d", count)
	}
}

func TestRefreshIdempotent(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)

	_, _ = s.Refresh(context.Background())
	res, err := s.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.GroupsCreated) != 0 {
		t.Errorf("expected 0 created on second run, got %v", res.GroupsCreated)
	}
	if len(res.GroupsExisting) != 2 {
		t.Errorf("expected 2 existing on second run, got %v", res.GroupsExisting)
	}
}

func TestRefreshEndpoint(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	Register(r, NewStore(db, nil))

	// With admin role
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/refresh-permissions", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_role", "admin"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp refreshResult
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK {
		t.Error("expected OK=true")
	}
}

func TestRefreshEndpointForbidden(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	Register(r, NewStore(db, nil))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/refresh-permissions", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_role", "tester"))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "admin") {
		t.Errorf("expected admin error, got: %s", rec.Body.String())
	}
}
