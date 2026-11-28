// Package initdb provides the /init-db/ endpoint (Kiwi TCMS compatible)
// for first-time database initialization and admin-only database reset.
package initdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// Store handles init-db operations against SQLite or PostgreSQL.
type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// Register mounts GET/POST /init-db/ and POST /api/v1/admin/reset-db on the router.
// The reset-db route requires an authenticated admin — guard is applied by caller
// via chi middleware, here we just check the role from context if present.
func Register(r chi.Router, s *Store) {
	r.Get("/init-db/", s.handleStatus)
	r.Post("/init-db/", s.handleInit)
	r.Post("/api/v1/admin/reset-db", s.handleReset)
}

type statusResponse struct {
	Initialized bool   `json:"initialized"`
	UserCount   int    `json:"user_count"`
	Message     string `json:"message"`
}

// handleStatus — GET /init-db/ — checks if DB has users. If yes, redirect to /.
func (s *Store) handleStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.userCount(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if count > 0 {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		Initialized: false,
		UserCount:   count,
		Message:     "Database not initialized. POST to /init-db/ to initialize.",
	})
}

type initRequest struct {
	AdminEmail    string `json:"admin_email"`
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

type initResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Admin   string `json:"admin,omitempty"`
}

// handleInit — POST /init-db/ — creates the first admin user + seed data.
// If DB already has users, returns 409 Conflict.
func (s *Store) handleInit(w http.ResponseWriter, r *http.Request) {
	count, err := s.userCount(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if count > 0 {
		writeJSON(w, http.StatusConflict, initResponse{
			OK:      false,
			Message: "Database already initialized. Use /api/v1/admin/reset-db to reset.",
		})
		return
	}

	var req initRequest
	// Body is optional — fall back to env vars / defaults
	_ = json.NewDecoder(r.Body).Decode(&req)

	email := req.AdminEmail
	if email == "" {
		email = os.Getenv("KIWIT_ADMIN_EMAIL")
	}
	if email == "" {
		email = "admin@kiwit.local"
	}
	username := req.AdminUsername
	if username == "" {
		username = os.Getenv("KIWIT_ADMIN_USERNAME")
	}
	if username == "" {
		username = "admin"
	}
	password := req.AdminPassword
	if password == "" {
		password = os.Getenv("KIWIT_ADMIN_PASSWORD")
	}
	if password == "" {
		password = "admin"
	}

	if err := s.seedAdmin(r.Context(), email, username, password); err != nil {
		writeJSON(w, http.StatusInternalServerError, initResponse{OK: false, Message: err.Error()})
		return
	}
	if err := s.seedDictionaries(r.Context()); err != nil {
		// non-fatal — dictionaries may already exist
		_ = err
	}
	if err := s.seedTemplatesAndFields(r.Context()); err != nil {
		_ = err
	}

	writeJSON(w, http.StatusOK, initResponse{
		OK:      true,
		Message: "Database initialized successfully.",
		Admin:   username,
	})
}

// handleReset — POST /api/v1/admin/reset-db — wipes all data and re-seeds.
// Requires admin role (checked via context value set by auth middleware).
func (s *Store) handleReset(w http.ResponseWriter, r *http.Request) {
	// Admin-only, fail-closed: the role is read via the typed context key set
	// by auth.RequireAuth — a plain-string key would never match and fail open.
	u, ok := auth.UserFromContext(r.Context())
	if !ok || u.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "admin role required"})
		return
	}
	if err := s.wipeAll(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if err := s.seedDictionaries(r.Context()); err != nil {
		_ = err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "Database reset. All data wiped, dictionaries re-seeded. Re-run /init-db/ to create admin.",
	})
}

func (s *Store) userCount(ctx context.Context) (int, error) {
	var count int
	var err error
	if s.pg != nil {
		err = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	}
	return count, err
}

