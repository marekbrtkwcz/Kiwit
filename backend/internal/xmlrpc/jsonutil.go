package xmlrpc

// JSON helper functions for Environment properties stored as JSONB.

import (
	"encoding/json"
	"strings"
)

// jsonObjectToList converts a flat JSON object {"k":"v"} into a list of
// {"name": k, "value": v} dicts, matching the kiwitcms Environment.properties shape.
func jsonObjectToList(raw string) []map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return []map[string]any{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(m))
	for k, v := range m {
		out = append(out, map[string]any{"name": k, "value": v})
	}
	return out
}

// mergeJSONProp merges a key/value into a JSON object string.
func mergeJSONProp(raw, key, value string) string {
	raw = strings.TrimSpace(raw)
	m := map[string]string{}
	if raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	m[key] = value
	b, _ := json.Marshal(m)
	return string(b)
}

// removeJSONProp deletes a key from a JSON object string.
func removeJSONProp(raw, key string) string {
	raw = strings.TrimSpace(raw)
	m := map[string]string{}
	if raw != "" && raw != "{}" {
		_ = json.Unmarshal([]byte(raw), &m)
	}
	delete(m, key)
	b, _ := json.Marshal(m)
	return string(b)
}
