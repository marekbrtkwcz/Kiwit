package xmlrpc

// This file implements the XML-RPC methods present in Kiwi TCMS but missing
// from the initial Kiwit implementation. Method names match the tcms-api
// contract exactly (e.g. "TestCase.add_component").

import (
	"fmt"
	"net/http"
	"strings"
)

// registerMissingMethods wires every method that kiwitcms exposes but Kiwit
// did not yet implement. Called from registerAll().
func (h *Handler) registerMissingMethods() {
	// --- Dictionaries: filters ---
	h.register("Classification.filter", h.filterRows("classifications", []string{"id", "name"}))
	h.register("PlanType.filter", h.filterRows("test_plan_types", []string{"id", "name"}))
	h.register("Template.filter", h.filterRows("templates", []string{"id", "name", "body"}))
	h.register("BugTracker.filter", h.filterRows("issue_trackers", []string{"id", "name", "provider", "base_url"}))

	// --- Dictionaries: create/update ---
	h.register("Template.create", h.simpleCreate("templates", []string{"name", "body"}))
	h.register("BugTracker.create", h.simpleCreate("issue_trackers", []string{"name", "provider", "base_url"}))
	h.register("Component.update", h.simpleUpdate("components", []string{"name", "description", "product_id"}))
	h.register("Build.update", h.simpleUpdate("builds", []string{"name", "version_id", "description"}))

	// --- Environment properties (stored in JSONB column) ---
	h.register("Environment.properties", h.envProperties)
	h.register("Environment.add_property", h.envAddProperty)
	h.register("Environment.remove_property", h.envRemoveProperty)

	// --- Group permissions ---
	h.register("Group.permissions", h.groupPermissions)

	// --- TestCase extras ---
	h.register("TestCase.add_component", h.tcAddComponent)
	h.register("TestCase.remove_component", h.tcRemoveComponent)
	h.register("TestCase.sortkeys", h.tcSortkeys)
	h.register("TestCase.remove_comment", h.tcRemoveComment)
	h.register("TestCase.properties", h.entityProperties("test_case"))
	h.register("TestCase.add_property", h.entityAddProperty("test_case"))
	h.register("TestCase.remove_property", h.entityRemoveProperty("test_case"))
	h.register("TestCase.add_notification_cc", h.tcAddNotificationCC)
	h.register("TestCase.remove_notification_cc", h.tcRemoveNotificationCC)
	h.register("TestCase.get_notification_cc", h.tcGetNotificationCC)

	// --- TestPlan extras ---
	h.register("TestPlan.clone", h.tpClone)
	h.register("TestPlan.update_case_order", h.tpUpdateCaseOrder)
	h.register("TestPlan.list_attachments", h.listAttachments("test_plan"))
	h.register("TestPlan.add_attachment", h.addAttachment("test_plan"))

	// --- TestRun extras ---
	h.register("TestRun.remove", h.trRemove)
	h.register("TestRun.properties", h.entityProperties("test_run"))
	h.register("TestRun.add_property", h.entityAddProperty("test_run"))
	h.register("TestRun.add_tag", h.runAddTag)
	h.register("TestRun.remove_tag", h.runRemoveTag)

	// --- TestExecution extras ---
	h.register("TestExecution.create", h.execCreate)
	h.register("TestExecution.remove", h.execRemove)
	h.register("TestExecution.count_attachments", h.countAttachments("test_execution"))

	// --- Bug / User / Attachment aliases ---
	h.register("Bug.details", h.bugDetails)
	h.register("User.add_attachment", h.addAttachment("user"))
	h.register("Attachment.remove_attachment", h.removeAttachment) // alias for Attachment.remove

	// --- Requirements + traceability ---
	h.register("Requirement.filter", h.filterRows("requirements", []string{"id", "product_id", "key", "name", "description", "version", "status", "priority", "category", "author_id", "created_at", "updated_at"}))
	h.register("Requirement.create", h.simpleCreate("requirements", []string{"product_id", "key", "name", "description", "version", "status", "priority", "category", "author_id"}))
	h.register("Requirement.update", h.simpleUpdate("requirements", []string{"product_id", "key", "name", "description", "version", "status", "priority", "category"}))
	h.register("Requirement.remove", h.reqRemove)
	h.register("Requirement.link_case", h.reqLinkCase)
	h.register("Requirement.unlink_case", h.reqUnlinkCase)
	h.register("Requirement.get_cases", h.reqGetCases)

	// --- Milestones (release / go-no-go) ---
	h.register("Milestone.filter", h.filterRows("milestones", []string{"id", "version_id", "name", "description", "due_date", "status", "signoff_decision", "signoff_note", "created_at", "updated_at"}))
	h.register("Milestone.create", h.simpleCreate("milestones", []string{"version_id", "name", "description", "due_date", "status"}))
	h.register("Milestone.update", h.simpleUpdate("milestones", []string{"version_id", "name", "description", "due_date", "status"}))
	h.register("Milestone.remove", h.msRemove)
	h.register("Milestone.add_run", h.msLinkRun)
	h.register("Milestone.remove_run", h.msUnlinkRun)
}

