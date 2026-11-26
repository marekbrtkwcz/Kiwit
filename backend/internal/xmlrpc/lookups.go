package xmlrpc

// Field lookup parser — translates Django-style query dicts into SQL WHERE
// clauses. Supports: exact, __startswith, __endswith, __contains, __icontains,
// __gt, __gte, __lt, __lte, __in, __isnull, __range, and relation traversal
// via double-underscore (e.g. "product__name__startswith").

import (
	"fmt"
	"strings"
)

// lookupResult holds a built WHERE fragment and its bound parameters.
type lookupResult struct {
	where string
	args  []any
}

// relationMap maps a relation prefix (e.g. "product") to a JOIN clause and
// the alias to use for the related table. Populated per-table by the caller.
type relationJoin struct {
	joinSQL string // e.g. "JOIN products p ON p.id = t.product_id"
	alias   string // e.g. "p"
}

// buildWhere converts a query map into a SQL WHERE clause.
// tableAlias is the alias for the main table (e.g. "t").
// relations maps relation field names to their JOIN definitions.
func buildWhere(query map[string]any, tableAlias string, relations map[string]relationJoin) lookupResult {
	if len(query) == 0 {
		return lookupResult{where: "1=1"}
	}
	clauses := make([]string, 0, len(query))
	args := make([]any, 0, len(query)*2)
	joins := make(map[string]bool) // track which joins we've already added

	// Sort keys for deterministic output
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	// simple insertion sort for determinism
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}

	for _, key := range keys {
		val := query[key]
		col, op := parseLookupKey(key)

		// Handle relation traversal: product__name__startswith → join products
		field := col
		alias := tableAlias
		if idx := strings.Index(col, "__"); idx > 0 {
			relName := col[:idx]
			if rel, ok := relations[relName]; ok {
				if !joins[relName] {
					// JOIN will be prepended by caller via joinsNeeded
					joins[relName] = true
				}
				alias = rel.alias
				field = col[idx+2:]
				// If field itself contains __ it's another lookup on the related table
				if idx2 := strings.Index(field, "__"); idx2 > 0 {
					// nested relation not supported beyond depth 1 for now
					field = strings.ReplaceAll(field, "__", "_")
				}
			}
		}

		colRef := alias + "." + field
		switch op {
		case "exact":
			clauses = append(clauses, colRef+" = ?")
			args = append(args, val)
		case "startswith":
			clauses = append(clauses, colRef+" LIKE ?")
			args = append(args, argStr(val)+"%")
		case "endswith":
			clauses = append(clauses, colRef+" LIKE ?")
			args = append(args, "%"+argStr(val))
		case "contains":
			clauses = append(clauses, colRef+" LIKE ?")
			args = append(args, "%"+argStr(val)+"%")
		case "icontains":
			clauses = append(clauses, "LOWER("+colRef+") LIKE LOWER(?)")
			args = append(args, "%"+argStr(val)+"%")
		case "gt":
			clauses = append(clauses, colRef+" > ?")
			args = append(args, val)
		case "gte":
			clauses = append(clauses, colRef+" >= ?")
			args = append(args, val)
		case "lt":
			clauses = append(clauses, colRef+" < ?")
			args = append(args, val)
		case "lte":
			clauses = append(clauses, colRef+" <= ?")
			args = append(args, val)
		case "in":
			arr, ok := val.([]any)
			if !ok {
				// single value wrapped
				clauses = append(clauses, colRef+" = ?")
				args = append(args, val)
				continue
			}
			if len(arr) == 0 {
				clauses = append(clauses, "1=0") // empty IN → false
				continue
			}
			placeholders := make([]string, len(arr))
			for i, v := range arr {
				placeholders[i] = "?"
				args = append(args, v)
			}
			clauses = append(clauses, colRef+" IN ("+strings.Join(placeholders, ",")+")")
		case "isnull":
			b, _ := val.(bool)
			if b {
				clauses = append(clauses, colRef+" IS NULL")
			} else {
				clauses = append(clauses, colRef+" IS NOT NULL")
			}
		case "range":
			arr, ok := val.([]any)
			if !ok || len(arr) != 2 {
				continue
			}
			clauses = append(clauses, colRef+" BETWEEN ? AND ?")
			args = append(args, arr[0], arr[1])
		default:
			// unknown lookup — treat as exact
			clauses = append(clauses, colRef+" = ?")
			args = append(args, val)
		}
	}

	return lookupResult{
		where: strings.Join(clauses, " AND "),
		args:  args,
	}
}

// parseLookupKey splits "summary__startswith" into ("summary", "startswith")
// and "id" into ("id", "exact").
func parseLookupKey(key string) (field, op string) {
	// Check if the last segment after __ is a known lookup operator
	knownOps := map[string]bool{
		"exact": true, "iexact": true, "startswith": true, "istartswith": true,
		"endswith": true, "iendswith": true, "contains": true, "icontains": true,
		"gt": true, "gte": true, "lt": true, "lte": true,
		"in": true, "isnull": true, "range": true,
	}
	idx := strings.LastIndex(key, "__")
	if idx > 0 {
		last := key[idx+2:]
		if knownOps[last] {
			return key[:idx], last
		}
	}
	return key, "exact"
}

// joinsForQuery returns the JOIN clauses needed for the given query's relation fields.
func joinsForQuery(query map[string]any, relations map[string]relationJoin) []string {
	seen := map[string]bool{}
	var joins []string
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	for _, key := range keys {
		idx := strings.Index(key, "__")
		if idx > 0 {
			relName := key[:idx]
			if rel, ok := relations[relName]; ok && !seen[relName] {
				seen[relName] = true
				joins = append(joins, rel.joinSQL)
			}
		}
	}
	return joins
}

// quoteIdent safely quotes a column reference for SQL.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

var _ = fmt.Sprintf // keep fmt import if needed later
