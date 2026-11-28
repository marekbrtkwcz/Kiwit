// Package settings provides a key-value store for instance-level settings
// (equivalent of Django's Site model / kiwitcms admin-editable settings).
// All keys can be changed at runtime via the admin API and take effect
// immediately through registered onChange callbacks.
package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
)

// Store persists instance settings in SQLite or PostgreSQL.
type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// Known setting keys (editable at runtime via admin API).
const (
	KeyDomain            = "domain"
	KeySMTPHost          = "smtp_host"
	KeySMTPPort          = "smtp_port"
	KeySMTPUser          = "smtp_user"
	KeySMTPPass          = "smtp_pass"
	KeySMTPFrom          = "smtp_from"
	KeyAllowRegistration = "allow_registration"
	KeyCaptchaEnabled    = "captcha_enabled"
	KeyAnonymousAccess   = "anonymous_access"
)

// SettingDef describes an editable setting: its key, whether it is public
// (readable without auth), and an optional normalizer.
type SettingDef struct {
	Key       string
	Public    bool // readable via GET /api/v1/settings/{key} without auth
	Normalize func(string) string
}

// Defs is the catalog of all runtime-editable settings.
var Defs = []SettingDef{
	{Key: KeyDomain, Public: true, Normalize: NormalizeDomain},
	{Key: KeySMTPHost, Public: false},
	{Key: KeySMTPPort, Public: false, Normalize: func(s string) string {
		s = strings.TrimSpace(s)
		if s == "" {
			return "587"
		}
		return s
	}},
	{Key: KeySMTPUser, Public: false},
	{Key: KeySMTPPass, Public: false}, // write-only-ish: returned masked
	{Key: KeySMTPFrom, Public: false},
	{Key: KeyAllowRegistration, Public: true, Normalize: NormalizeBool},
	{Key: KeyCaptchaEnabled, Public: true, Normalize: NormalizeBool},
	{Key: KeyAnonymousAccess, Public: true, Normalize: NormalizeBool},
}

// NormalizeDomain trims whitespace, adds https:// if no scheme, strips trailing slash.
func NormalizeDomain(d string) string {
	d = strings.TrimSpace(d)
	if d == "" {
		return ""
	}
	if !strings.HasPrefix(d, "http://") && !strings.HasPrefix(d, "https://") {
		d = "https://" + d
	}
	return strings.TrimRight(d, "/")
}

// NormalizeBool parses common truthy/falsy strings.
func NormalizeBool(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "true" || s == "1" || s == "yes" || s == "on" {
		return "true"
	}
	return "false"
}

// envDefaults holds fallback values (seeded from env/config at startup via
// SetEnvDefaults) used when a key has no value in the database yet, so the
// API always reports the *effective* value.
var envDefaults = map[string]string{}

var defaultsMu sync.RWMutex

// SetEnvDefaults registers fallback values for keys with no DB row.
func SetEnvDefaults(d map[string]string) {
	defaultsMu.Lock()
	defer defaultsMu.Unlock()
	for k, v := range d {
		envDefaults[k] = v
	}
}

// GetEffective returns the DB value, falling back to the env default.
func (s *Store) GetEffective(ctx context.Context, key string) string {
	v, err := s.Get(ctx, key)
	if err == nil && v != "" {
		return v
	}
	defaultsMu.RLock()
	defer defaultsMu.RUnlock()
	return envDefaults[key]
}

// Get returns the value for key, or "" if not set.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var val string
	var err error
	if s.pg != nil {
		err = s.pg.QueryRow(ctx, `SELECT value FROM instance_settings WHERE key = $1`, key).Scan(&val)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT value FROM instance_settings WHERE key = ?`, key).Scan(&val)
	}
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// Set upserts a key-value pair.
func (s *Store) Set(ctx context.Context, key, value string) error {
	value = strings.TrimSpace(value)
	now := time.Now().UTC()
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `
			INSERT INTO instance_settings (key, value, updated_at) VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = $3`, key, value, now)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO instance_settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now.Format("2006-01-02 15:04:05"))
	return err
}

// GetAll returns all settings as a map (passwords masked).
func (s *Store) GetAll(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	var rows *sql.Rows
	var err error
	if s.pg != nil {
		rows2, e := s.pg.Query(ctx, `SELECT key, value FROM instance_settings`)
		if e != nil {
			return nil, e
		}
		defer rows2.Close()
		for rows2.Next() {
			var k, v string
			if err := rows2.Scan(&k, &v); err == nil {
				out[k] = v
			}
		}
		return out, rows2.Err()
	}
	rows, err = s.db.QueryContext(ctx, `SELECT key, value FROM instance_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			out[k] = v
		}
	}
	return out, rows.Err()
}

// --- HTTP handlers ---

var (
	mu       sync.RWMutex
	onChange = map[string]func(string){} // key → callback
)

// OnChange registers a callback fired after a setting is changed via PUT.
func OnChange(key string, fn func(string)) {
	mu.Lock()
	defer mu.Unlock()
	onChange[key] = fn
}

