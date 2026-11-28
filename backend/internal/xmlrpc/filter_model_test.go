package xmlrpc

import (
	"context"
	"reflect"
	"testing"
)

func TestSanitizeQuery(t *testing.T) {
	q := map[string]any{
		"summary__contains":         "login",
		"id__gte":                   float64(5),
		"evil; DROP TABLE users--":  "x",
		"product__name__startswith": "Kiwit",
		"ids__in":                   []any{1, "2", true, []any{"nested"}},
		"weird":                     map[string]any{"nested": 1},
	}
	got := sanitizeQuery(q)
	want := map[string]any{
		"summary__contains":         "login",
		"id__gte":                   float64(5),
		"product__name__startswith": "Kiwit",
		"ids__in":                   []any{1, "2", true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sanitizeQuery = %#v, want %#v", got, want)
	}
}

func TestRebindPlaceholders(t *testing.T) {
	got := rebindPlaceholders("SELECT * FROM t WHERE a = ? AND b IN (?,?) AND c LIKE ?")
	want := "SELECT * FROM t WHERE a = $1 AND b IN ($2,$3) AND c LIKE $4"
	if got != want {
		t.Errorf("rebindPlaceholders = %q, want %q", got, want)
	}
}

func TestFilterModelWhitelist(t *testing.T) {
	if _, err := FilterModel(context.Background(), nil, nil, "nope", nil, 0); err == nil {
		t.Error("expected error for unknown model")
	}
	for m := range FilterModels {
		if _, err := FilterModel(context.Background(), nil, nil, m, nil, 0); err == nil {
			t.Errorf("model %s: expected error with nil DBs", m)
		}
	}
}
