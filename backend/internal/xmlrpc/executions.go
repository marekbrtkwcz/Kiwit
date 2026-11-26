package xmlrpc

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// registerExecutionMethods adds TestExecution.* and TestPlan.tree/tag methods.
func (h *Handler) registerExecutionMethods() {
	// TestExecution
	h.register("TestExecution.filter", h.filterRows("test_executions", []string{"t.id", "t.test_run_id AS run_id", "t.test_case_id AS case_id", "t.status", "COALESCE(t.assignee_id,0) AS assignee_id", "COALESCE(t.tested_by_id,0) AS tested_by_id"}, map[string]relationJoin{
		"run":    {joinSQL: "JOIN test_runs tr ON tr.id = t.test_run_id", alias: "tr"},
		"case":   {joinSQL: "JOIN test_cases tc ON tc.id = t.test_case_id", alias: "tc"},
		"assignee": {joinSQL: "JOIN users ua ON ua.id = t.assignee_id", alias: "ua"},
	}))
	h.register("TestExecution.update", h.execUpdate)
	h.register("TestExecution.add_comment", h.execAddComment)
	h.register("TestExecution.get_comments", h.execGetComments)
	h.register("TestExecution.remove_comment", h.execRemoveComment)
	h.register("TestExecution.history", h.execHistory)
	h.register("TestExecution.add_tag", h.execAddTag)
	h.register("TestExecution.remove_tag", h.execRemoveTag)
	h.register("TestExecution.add_property", h.execAddProperty)
	h.register("TestExecution.properties", h.execProperties)
	h.register("TestExecution.remove_property", h.execRemoveProperty)

	// TestPlan extras
	h.register("TestPlan.tree", h.planTree)
	h.register("TestPlan.add_tag", h.planAddTag)
	h.register("TestPlan.remove_tag", h.planRemoveTag)
}

func (h *Handler) execUpdate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	id := argInt(args[0])
	m := argMap(args, "values")
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	status := argStr(m["status"])
	comment := argStr(m["comment"])
	_, err := h.db.ExecContext(r.Context(), `UPDATE test_executions SET status = COALESCE(?, status), comment = COALESCE(?, comment), updated_at = ?, executed_at = COALESCE(executed_at, ?) WHERE id = ?`,
		nullIfEmpty(status), nullIfEmpty(comment), now, now, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (h *Handler) execAddComment(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	res, err := h.db.ExecContext(r.Context(), `INSERT INTO comments (entity_type, entity_id, body) VALUES ('test_execution', ?, ?)`, argInt(args[0]), argStr(args[1]))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id}, nil
}

func (h *Handler) execGetComments(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, COALESCE(author_id,0), body, created_at FROM comments WHERE entity_type = 'test_execution' AND entity_id = ? ORDER BY id`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, authorID int64
		var body, created string
		if err := rows.Scan(&id, &authorID, &body, &created); err == nil {
			out = append(out, map[string]any{"id": id, "author_id": authorID, "body": body, "date": created})
		}
	}
	return out, nil
}

func (h *Handler) execRemoveComment(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM comments WHERE id = ? AND entity_type = 'test_execution'`, argInt(args[0]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) execHistory(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, status, comment, updated_at FROM test_executions WHERE id = ? ORDER BY updated_at DESC LIMIT 1`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		var id int64
		var status, comment, updated string
		if err := rows.Scan(&id, &status, &comment, &updated); err == nil {
			return []map[string]any{{"id": id, "status": status, "comment": comment, "changed_at": updated}}, nil
		}
	}
	return []map[string]any{}, nil
}

func (h *Handler) execAddTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	return h.addTagTo("test_execution_tags", "test_execution_id", r, args)
}

func (h *Handler) execRemoveTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_execution_tags WHERE test_execution_id = ? AND tag_id = (SELECT id FROM tags WHERE name = ?)`, argInt(args[0]), argStr(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) execAddProperty(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO entity_properties (entity_type, entity_id, name, value) VALUES ('test_execution', ?, ?, ?)
		 ON CONFLICT (entity_type, entity_id, name) DO UPDATE SET value = excluded.value`,
		argInt(args[0]), argStr(args[1]), argStr(args[2]))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id}, nil
}

func (h *Handler) execProperties(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rows, err := h.db.QueryContext(r.Context(), `SELECT name, value FROM entity_properties WHERE entity_type = 'test_execution' AND entity_id = ? ORDER BY name`, argInt(args[0]))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]any{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			out[k] = v
		}
	}
	return out, nil
}

func (h *Handler) execRemoveProperty(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM entity_properties WHERE entity_type = 'test_execution' AND entity_id = ? AND name = ?`, argInt(args[0]), argStr(args[1]))
	return map[string]any{"ok": err == nil}, err
}

// addTagTo is a helper for tag M2M tables.
func (h *Handler) addTagTo(table, fkCol string, r *http.Request, args []any) (any, error) {
	entityID := argInt(args[0])
	tagName := argStr(args[1])
	var tagID int64
	err := h.db.QueryRowContext(r.Context(), `SELECT id FROM tags WHERE name = ?`, tagName).Scan(&tagID)
	if err == sql.ErrNoRows {
		res, err := h.db.ExecContext(r.Context(), `INSERT INTO tags (name) VALUES (?)`, tagName)
		if err != nil {
			return nil, err
		}
		tagID, _ = res.LastInsertId()
	} else if err != nil {
		return nil, err
	}
	_, err = h.db.ExecContext(r.Context(),
		fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s, tag_id) VALUES (?, ?)`, table, fkCol), entityID, tagID)
	return map[string]any{"ok": err == nil}, err
}

// --- TestPlan tree & tags ---

func (h *Handler) planTree(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	rootID := argInt(args[0])
	// Return the plan with its children recursively (one level deep per call, like Kiwi's tree view).
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, name, COALESCE(parent_id, 0) FROM test_plans WHERE id = ? OR parent_id = ? ORDER BY id`, rootID, rootID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, parentID int64
		var name string
		if err := rows.Scan(&id, &name, &parentID); err == nil {
			out = append(out, map[string]any{"id": id, "name": name, "parent_id": parentID})
		}
	}
	return out, nil
}

func (h *Handler) planAddTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	return h.addTagTo("test_plan_tags", "test_plan_id", r, args)
}

func (h *Handler) planRemoveTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_plan_tags WHERE test_plan_id = ? AND tag_id = (SELECT id FROM tags WHERE name = ?)`, argInt(args[0]), argStr(args[1]))
	return map[string]any{"ok": err == nil}, err
}

var _ = strings.TrimSpace