// --- generic update helper ---

func (h *Handler) simpleUpdate(table string, cols []string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		id := argInt(args[0])
		m := argMap(args, "values")
		setClauses := make([]string, 0, len(cols))
		vals := make([]any, 0, len(cols)+1)
		for _, c := range cols {
			if v, ok := m[c]; ok {
				setClauses = append(setClauses, c+" = ?")
				vals = append(vals, v)
			}
		}
		if len(setClauses) == 0 {
			return map[string]any{"id": id}, nil
		}
		vals = append(vals, id)
		_, err := h.db.ExecContext(r.Context(),
			fmt.Sprintf("UPDATE %s SET %s WHERE id = ?", table, strings.Join(setClauses, ", ")), vals...)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": id}, nil
	}
}

// --- Environment properties (JSONB column on environments table) ---

func (h *Handler) envProperties(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	envID := argInt(args[0])
	var props string
	if err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(properties,'{}') FROM environments WHERE id = ?`, envID).Scan(&props); err != nil {
		return nil, fmt.Errorf("environment %d not found", envID)
	}
	// Return as list of {name, value} dicts like kiwitcms
	var out []map[string]any
	if props != "" && props != "{}" {
		// simple JSON object parse — properties stored as {"key":"val"}
		out = jsonObjectToList(props)
	}
	return out, nil
}

func (h *Handler) envAddProperty(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	envID := argInt(args[0])
	name := argStr(args[1])
	value := argStr(args[2])
	// Read current, merge, write back
	var props string
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(properties,'{}') FROM environments WHERE id = ?`, envID).Scan(&props)
	if err != nil {
		return nil, fmt.Errorf("environment %d not found", envID)
	}
	merged := mergeJSONProp(props, name, value)
	if _, err := h.db.ExecContext(r.Context(), `UPDATE environments SET properties = ? WHERE id = ?`, merged, envID); err != nil {
		return nil, err
	}
	return map[string]any{"name": name, "value": value}, nil
}

func (h *Handler) envRemoveProperty(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	envID := argInt(args[0])
	name := argStr(args[1])
	var props string
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(properties,'{}') FROM environments WHERE id = ?`, envID).Scan(&props)
	if err != nil {
		return nil, fmt.Errorf("environment %d not found", envID)
	}
	merged := removeJSONProp(props, name)
	if _, err := h.db.ExecContext(r.Context(), `UPDATE environments SET properties = ? WHERE id = ?`, merged, envID); err != nil {
		return nil, err
	}
	return true, nil
}

// --- Group permissions ---

func (h *Handler) groupPermissions(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	gid := argInt(args[0])
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT p.id, p.name, g.role FROM group_product_permissions g
		JOIN products p ON p.id = g.product_id WHERE g.group_id = ? ORDER BY p.name`, gid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var pid int64
		var pname, role string
		if err := rows.Scan(&pid, &pname, &role); err == nil {
			out = append(out, map[string]any{"product_id": pid, "product": pname, "role": role})
		}
	}
	return out, nil
}