// Register mounts the settings API:
//
//	GET  /api/v1/settings            — public: all public settings
//	GET  /api/v1/settings/{key}      — public if def.Public, else admin
//	GET  /api/v1/admin/settings      — admin: all settings (password masked)
//	PUT  /api/v1/admin/settings/{key} — admin: update one setting
//	POST /api/v1/admin/settings       — admin: bulk update {"settings": {"key": "val"}}
func Register(r chi.Router, s *Store) {
	// Public list — returns effective values (DB value, else env default)
	r.Get("/api/v1/settings", func(w http.ResponseWriter, req *http.Request) {
		out := map[string]string{}
		for _, def := range Defs {
			if def.Public {
				out[def.Key] = s.GetEffective(req.Context(), def.Key)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Public read of a single key (if public)
	r.Get("/api/v1/settings/{key}", func(w http.ResponseWriter, req *http.Request) {
		key := chi.URLParam(req, "key")
		def := findDef(key)
		if def == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown setting"})
			return
		}
		if !def.Public && !isAdmin(req) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		val := s.GetEffective(req.Context(), key)
		writeJSON(w, http.StatusOK, map[string]string{"key": key, "value": maskIfNeeded(key, val)})
	})

	// Admin: list all
	r.Get("/api/v1/admin/settings", func(w http.ResponseWriter, req *http.Request) {
		if !isAdmin(req) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		out := map[string]string{}
		for _, def := range Defs {
			out[def.Key] = maskIfNeeded(def.Key, s.GetEffective(req.Context(), def.Key))
		}
		writeJSON(w, http.StatusOK, out)
	})

	// Admin: update one
	r.Put("/api/v1/admin/settings/{key}", func(w http.ResponseWriter, req *http.Request) {
		if !isAdmin(req) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		key := chi.URLParam(req, "key")
		def := findDef(key)
		if def == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown setting"})
			return
		}
		var body struct {
			Value string `json:"value"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		val := body.Value
		if def.Normalize != nil {
			val = def.Normalize(val)
		}
		if key == KeyDomain && val == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain required"})
			return
		}
		if isMaskedOrEmptyPassword(key, val) {
			// Client echoed back the masked placeholder — keep the stored secret.
			writeJSON(w, http.StatusOK, map[string]string{"key": key, "value": maskIfNeeded(key, s.GetEffective(req.Context(), key)), "message": "unchanged"})
			return
		}
		if err := s.Set(req.Context(), key, val); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		fireOnChange(key, val)
		writeJSON(w, http.StatusOK, map[string]string{"key": key, "value": maskIfNeeded(key, val), "message": "updated"})
	})

	// Admin: bulk update
	r.Post("/api/v1/admin/settings", func(w http.ResponseWriter, req *http.Request) {
		if !isAdmin(req) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		var body struct {
			Settings map[string]string `json:"settings"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		updated := map[string]string{}
		for key, val := range body.Settings {
			def := findDef(key)
			if def == nil {
				continue // skip unknown keys silently
			}
			if isMaskedOrEmptyPassword(key, val) {
				continue // never overwrite the stored SMTP secret with ""/********
			}
			if def.Normalize != nil {
				val = def.Normalize(val)
			}
			if err := s.Set(req.Context(), key, val); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			fireOnChange(key, val)
			updated[key] = maskIfNeeded(key, val)
		}
		writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "count": len(updated)})
	})

	// Legacy domain endpoints (backwards compat)
	r.Get("/api/v1/settings/domain", func(w http.ResponseWriter, req *http.Request) {
		val, _ := s.Get(req.Context(), KeyDomain)
		writeJSON(w, http.StatusOK, map[string]string{"domain": val})
	})
	r.Get("/api/v1/admin/domain", func(w http.ResponseWriter, req *http.Request) {
		if !isAdmin(req) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		val, _ := s.Get(req.Context(), KeyDomain)
		writeJSON(w, http.StatusOK, map[string]string{"domain": val})
	})
	r.Put("/api/v1/admin/domain", func(w http.ResponseWriter, req *http.Request) {
		if !isAdmin(req) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		var body struct {
			Domain string `json:"domain"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		val := NormalizeDomain(body.Domain)
		if val == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain required"})
			return
		}
		if err := s.Set(req.Context(), KeyDomain, val); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		fireOnChange(KeyDomain, val)
		writeJSON(w, http.StatusOK, map[string]string{"domain": val, "message": "Domain updated successfully."})
	})
}

func findDef(key string) *SettingDef {
	for i := range Defs {
		if Defs[i].Key == key {
			return &Defs[i]
		}
	}
	return nil
}

// maskIfNeeded masks password-like values in API responses.
func maskIfNeeded(key, val string) string {
	if key == KeySMTPPass && val != "" {
		return "********"
	}
	return val
}

// isMaskedOrEmptyPassword reports whether a submitted value for a password
// key is empty or the masked placeholder — such values must never overwrite
// the stored secret.
func isMaskedOrEmptyPassword(key, val string) bool {
	return key == KeySMTPPass && (val == "" || val == "********")
}

func fireOnChange(key, val string) {
	mu.RLock()
	fn, ok := onChange[key]
	mu.RUnlock()
	if ok && fn != nil {
		fn(val)
	}
}

// BoolVal parses a stored bool setting.
func BoolVal(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}

func isAdmin(r *http.Request) bool {
	u, ok := auth.UserFromContext(r.Context())
	return ok && u.Role == "admin"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
