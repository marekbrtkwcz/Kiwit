package xmlrpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	_ "modernc.org/sqlite"
)

func xmlCall(h http.Handler, method, params string) string {
	body := `<?xml version="1.0"?><methodCall><methodName>` + method + `</methodName><params>` + params + `</params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Body.String()
}

// Unauthenticated calls to any non-public RPC method must be rejected.
func TestRPCUnauthenticatedRejected(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	for _, m := range []string{"TestCase.filter", "TestCase.create", "User.filter", "TestRun.filter", "Markdown.render", "Auth.logout"} {
		out := xmlCall(r, m, "")
		if !strings.Contains(out, "fault") || !strings.Contains(out, "authentication required") {
			t.Errorf("%s: expected authentication fault, got: %s", m, out)
		}
	}

	// JSON-RPC variant
	body := `{"jsonrpc":"2.0","method":"TestCase.filter","params":[],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/json-rpc/", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var resp jsonRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != -32001 {
		t.Fatalf("expected -32001 authentication error, got %+v", resp.Error)
	}
}

// Public methods must still work without a token; Auth.login must reach the
// handler (it fails with an auth error, not "authentication required").
func TestRPCPublicMethodsWithoutAuth(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	if out := xmlCall(r, "KiwiTCMS.version", ""); !strings.Contains(out, "Kiwit TCMS") {
		t.Fatalf("KiwiTCMS.version must stay public, got: %s", out)
	}
	out := xmlCall(r, "Auth.login", `<param><value><string>nobody</string></value></param><param><value><string>wrong</string></value></param>`)
	if strings.Contains(out, "authentication required") {
		t.Fatalf("Auth.login must not require prior auth, got: %s", out)
	}
	if !strings.Contains(out, "fault") {
		t.Fatalf("expected auth failure fault from Auth.login, got: %s", out)
	}
}

// Authenticated calls work via the Authorization bearer header.
func TestRPCAuthenticatedBearer(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	body := `<?xml version="1.0"?><methodCall><methodName>Product.filter</methodName><params></params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Demo") {
		t.Fatalf("expected authenticated filter to return rows, got: %s", rec.Body.String())
	}
}

// buildWhere must reject lookup keys that are not whitelisted or contain
// SQL metacharacters, and unknown relation prefixes.
func TestBuildWhereRejectsInjection(t *testing.T) {
	userCols := []string{"t.id", "t.username", "t.email", "COALESCE(t.role,'') AS role", "t.is_active"}
	allowed := deriveAllowed(userCols, nil)

	bad := []string{
		`id" OR "1"="1`,
		`id; DROP TABLE users; --`,
		"password_hash__contains", // not in the SELECT whitelist
		"foo__bar",                // unknown relation
		"id__in__extra",           // nested/garbage
	}
	for _, key := range bad {
		_, err := buildWhere(map[string]any{key: "x"}, "t", nil, allowed)
		if err == nil {
			t.Errorf("expected error for malicious/unallowed key %q", key)
		}
	}

	// Whitelisted key still works and quotes the identifier
	res, err := buildWhere(map[string]any{"username__contains": "adm"}, "t", nil, allowed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.where, `t."username" LIKE ?`) {
		t.Fatalf("expected quoted identifier, got: %s", res.where)
	}
}

// Role-based authorization: a viewer may read but not mutate; admin-only
// methods reject testers; writes require a write role.
func TestRPCRoleAuthorization(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	call := func(token, method, params string) string {
		body := `<?xml version="1.0"?><methodCall><methodName>` + method + `</methodName><params>` + params + `</params></methodCall>`
		req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	viewer := userToken(t, svc, 3, "viewer")
	tester := userToken(t, svc, 2, "tester")
	admin := adminToken(t, svc)

	// viewer: read allowed, write forbidden
	if out := call(viewer, "TestCase.filter", ""); strings.Contains(out, "fault") {
		t.Errorf("viewer must be allowed to filter, got: %s", out)
	}
	if out := call(viewer, "TestCase.create", `<param><value><struct><member><name>summary</name><value><string>x</string></value></member></struct></value></param>`); !strings.Contains(out, "fault") {
		t.Errorf("viewer must NOT create test cases, got: %s", out)
	}
	if out := call(viewer, "TestCase.remove", `<param><value><int>1</int></value></param>`); !strings.Contains(out, "fault") {
		t.Errorf("viewer must NOT remove test cases, got: %s", out)
	}
	// tester: write allowed, admin-only forbidden
	if out := call(tester, "TestCase.create", `<param><value><struct><member><name>summary</name><value><string>x</string></value></member></struct></value></param>`); strings.Contains(out, "fault") {
		t.Errorf("tester must be allowed to create, got: %s", out)
	}
	if out := call(tester, "User.join_group", `<param><value><string>tester</string></value></param><param><value><string>Administrators</string></value></param>`); !strings.Contains(out, "fault") {
		t.Errorf("tester must NOT call admin-only User.join_group, got: %s", out)
	}
	// admin: admin-only allowed (reaches the handler; may fault on data, not on authz)
	if out := call(admin, "User.join_group", `<param><value><string>tester</string></value></param><param><value><string>x</string></value></param>`); strings.Contains(out, "insufficient role") {
		t.Errorf("admin must pass admin-only methods, got: %s", out)
	}
	// JSON-RPC: viewer write rejected with -32003
	body := `{"jsonrpc":"2.0","method":"TestCase.create","params":[{"summary":"x"}],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/json-rpc/", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+viewer)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var resp jsonRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil || resp.Error.Code != -32003 {
		t.Fatalf("expected -32003 forbidden for viewer write, got %+v", resp.Error)
	}
}

