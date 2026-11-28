// Package permissions implements the refresh-permissions admin endpoint
// (equivalent of kiwitcms `manage.py refresh_permissions`): ensures the
// default groups (Administrator, Tester) exist and assigns them default
// per-product roles.
package permissions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
)

// Store handles default group/permission setup.
type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// DefaultGroups are the built-in groups, mirroring kiwitcms DEFAULT_GROUPS.
var DefaultGroups = []struct {
	Name        string
	Description string
}{
	{"Administrator", "Full access to all products and admin functions"},
	{"Tester", "Can create/edit test cases, plans, runs (default group for new users)"},
}

// Register mounts POST /api/v1/admin/refresh-permissions (admin-only via
// role check in the handler context).
func Register(r chi.Router, s *Store) {
	r.Post("/api/v1/admin/refresh-permissions", s.handleRefresh)
}

type refreshResult struct {
	OK             bool     `json:"ok"`
	GroupsCreated  []string `json:"groups_created"`
	GroupsExisting []string `json:"groups_existing"`
	Message        string   `json:"message"`
}

func (s *Store) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Admin-only, fail-closed: role is read via the typed context key set by
	// auth.RequireAuth — a plain-string key would never match and fail open.
	u, ok := auth.UserFromContext(r.Context())
	if !ok || u.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
		return
	}
	res, err := s.Refresh(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// Refresh ensures default groups exist and assigns Tester as the default
// group role for all products without explicit group permissions.
func (s *Store) Refresh(ctx context.Context) (refreshResult, error) {
	res := refreshResult{OK: true}
	for _, g := range DefaultGroups {
		exists, err := s.groupExists(ctx, g.Name)
		if err != nil {
			return res, fmt.Errorf("check group %s: %w", g.Name, err)
		}
		if exists {
			res.GroupsExisting = append(res.GroupsExisting, g.Name)
			continue
		}
		if err := s.createGroup(ctx, g.Name); err != nil {
			return res, fmt.Errorf("create group %s: %w", g.Name, err)
		}
		res.GroupsCreated = append(res.GroupsCreated, g.Name)
	}
	// Assign Tester group as 'tester' role on all products lacking a group permission
	if err := s.assignTesterToAllProducts(ctx); err != nil {
		return res, fmt.Errorf("assign tester defaults: %w", err)
	}
	res.Message = "Default groups refreshed. Administrator = full access, Tester = read/write on all products."
	return res, nil
}

func (s *Store) groupExists(ctx context.Context, name string) (bool, error) {
	var count int
	var err error
	if s.pg != nil {
		err = s.pg.QueryRow(ctx, `SELECT COUNT(*) FROM user_groups WHERE name = $1`, name).Scan(&count)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_groups WHERE name = ?`, name).Scan(&count)
	}
	return count > 0, err
}

func (s *Store) createGroup(ctx context.Context, name string) error {
	now := time.Now().UTC()
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `INSERT INTO user_groups (name, created_at) VALUES ($1, $2)`, name, now)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_groups (name, created_at) VALUES (?, ?)`, name, now.Format("2006-01-02 15:04:05"))
	return err
}

// assignTesterToAllProducts gives the Tester group 'tester' role on every
// product that has no group_product_permissions row for Tester yet.
func (s *Store) assignTesterToAllProducts(ctx context.Context) error {
	var testerID int64
	var err error
	if s.pg != nil {
		err = s.pg.QueryRow(ctx, `SELECT id FROM user_groups WHERE name = 'Tester'`).Scan(&testerID)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT id FROM user_groups WHERE name = 'Tester'`).Scan(&testerID)
	}
	if err != nil {
		return err // Tester group should exist after createGroup
	}
	q := `
		INSERT INTO group_product_permissions (group_id, product_id, role)
		SELECT ?, p.id, 'tester' FROM products p
		WHERE NOT EXISTS (
			SELECT 1 FROM group_product_permissions gpp
			WHERE gpp.group_id = ? AND gpp.product_id = p.id
		)`
	if s.pg != nil {
		_, err = s.pg.Exec(ctx, q, testerID, testerID)
	} else {
		_, err = s.db.ExecContext(ctx, q, testerID, testerID)
	}
	return err
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
