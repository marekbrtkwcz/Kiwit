package xmlrpc

// Exported, whitelisted access to the field-lookup filter machinery for the
// REST/AI layer (natural-language search translated into Django-style
// lookups). Only the models listed below are filterable; field names are
// sanitized before they reach SQL.

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FilterableModel mirrors one .filter registration above.
type FilterableModel struct {
	Table     string
	Cols      []string
	Relations map[string]relationJoin
}

// FilterModels is the whitelist of models usable by FilterModel.
// Keys are stable snake_case identifiers exposed to the LLM.
var FilterModels = map[string]FilterableModel{
	"test_case": {
		Table: "test_cases",
		Cols:  []string{"t.id", "t.summary", "t.category_id", "t.is_automated", "t.priority", "t.status", "t.product_id"},
		Relations: map[string]relationJoin{
			"category": {joinSQL: "JOIN categories c ON c.id = t.category_id", alias: "c"},
			"product":  {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
			"author":   {joinSQL: "JOIN users u ON u.id = t.author_id", alias: "u"},
		},
	},
	"test_run": {
		Table: "test_runs",
		Cols:  []string{"t.id", "t.summary", "t.test_plan_id AS plan_id", "t.build_id", "t.stopped_at IS NOT NULL AS stopped"},
		Relations: map[string]relationJoin{
			"plan":    {joinSQL: "JOIN test_plans tp ON tp.id = t.test_plan_id", alias: "tp"},
			"build":   {joinSQL: "JOIN builds b ON b.id = t.build_id", alias: "b"},
			"manager": {joinSQL: "JOIN users u ON u.id = t.manager_id", alias: "u"},
		},
	},
	"test_plan": {
		Table: "test_plans",
		Cols:  []string{"t.id", "t.name", "t.product_id", "t.version_id", "t.is_active", "COALESCE(t.parent_id,0) AS parent_id"},
		Relations: map[string]relationJoin{
			"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
			"version": {joinSQL: "JOIN versions v ON v.id = t.version_id", alias: "v"},
			"author":  {joinSQL: "JOIN users u ON u.id = t.author_id", alias: "u"},
			"parent":  {joinSQL: "JOIN test_plans pt ON pt.id = t.parent_id", alias: "pt"},
		},
	},
	"product": {
		Table: "products",
		Cols:  []string{"t.id", "t.name", "COALESCE(t.description,'') AS description", "COALESCE(t.classification_id,0) AS classification_id"},
		Relations: map[string]relationJoin{
			"classification": {joinSQL: "JOIN classifications c ON c.id = t.classification_id", alias: "c"},
		},
	},
	"build": {
		Table: "builds",
		Cols:  []string{"t.id", "t.name", "t.version_id"},
		Relations: map[string]relationJoin{
			"version": {joinSQL: "JOIN versions v ON v.id = t.version_id", alias: "v"},
		},
	},
	"version": {
		Table: "versions",
		Cols:  []string{"t.id", "t.value", "t.product_id"},
		Relations: map[string]relationJoin{
			"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
		},
	},
	"component": {
		Table: "components",
		Cols:  []string{"t.id", "t.name", "t.product_id", "COALESCE(t.description,'') AS description"},
		Relations: map[string]relationJoin{
			"product": {joinSQL: "JOIN products p ON p.id = t.product_id", alias: "p"},
		},
	},
	"tag":         {Table: "tags", Cols: []string{"t.id", "t.name"}},
	"environment": {Table: "environments", Cols: []string{"t.id", "t.name", "COALESCE(t.description,'') AS description"}},
	"user":        {Table: "users", Cols: []string{"t.id", "t.username", "t.email", "COALESCE(t.role,'') AS role", "t.is_active"}},
}

var filterKeyRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// sanitizeQuery drops keys that are not plain identifiers (SQL-injection
// guard: field names are interpolated into SQL by buildWhere) and keeps only
// scalar/array-of-scalar values.
func sanitizeQuery(query map[string]any) map[string]any {
	clean := map[string]any{}
	for k, v := range query {
		if !filterKeyRe.MatchString(k) {
			continue
		}
		switch val := v.(type) {
		case string, bool, float64, int, int64, nil:
			clean[k] = v
		case []any:
			arr := make([]any, 0, len(val))
			for _, item := range val {
				switch item.(type) {
				case string, bool, float64, int, int64:
					arr = append(arr, item)
				}
			}
			clean[k] = arr
		}
	}
	return clean
}

// rebindPlaceholders converts SQLite-style "?" placeholders to Postgres "$n".
func rebindPlaceholders(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FilterModel executes a Django-style lookup query against a whitelisted
// model and returns rows as maps. Works on SQLite (db) and Postgres (pg).
func FilterModel(ctx context.Context, db *sql.DB, pg *pgxpool.Pool, model string, query map[string]any, limit int) ([]map[string]any, error) {
	spec, ok := FilterModels[model]
	if !ok {
		return nil, fmt.Errorf("unknown filter model %q", model)
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	clean := sanitizeQuery(query)
	relMap := spec.Relations
	if relMap == nil {
		relMap = map[string]relationJoin{}
	}
	lw, err := buildWhere(clean, "t", relMap, deriveAllowed(spec.Cols, relMap))
	if err != nil {
		return nil, err
	}
	joins := joinsForQuery(clean, relMap)
	joinClause := ""
	if len(joins) > 0 {
		joinClause = strings.Join(joins, " ") + " "
	}
	q := fmt.Sprintf("SELECT %s FROM %s t %sWHERE %s ORDER BY t.id DESC LIMIT %d",
		strings.Join(spec.Cols, ", "), spec.Table, joinClause, lw.where, limit)
	if pg != nil {
		rows, err := pg.Query(ctx, rebindPlaceholders(q), lw.args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				continue
			}
			cols := make([]string, len(rows.FieldDescriptions()))
			for i, fd := range rows.FieldDescriptions() {
				cols[i] = string(fd.Name)
			}
			m := map[string]any{}
			for i, c := range cols {
				if b, ok := vals[i].([]byte); ok {
					m[c] = string(b)
				} else {
					m[c] = vals[i]
				}
			}
			out = append(out, m)
		}
		return out, rows.Err()
	}
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := db.QueryContext(ctx, q, lw.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFilterRows(rows)
}

func scanFilterRows(rows interface {
	Next() bool
	Columns() ([]string, error)
	Scan(...any) error
	Err() error
}) ([]map[string]any, error) {
	out := []map[string]any{}
	colNames, _ := rows.Columns()
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
	return out, rows.Err()
}
