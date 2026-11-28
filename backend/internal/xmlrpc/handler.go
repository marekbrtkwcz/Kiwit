package xmlrpc

import (
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/webhook"
)

const sessionCookie = "kiwit_session"

// Handler exposes a Kiwi TCMS-compatible XML-RPC API at /xml-rpc/.
type Handler struct {
	svc       *auth.Service
	db        *sql.DB
	pg        *pgxpool.Pool
	pgDB      *sql.DB // database/sql handle over the pgx pool (for PostgreSQL mode)
	version   string
	methods   map[string]reflect.Value // "Service.Method" -> func(r *http.Request, args []any) (any, error)
	uploadDir string
	webhooks  *webhook.Store // optional — enables bug.created event emission (EXTERNAL_ISSUE_POST_PROCESSORS equivalent)
}

// NewHandlerWithWebhooks is like NewHandler but also wires the webhook store so
// that Bug.report emits a "bug.created" event with full issue details.
// pg may be nil (SQLite mode); when set, all queries go through the pgx pool
// with ? placeholders rebound to $n.
func NewHandlerWithWebhooks(svc *auth.Service, db *sql.DB, pg *pgxpool.Pool, version string, uploadDir string, wh *webhook.Store) *Handler {
	h := &Handler{svc: svc, db: db, pg: pg, version: version, methods: map[string]reflect.Value{}, uploadDir: uploadDir, webhooks: wh}
	if pg != nil {
		h.pgDB = stdlib.OpenDBFromPool(pg)
	}
	h.registerAll()
	h.registerExtrasMethods(uploadDir)
	return h
}

func NewHandler(svc *auth.Service, db *sql.DB, version string, uploadDir ...string) *Handler {
	h := &Handler{svc: svc, db: db, version: version, methods: map[string]reflect.Value{}}
	if len(uploadDir) > 0 {
		h.uploadDir = uploadDir[0]
	}
	h.registerAll()
	h.registerExtrasMethods(h.uploadDir)
	return h
}

// available reports whether any database backend is configured.
func (h *Handler) available() bool { return h.db != nil || h.pgDB != nil }

// exec runs a write query on the configured backend (PostgreSQL: ? → $n).
func (h *Handler) exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if h.pgDB != nil {
		return h.pgDB.ExecContext(ctx, rebindPlaceholders(q), args...)
	}
	return h.db.ExecContext(ctx, q, args...)
}

// query runs a SELECT on the configured backend (PostgreSQL: ? → $n).
func (h *Handler) query(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if h.pgDB != nil {
		return h.pgDB.QueryContext(ctx, rebindPlaceholders(q), args...)
	}
	return h.db.QueryContext(ctx, q, args...)
}

// queryRow runs a single-row query on the configured backend.
func (h *Handler) queryRow(ctx context.Context, q string, args ...any) *sql.Row {
	if h.pgDB != nil {
		return h.pgDB.QueryRowContext(ctx, rebindPlaceholders(q), args...)
	}
	return h.db.QueryRowContext(ctx, q, args...)
}

// insertID runs an INSERT and returns the new row id via RETURNING
// (supported by both SQLite >= 3.35 and PostgreSQL; LastInsertId does not
// work over the pgx stdlib driver).
func (h *Handler) insertID(ctx context.Context, q string, args ...any) (int64, error) {
	var id int64
	err := h.queryRow(ctx, q+" RETURNING id", args...).Scan(&id)
	return id, err
}

// execIgnore runs an idempotent INSERT: INSERT OR IGNORE on SQLite,
// INSERT ... ON CONFLICT DO NOTHING on PostgreSQL.
func (h *Handler) execIgnore(ctx context.Context, q string, args ...any) error {
	if h.pgDB != nil {
		q = strings.Replace(q, "INSERT OR IGNORE INTO", "INSERT INTO", 1) + " ON CONFLICT DO NOTHING"
	}
	_, err := h.exec(ctx, q, args...)
	return err
}

func (h *Handler) register(name string, fn func(r *http.Request, args []any) (any, error)) {
	h.methods[name] = reflect.ValueOf(fn)
}

// Register mounts POST /xml-rpc/ and POST /json-rpc/ on the chi router.
func (h *Handler) Register(r chi.Router) {
	r.Post("/xml-rpc/", h.serveHTTP)
	r.Post("/json-rpc/", h.serveJSONRPC)
}

