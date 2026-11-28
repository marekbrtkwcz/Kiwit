package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE instance_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestGetSetDomain(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)
	ctx := context.Background()

	// Initially empty
	val, err := s.Get(ctx, KeyDomain)
	if err != nil || val != "" {
		t.Fatalf("expected empty, got %q err=%v", val, err)
	}

	// Set
	if err := s.Set(ctx, KeyDomain, "https://kiwit.example.com"); err != nil {
		t.Fatal(err)
	}
	val, _ = s.Get(ctx, KeyDomain)
	if val != "https://kiwit.example.com" {
		t.Errorf("expected domain, got %q", val)
	}

	// Update
	_ = s.Set(ctx, KeyDomain, "https://new.example.com")
	val, _ = s.Get(ctx, KeyDomain)
	if val != "https://new.example.com" {
		t.Errorf("expected updated domain, got %q", val)
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct{ in, want string }{
		{"kiwit.example.com", "https://kiwit.example.com"},
		{"http://localhost:8080", "http://localhost:8080"},
		{"https://kiwit.example.com/", "https://kiwit.example.com"},
		{"  kiwit.example.com  ", "https://kiwit.example.com"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := NormalizeDomain(tc.in); got != tc.want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDomainEndpoint(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	changed := ""
	Register(r, NewStore(db, nil))
	OnChange(KeyDomain, func(d string) { changed = d })

	// PUT as admin
	body := `{"domain": "kiwit.example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/domain", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "admin"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["domain"] != "https://kiwit.example.com" {
		t.Errorf("expected normalized domain, got %q", resp["domain"])
	}
	if changed != "https://kiwit.example.com" {
		t.Errorf("expected onDomainChange called, got %q", changed)
	}

	// GET public
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/settings/domain", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET status = %d", rec2.Code)
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp["domain"] != "https://kiwit.example.com" {
		t.Errorf("expected domain from DB, got %q", resp["domain"])
	}
}

func TestDomainEndpointForbidden(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	Register(r, NewStore(db, nil))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/domain", strings.NewReader(`{"domain":"x.com"}`))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "tester"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestDomainEndpointBadRequest(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	Register(r, NewStore(db, nil))

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/domain", strings.NewReader(`{"domain":""}`))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "admin"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestBulkUpdateAndCallbacks(t *testing.T) {
	db := setupTestDB(t)
	r := chi.NewRouter()
	Register(r, NewStore(db, nil))

	var gotAllowReg, gotAnon string
	OnChange(KeyAllowRegistration, func(v string) { gotAllowReg = v })
	OnChange(KeyAnonymousAccess, func(v string) { gotAnon = v })

	body := `{"settings": {"allow_registration": "false", "anonymous_access": "true", "unknown_key": "x"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "admin"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotAllowReg != "false" || gotAnon != "true" {
		t.Errorf("callbacks not fired: allowReg=%q anon=%q", gotAllowReg, gotAnon)
	}

	// Verify persisted
	s := NewStore(db, nil)
	v, _ := s.Get(context.Background(), KeyAllowRegistration)
	if v != "false" {
		t.Errorf("expected allow_registration=false in DB, got %q", v)
	}
}

func TestPublicSettingsList(t *testing.T) {
	db := setupTestDB(t)
	_ = NewStore(db, nil).Set(context.Background(), KeyDomain, "https://kiwit.test")
	_ = NewStore(db, nil).Set(context.Background(), KeySMTPHost, "secret-host")

	r := chi.NewRouter()
	Register(r, NewStore(db, nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp[KeyDomain] != "https://kiwit.test" {
		t.Errorf("expected domain in public list, got %v", resp)
	}
	if _, has := resp[KeySMTPHost]; has {
		t.Error("smtp_host should NOT be in public list")
	}
}

func TestMaskedPassword(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)
	_ = s.Set(context.Background(), KeySMTPPass, "supersecret")

	r := chi.NewRouter()
	Register(r, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "admin"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp[KeySMTPPass] != "********" {
		t.Errorf("expected masked password, got %q", resp[KeySMTPPass])
	}
}

// Regression: the admin UI echoes the masked smtp_pass ("********") back in
// bulk updates — that must never overwrite the stored secret.
func TestBulkUpdateSkipsMaskedSMTPPass(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)
	if err := s.Set(context.Background(), KeySMTPPass, "real-secret"); err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	Register(r, s)

	body := `{"settings": {"smtp_pass": "********", "allow_registration": "true"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "admin"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	stored, _ := s.Get(context.Background(), KeySMTPPass)
	if stored != "real-secret" {
		t.Errorf("masked placeholder overwrote the secret: %q", stored)
	}
	reg, _ := s.Get(context.Background(), KeyAllowRegistration)
	if reg != "true" {
		t.Errorf("allow_registration should have been updated, got %q", reg)
	}
}

// Empty smtp_pass must also be ignored (not stored as empty string).
func TestBulkUpdateSkipsEmptySMTPPass(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)
	_ = s.Set(context.Background(), KeySMTPPass, "real-secret")
	r := chi.NewRouter()
	Register(r, s)

	body := `{"settings": {"smtp_pass": ""}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), auth.User{Role: "admin"}))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	stored, _ := s.Get(context.Background(), KeySMTPPass)
	if stored != "real-secret" {
		t.Errorf("empty value overwrote the secret: %q", stored)
	}
}

// GET /api/v1/settings must report effective values: DB value if set,
// otherwise the env default registered via SetEnvDefaults.
func TestPublicSettingsEffectiveDefaults(t *testing.T) {
	db := setupTestDB(t)
	s := NewStore(db, nil)
	SetEnvDefaults(map[string]string{KeyAllowRegistration: "true", KeyCaptchaEnabled: "false", KeyAnonymousAccess: "false"})
	r := chi.NewRouter()
	Register(r, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp[KeyAllowRegistration] != "true" {
		t.Errorf("effective allow_registration should default to true, got %q", resp[KeyAllowRegistration])
	}
	if resp[KeyCaptchaEnabled] != "false" {
		t.Errorf("effective captcha_enabled should default to false, got %q", resp[KeyCaptchaEnabled])
	}
	// DB value wins over the env default
	_ = s.Set(context.Background(), KeyCaptchaEnabled, "true")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp[KeyCaptchaEnabled] != "true" {
		t.Errorf("DB value should win over env default, got %q", resp[KeyCaptchaEnabled])
	}
}