// --- TestCase: components ---

func (h *Handler) tcAddComponent(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	compName := argStr(args[1])
	var compID int64
	if err := h.db.QueryRowContext(r.Context(), `SELECT id FROM components WHERE name = ?`, compName).Scan(&compID); err != nil {
		return nil, fmt.Errorf("component %q not found", compName)
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO test_case_components (test_case_id, component_id) VALUES (?, ?)`, caseID, compID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": compID, "name": compName}, nil
}

func (h *Handler) tcRemoveComponent(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	compID := argInt(args[1])
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_case_components WHERE test_case_id = ? AND component_id = ?`, caseID, compID)
	return map[string]any{"ok": err == nil}, err
}

// --- TestCase: sortkeys ---

func (h *Handler) tcSortkeys(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	var sk int64
	err := h.db.QueryRowContext(r.Context(), `SELECT COALESCE(sortkey, id) FROM test_cases WHERE id = ?`, caseID).Scan(&sk)
	if err != nil {
		return int64(0), nil
	}
	return sk, nil
}

// --- TestCase: remove_comment ---

func (h *Handler) tcRemoveComment(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	commentID := int64(0)
	if len(args) > 1 {
		commentID = argInt(args[1])
	}
	if commentID > 0 {
		_, err := h.db.ExecContext(r.Context(), `DELETE FROM comments WHERE id = ? AND entity_type = 'test_case' AND entity_id = ?`, commentID, caseID)
		return map[string]any{"ok": err == nil}, err
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM comments WHERE entity_type = 'test_case' AND entity_id = ?`, caseID)
	return map[string]any{"ok": err == nil}, err
}

// --- Entity properties (entity_properties table) ---

func (h *Handler) entityProperties(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		query := map[string]any{}
		if len(args) > 0 {
			if m, ok := args[0].(map[string]any); ok {
				query = m
			}
		}
		where := "entity_type = ?"
		vals := []any{entityType}
		if id, ok := query["case_id"]; ok {
			where += " AND entity_id = ?"
			vals = append(vals, argInt(id))
		} else if id, ok := query["run_id"]; ok {
			where += " AND entity_id = ?"
			vals = append(vals, argInt(id))
		}
		rows, err := h.db.QueryContext(r.Context(),
			`SELECT id, entity_id, name, value FROM entity_properties WHERE `+where+` ORDER BY entity_id, name`, vals...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var id, eid int64
			var name, value string
			if err := rows.Scan(&id, &eid, &name, &value); err == nil {
				out = append(out, map[string]any{"id": id, "case": eid, "name": name, "value": value})
			}
		}
		return out, nil
	}
}

func (h *Handler) entityAddProperty(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		entityID := argInt(args[0])
		name := argStr(args[1])
		value := argStr(args[2])
		res, err := h.db.ExecContext(r.Context(),
			`INSERT INTO entity_properties (entity_type, entity_id, name, value) VALUES (?, ?, ?, ?)
			 ON CONFLICT (entity_type, entity_id, name) DO UPDATE SET value = excluded.value`,
			entityType, entityID, name, value)
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		return map[string]any{"id": id, "name": name, "value": value}, nil
	}
}

func (h *Handler) entityRemoveProperty(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		m := argMap(args, "query")
		where := "entity_type = ?"
		vals := []any{entityType}
		if id, ok := m["case_id"]; ok {
			where += " AND entity_id = ?"
			vals = append(vals, argInt(id))
		}
		if id, ok := m["run_id"]; ok {
			where += " AND entity_id = ?"
			vals = append(vals, argInt(id))
		}
		if n, ok := m["name"]; ok {
			where += " AND name = ?"
			vals = append(vals, argStr(n))
		}
		_, err := h.db.ExecContext(r.Context(), `DELETE FROM entity_properties WHERE `+where, vals...)
		return map[string]any{"ok": err == nil}, err
	}
}

// --- TestCase: notification CC (stored as entity_properties with prefix) ---

func (h *Handler) tcAddNotificationCC(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	email := argStr(args[1])
	// Store as property with special name prefix
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO entity_properties (entity_type, entity_id, name, value) VALUES ('test_case', ?, ?, ?)
		 ON CONFLICT (entity_type, entity_id, name) DO NOTHING`,
		caseID, "notification_cc:"+email, email)
	if err != nil {
		return nil, err
	}
	return map[string]any{"cc": email}, nil
}

