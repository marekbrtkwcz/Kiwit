package xmlrpc

// Field lookup parser — translates Django-style query dicts into SQL WHERE
// clauses. Supports: exact, __startswith, __endswith, __contains, __icontains,
// __gt, __gte, __lt, __lte, __in, __isnull, __range, and relation traversal
// via double-underscore (e.g. "product__name__startswith").

import (
	"fmt"
	"regexp"
	"strings"
)

// identRe matches a safe SQL identifier segment (letters, digits, underscore,
// not starting with a digit). Every field name that reaches SQL must pass it.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
// allowed is the whitelist of lookup keys (without operator, e.g. "id" or
// "product__name"); when non-nil, any key not in it is rejected with an
// error. Field names are additionally validated against identRe and quoted,
// so a malicious key can never inject SQL.
func buildWhere(query map[string]any, tableAlias string, relations map[string]relationJoin, allowed map[string]bool) (lookupResult, error) {
	if len(query) == 0 {
		return lookupResult{where: "1=1"}, nil
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

		// Whitelist check: reject any lookup key the caller did not declare.
		if allowed != nil && !allowed[col] {
			return lookupResult{}, fmt.Errorf("lookup field not allowed: %q", col)
		}

		// Handle relation traversal: product__name__startswith → join products
		field := col
		alias := tableAlias
		if idx := strings.Index(col, "__"); idx > 0 {
			relName := col[:idx]
			if !identRe.MatchString(relName) {
				return lookupResult{}, fmt.Errorf("invalid relation name: %q", relName)
			}
			if rel, ok := relations[relName]; ok {
				if !joins[relName] {
					// JOIN will be prepended by caller via joinsNeeded
					joins[relName] = true
				}
				alias = rel.alias
				field = col[idx+2:]
				// nested relation beyond depth 1 is not supported — reject
				if strings.Contains(field, "__") {
					return lookupResult{}, fmt.Errorf("nested relation not supported: %q", col)
				}
			} else {
				// Unknown relation prefix: the whole key is a field name on the
				// main table and must be a single plain identifier.
				return lookupResult{}, fmt.Errorf("unknown relation in lookup: %q", col)
			}
		}
		if !identRe.MatchString(field) {
			return lookupResult{}, fmt.Errorf("invalid field name: %q", field)
		}

		colRef := alias + "." + quoteIdent(field)
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
	}, nil
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

var (
	colRefRe  = regexp.MustCompile(`\bt\.([A-Za-z_][A-Za-z0-9_]*)\b`)
	bareColRe = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)$`)
)

// safeRelatedFields are the only fields filterable through a relation
// (e.g. product__name). Sensitive columns (password_hash, tokens, secrets)
// are deliberately not in this set.
var safeRelatedFields = map[string]bool{
	"id": true, "name": true, "value": true, "summary": true,
	"username": true, "email": true, "title": true, "description": true,
	"status": true, "key": true,
}

// deriveAllowed builds the lookup-key whitelist for a filter: main-table
// columns taken from the SELECT list (so only already-exposed columns are
// filterable), plus relation lookups limited to safeRelatedFields.
func deriveAllowed(cols []string, relations map[string]relationJoin) map[string]bool {
	allowed := map[string]bool{}
	for _, c := range cols {
		if m := bareColRe.FindStringSubmatch(c); m != nil {
			allowed[m[1]] = true
			continue
		}
		for _, m := range colRefRe.FindAllStringSubmatch(c, -1) {
			allowed[m[1]] = true
		}
	}
	for rel := range relations {
		for f := range safeRelatedFields {
			allowed[rel+"__"+f] = true
		}
	}
	return allowed
}

var _ = fmt.Sprintf // keep fmt import if needed later