// publicRPCMethods may be called without authentication (login and version
// probe only). Every other method requires an authenticated user.
var publicRPCMethods = map[string]bool{
	"Auth.login":       true,
	"KiwiTCMS.version": true,
}

// errRPCAuthRequired is returned by authorize when no user is authenticated.
var errRPCAuthRequired = errors.New("authentication required")

// methodClass is the authorization level of an RPC method.
type methodClass int

const (
	methodPublic methodClass = iota // no authentication required
	methodRead                       // any authenticated user
	methodWrite                      // write role (tester/admin)
	methodAdmin                      // admin only
)

// classifyMethod maps a method name to its authorization class.
// Anything not explicitly read-only or public is treated as a write —
// fail-closed.
func classifyMethod(name string) methodClass {
	if publicRPCMethods[name] {
		return methodPublic
	}
	// Admin-only: user/group administration and issue-tracker configuration.
	if strings.HasPrefix(name, "User.") && name != "User.filter" {
		return methodAdmin
	}
	if name == "Group.create" ||
		(strings.HasPrefix(name, "BugTracker.") && name != "BugTracker.filter") {
		return methodAdmin
	}
	// Read-only: filters, getters, comments/history, telemetry, misc reads.
	if strings.HasSuffix(name, ".filter") ||
		strings.Contains(name, ".get_") ||
		strings.HasSuffix(name, ".comments") ||
		strings.HasSuffix(name, ".history") ||
		strings.HasSuffix(name, ".sortkeys") ||
		strings.HasSuffix(name, ".properties") ||
		strings.HasSuffix(name, ".details") ||
		strings.HasSuffix(name, ".count_attachments") ||
		strings.HasSuffix(name, ".list_attachments") ||
		strings.HasPrefix(name, "Telemetry.") ||
		name == "Markdown.render" ||
		name == "Auth.logout" ||
		name == "Group.users" ||
		name == "Group.permissions" ||
		name == "TestPlan.tree" ||
		name == "Search.query" {
		return methodRead
	}
	return methodWrite
}

// authorize enforces authentication (who) and role-based authorization
// (what may be done) for an RPC method call.
func (h *Handler) authorize(req *http.Request, method string) error {
	switch classifyMethod(method) {
	case methodPublic:
		return nil
	case methodRead:
		if _, ok := auth.UserFromContext(req.Context()); !ok {
			return errRPCAuthRequired
		}
		return nil
	case methodAdmin:
		if _, ok := auth.UserFromContext(req.Context()); !ok {
			return errRPCAuthRequired
		}
		return auth.CheckRole(req.Context(), "admin")
	default: // methodWrite
		if _, ok := auth.UserFromContext(req.Context()); !ok {
			return errRPCAuthRequired
		}
		return auth.CheckWriteRole(req.Context(), "admin", "tester")
	}
}

// authenticate resolves the caller from the Authorization bearer token or
// the session cookie and stores the user in the request context. Requests
// with an invalid credential are left unauthenticated (fail-closed below).
func (h *Handler) authenticate(req *http.Request) *http.Request {
	if header := req.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimPrefix(header, "Bearer ")
		if u, err := h.svc.AuthenticateBearer(req.Context(), token); err == nil {
			return req.WithContext(auth.WithUser(req.Context(), u))
		}
		return req
	}
	if c, err := req.Cookie(sessionCookie); err == nil && c.Value != "" {
		if u, err := h.svc.AuthenticateBearer(req.Context(), c.Value); err == nil {
			return req.WithContext(auth.WithUser(req.Context(), u))
		}
	}
	return req
}

