package xmlrpc

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// registerAll registers the full Kiwi TCMS-compatible method set.
func (h *Handler) registerAll() {
	// Auth & meta
	h.register("Auth.login", h.authLogin)
	h.register("Auth.logout", h.authLogout)
	h.register("KiwiTCMS.version", h.kiwiVersion)
	h.register("Markdown.render", h.markdownRender)

	// Filters — with relation maps for field lookups (product__name, etc.)
	h.register("TestRun.filter", h.filterRows("test_runs", []string{"t.id", "t.summary", "t.test_plan_id AS plan_id", "t.build_id", "t.stopped_at IS NOT NULL AS stopped"}, map[string]relationJoin{
		"plan":    {joinSQL: "JOIN test_plans tp ON tp.id = t.test_plan_id", alias: "tp"},
		"build":   {joinSQL: "JOIN builds b ON b.id = t.build_id", alias: "b"},
		"manager": {joinSQL: "JOIN users u ON u.id = t.manager_id", alias: "u"},
	}))
	h.register("TestCase.filter", h.filterRowsWhere("test_cases", "t.deleted_at IS NULL", []string{"t.id", "t.summary", "t.category_id", "t.is_automated", "t.priority", "t.status", "t.product_id"}, map[string]relationJoin{
		"category": {joinSQL: "JOIN categories c ON c.id = t.category_id", alias: "c"},
		"product":  {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
		"author":   {joinSQL: "JOIN users u ON u.id = t.author_id", alias: "u"},
	}))
	h.register("TestPlan.filter", h.filterRows("test_plans", []string{"t.id", "t.name", "t.product_id", "t.version_id", "t.is_active", "COALESCE(t.parent_id,0) AS parent_id"}, map[string]relationJoin{
		"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
		"version": {joinSQL: "JOIN versions v ON v.id = t.version_id", alias: "v"},
		"author":  {joinSQL: "JOIN users u ON u.id = t.author_id", alias: "u"},
		"parent":  {joinSQL: "JOIN test_plans pt ON pt.id = t.parent_id", alias: "pt"},
	}))
	h.register("TestPlan.sortkeys", h.sortKeys)
	h.register("Build.filter", h.filterRows("builds", []string{"t.id", "t.name", "t.version_id"}, map[string]relationJoin{
		"version": {joinSQL: "JOIN versions v ON v.id = t.version_id", alias: "v"},
	}))
	h.register("Product.filter", h.filterRows("products", []string{"t.id", "t.name", "COALESCE(t.description,'') AS description", "COALESCE(t.classification_id,0) AS classification_id"}, map[string]relationJoin{
		"classification": {joinSQL: "JOIN classifications c ON c.id = t.classification_id", alias: "c"},
	}))
	h.register("Version.filter", h.filterRows("versions", []string{"t.id", "t.value", "t.product_id"}, map[string]relationJoin{
		"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
	}))
	h.register("Category.filter", h.filterRows("categories", []string{"t.id", "t.name"}))
	h.register("Component.filter", h.filterRows("components", []string{"t.id", "t.name", "t.product_id", "COALESCE(t.description,'') AS description"}, map[string]relationJoin{
		"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
	}))
	h.register("Tag.filter", h.filterRows("tags", []string{"t.id", "t.name"}))
	h.register("Priority.filter", h.filterRows("priorities", []string{"t.id", "t.value", "t.is_active"}))
	h.register("TestExecutionStatus.filter", h.filterRows("execution_statuses", []string{"t.id", "t.name", "t.weight", "COALESCE(t.color,'') AS color"}))
	h.register("TestCaseStatus.filter", h.filterRows("test_case_statuses", []string{"t.id", "t.name", "COALESCE(t.description,'') AS description"}))
	h.register("User.filter", h.filterRows("users", []string{"t.id", "t.username", "t.email", "COALESCE(t.role,'') AS role", "t.is_active"}))
	h.register("Group.filter", h.filterRows("user_groups", []string{"t.id", "t.name"}))
	h.register("Environment.filter", h.filterRows("environments", []string{"t.id", "t.name", "COALESCE(t.description,'') AS description"}))

	// TestCase CRUD + relations
	h.register("TestCase.create", h.tcCreate)
	h.register("TestCase.update", h.tcUpdate)
	h.register("TestCase.remove", h.tcRemove)
	h.register("TestCase.add_tag", h.tcAddTag)
	h.register("TestCase.remove_tag", h.tcRemoveTag)
	h.register("TestCase.add_comment", h.tcAddComment)
	h.register("TestCase.comments", h.tcComments)
	h.register("TestCase.history", h.tcHistory)

	// TestPlan CRUD + relations
	h.register("TestPlan.create", h.tpCreate)
	h.register("TestPlan.update", h.tpUpdate)
	h.register("TestPlan.remove", h.tpRemove)
	h.register("TestPlan.add_case", h.tpAddCase)
	h.register("TestPlan.remove_case", h.tpRemoveCase)
	h.register("TestPlan.get_cases", h.tpGetCases)

	// TestRun CRUD + relations
	h.register("TestRun.create", h.trCreate)
	h.register("TestRun.update", h.trUpdate)
	h.register("TestRun.add_case", h.trAddCase)
	h.register("TestRun.remove_case", h.trRemoveCase)
	h.register("TestRun.get_cases", h.trGetCases)
	h.register("TestRun.add_cc", h.trAddCC)
	h.register("TestRun.remove_cc", h.trRemoveCC)
	h.register("TestRun.get_cc", h.trGetCC)

	// Simple CRUD
	h.register("Build.create", h.simpleCreate("builds", []string{"name", "version_id"}))
	h.register("Product.create", h.simpleCreate("products", []string{"name", "description"}))
	h.register("Version.create", h.simpleCreate("versions", []string{"value", "product_id"}))
	h.register("Category.create", h.simpleCreate("categories", []string{"name", "product_id"}))
	h.register("Component.create", h.simpleCreate("components", []string{"name", "product_id", "description"}))
	h.register("Tag.create", h.simpleCreate("tags", []string{"name"}))
	h.register("Priority.create", h.simpleCreate("priorities", []string{"value"}))
	h.register("Environment.create", h.simpleCreate("environments", []string{"name", "description"}))
	h.register("Group.create", h.simpleCreate("user_groups", []string{"name"}))
	h.register("Group.users", h.groupUsers)
	h.register("User.join_group", h.userJoinGroup)

	// LinkReference
	h.register("TestExecution.add_link", h.addLink("test_execution"))
	h.register("TestExecution.get_links", h.getLinks("test_execution"))
	h.register("TestExecution.remove_link", h.removeLink)
	h.register("TestRun.add_link", h.addLink("test_run"))
	h.register("TestRun.get_links", h.getLinks("test_run"))

	// Telemetry
	h.register("Telemetry.breakdown", h.telemetryBreakdown)
	h.register("Telemetry.status_matrix", h.telemetryMatrix)
	h.register("Telemetry.execution_trends", h.telemetryTrends)
	h.register("Telemetry.test_case_health", h.telemetryHealth)

	h.registerExecutionMethods()
	h.registerMissingMethods()
}

func argMap(args []any, keys ...string) map[string]any {
	if len(args) == 0 {
		return map[string]any{}
	}
	if m, ok := args[0].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	for i, k := range keys {
		if i < len(args) {
			m[k] = args[i]
		}
	}
	return m
}

func argInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

func argStr(v any) string {
	s, _ := v.(string)
	return s
}

func (h *Handler) markdownRender(_ *http.Request, args []any) (any, error) {
	m := argMap(args, "text")
	return map[string]any{"html": renderMarkdown(argStr(m["text"]))}, nil
}

// --- TestCase ---

func (h *Handler) tcCreate(r *http.Request, args []any) (any, error) {
	m := argMap(args, "values")
	if h.db == nil {
		return nil, errNoDB
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO test_cases (product_id, summary, preconditions, priority, status, is_automated, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'confirmed', 0, ?, ?)`,
		argInt(m["product_id"]), argStr(m["summary"]), argStr(m["preconditions"]), argStr(m["priority"]), now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id, "summary": argStr(m["summary"])}, nil
}

func (h *Handler) tcUpdate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	id := argInt(args[0])
	m := argMap(args, "values")
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := h.db.ExecContext(r.Context(),
		`UPDATE test_cases SET summary = COALESCE(?, summary), preconditions = COALESCE(?, preconditions), priority = COALESCE(?, priority), updated_at = ? WHERE id = ?`,
		argStr(m["summary"]), argStr(m["preconditions"]), argStr(m["priority"]), now, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (h *Handler) tcRemove(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	// Soft-delete: the case moves to the trash (deleted_at) like the REST API.
	_, err := h.db.ExecContext(r.Context(), `UPDATE test_cases SET deleted_at = ? WHERE id = ? AND deleted_at IS NULL`, time.Now().UTC(), argInt(args[0]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tcAddTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	tag := argStr(args[1])
	var tagID int64
	err := h.db.QueryRowContext(r.Context(), `SELECT id FROM tags WHERE name = ?`, tag).Scan(&tagID)
	if err == sql.ErrNoRows {
		res, err := h.db.ExecContext(r.Context(), `INSERT INTO tags (name) VALUES (?)`, tag)
		if err != nil {
			return nil, err
		}
		tagID, _ = res.LastInsertId()
	} else if err != nil {
		return nil, err
	}
	_, err = h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO test_case_tags (test_case_id, tag_id) VALUES (?, ?)`, caseID, tagID)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tcRemoveTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_case_tags WHERE test_case_id = ? AND tag_id = (SELECT id FROM tags WHERE name = ?)`, argInt(args[0]), argStr(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tcAddComment(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	res, err := h.db.ExecContext(r.Context(), `INSERT INTO comments (entity_type, entity_id, body) VALUES ('test_case', ?, ?)`, argInt(args[0]), argStr(args[1]))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id}, nil
}

func (h *Handler) tcComments(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, COALESCE(author_id,0), body, created_at FROM comments WHERE entity_type = 'test_case' AND entity_id = ? ORDER BY id`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, authorID int64
		var body, created string
		if err := rows.Scan(&id, &authorID, &body, &created); err == nil {
			out = append(out, map[string]any{"id": id, "author_id": authorID, "body": body, "created_at": created})
		}
	}
	return out, nil
}

