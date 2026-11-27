package search

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE test_cases (id INTEGER PRIMARY KEY, summary TEXT, preconditions TEXT, deleted_at DATETIME NULL)`,
		`CREATE TABLE test_case_steps (id INTEGER PRIMARY KEY, test_case_id INTEGER, position INTEGER, action TEXT, expected_result TEXT)`,
		`CREATE TABLE test_plans (id INTEGER PRIMARY KEY, name TEXT, description TEXT)`,
		`CREATE TABLE test_runs (id INTEGER PRIMARY KEY, summary TEXT)`,
		`INSERT INTO test_cases VALUES (1, 'Login with valid credentials', 'User exists', NULL), (2, 'Logout flow', '', NULL)`,
		`INSERT INTO test_case_steps VALUES (1, 1, 0, 'Open login page', 'Form visible')`,
		`INSERT INTO test_plans VALUES (1, 'Smoke plan', 'Regression smoke')`,
		`INSERT INTO test_runs VALUES (1, 'Smoke run v1')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestSearch(t *testing.T) {
	db := setupDB(t)
	s := NewStore(db, nil)

	// "login" matches test case 1 (summary + step)
	items, err := s.Search(context.Background(), []string{"login"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 || items[0].Type != "test_case" || items[0].ID != 1 {
		t.Fatalf("expected test_case 1, got %+v", items)
	}

	// multi-term AND: "smoke" matches plan + run
	items, err = s.Search(context.Background(), []string{"smoke"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 2 {
		t.Fatalf("expected plan and run, got %+v", items)
	}

	// no match
	items, err = s.Search(context.Background(), []string{"nonexistentterm"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no results, got %+v", items)
	}
}
