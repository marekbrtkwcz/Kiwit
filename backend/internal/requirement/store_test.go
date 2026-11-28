package requirement

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
	// Parent rows for FK constraints
	for _, stmt := range []string{
		`INSERT INTO products (id, name) VALUES (1, 'Demo')`,
		`INSERT INTO test_cases (id, product_id, summary) VALUES (10, 1, 'Login works')`,
		`INSERT INTO test_cases (id, product_id, summary) VALUES (11, 1, 'Logout works')`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return NewStore(sqlDB, nil)
}

func TestRequirementCRUD(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	created, err := s.Create(ctx, RequirementInput{ProductID: 1, Key: "REQ-1", Name: "Login", Status: "draft", Priority: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Key != "REQ-1" {
		t.Fatalf("unexpected created requirement: %+v", created)
	}

	got, err := s.Get(ctx, created.ID)
	if err != nil || got.Name != "Login" {
		t.Fatalf("Get: %+v err=%v", got, err)
	}

	updated, err := s.Update(ctx, created.ID, RequirementInput{ProductID: 1, Key: "REQ-1", Name: "Login v2", Status: "approved", Priority: "critical"})
	if err != nil || updated.Status != "approved" || updated.Name != "Login v2" {
		t.Fatalf("Update: %+v err=%v", updated, err)
	}

	// List with text filter
	reqs, total, err := s.List(ctx, filter{q: "login"}, 10, 0)
	if err != nil || total != 1 || len(reqs) != 1 {
		t.Fatalf("List q=login: total=%d len=%d err=%v", total, len(reqs), err)
	}
	// Filter by status
	reqs, total, err = s.List(ctx, filter{status: "draft"}, 10, 0)
	if err != nil || total != 0 || len(reqs) != 0 {
		t.Fatalf("List status=draft after update: total=%d err=%v", total, err)
	}
	reqs, total, err = s.List(ctx, filter{status: "approved"}, 10, 0)
	if err != nil || total != 1 {
		t.Fatalf("List status=approved: total=%d err=%v", total, err)
	}

	if err := s.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, created.ID); err == nil {
		t.Error("expected error getting deleted requirement")
	}
}

func TestRequirementCoverageAndAIQuality(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	r, err := s.Create(ctx, RequirementInput{ProductID: 1, Key: "REQ-2", Name: "Logout", Status: "draft", Priority: "low"})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.LinkCase(ctx, r.ID, 10); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkCase(ctx, r.ID, 11); err != nil {
		t.Fatal(err)
	}
	// Idempotent re-link
	if err := s.LinkCase(ctx, r.ID, 10); err != nil {
		t.Fatalf("re-link should be idempotent: %v", err)
	}

	cov, err := s.Coverage(ctx, r.ID)
	if err != nil || len(cov) != 2 {
		t.Fatalf("Coverage: len=%d err=%v", len(cov), err)
	}

	viaCase, err := s.RequirementsForCase(ctx, 10)
	if err != nil || len(viaCase) != 1 || viaCase[0].ID != r.ID {
		t.Fatalf("RequirementsForCase: %+v err=%v", viaCase, err)
	}

	if err := s.UnlinkCase(ctx, r.ID, 10); err != nil {
		t.Fatal(err)
	}
	cov, _ = s.Coverage(ctx, r.ID)
	if len(cov) != 1 {
		t.Fatalf("after unlink expected 1 coverage entry, got %d", len(cov))
	}

	if err := s.UpdateAIQuality(ctx, r.ID, `{"score": 0.9}`); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(ctx, r.ID)
	if got.AIQuality != `{"score": 0.9}` {
		t.Fatalf("AIQuality not persisted: %q", got.AIQuality)
	}
}