func (h *Handler) tcHistory(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, summary, updated_at FROM test_case_revisions WHERE test_case_id = ? ORDER BY id DESC LIMIT 50`, argInt(args[0]))
	if err != nil {
		// table may not exist in older schemas — return empty
		return []map[string]any{}, nil
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var summary, updated string
		if err := rows.Scan(&id, &summary, &updated); err == nil {
			out = append(out, map[string]any{"id": id, "summary": summary, "changed_at": updated})
		}
	}
	return out, nil
}

// --- TestPlan ---

func (h *Handler) tpCreate(r *http.Request, args []any) (any, error) {
	m := argMap(args, "values")
	if h.db == nil {
		return nil, errNoDB
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO test_plans (product_id, version_id, name, description, is_active, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`,
		argInt(m["product_id"]), argInt(m["version_id"]), argStr(m["name"]), argStr(m["description"]), now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id, "name": argStr(m["name"])}, nil
}

func (h *Handler) tpUpdate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	id := argInt(args[0])
	m := argMap(args, "values")
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := h.db.ExecContext(r.Context(), `UPDATE test_plans SET name = COALESCE(?, name), description = COALESCE(?, description), updated_at = ? WHERE id = ?`,
		argStr(m["name"]), argStr(m["description"]), now, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (h *Handler) tpRemove(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_plans WHERE id = ?`, argInt(args[0]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tpAddCase(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	var pos int64
	_ = h.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(position),0)+1 FROM test_plan_cases WHERE test_plan_id = ?`, argInt(args[0])).Scan(&pos)
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO test_plan_cases (test_plan_id, test_case_id, position) VALUES (?, ?, ?)`, argInt(args[0]), argInt(args[1]), pos)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tpRemoveCase(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_plan_cases WHERE test_plan_id = ? AND test_case_id = ?`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tpGetCases(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT tc.id, tc.summary, tpc.position FROM test_plan_cases tpc
		JOIN test_cases tc ON tc.id = tpc.test_case_id
		WHERE tpc.test_plan_id = ? ORDER BY tpc.position, tc.id`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, pos int64
		var summary string
		if err := rows.Scan(&id, &summary, &pos); err == nil {
			out = append(out, map[string]any{"id": id, "summary": summary, "sortkey": pos})
		}
	}
	return out, nil
}

// --- TestRun ---

func (h *Handler) trCreate(r *http.Request, args []any) (any, error) {
	m := argMap(args, "values")
	if h.db == nil {
		return nil, errNoDB
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO test_runs (plan_id, build_id, summary, started_at) VALUES (?, ?, ?, ?)`,
		argInt(m["plan_id"]), argInt(m["build_id"]), argStr(m["summary"]), now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id, "summary": argStr(m["summary"])}, nil
}