func (h *Handler) tcRemoveNotificationCC(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	email := argStr(args[1])
	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM entity_properties WHERE entity_type = 'test_case' AND entity_id = ? AND name = ?`,
		caseID, "notification_cc:"+email)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) tcGetNotificationCC(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	caseID := argInt(args[0])
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT value FROM entity_properties WHERE entity_type = 'test_case' AND entity_id = ? AND name LIKE 'notification_cc:%' ORDER BY value`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err == nil {
			out = append(out, map[string]any{"cc": email, "id": email})
		}
	}
	return out, nil
}

// --- TestPlan: clone ---

func (h *Handler) tpClone(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	srcID := argInt(args[0])
	// Read source plan
	var productID, versionID, authorID int64
	var name, desc string
	var active bool
	err := h.db.QueryRowContext(r.Context(),
		`SELECT product_id, version_id, COALESCE(author_id,0), name, description, is_active FROM test_plans WHERE id = ?`, srcID).
		Scan(&productID, &versionID, &authorID, &name, &desc, &active)
	if err != nil {
		return nil, fmt.Errorf("source plan %d not found", srcID)
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO test_plans (product_id, version_id, name, description, is_active, author_id) VALUES (?, ?, ?, ?, ?, ?)`,
		productID, versionID, name+" (clone)", desc, active, authorID)
	if err != nil {
		return nil, err
	}
	newID, _ := res.LastInsertId()
	// Copy case associations with positions
	_, _ = h.db.ExecContext(r.Context(),
		`INSERT INTO test_plan_cases (test_plan_id, test_case_id, position) SELECT ?, test_case_id, position FROM test_plan_cases WHERE test_plan_id = ?`,
		newID, srcID)
	// Copy tags
	_, _ = h.db.ExecContext(r.Context(),
		`INSERT OR IGNORE INTO test_plan_tags (test_plan_id, tag_id) SELECT ?, tag_id FROM test_plan_tags WHERE test_plan_id = ?`,
		newID, srcID)
	return map[string]any{"id": newID, "name": name + " (clone)"}, nil
}

// --- TestPlan: update_case_order ---

func (h *Handler) tpUpdateCaseOrder(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	planID := argInt(args[0])
	caseID := argInt(args[1])
	sortkey := argInt(args[2])
	_, err := h.db.ExecContext(r.Context(),
		`UPDATE test_plan_cases SET position = ? WHERE test_plan_id = ? AND test_case_id = ?`, sortkey, planID, caseID)
	return map[string]any{"ok": err == nil}, err
}

// --- TestRun: remove ---

func (h *Handler) trRemove(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "query")
	id := argInt(m["id"])
	if id == 0 && len(args) > 0 {
		id = argInt(args[0])
	}
	if id == 0 {
		return nil, fmt.Errorf("id required")
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_runs WHERE id = ?`, id)
	return map[string]any{"ok": err == nil}, err
}

// --- TestExecution: create / remove / count_attachments ---

