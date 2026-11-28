package xmlrpc

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/webhook"
)

const sessionCookie = "kiwit_session"

// Handler exposes a Kiwi TCMS-compatible XML-RPC API at /xml-rpc/.
type Handler struct {
	svc       *auth.Service
	db        *sql.DB
	version   string
	methods   map[string]reflect.Value // "Service.Method" -> func(r *http.Request, args []any) (any, error)
	uploadDir string
	webhooks  *webhook.Store // optional — enables bug.created event emission (EXTERNAL_ISSUE_POST_PROCESSORS equivalent)
}

// NewHandlerWithWebhooks is like NewHandler but also wires the webhook store so
// that Bug.report emits a "bug.created" event with full issue details.
func NewHandlerWithWebhooks(svc *auth.Service, db *sql.DB, version string, uploadDir string, wh *webhook.Store) *Handler {
	h := &Handler{svc: svc, db: db, version: version, methods: map[string]reflect.Value{}, uploadDir: uploadDir, webhooks: wh}
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
	if !publicRPCMethods[call.MethodName] {
		if _, ok := auth.UserFromContext(req.Context()); !ok {
			writeFault(w, 401, "authentication required")
			return
		}
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
		if h.db == nil {
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
		where := lw.where
		if baseCond != "" {
			if where == "1=1" {
				where = baseCond
			} else {
				where = baseCond + " AND (" + where + ")"
			}
		}
		joins := joinsForQuery(query, relMap)
		joinClause := ""
		if len(joins) > 0 {
			joinClause = strings.Join(joins, " ") + " "
		}
		q := fmt.Sprintf("SELECT %s FROM %s t %sWHERE %s ORDER BY t.id DESC LIMIT 100",
			strings.Join(cols, ", "), table, joinClause, where)
		rows, err := h.db.QueryContext(r.Context(), q, lw.args...)
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
	if h.db == nil {
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
	rows, err := h.db.QueryContext(r.Context(), `
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
	if !publicRPCMethods[call.Method] {
		if _, ok := auth.UserFromContext(req.Context()); !ok {
			writeJSONFault(w, call.ID, -32001, "authentication required")
			return
		}
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