func (h *Handler) trUpdate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	id := argInt(args[0])
	m := argMap(args, "values")
	_, err := h.db.ExecContext(r.Context(), `UPDATE test_runs SET summary = COALESCE(?, summary) WHERE id = ?`, argStr(m["summary"]), id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func (h *Handler) trAddCase(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO test_run_cases (run_id, case_id) VALUES (?, ?)`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) trRemoveCase(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_run_cases WHERE run_id = ? AND case_id = ?`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) trGetCases(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT tc.id, tc.summary FROM test_run_cases trc JOIN test_cases tc ON tc.id = trc.case_id WHERE trc.run_id = ? ORDER BY tc.id`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var summary string
		if err := rows.Scan(&id, &summary); err == nil {
			out = append(out, map[string]any{"id": id, "summary": summary})
		}
	}
	return out, nil
}

func (h *Handler) trAddCC(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO test_run_cc (run_id, user_id) VALUES (?, ?)`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) trRemoveCC(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_run_cc WHERE run_id = ? AND user_id = ?`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) trGetCC(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT user_id FROM test_run_cc WHERE run_id = ? ORDER BY user_id`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			out = append(out, id)
		}
	}
	return out, nil
}

// --- Simple CRUD ---

func (h *Handler) simpleCreate(table string, cols []string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		m := argMap(args, "values")
		placeholders := make([]string, len(cols))
		vals := make([]any, len(cols))
		for i, c := range cols {
			placeholders[i] = "?"
			v := m[c]
			if argStr(v) == "" && argInt(v) != 0 {
				vals[i] = argInt(v)
			} else {
				vals[i] = argStr(v)
			}
		}
		res, err := h.db.ExecContext(r.Context(),
			fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, table, strings.Join(cols, ", "), strings.Join(placeholders, ", ")), vals...)
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		return map[string]any{"id": id}, nil
	}
}

