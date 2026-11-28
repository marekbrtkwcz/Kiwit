package xmlrpc

// Tests for the field-lookup WHERE builder.

import (
	"strings"
	"testing"
)

func TestParseLookupKey(t *testing.T) {
	tests := []struct {
		key   string
		field string
		op    string
	}{
		{"id", "id", "exact"},
		{"summary", "summary", "exact"},
		{"summary__startswith", "summary", "startswith"},
		{"summary__contains", "summary", "contains"},
		{"summary__icontains", "summary", "icontains"},
		{"id__gte", "id", "gte"},
		{"id__lte", "id", "lte"},
		{"id__gt", "id", "gt"},
		{"id__lt", "id", "lt"},
		{"id__in", "id", "in"},
		{"deleted_at__isnull", "deleted_at", "isnull"},
		{"id__range", "id", "range"},
		{"name__endswith", "name", "endswith"},
		{"product__name", "product__name", "exact"}, // relation, not a lookup op
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			f, op := parseLookupKey(tc.key)
			if f != tc.field || op != tc.op {
				t.Errorf("parseLookupKey(%q) = (%q, %q), want (%q, %q)", tc.key, f, op, tc.field, tc.op)
			}
		})
	}
}

func TestBuildWhereExact(t *testing.T) {
	res, _ := buildWhere(map[string]any{"id": int64(5)}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"id\" = ?") {
		t.Errorf("expected t.\"id\" = ?, got: %s", res.where)
	}
	if len(res.args) != 1 || res.args[0] != int64(5) {
		t.Errorf("expected args [5], got: %v", res.args)
	}
}

func TestBuildWhereStartswith(t *testing.T) {
	res, _ := buildWhere(map[string]any{"summary__startswith": "test"}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"summary\" LIKE ?") {
		t.Errorf("expected LIKE, got: %s", res.where)
	}
	if len(res.args) != 1 || res.args[0] != "test%" {
		t.Errorf("expected arg 'test%%', got: %v", res.args)
	}
}

func TestBuildWhereContains(t *testing.T) {
	res, _ := buildWhere(map[string]any{"summary__contains": "login"}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"summary\" LIKE ?") {
		t.Errorf("expected LIKE, got: %s", res.where)
	}
	if res.args[0] != "%login%" {
		t.Errorf("expected '%%login%%', got: %v", res.args[0])
	}
}

func TestBuildWhereIContains(t *testing.T) {
	res, _ := buildWhere(map[string]any{"email__icontains": "@Example.COM"}, "t", nil, nil)
	if !strings.Contains(res.where, "LOWER(t.\"email\") LIKE LOWER(?)") {
		t.Errorf("expected LOWER LIKE, got: %s", res.where)
	}
}

func TestBuildWhereGte(t *testing.T) {
	res, _ := buildWhere(map[string]any{"id__gte": int64(100)}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"id\" >= ?") {
		t.Errorf("expected >=, got: %s", res.where)
	}
}

func TestBuildWhereIn(t *testing.T) {
	res, _ := buildWhere(map[string]any{"id__in": []any{int64(1), int64(2), int64(3)}}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"id\" IN (?,?,?)") {
		t.Errorf("expected IN (?,?,?), got: %s", res.where)
	}
	if len(res.args) != 3 {
		t.Errorf("expected 3 args, got: %v", res.args)
	}
}

func TestBuildWhereInEmpty(t *testing.T) {
	res, _ := buildWhere(map[string]any{"id__in": []any{}}, "t", nil, nil)
	if !strings.Contains(res.where, "1=0") {
		t.Errorf("expected 1=0 for empty IN, got: %s", res.where)
	}
}

func TestBuildWhereIsNull(t *testing.T) {
	res, _ := buildWhere(map[string]any{"stopped_at__isnull": true}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"stopped_at\" IS NULL") {
		t.Errorf("expected IS NULL, got: %s", res.where)
	}
	res2, _ := buildWhere(map[string]any{"stopped_at__isnull": false}, "t", nil, nil)
	if !strings.Contains(res2.where, "t.\"stopped_at\" IS NOT NULL") {
		t.Errorf("expected IS NOT NULL, got: %s", res2.where)
	}
}

func TestBuildWhereRange(t *testing.T) {
	res, _ := buildWhere(map[string]any{"id__range": []any{int64(10), int64(20)}}, "t", nil, nil)
	if !strings.Contains(res.where, "t.\"id\" BETWEEN ? AND ?") {
		t.Errorf("expected BETWEEN, got: %s", res.where)
	}
	if len(res.args) != 2 {
		t.Errorf("expected 2 args, got: %v", res.args)
	}
}

func TestBuildWhereMultiple(t *testing.T) {
	res, _ := buildWhere(map[string]any{
		"summary__startswith": "test",
		"id__gte":             int64(10),
		"is_active":           true,
	}, "t", nil, nil)
	if !strings.Contains(res.where, "AND") {
		t.Errorf("expected AND between clauses, got: %s", res.where)
	}
	// Should have 3 clauses
	parts := strings.Split(res.where, " AND ")
	if len(parts) != 3 {
		t.Errorf("expected 3 clauses, got %d: %s", len(parts), res.where)
	}
}

func TestBuildWhereEmpty(t *testing.T) {
	res, _ := buildWhere(map[string]any{}, "t", nil, nil)
	if res.where != "1=1" {
		t.Errorf("expected 1=1, got: %s", res.where)
	}
}

func TestBuildWhereRelation(t *testing.T) {
	relations := map[string]relationJoin{
		"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
	}
	res, _ := buildWhere(map[string]any{"product__name__startswith": "My"}, "t", relations, nil)
	if !strings.Contains(res.where, "p.\"name\" LIKE ?") {
		t.Errorf("expected p.name LIKE, got: %s", res.where)
	}
	if res.args[0] != "My%" {
		t.Errorf("expected 'My%%', got: %v", res.args[0])
	}
}

func TestJoinsForQuery(t *testing.T) {
	relations := map[string]relationJoin{
		"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
		"author":  {joinSQL: "JOIN users u ON u.id = t.author_id", alias: "u"},
	}
	joins := joinsForQuery(map[string]any{
		"product__name":                "X",
		"author__username__startswith": "ad",
		"summary":                      "test",
	}, relations)
	if len(joins) != 2 {
		t.Errorf("expected 2 joins, got %d: %v", len(joins), joins)
	}
}