func (h *Handler) execCreate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "values")
	runID := argInt(m["run_id"])
	caseID := argInt(m["case_id"])
	if runID == 0 {
		runID = argInt(m["test_run_id"])
	}
	status := argStr(m["status"])
	if status == "" {
		status = "idle"
	}
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO test_executions (test_run_id, test_case_id, status) VALUES (?, ?, ?)`, runID, caseID, status)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return map[string]any{"id": id, "run_id": runID, "case_id": caseID, "status": status}, nil
}

func (h *Handler) execRemove(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "query")
	id := argInt(m["id"])
	if id == 0 && len(args) > 0 {
		id = argInt(args[0])
	}
	if id == 0 {
		return nil, fmt.Errorf("id required")
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM test_executions WHERE id = ?`, id)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) countAttachments(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		entityID := argInt(args[0])
		var cnt int64
		err := h.db.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM attachments WHERE entity_type = ? AND entity_id = ?`, entityType, entityID).Scan(&cnt)
		if err != nil {
			return int64(0), nil
		}
		return cnt, nil
	}
}

// --- Bug.details ---

func (h *Handler) bugDetails(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	bugID := argInt(args[0])
	var id, execID, trackerID int64
	var extKey, url, title, status string
	err := h.db.QueryRowContext(r.Context(),
		`SELECT id, test_execution_id, COALESCE(tracker_id,0), external_key, url, title, status FROM bugs WHERE id = ?`, bugID).
		Scan(&id, &execID, &trackerID, &extKey, &url, &title, &status)
	if err != nil {
		return nil, fmt.Errorf("bug %d not found", bugID)
	}
	return map[string]any{
		"id": id, "execution_id": execID, "tracker_id": trackerID,
		"external_key": extKey, "url": url, "title": title, "status": status,
	}, nil
}

// --- TestRun tags ---

func (h *Handler) runAddTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	runID := argInt(args[0])
	tagName := argStr(args[1])
	var tagID int64
	// get or create tag
	err := h.db.QueryRowContext(r.Context(), `SELECT id FROM tags WHERE name = ?`, tagName).Scan(&tagID)
	if err != nil {
		res, err2 := h.db.ExecContext(r.Context(), `INSERT INTO tags (name) VALUES (?)`, tagName)
		if err2 != nil {
			return nil, err2
		}
		tagID, _ = res.LastInsertId()
	}
	_, err = h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO test_run_tags (test_run_id, tag_id) VALUES (?, ?)`, runID, tagID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": tagID, "name": tagName}, nil
}

func (h *Handler) runRemoveTag(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	runID := argInt(args[0])
	tagName := argStr(args[1])
	_, err := h.db.ExecContext(r.Context(),
		`DELETE FROM test_run_tags WHERE test_run_id = ? AND tag_id IN (SELECT id FROM tags WHERE name = ?)`, runID, tagName)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) reqLinkCase(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	reqID := argInt(args[0])
	caseID := argInt(args[1])
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO requirement_coverage (requirement_id, test_case_id) VALUES (?, ?)`, reqID, caseID)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) reqUnlinkCase(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	reqID := argInt(args[0])
	caseID := argInt(args[1])
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM requirement_coverage WHERE requirement_id = ? AND test_case_id = ?`, reqID, caseID)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) reqGetCases(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	reqID := argInt(args[0])
	rows, err := h.db.QueryContext(r.Context(), `SELECT tc.id, tc.summary, tc.status FROM requirement_coverage rc JOIN test_cases tc ON tc.id = rc.test_case_id WHERE rc.requirement_id = ? ORDER BY tc.id`, reqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var summary, status string
		if err := rows.Scan(&id, &summary, &status); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "summary": summary, "status": status})
	}
	return out, rows.Err()
}

func (h *Handler) reqRemove(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	id := argInt(args[0])
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM requirements WHERE id = ?`, id)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) msRemove(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	id := argInt(args[0])
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM milestones WHERE id = ?`, id)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) msLinkRun(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `INSERT OR IGNORE INTO milestone_runs (milestone_id, test_run_id) VALUES (?, ?)`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) msUnlinkRun(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM milestone_runs WHERE milestone_id = ? AND test_run_id = ?`, argInt(args[0]), argInt(args[1]))
	return map[string]any{"ok": err == nil}, err
}