func (h *Handler) serveHTTP(w http.ResponseWriter, req *http.Request) {
	req = h.authenticate(req)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var call methodCall
	if err := xml.Unmarshal(body, &call); err != nil {
		writeFault(w, 1, "parse error: "+err.Error())
		return
	}
	fn, ok := h.methods[call.MethodName]
	if !ok {
		writeFault(w, 2, "method not found: "+call.MethodName)
		return
	}
	if err := h.authorize(req, call.MethodName); err != nil {
		if errors.Is(err, errRPCAuthRequired) {
			writeFault(w, 401, err.Error())
		} else {
			writeFault(w, 403, err.Error())
		}
		return
	}
	args := make([]any, len(call.Params))
	for i, p := range call.Params {
		args[i] = p.Value.any()
	}
	out := fn.Call([]reflect.Value{reflect.ValueOf(req), reflect.ValueOf(args)})
	resultVal := out[0].Interface()
	resultErr, _ := out[1].Interface().(error)
	if resultErr != nil {
		writeFault(w, 3, resultErr.Error())
		return
	}
	// If login stashed a token, set the session cookie.
	if tok, ok := req.Context().Value(tokenKey).(string); ok && tok != "" {
		if tok == "\x00logout" {
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		} else {
			http.SetCookie(w, &http.Cookie{
				Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(24 * time.Hour),
			})
		}
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header + encodeResponse(resultVal)))
}

type funcResult struct {
	value any
	err   error
}

// --- Service methods ---

func (h *Handler) authLogin(r *http.Request, args []any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("Auth.login requires username and password")
	}
	username, _ := args[0].(string)
	password, _ := args[1].(string)
	token, u, err := h.svc.Login(r.Context(), username, password)
	if err != nil {
		return nil, fmt.Errorf("Authentication failed")
	}
	*r = *r.WithContext(contextWithToken(r.Context(), token))
	return map[string]any{
		"token": token, "id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role,
	}, nil
}

func (h *Handler) authLogout(r *http.Request, _ []any) (any, error) {
	*r = *r.WithContext(contextWithToken(r.Context(), "\x00logout"))
	return true, nil
}

func (h *Handler) kiwiVersion(_ *http.Request, _ []any) (any, error) {
	return map[string]any{"version": h.version, "name": "Kiwit TCMS"}, nil
}

// productScopedTables maps filterable tables to a SQL expression yielding the
// row's product_id, used to enforce product_members isolation on .filter for
// non-admin users.
var productScopedTables = map[string]string{
	"test_cases":         "t.product_id",
	"test_plans":         "t.product_id",
	"products":           "t.id",
	"versions":           "t.product_id",
	"components":         "t.product_id",
	"requirements":       "t.product_id",
	"campaigns":          "t.product_id",
	"test_runs":          "(SELECT tp.product_id FROM test_plans tp WHERE tp.id = t.test_plan_id)",
	"test_executions":    "(SELECT tp.product_id FROM test_runs tr JOIN test_plans tp ON tp.id = tr.test_plan_id WHERE tr.id = t.test_run_id)",
	"milestones":         "(SELECT v.product_id FROM versions v WHERE v.id = t.version_id)",
	"test_case_datasets": "(SELECT tc.product_id FROM test_cases tc WHERE tc.id = t.test_case_id)",
	"builds":             "(SELECT v.product_id FROM versions v WHERE v.id = t.version_id)",
	"campaign_iterations": "(SELECT c.product_id FROM campaigns c WHERE c.id = t.campaign_id)",
}

// productAccessCond returns a SQL condition (mirroring AccessGuard semantics:
// a product with no members is public, otherwise membership or a group
// permission is required) plus its bound args.
func productAccessCond(productExpr string, userID int64) (string, []any) {
	cond := `(NOT EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = ` + productExpr + `)` +
		` OR EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = ` + productExpr + ` AND pm.user_id = ?)` +
		` OR EXISTS (SELECT 1 FROM group_product_permissions gpp` +
		` JOIN user_group_members ugm ON ugm.group_id = gpp.group_id` +
		` WHERE gpp.product_id = ` + productExpr + ` AND ugm.user_id = ?))`
	return cond, []any{userID, userID}
}

func (h *Handler) filterRows(table string, cols []string, relations ...map[string]relationJoin) func(r *http.Request, args []any) (any, error) {
	return h.filterRowsWhere(table, "", cols, relations...)
}