// product_members isolation: a non-admin user only sees products that are
// public (no members) or where they are a member / have a group permission.
func TestRPCFilterProductIsolation(t *testing.T) {
	db := setupTestDB(t)
	db.Exec(`INSERT INTO products (id, name, description) VALUES (2, 'Secret', 'restricted')`)
	// Product 2 is restricted to user 99; tester (id 2) is not a member.
	db.Exec(`INSERT INTO product_members (product_id, user_id, role) VALUES (2, 99, 'tester')`)

	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	call := func(token string) string {
		body := `<?xml version="1.0"?><methodCall><methodName>Product.filter</methodName><params></params></methodCall>`
		req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Body.String()
	}

	testerOut := call(userToken(t, svc, 2, "tester"))
	if strings.Contains(testerOut, "Secret") {
		t.Errorf("tester must not see restricted product, got: %s", testerOut)
	}
	if !strings.Contains(testerOut, "Demo") {
		t.Errorf("tester must see public product, got: %s", testerOut)
	}
	adminOut := call(adminToken(t, svc))
	if !strings.Contains(adminOut, "Secret") || !strings.Contains(adminOut, "Demo") {
		t.Errorf("admin must see all products, got: %s", adminOut)
	}
}

// Oversized RPC bodies are rejected instead of being read into memory.
func TestRPCBodyLimit(t *testing.T) {
	db := setupTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	big := make([]byte, maxRPCBodyBytes+1)
	for i := range big {
		big[i] = 'x'
	}
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(string(big)))
	req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("oversized body must not return 200, got body: %.100s", rec.Body.String())
	}

	// JSON-RPC variant
	req = httptest.NewRequest(http.MethodPost, "/json-rpc/", strings.NewReader(string(big)))
	req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("oversized JSON-RPC body must not return 200")
	}
}

// End-to-end: a malicious struct member name in TestCase.filter must produce
// a fault, not dump rows.
func TestRPCFilterInjectionEndToEnd(t *testing.T) {
	db := setupTestDB(t)
	db.Exec(`INSERT INTO test_cases (id, summary, category_id, is_automated) VALUES (1, 'Login test', 0, 0)`)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0")
	r := chi.NewRouter()
	h.Register(r)

	mal := `<?xml version="1.0"?><methodCall><methodName>TestCase.filter</methodName><params><param><value><struct>` +
		`<member><name>summary" OR "1"="1</name><value><string>x</string></value></member>` +
		`</struct></value></param></params></methodCall>`
	req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(mal))
	req.Header.Set("Authorization", "Bearer "+adminToken(t, svc))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	out := rec.Body.String()
	if !strings.Contains(out, "fault") {
		t.Fatalf("expected fault for injection attempt, got: %s", out)
	}
	if strings.Contains(out, "Login test") {
		t.Fatalf("injection leaked rows: %s", out)
	}
}
