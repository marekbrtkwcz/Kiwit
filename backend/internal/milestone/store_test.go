package milestone

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
	for _, stmt := range []string{
		`INSERT INTO products (id, name) VALUES (1, 'Demo')`,
		`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (42, 'boss@x.io', 'boss', 'x', 'admin', 1)`,
		`INSERT INTO versions (id, value, product_id) VALUES (5, '1.0', 1)`,
		`INSERT INTO builds (id, name, version_id) VALUES (1, 'b1', 5)`,
		`INSERT INTO test_plans (id, name, product_id, version_id) VALUES (3, 'Plan', 1, 5)`,
		`INSERT INTO test_runs (id, test_plan_id, build_id, summary) VALUES (7, 3, 1, 'Run A')`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return NewStore(sqlDB, nil)
}

func TestMilestoneCRUDAndSignoff(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	m, err := s.Create(ctx, MilestoneInput{VersionID: 5, Name: "M1", Status: "planned"})
	if err != nil {
		t.Fatal(err)
	}
	if m.ID == 0 || m.Status != "planned" {
		t.Fatalf("unexpected milestone: %+v", m)
	}

	list, err := s.List(ctx, 5)
	if err != nil || len(list) != 1 {
		t.Fatalf("List(version 5): %d err=%v", len(list), err)
	}
	list, err = s.List(ctx, 0) // all versions
	if err != nil || len(list) != 1 {
		t.Fatalf("List(all): %d err=%v", len(list), err)
	}

	upd, err := s.Update(ctx, m.ID, MilestoneInput{VersionID: 5, Name: "M1 renamed", Status: "in_progress"})
	if err != nil || upd.Name != "M1 renamed" || upd.Status != "in_progress" {
		t.Fatalf("Update: %+v err=%v", upd, err)
	}

	signed, err := s.Signoff(ctx, m.ID, 42, "go", "LGTM")
	if err != nil || signed.SignoffDecision == nil || *signed.SignoffDecision != "go" {
		t.Fatalf("Signoff: %+v err=%v", signed, err)
	}
	if signed.SignoffUserID == nil || *signed.SignoffUserID != 42 {
		t.Fatalf("Signoff user not recorded: %+v", signed)
	}

	if err := s.Delete(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, m.ID); err == nil {
		t.Error("expected error for deleted milestone")
	}
}

func TestMilestoneRunsAndProgress(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	m, err := s.Create(ctx, MilestoneInput{VersionID: 5, Name: "M2", Status: "in_progress"})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.LinkRun(ctx, m.ID, 7); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRun(ctx, m.ID, 7); err != nil {
		t.Fatalf("re-link should be idempotent: %v", err)
	}
	runs, err := s.Runs(ctx, m.ID)
	if err != nil || len(runs) != 1 || runs[0] != 7 {
		t.Fatalf("Runs: %v err=%v", runs, err)
	}

	prog, err := s.Progress(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prog.MilestoneID != m.ID || prog.Total != 0 || prog.PercentDone == "" {
		t.Fatalf("Progress with no executions: %+v", prog)
	}

	if err := s.UnlinkRun(ctx, m.ID, 7); err != nil {
		t.Fatal(err)
	}
	runs, _ = s.Runs(ctx, m.ID)
	if len(runs) != 0 {
		t.Fatalf("expected no runs after unlink, got %v", runs)
	}
}