// filterRowsWhere is filterRows with an extra base condition appended to the
// where clause (used to exclude soft-deleted rows, e.g. test cases in trash).
func (h *Handler) filterRowsWhere(table, baseCond string, cols []string, relations ...map[string]relationJoin) func(r *http.Request, args []any) (any, error) {
	relMap := map[string]relationJoin{}
	if len(relations) > 0 {
		relMap = relations[0]
	}
	return func(r *http.Request, args []any) (any, error) {
		if !h.available() {
			return nil, fmt.Errorf("database not available")
		}
		// Parse optional query dict from args[0] for field lookups
		query := map[string]any{}
		if len(args) > 0 {
			if m, ok := args[0].(map[string]any); ok {
				query = m
			}
		}
		lw, err := buildWhere(query, "t", relMap, deriveAllowed(cols, relMap))
		if err != nil {
			return nil, err
		}
		// Assemble the WHERE clause in argument order: product-members
		// isolation first, then the base condition, then the lookup clause.
		parts := make([]string, 0, 3)
		qargs := lw.args
		if uid := auth.FilterUserID(r.Context()); uid != nil {
			if expr, ok := productScopedTables[table]; ok {
				cond, condArgs := productAccessCond(expr, *uid)
				parts = append(parts, cond)
				qargs = append(condArgs, qargs...)
			}
		}
		if baseCond != "" {
			parts = append(parts, baseCond)
		}
		if lw.where != "1=1" {
			parts = append(parts, "("+lw.where+")")
		}
		where := "1=1"
		if len(parts) > 0 {
			where = strings.Join(parts, " AND ")
		}
		joins := joinsForQuery(query, relMap)
		joinClause := ""
		if len(joins) > 0 {
			joinClause = strings.Join(joins, " ") + " "
		}
		q := fmt.Sprintf("SELECT %s FROM %s t %sWHERE %s ORDER BY t.id DESC LIMIT 100",
			strings.Join(cols, ", "), table, joinClause, where)
		rows, err := h.query(r.Context(), q, qargs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		colNames, _ := rows.Columns()
		out := []map[string]any{}
		for rows.Next() {
			vals := make([]any, len(colNames))
			ptrs := make([]any, len(colNames))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				continue
			}
			m := map[string]any{}
			for i, c := range colNames {
				if b, ok := vals[i].([]byte); ok {
					m[c] = string(b)
				} else {
					m[c] = vals[i]
				}
			}
			out = append(out, m)
		}
		return out, nil
	}
}

