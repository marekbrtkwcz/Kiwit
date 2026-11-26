package xmlrpc

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kiwit/kiwit/internal/auth"
)

// registerExtrasMethods adds Attachment, Bug, dictionary, search and admin methods.
func (h *Handler) registerExtrasMethods(uploadDir string) {
	h.uploadDir = uploadDir

	// Attachments (base64 content like Kiwi TCMS)
	h.register("TestCase.add_attachment", h.addAttachment("test_case"))
	h.register("TestCase.list_attachments", h.listAttachments("test_case"))
	h.register("TestRun.add_attachment", h.addAttachment("test_run"))
	h.register("TestRun.list_attachments", h.listAttachments("test_run"))
	h.register("TestExecution.add_attachment", h.addAttachment("test_execution"))
	h.register("TestExecution.list_attachments", h.listAttachments("test_execution"))
	h.register("Attachment.remove", h.removeAttachment)

	// Bugs
	h.register("Bug.report", h.bugReport)
	h.register("Bug.filter", h.filterRows("bugs", []string{"t.id", "COALESCE(t.title,'') AS summary", "COALESCE(t.url,'') AS description", "t.status", "COALESCE(t.external_key,'') AS severity"}, map[string]relationJoin{
		"execution": {joinSQL: "JOIN test_executions te ON te.id = t.test_execution_id", alias: "te"},
		"tracker":   {joinSQL: "JOIN issue_trackers it ON it.id = t.tracker_id", alias: "it"},
	}))

	// Dictionaries create
	h.register("TestCaseStatus.create", h.simpleCreate("test_case_statuses", []string{"name", "description"}))
	h.register("TestExecutionStatus.create", h.simpleCreate("execution_statuses", []string{"name", "weight", "color"}))
	h.register("Classification.create", h.simpleCreate("classifications", []string{"name"}))
	h.register("PlanType.create", h.simpleCreate("test_plan_types", []string{"name"}))

	// Search
	h.register("Search.query", h.xmlSearch)

	// Admin user ops
	h.register("User.update", h.userUpdate)
	h.register("User.deactivate", h.userDeactivate)
}

func (h *Handler) addAttachment(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		// args: entity_id, filename, b64content
		entityID := argInt(args[0])
		filename := argStr(args[1])
		b64 := argStr(args[2])
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 content")
		}
		if h.uploadDir != "" {
			_ = os.MkdirAll(h.uploadDir, 0o755)
			p := filepath.Join(h.uploadDir, fmt.Sprintf("%d_%s", entityID, filepath.Base(filename)))
			if err := os.WriteFile(p, data, 0o644); err == nil {
				defer func() { _ = os.Remove(p) }()
			}
		}
		res, err := h.db.ExecContext(r.Context(),
			`INSERT INTO attachments (entity_type, entity_id, filename, content_type, size_bytes, storage_path) VALUES (?, ?, ?, ?, ?, ?)`,
			entityType, entityID, filename, "application/octet-stream", len(data), filename)
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		return map[string]any{"id": id, "filename": filename}, nil
	}
}

func (h *Handler) listAttachments(entityType string) func(r *http.Request, args []any) (any, error) {
	return func(r *http.Request, args []any) (any, error) {
		if h.db == nil {
			return nil, errNoDB
		}
		rows, err := h.db.QueryContext(r.Context(), `SELECT id, filename, size_bytes FROM attachments WHERE entity_type = ? AND entity_id = ? ORDER BY id`, entityType, argInt(args[0]))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var id, size int64
			var name string
			if err := rows.Scan(&id, &name, &size); err == nil {
				out = append(out, map[string]any{"id": id, "filename": name, "size": size})
			}
		}
		return out, nil
	}
}

func (h *Handler) removeAttachment(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM attachments WHERE id = ?`, argInt(args[0]))
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) bugReport(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	// args: execution_id, tracker_id, summary, description (simplified: store locally)
	m := argMap(args, "values")
	execID := argInt(args[0])
	if execID == 0 {
		execID = argInt(m["execution_id"])
	}
	trackerID := argInt(m["tracker_id"])
	summary := argStr(m["summary"])
	description := argStr(m["description"])
	res, err := h.db.ExecContext(r.Context(),
		`INSERT INTO bugs (test_execution_id, tracker_id, external_key, url, title, status) VALUES (?, ?, ?, ?, ?, 'open')`,
		execID, trackerID, argStr(m["external_key"]), argStr(m["url"]), summary)
	if err != nil {
		// bugs table may have different columns — fallback generic
		return nil, err
	}
	id, _ := res.LastInsertId()
	url := argStr(m["url"])
	if url == "" {
		url = fmt.Sprintf("/bugs/%d", id)
	}

	// Emit bug.created webhook event (EXTERNAL_ISSUE_POST_PROCESSORS equivalent)
	if h.webhooks != nil {
		data := map[string]any{
			"bug_id":        id,
			"execution_id":  execID,
			"tracker_id":    trackerID,
			"url":           url,
			"summary":       summary,
			"description":   description,
			"external_key":  argStr(m["external_key"]),
		}
		// Look up tracker name and reporter if available
		if trackerID > 0 {
			var tname string
			if err := h.db.QueryRowContext(r.Context(), `SELECT name FROM issue_trackers WHERE id = ?`, trackerID).Scan(&tname); err == nil {
				data["tracker"] = tname
			}
		}
		if u, ok := auth.UserFromContext(r.Context()); ok {
			data["reporter_id"] = u.ID
			data["reporter"] = u.Username
		}
		h.webhooks.EmitEventWithData("bug.created", "bug", id, summary, description, data)
	}

	return map[string]any{"id": id, "url": url}, nil
}

func (h *Handler) xmlSearch(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "query")
	q := strings.ToLower(argStr(m["q"]))
	if q == "" && len(args) > 0 {
		q = strings.ToLower(argStr(args[0]))
	}
	if len(q) < 2 {
		return []map[string]any{}, nil
	}
	terms := strings.Fields(q)
	var out []map[string]any
	for _, t := range terms {
		rows, err := h.db.QueryContext(r.Context(), `SELECT 'test_case', id, summary FROM test_cases WHERE LOWER(summary) LIKE ? LIMIT 10`, "%"+t+"%")
		if err != nil {
			continue
		}
		for rows.Next() {
			var typ string
			var id int64
			var title string
			if err := rows.Scan(&typ, &id, &title); err == nil {
				out = append(out, map[string]any{"type": typ, "id": id, "title": title})
			}
		}
		rows.Close()
	}
	return out, nil
}

func (h *Handler) userUpdate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	m := argMap(args, "values")
	username := argStr(args[0])
	role := argStr(m["role"])
	isActive := m["is_active"]
	active := 1
	if b, ok := isActive.(bool); ok && !b {
		active = 0
	}
	_, err := h.db.ExecContext(r.Context(), `UPDATE users SET role = COALESCE(?, role), is_active = ? WHERE username = ? OR email = ?`,
		nullIfEmpty(role), active, username, username)
	return map[string]any{"ok": err == nil}, err
}

func (h *Handler) userDeactivate(r *http.Request, args []any) (any, error) {
	if h.db == nil {
		return nil, errNoDB
	}
	_, err := h.db.ExecContext(r.Context(), `UPDATE users SET is_active = 0 WHERE username = ? OR email = ?`, argStr(args[0]), argStr(args[0]))
	return map[string]any{"ok": err == nil}, err
}
