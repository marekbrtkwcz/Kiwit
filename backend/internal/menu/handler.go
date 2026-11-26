// Package menu provides a configurable navigation menu API (MENU_ITEMS /
// HELP_MENU_ITEMS equivalent from Kiwi TCMS). Menu structure can be overridden
// via the KIWIT_MENU_JSON environment variable (JSON) or falls back to defaults.
package menu

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
)

// Item is a single menu entry (leaf or separator "-").
type Item struct {
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

// Group is a top-level menu section containing items (which may be nested).
type Group struct {
	Label string `json:"label"`
	Items []any  `json:"items"` // []Item or nested []Group — flattened from JSON
}

// MenuResponse is the full menu structure returned by GET /api/v1/menu.
type MenuResponse struct {
	Main []Group `json:"main"`
	Help []Item  `json:"help"`
}

// Default returns the built-in menu matching the Kiwit frontend routes.
func Default() MenuResponse {
	return MenuResponse{
		Main: []Group{
			{Label: "TESTING", Items: []any{
				Item{Label: "New Test Plan", URL: "/plans?new=1", Icon: "folder-kanban"},
				Item{Label: "-", URL: "-"},
				Item{Label: "New Test Case", URL: "/cases?new=1", Icon: "file-check"},
				Item{Label: "-", URL: "-"},
				Item{Label: "New Test Run", URL: "/runs?new=1", Icon: "play-circle"},
				Item{Label: "-", URL: "-"},
				Item{Label: "New Bug", URL: "/bugs?new=1", Icon: "bug"},
			}},
			{Label: "SEARCH", Items: []any{
				Item{Label: "Search Test Plans", URL: "/plans", Icon: "folder-kanban"},
				Item{Label: "Search Test Cases", URL: "/cases", Icon: "file-check"},
				Item{Label: "Search Test Runs", URL: "/runs", Icon: "play-circle"},
				Item{Label: "Search Test Executions", URL: "/dashboard", Icon: "layout-dashboard"},
				Item{Label: "Search Bugs", URL: "/bugs", Icon: "bug"},
			}},
			{Label: "NAVIGATE", Items: []any{
				Item{Label: "Dashboard", URL: "/dashboard", Icon: "layout-dashboard"},
				Item{Label: "Products", URL: "/products", Icon: "package"},
				Item{Label: "Settings", URL: "/settings", Icon: "settings"},
				Item{Label: "Account", URL: "/auth", Icon: "user-circle"},
			}},
		},
		Help: []Item{
			{Label: "Report an Issue", URL: "https://github.com/kiwit/kiwit/issues/new/choose"},
			{Label: "Administration Guide", URL: "/docs/admin.md"},
			{Label: "User Guide", URL: "/docs/user.md"},
			{Label: "API Help", URL: "/docs/xmlrpc-api.md"},
		},
	}
}

// Load returns the menu from KIWIT_MENU_JSON env var, or defaults if unset/invalid.
func Load() MenuResponse {
	raw := os.Getenv("KIWIT_MENU_JSON")
	if raw == "" {
		return Default()
	}
	var m MenuResponse
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return Default()
	}
	return m
}

// Register mounts GET /api/v1/menu on the router.
func Register(r chi.Router) {
	r.Get("/api/v1/menu", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(Load())
	})
}