func (s *Store) seedAdmin(ctx context.Context, email, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	now := time.Now().UTC()
	if s.pg != nil {
		_, err = s.pg.Exec(ctx, `
			INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, 'admin', TRUE, $4, $5)`,
			email, username, string(hash), now, now)
	} else {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO users (email, username, password_hash, role, is_active, created_at, updated_at)
			VALUES (?, ?, ?, 'admin', 1, ?, ?)`,
			email, username, string(hash), now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))
	}
	return err
}

// seedDictionaries inserts default categories, priorities, statuses if tables are empty.
func (s *Store) seedDictionaries(ctx context.Context) error {
	stmts := []string{
		`INSERT OR IGNORE INTO categories (name) VALUES ('Functional'), ('Web API'), ('Performance'), ('UX / Usability'), ('Security'), ('Smoke'), ('Regression')`,
		`INSERT OR IGNORE INTO priorities (value, is_active) VALUES ('low', 1), ('medium', 1), ('high', 1), ('critical', 1)`,
		`INSERT OR IGNORE INTO execution_statuses (name, color, weight) VALUES ('idle', '#9ca3af', 0), ('running', '#2563eb', 10), ('passed', '#16a34a', 20), ('failed', '#dc2626', -20), ('blocked', '#d97706', -10), ('skipped', '#6b7280', 0)`,
	}
	for _, q := range stmts {
		if s.pg != nil {
			// PostgreSQL has no INSERT OR IGNORE — translate to ON CONFLICT DO NOTHING.
			q = strings.Replace(q, "INSERT OR IGNORE INTO", "INSERT INTO", 1) + " ON CONFLICT DO NOTHING"
			if _, err := s.pg.Exec(ctx, q); err != nil {
				return err
			}
		} else {
			if _, err := s.db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
	}
	return nil
}


// seedTemplatesAndFields inserts built-in test templates (OWASP security audit)
// and UX metric custom fields for test executions.
// seedTemplatesAndFields inserts built-in test templates (OWASP security audit)
// and UX metric custom fields for test executions.
func (s *Store) seedTemplatesAndFields(ctx context.Context) error {
	owaspBody := "OWASP Top 10 - one line per class:\nA01 Broken Access Control\nA02 Cryptographic Failures\nA03 Injection\nA04 Insecure Design\nA05 Security Misconfiguration\nA06 Vulnerable Components\nA07 Auth Failures\nA08 Data Integrity Failures\nA09 Logging/Monitoring Failures\nA10 SSRF"
	if s.pg != nil {
		if _, err := s.pg.Exec(ctx, `INSERT INTO templates (name, body) VALUES ($1, $2) ON CONFLICT DO NOTHING`, "OWASP Top 10 Audit (checklist)", owaspBody); err != nil {
			return err
		}
		if _, err := s.pg.Exec(ctx, `INSERT INTO custom_field_defs (entity_type, name, field_type, options, required) VALUES
			('test_execution', 'sus_score', 'number', '[]', FALSE),
			('test_execution', 'task_time_sec', 'number', '[]', FALSE),
			('test_execution', 'error_count', 'number', '[]', FALSE),
			('test_execution', 'heuristic_findings', 'text', '[]', FALSE) ON CONFLICT DO NOTHING`); err != nil {
			return err
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO templates (name, body) VALUES (?, ?)`, "OWASP Top 10 Audit (checklist)", owaspBody); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO custom_field_defs (entity_type, name, field_type, options, required) VALUES
		('test_execution', 'sus_score', 'number', '[]', 0),
		('test_execution', 'task_time_sec', 'number', '[]', 0),
		('test_execution', 'error_count', 'number', '[]', 0),
		('test_execution', 'heuristic_findings', 'text', '[]', 0)`); err != nil {
		return err
	}
	return nil
}

// wipeAll deletes all rows from data tables (keeps schema). Order matters for FK constraints.
func (s *Store) wipeAll(ctx context.Context) error {
	tables := []string{
		"audit_log", "link_references", "entity_properties", "group_product_permissions",
		"user_group_members", "test_execution_tags", "test_run_tags", "test_plan_tags",
		"test_case_tags", "test_case_components", "test_plan_cases", "test_executions",
		"bugs", "internal_bugs", "attachments", "comments", "notifications",
		"user_notification_settings", "webhooks", "password_reset_tokens",
		"api_tokens", "test_runs", "test_cases", "test_case_steps", "test_case_revisions",
		"test_plans", "builds", "versions", "components", "products", "categories",
		"environments", "templates", "issue_trackers", "user_groups", "users",
	}
	for _, t := range tables {
		var err error
		if s.pg != nil {
			_, err = s.pg.Exec(ctx, fmt.Sprintf(`DELETE FROM %s`, t))
		} else {
			_, err = s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s`, t))
		}
		if err != nil {
			return fmt.Errorf("wipe %s: %w", t, err)
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
