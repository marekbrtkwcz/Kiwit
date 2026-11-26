package menu

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestDefaultMenu(t *testing.T) {
	m := Default()
	if len(m.Main) == 0 {
		t.Fatal("expected default main menu groups")
	}
	if len(m.Help) == 0 {
		t.Fatal("expected default help menu items")
	}
	// Verify expected groups exist
	labels := map[string]bool{}
	for _, g := range m.Main {
		labels[g.Label] = true
	}
	for _, want := range []string{"TESTING", "SEARCH"} {
		if !labels[want] {
			t.Errorf("missing group %q in default menu", want)
		}
	}
}

func TestLoadFromEnv(t *testing.T) {
	custom := `{"main":[{"label":"CUSTOM","items":[{"label":"X","url":"/x"}]}],"help":[{"label":"H","url":"/h"}]}`
	t.Setenv("KIWIT_MENU_JSON", custom)
	m := Load()
	if len(m.Main) != 1 || m.Main[0].Label != "CUSTOM" {
		t.Errorf("expected custom menu, got %+v", m.Main)
	}
}

func TestLoadInvalidEnvFallsBack(t *testing.T) {
	t.Setenv("KIWIT_MENU_JSON", "not json{{{")
	m := Load()
	if len(m.Main) == 0 {
		t.Error("expected fallback to default on invalid JSON")
	}
}

func TestMenuEndpoint(t *testing.T) {
	r := chi.NewRouter()
	Register(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/menu", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp MenuResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Main) == 0 || len(resp.Help) == 0 {
		t.Error("expected non-empty menu response")
	}
}

func TestMenuEndpointWithEnv(t *testing.T) {
	custom := `{"main":[{"label":"ONLY","items":[]}],"help":[]}`
	os.Setenv("KIWIT_MENU_JSON", custom)
	defer os.Unsetenv("KIWIT_MENU_JSON")

	r := chi.NewRouter()
	Register(r)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/menu", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp MenuResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Main) != 1 || resp.Main[0].Label != "ONLY" {
		t.Errorf("expected ONLY group from env, got %+v", resp.Main)
	}
}
