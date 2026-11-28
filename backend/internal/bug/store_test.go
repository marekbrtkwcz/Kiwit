package bug

import (
	"context"
	"testing"

	"github.com/kiwit/kiwit/internal/platform/db"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	sqlDB, err := db.OpenSQLite(context.Background(), "sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, err := sqlDB.Exec(`INSERT INTO products (id, name) VALUES (1, 'Demo')`); err != nil {
		t.Fatal(err)
	}
	return NewStore(sqlDB, nil)
}

func TestBugCRUDAndFilters(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	b, err := s.Create(ctx, InternalBug{ProductID: 1, Summary: "Login broken", Status: "open", Severity: "critical"})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID == 0 {
		t.Fatal("expected assigned ID")
	}
	b2, err := s.Create(ctx, InternalBug{ProductID: 1, Summary: "Typo on page", Status: "open", Severity: "low"})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetByID(ctx, b.ID)
	if err != nil || got.Summary != "Login broken" {
		t.Fatalf("GetByID: %+v err=%v", got, err)
	}

	// Filter by severity
	list, err := s.List(ctx, Filter{ProductID: 1, Severity: "critical"})
	if err != nil || len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("List severity=critical: %d err=%v", len(list), err)
	}
	// Text search
	list, err = s.List(ctx, Filter{ProductID: 1, Q: "typo"})
	if err != nil || len(list) != 1 || list[0].ID != b2.ID {
		t.Fatalf("List q=typo: %d err=%v", len(list), err)
	}
	// All in product
	list, err = s.List(ctx, Filter{ProductID: 1})
	if err != nil || len(list) != 2 {
		t.Fatalf("List all: %d err=%v", len(list), err)
	}

	upd, err := s.Update(ctx, b.ID, "Login broken (fixed)", "", "resolved", "high", nil)
	if err != nil || upd.Status != "resolved" || upd.Summary != "Login broken (fixed)" {
		t.Fatalf("Update: %+v err=%v", upd, err)
	}

	if err := s.Delete(ctx, b2.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List(ctx, Filter{ProductID: 1})
	if len(list) != 1 {
		t.Fatalf("after delete expected 1 bug, got %d", len(list))
	}
}