// sortKeys implements TestPlan.sortkeys — returns case IDs with their sort position for a plan.
// args: [plan_id] or [{plan_id: N}]
func (h *Handler) sortKeys(r *http.Request, args []any) (any, error) {
	if !h.available() {
		return nil, fmt.Errorf("database not available")
	}
	var planID int64
	if len(args) > 0 {
		switch v := args[0].(type) {
		case int64:
			planID = v
		case float64:
			planID = int64(v)
		case map[string]any:
			if pid, ok := v["plan_id"].(int64); ok {
				planID = pid
			}
		}
	}
	if planID == 0 {
		return nil, fmt.Errorf("plan_id required")
	}
	rows, err := h.query(r.Context(), `
		SELECT test_case_id, position FROM test_plan_cases WHERE test_plan_id = ? ORDER BY position, test_case_id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var caseID, pos int64
		if err := rows.Scan(&caseID, &pos); err != nil {
			continue
		}
		out = append(out, map[string]any{"case_id": caseID, "sortkey": pos})
	}
	return out, nil
}

// --- XML-RPC wire format ---

type methodCall struct {
	XMLName    xml.Name `xml:"methodCall"`
	MethodName string   `xml:"methodName"`
	Params     []param  `xml:"params>param"`
}

type param struct {
	Value value `xml:"value"`
}

type structVal struct {
	Members []member `xml:"member"`
}

type arrayVal struct {
	Values []value `xml:"data>value"`
}

type value struct {
	XMLName xml.Name  `xml:"value"`
	Str     string    `xml:"string"`
	Int     string    `xml:"int"`
	I4      string    `xml:"i4"`
	Bool    string    `xml:"boolean"`
	Double  string    `xml:"double"`
	Struct  structVal `xml:"struct"`
	Array   arrayVal  `xml:"array"`
}

type member struct {
	Name  string `xml:"name"`
	Value value  `xml:"value"`
}

func (v *value) any() any {
	switch {
	case v.Str != "":
		return v.Str
	case v.Int != "" || v.I4 != "":
		var i int64
		fmt.Sscanf(v.Int+v.I4, "%d", &i)
		return i
	case v.Bool != "":
		return v.Bool == "1"
	case v.Double != "":
		var f float64
		fmt.Sscanf(v.Double, "%g", &f)
		return f
	case len(v.Struct.Members) > 0:
		m := map[string]any{}
		for _, mem := range v.Struct.Members {
			m[mem.Name] = mem.Value.any()
		}
		return m
	case len(v.Array.Values) > 0:
		arr := make([]any, len(v.Array.Values))
		for i, av := range v.Array.Values {
			arr[i] = av.any()
		}
		return arr
	}
	return v.Str
}

func encodeResponse(v any) string {
	return `<methodResponse><params><param><value>` + encodeValue(v) + `</value></param></params></methodResponse>`
}

func encodeValue(v any) string {
	switch t := v.(type) {
	case nil:
		return `<nil/>`
	case string:
		return `<string>` + xmlEscape(t) + `</string>`
	case bool:
		if t {
			return `<boolean>1</boolean>`
		}
		return `<boolean>0</boolean>`
	case int:
		return fmt.Sprintf(`<int>%d</int>`, t)
	case int64:
		return fmt.Sprintf(`<int>%d</int>`, t)
	case float64:
		return fmt.Sprintf(`<double>%g</double>`, t)
	case []map[string]any:
		var items strings.Builder
		for _, m := range t {
			items.WriteString(`<value><struct>`)
			for k, val := range m {
				items.WriteString(`<member><name>` + xmlEscape(k) + `</name><value>` + encodeValue(val) + `</value></member>`)
			}
			items.WriteString(`</struct></value>`)
		}
		return `<array><data>` + items.String() + `</data></array>`
	case map[string]any:
		var s strings.Builder
		s.WriteString(`<struct>`)
		for k, val := range t {
			s.WriteString(`<member><name>` + xmlEscape(k) + `</name><value>` + encodeValue(val) + `</value></member>`)
		}
		s.WriteString(`</struct>`)
		return s.String()
	default:
		return `<string>` + xmlEscape(fmt.Sprintf("%v", t)) + `</string>`
	}
}

func writeFault(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header + `<methodResponse><fault><value><struct>` +
		`<member><name>faultCode</name><value><int>` + fmt.Sprint(code) + `</int></value></member>` +
		`<member><name>faultString</name><value><string>` + xmlEscape(msg) + `</string></value></member>` +
		`</struct></value></fault></methodResponse>`))
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// --- JSON-RPC 2.0 endpoint (POST /json-rpc/) ---

type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      any    `json:"id"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID any `json:"id"`
}

func (h *Handler) serveJSONRPC(w http.ResponseWriter, req *http.Request) {
	req = h.authenticate(req)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var call jsonRPCRequest
	if err := json.Unmarshal(body, &call); err != nil {
		writeJSONFault(w, nil, -32700, "parse error: "+err.Error())
		return
	}
	fn, ok := h.methods[call.Method]
	if !ok {
		writeJSONFault(w, call.ID, -32601, "method not found: "+call.Method)
		return
	}
	if err := h.authorize(req, call.Method); err != nil {
		if errors.Is(err, errRPCAuthRequired) {
			writeJSONFault(w, call.ID, -32001, err.Error())
		} else {
			writeJSONFault(w, call.ID, -32003, err.Error())
		}
		return
	}
	args := call.Params
	if args == nil {
		args = []any{}
	}
	out := fn.Call([]reflect.Value{reflect.ValueOf(req), reflect.ValueOf(args)})
	resultVal := out[0].Interface()
	resultErr, _ := out[1].Interface().(error)

	// Handle session cookie like XML-RPC
	if tok, ok := req.Context().Value(tokenKey).(string); ok && tok != "" {
		if tok == "\x00logout" {
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		} else {
			http.SetCookie(w, &http.Cookie{
				Name: sessionCookie, Value: tok, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(24 * time.Hour),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if resultErr != nil {
		writeJSONFault(w, call.ID, -32000, resultErr.Error())
		return
	}
	resp := jsonRPCResponse{JSONRPC: "2.0", Result: resultVal, ID: call.ID}
	_ = json.NewEncoder(w).Encode(resp)
}

func writeJSONFault(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}{Code: code, Message: msg},
		ID: id,
	}
	_ = json.NewEncoder(w).Encode(resp)
}
