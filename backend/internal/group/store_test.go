package group

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
		`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (1, 'a@x.io', 'alice', 'x', 'tester', 1)`,
		`INSERT INTO users (id, email, username, password_hash, role, is_active) VALUES (2, 'b@x.io', 'bob', 'x', 'viewer', 1)`,
		`INSERT INTO products (id, name) VALUES (1, 'Demo')`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	return NewStore(sqlDB, nil)
}

func TestGroupCRUD(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	g, err := s.Create(ctx, "QA Team")
	if err != nil {
		t.Fatal(err)
	}
	if g.ID == 0 || g.Name != "QA Team" {
		t.Fatalf("unexpected group: %+v", g)
	}

	list, err := s.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List: %d err=%v", len(list), err)
	}

	if err := s.Delete(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = s.List(ctx)
	if len(list) != 0 {
		t.Fatalf("expected no groups after delete, got %d", len(list))
	}
}

func TestGroupMembers(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	g, _ := s.Create(ctx, "Devs")

	if err := s.AddMember(ctx, g.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, g.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.AddMember(ctx, g.ID, 1); err != nil {
		t.Fatalf("duplicate add should be idempotent: %v", err)
	}

	members, err := s.Members(ctx, g.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("Members: %v err=%v", members, err)
	}

	groups, err := s.GroupsForUser(ctx, 1)
	if err != nil || len(groups) != 1 || groups[0] != g.ID {
		t.Fatalf("GroupsForUser: %v err=%v", groups, err)
	}

	if err := s.RemoveMember(ctx, g.ID, 1); err != nil {
		t.Fatal(err)
	}
	members, _ = s.Members(ctx, g.ID)
	if len(members) != 1 || members[0] != 2 {
		t.Fatalf("after remove: %v", members)
	}
}

func TestGroupProductPerms(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	g, _ := s.Create(ctx, "Admins")

	if err := s.SetProductPerm(ctx, g.ID, 1, "maintainer"); err != nil {
		t.Fatal(err)
	}
	// Upsert to another role
	if err := s.SetProductPerm(ctx, g.ID, 1, "viewer"); err != nil {
		t.Fatal(err)
	}

	perms, err := s.ListProductPerms(ctx, g.ID)
	if err != nil || len(perms) != 1 || perms[0].Role != "viewer" {
		t.Fatalf("ListProductPerms after upsert: %+v err=%v", perms, err)
	}

	if err := s.DeleteProductPerm(ctx, g.ID, 1); err != nil {
		t.Fatal(err)
	}
	perms, _ = s.ListProductPerms(ctx, g.ID)
	if len(perms) != 0 {
		t.Fatalf("expected no perms after delete, got %+v", perms)
	}
}
