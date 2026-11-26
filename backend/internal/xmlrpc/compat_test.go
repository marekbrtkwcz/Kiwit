package xmlrpc

// Compatibility tests verifying that all XML-RPC methods exposed by kiwitcms
// are also registered in Kiwit, and that JSON-RPC endpoint works.

import (
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

// allKiwiTCMSMethods is the full list of RPC method names from tcms/rpc/api/.
var allKiwiTCMSMethods = []string{
	"Attachment.remove_attachment",
	"Auth.login", "Auth.logout",
	"Bug.details", "Bug.report",
	"BugTracker.filter", "BugTracker.create",
	"Build.filter", "Build.create", "Build.update",
	"Category.filter", "Category.create",
	"Classification.filter", "Classification.create",
	"Component.filter", "Component.create", "Component.update",
	"Environment.properties", "Environment.remove_property", "Environment.add_property",
	"Environment.filter", "Environment.create",
	"Group.filter", "Group.permissions", "Group.users",
	"KiwiTCMS.version",
	"Markdown.render",
	"PlanType.create", "PlanType.filter",
	"Priority.filter", "Priority.create",
	"Product.create", "Product.filter",
	"Tag.filter", "Tag.create",
	"Template.filter", "Template.create",
	"TestCase.add_component", "TestCase.remove_component",
	"TestCase.add_notification_cc", "TestCase.remove_notification_cc", "TestCase.get_notification_cc",
	"TestCase.add_tag", "TestCase.remove_tag",
	"TestCase.create", "TestCase.filter", "TestCase.history", "TestCase.sortkeys",
	"TestCase.update", "TestCase.remove",
	"TestCase.list_attachments", "TestCase.add_attachment",
	"TestCase.add_comment", "TestCase.remove_comment", "TestCase.comments",
	"TestCase.properties", "TestCase.remove_property", "TestCase.add_property",
	"TestCaseStatus.filter", "TestCaseStatus.create",
	"TestExecution.add_comment", "TestExecution.remove_comment", "TestExecution.get_comments",
	"TestExecution.filter", "TestExecution.history", "TestExecution.update",
	"TestExecution.add_link", "TestExecution.remove_link", "TestExecution.get_links",
	"TestExecution.properties", "TestExecution.add_property", "TestExecution.remove",
	"TestExecution.list_attachments", "TestExecution.count_attachments", "TestExecution.add_attachment",
	"TestExecution.create", "TestExecution.add_tag", "TestExecution.remove_tag",
	"TestExecutionStatus.filter", "TestExecutionStatus.create",
	"TestPlan.create", "TestPlan.clone", "TestPlan.filter",
	"TestPlan.add_tag", "TestPlan.remove_tag", "TestPlan.update",
	"TestPlan.add_case", "TestPlan.remove_case", "TestPlan.update_case_order",
	"TestPlan.list_attachments", "TestPlan.add_attachment", "TestPlan.tree",
	"TestRun.add_case", "TestRun.remove_case", "TestRun.get_cases",
	"TestRun.add_tag", "TestRun.remove_tag",
	"TestRun.create", "TestRun.filter", "TestRun.update",
	"TestRun.add_cc", "TestRun.remove_cc", "TestRun.get_cc",
	"TestRun.properties", "TestRun.add_property",
	"TestRun.list_attachments", "TestRun.add_attachment", "TestRun.remove",
	"User.filter", "User.update", "User.deactivate", "User.join_group", "User.add_attachment",
	"Version.filter", "Version.create",
}

func setupFullTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT, username TEXT, password_hash TEXT, role TEXT, is_active INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE api_tokens (id INTEGER PRIMARY KEY, user_id INTEGER, token TEXT, created_at TEXT, expires_at TEXT)`,
		`CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, description TEXT DEFAULT '', classification_id INTEGER)`,
		`CREATE TABLE versions (id INTEGER PRIMARY KEY, value TEXT, product_id INTEGER)`,
		`CREATE TABLE builds (id INTEGER PRIMARY KEY, name TEXT, version_id INTEGER, description TEXT DEFAULT '')`,
		`CREATE TABLE components (id INTEGER PRIMARY KEY, name TEXT, product_id INTEGER, description TEXT DEFAULT '')`,
		`CREATE TABLE tags (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT, product_id INTEGER)`,
		`CREATE TABLE test_cases (id INTEGER PRIMARY KEY, product_id INTEGER, summary TEXT, preconditions TEXT DEFAULT '', format TEXT DEFAULT 'steps', gherkin_body TEXT DEFAULT '', priority TEXT DEFAULT 'medium', status TEXT DEFAULT 'confirmed', author_id INTEGER, category_id INTEGER, is_automated INTEGER DEFAULT 0, script TEXT DEFAULT '', arguments TEXT DEFAULT '', created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE test_case_steps (id INTEGER PRIMARY KEY, test_case_id INTEGER, position INTEGER, action TEXT, expected_result TEXT DEFAULT '')`,
		`CREATE TABLE test_plans (id INTEGER PRIMARY KEY, product_id INTEGER, version_id INTEGER, name TEXT, description TEXT DEFAULT '', is_active INTEGER DEFAULT 1, parent_id INTEGER, plan_type_id INTEGER, author_id INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE test_plan_cases (test_plan_id INTEGER, test_case_id INTEGER, position INTEGER DEFAULT 0, PRIMARY KEY (test_plan_id, test_case_id))`,
		`CREATE TABLE test_runs (id INTEGER PRIMARY KEY, test_plan_id INTEGER, build_id INTEGER, summary TEXT, notes TEXT DEFAULT '', manager_id INTEGER, started_at TEXT, stopped_at TEXT)`,
		`CREATE TABLE test_executions (id INTEGER PRIMARY KEY, test_run_id INTEGER, test_case_id INTEGER, status TEXT DEFAULT 'idle', assignee_id INTEGER, tested_by_id INTEGER, steps_snapshot TEXT DEFAULT '[]', comment TEXT DEFAULT '', executed_at TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE attachments (id INTEGER PRIMARY KEY, entity_type TEXT, entity_id INTEGER, filename TEXT, content_type TEXT DEFAULT '', size_bytes INTEGER DEFAULT 0, storage_path TEXT DEFAULT '', created_at TEXT)`,
		`CREATE TABLE test_case_revisions (id INTEGER PRIMARY KEY, test_case_id INTEGER, version INTEGER, summary TEXT, created_at TEXT)`,
		`CREATE TABLE issue_trackers (id INTEGER PRIMARY KEY, name TEXT, provider TEXT, base_url TEXT, api_username TEXT DEFAULT '', created_at TEXT)`,
		`CREATE TABLE bugs (id INTEGER PRIMARY KEY, test_execution_id INTEGER, tracker_id INTEGER, external_key TEXT, url TEXT, title TEXT DEFAULT '', status TEXT DEFAULT 'open', last_synced_at TEXT, created_at TEXT)`,
		`CREATE TABLE internal_bugs (id INTEGER PRIMARY KEY, product_id INTEGER, build_id INTEGER, test_execution_id INTEGER, summary TEXT, description TEXT DEFAULT '', status TEXT DEFAULT 'open', severity TEXT DEFAULT 'medium', reporter_id INTEGER, assignee_id INTEGER, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE comments (id INTEGER PRIMARY KEY, entity_type TEXT, entity_id INTEGER, author_id INTEGER, body TEXT, created_at TEXT)`,
		`CREATE TABLE product_members (product_id INTEGER, user_id INTEGER, role TEXT DEFAULT 'tester', created_at TEXT, PRIMARY KEY (product_id, user_id))`,
		`CREATE TABLE environments (id INTEGER PRIMARY KEY, name TEXT, description TEXT DEFAULT '', properties TEXT DEFAULT '{}', created_at TEXT)`,
		`CREATE TABLE priorities (id INTEGER PRIMARY KEY, value TEXT, is_active INTEGER DEFAULT 1, created_at TEXT)`,
		`CREATE TABLE execution_statuses (id INTEGER PRIMARY KEY, name TEXT, color TEXT DEFAULT '', weight INTEGER DEFAULT 0, created_at TEXT)`,
		`CREATE TABLE test_case_statuses (id INTEGER PRIMARY KEY, name TEXT, description TEXT DEFAULT '')`,
		`CREATE TABLE email_settings (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE test_case_tags (test_case_id INTEGER, tag_id INTEGER, PRIMARY KEY (test_case_id, tag_id))`,
		`CREATE TABLE test_plan_tags (test_plan_id INTEGER, tag_id INTEGER, PRIMARY KEY (test_plan_id, tag_id))`,
		`CREATE TABLE test_run_tags (test_run_id INTEGER, tag_id INTEGER, PRIMARY KEY (test_run_id, tag_id))`,
		`CREATE TABLE test_case_components (test_case_id INTEGER, component_id INTEGER, PRIMARY KEY (test_case_id, component_id))`,
		`CREATE TABLE test_execution_tags (test_execution_id INTEGER, tag_id INTEGER, PRIMARY KEY (test_execution_id, tag_id))`,
		`CREATE TABLE templates (id INTEGER PRIMARY KEY, name TEXT, body TEXT, created_at TEXT)`,
		`CREATE TABLE execution_links (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE notifications (id INTEGER PRIMARY KEY, user_id INTEGER, event_type TEXT, entity_type TEXT DEFAULT '', entity_id INTEGER DEFAULT 0, title TEXT, body TEXT DEFAULT '', read_at TEXT, created_at TEXT)`,
		`CREATE TABLE user_notification_settings (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE webhooks (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE password_reset_tokens (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE user_groups (id INTEGER PRIMARY KEY, name TEXT, created_at TEXT)`,
		`CREATE TABLE user_group_members (group_id INTEGER, user_id INTEGER, PRIMARY KEY (group_id, user_id))`,
		`CREATE TABLE entity_properties (id INTEGER PRIMARY KEY, entity_type TEXT, entity_id INTEGER, name TEXT, value TEXT DEFAULT '', UNIQUE (entity_type, entity_id, name))`,
		`CREATE TABLE test_run_cc (run_id INTEGER, user_id INTEGER, PRIMARY KEY (run_id, user_id))`,
		`CREATE TABLE test_plan_types (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE classifications (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE link_references (id INTEGER PRIMARY KEY, entity_type TEXT, entity_id INTEGER, name TEXT, url TEXT, created_at TEXT)`,
		`CREATE TABLE group_product_permissions (group_id INTEGER, product_id INTEGER, role TEXT, PRIMARY KEY (group_id, product_id))`,
		`CREATE TABLE audit_log (id INTEGER PRIMARY KEY)`,
		`INSERT INTO products (id, name, description) VALUES (1, 'Demo', 'Test')`,
		`INSERT INTO versions (id, value, product_id) VALUES (1, 'v1', 1)`,
		`INSERT INTO test_plans (id, product_id, version_id, name) VALUES (1, 1, 1, 'Plan A')`,
		`INSERT INTO test_cases (id, product_id, summary) VALUES (1, 1, 'Case A')`,
		`INSERT INTO builds (id, name, version_id) VALUES (1, 'build-1', 1)`,
		`INSERT INTO test_runs (id, test_plan_id, build_id, summary) VALUES (1, 1, 1, 'Run A')`,
		`INSERT INTO test_executions (id, test_run_id, test_case_id, status) VALUES (1, 1, 1, 'idle')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("setup: %v\n%s", err, s)
		}
	}
	return db
}

// TestAllMethodsRegistered verifies every kiwitcms RPC method name is known.
func TestAllMethodsRegistered(t *testing.T) {
	db := setupFullTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0-test")

	missing := []string{}
	for _, name := range allKiwiTCMSMethods {
		if _, ok := h.methods[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("missing %d RPC methods:\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
}

// TestJSONRPCEndpoint verifies the /json-rpc/ endpoint responds correctly.
func TestJSONRPCEndpoint(t *testing.T) {
	db := setupFullTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0-test")

	r := chi.NewRouter()
	h.Register(r)

	body := `{"jsonrpc":"2.0","method":"KiwiTCMS.version","params":[],"id":1}`
	req := httptest.NewRequest(http.MethodPost, "/json-rpc/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp jsonRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\n%s", err, rec.Body.String())
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["version"] != "0.1.0-test" {
		t.Fatalf("expected version 0.1.0-test, got %v", result["version"])
	}
}

// TestJSONRPCMethodNotFound verifies fault response for unknown method.
func TestJSONRPCMethodNotFound(t *testing.T) {
	db := setupFullTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0-test")

	r := chi.NewRouter()
	h.Register(r)

	body := `{"jsonrpc":"2.0","method":"NoSuch.method","params":[],"id":42}`
	req := httptest.NewRequest(http.MethodPost, "/json-rpc/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var resp jsonRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected -32601 method not found, got %+v", resp.Error)
	}
}

// TestXMLRPCNewMethods verifies a few of the newly added methods respond via XML-RPC.
func TestXMLRPCNewMethods(t *testing.T) {
	db := setupFullTestDB(t)
	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0-test")

	r := chi.NewRouter()
	h.Register(r)

	tests := []struct {
		method string
		body   string
		want   string
	}{
		{"Classification.filter", `<?xml version="1.0"?><methodCall><methodName>Classification.filter</methodName><params></params></methodCall>`, "<array>"},
		{"PlanType.filter", `<?xml version="1.0"?><methodCall><methodName>PlanType.filter</methodName><params></params></methodCall>`, "<array>"},
		{"BugTracker.filter", `<?xml version="1.0"?><methodCall><methodName>BugTracker.filter</methodName><params></params></methodCall>`, "<array>"},
		{"Group.permissions", `<?xml version="1.0"?><methodCall><methodName>Group.permissions</methodName><params><param><value><int>1</int></value></param></params></methodCall>`, "<array>"},
		{"TestCase.properties", `<?xml version="1.0"?><methodCall><methodName>TestCase.properties</methodName><params><param><value><struct><member><name>case_id</name><value><int>1</int></value></member></struct></value></param></params></methodCall>`, "<array>"},
		{"Bug.details", `<?xml version="1.0"?><methodCall><methodName>Bug.details</methodName><params><param><value><int>999</int></value></param></params></methodCall>`, "fault"},
		{"TestExecution.count_attachments", `<?xml version="1.0"?><methodCall><methodName>TestExecution.count_attachments</methodName><params><param><value><int>1</int></value></param></params></methodCall>`, "<int>0</int>"},
	}
	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "text/xml")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("expected %q in response, got: %s", tc.want, rec.Body.String())
			}
		})
	}
}

// TestXMLRPCFilterWithLookups verifies that .filter methods accept field-lookup
// query dicts and return filtered results via XML-RPC.
func TestXMLRPCFilterWithLookups(t *testing.T) {
	db := setupFullTestDB(t)
	_, _ = db.Exec(`INSERT INTO test_cases (id, product_id, summary, priority) VALUES (2, 1, 'test login flow', 'high')`)
	_, _ = db.Exec(`INSERT INTO test_cases (id, product_id, summary, priority) VALUES (3, 1, 'test logout', 'low')`)
	_, _ = db.Exec(`INSERT INTO test_cases (id, product_id, summary, priority) VALUES (4, 1, 'production smoke', 'critical')`)

	svc := auth.NewService(db, "test-secret")
	h := NewHandler(svc, db, "0.1.0-test")
	r := chi.NewRouter()
	h.Register(r)

	tests := []struct {
		name     string
		method   string
		queryXML string
		wantIDs  []string
		notWant  []string
	}{
		{
			name:     "startswith",
			method:   "TestCase.filter",
			queryXML: `<param><value><struct><member><name>summary__startswith</name><value><string>test</string></value></member></struct></value></param>`,
			wantIDs:  []string{"<int>2</int>", "<int>3</int>"},
			notWant:  []string{"<int>4</int>"},
		},
		{
			name:     "contains",
			method:   "TestCase.filter",
			queryXML: `<param><value><struct><member><name>summary__contains</name><value><string>login</string></value></member></struct></value></param>`,
			wantIDs:  []string{"<int>2</int>"},
			notWant:  []string{"<int>3</int>", "<int>4</int>"},
		},
		{
			name:     "gte",
			method:   "TestCase.filter",
			queryXML: `<param><value><struct><member><name>id__gte</name><value><int>3</int></value></member></struct></value></param>`,
			wantIDs:  []string{"<int>3</int>", "<int>4</int>"},
			notWant:  []string{"<int>2</int>"},
		},
		{
			name:     "in",
			method:   "TestCase.filter",
			queryXML: `<param><value><struct><member><name>id__in</name><value><array><data><value><int>2</int></value><value><int>4</int></value></data></array></value></member></struct></value></param>`,
			wantIDs:  []string{"<int>2</int>", "<int>4</int>"},
			notWant:  []string{"<int>3</int>"},
		},
		{
			name:   "multiple_lookups",
			method: "TestCase.filter",
			queryXML: `<param><value><struct>
				<member><name>summary__startswith</name><value><string>test</string></value></member>
				<member><name>priority</name><value><string>high</string></value></member>
			</struct></value></param>`,
			wantIDs: []string{"<int>2</int>"},
			notWant: []string{"<int>3</int>"},
		},
		{
			name:     "no_query_returns_all",
			method:   "TestCase.filter",
			queryXML: ``,
			wantIDs:  []string{"<int>1</int>", "<int>2</int>", "<int>3</int>", "<int>4</int>"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := `<?xml version="1.0"?><methodCall><methodName>` + tc.method + `</methodName><params>` + tc.queryXML + `</params></methodCall>`
			req := httptest.NewRequest(http.MethodPost, "/xml-rpc/", strings.NewReader(body))
			req.Header.Set("Content-Type", "text/xml")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			resp := rec.Body.String()
			if strings.Contains(resp, "<fault>") {
				t.Fatalf("unexpected fault: %s", resp)
			}
			for _, want := range tc.wantIDs {
				if !strings.Contains(resp, want) {
					t.Errorf("expected %s in response, got: %s", want, resp)
				}
			}
			for _, notWant := range tc.notWant {
				if strings.Contains(resp, notWant) {
					t.Errorf("did not expect %s in response, got: %s", notWant, resp)
				}
			}
		})
	}
}