func (h *Handler) groupUsers(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT u.id, u.username FROM user_group_members m JOIN users u ON u.id = m.user_id WHERE m.group_id = ? ORDER BY u.id`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			out = append(out, map[string]any{"id": id, "username": name})
		}
	}
	return out, nil
}

func (h *Handler) userJoinGroup(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	// args: username, groupname
	var uid, gid int64
	if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE username = ? OR email = ?`, argStr(args[0]), argStr(args[0])).Scan(&uid); err != nil {
		return nil, err
	}
	if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM user_groups WHERE name = ?`, argStr(args[1])).Scan(&gid); err != nil {
		return nil, err
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO user_group_members (group_id, user_id) VALUES (?, ?)`, gid, uid)
	return map[string]any{"ok": err == nil}, err
}

// --- LinkReference ---

func (h *Handler) addLink(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		m := argMap(args, "values")
		entityID := argInt(args[0])
		if entityID == 0 {
			entityID = argInt(m["execution_id"])
		}
		if entityID == 0 {
			entityID = argInt(m["run_id"])
		}
		res, err := h.db.ExecContext(r.Context(),
			`INSERT INTO link_references (entity_type, entity_id, name, url) VALUES (?, ?, ?, ?)`,
			entityType, entityID, argStr(m["name"]), argStr(m["url"]))
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		return map[string]any{"id": id}, nil
	}
}

func (h *Handler) getLinks(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		entityID := argInt(args[0])
		rows, err := h.db.QueryContext(r.Context(),
			`SELECT id, name, url FROM link_references WHERE entity_type = ? AND entity_id = ? ORDER BY id`, entityType, entityID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id int64
			var name, url string
			if err := rows.Scan(&id, &name, &url); err == nil {
				out = append(out, map[string]any{"id": id, "name": name, "url": url})
			}
		}
		return out, nil
	}
}

func (h *Handler) removeLink(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM link_references WHERE id = ?`, argInt(args[0]))
	return map[string]any{"ok": err == nil}, err
}

var errNoDB = fmt.Errorf("database not available")
