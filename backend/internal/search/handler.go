package search

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Result is one search hit across entities.
type Result struct {
	Type    string `json:"type"` // test_case | test_plan | test_run
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// RegisterRoutes mounts GET /api/v1/search?q=... — full-text-ish search across cases, plans, runs.
func RegisterRoutes(api huma.API, s *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "global-search",
		Method:      http.MethodGet,
		Path:        "/api/v1/search",
		Summary:     "Search test cases, plans and runs by keywords (all terms must match)",
		Tags:        []string{"Search"},
	}, func(ctx context.Context, input *struct {
		Q     string `query:"q" minLength:"2" maxLength:"200"`
		Limit int    `query:"limit" minimum:"1" maximum:"100" default:"20"`
	}) (*struct {
		Body struct {
			Items []Result `json:"items"`
		}
	}, error) {
		terms := strings.Fields(strings.ToLower(input.Q))
		if len(terms) == 0 {
			return &struct {
				Body struct {
					Items []Result `json:"items"`
				}
			}{Body: struct {
				Items []Result `json:"items"`
			}{Items: []Result{}}}, nil
		}
		items, err := s.Search(ctx, terms, input.Limit)
		if err != nil {
			return nil, huma.Error500InternalServerError("search failed", err)
		}
		out := &struct {
			Body struct {
				Items []Result `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})
}

func (s *Store) Search(ctx context.Context, terms []string, limit int) ([]Result, error) {
	var out []Result
	var err error
	if s.pg != nil {
		out, err = s.searchPG(ctx, terms, limit)
	} else {
		out, err = s.searchSQLite(ctx, terms, limit)
	}
	return out, err
}

// likeClause builds AND-of-LOWER(col) LIKE ? for each term.
func likeClauses(cols []string, terms []string, ph func(i int) string) (string, []any) {
	var parts []string
	var args []any
	idx := 0
	for _, t := range terms {
		var ors []string
		for _, c := range cols {
			ors = append(ors, "LOWER("+c+") LIKE "+ph(idx))
			args = append(args, "%"+t+"%")
			idx++
		}
		parts = append(parts, "("+strings.Join(ors, " OR ")+")")
	}
	return strings.Join(parts, " AND "), args
}

func (s *Store) searchSQLite(ctx context.Context, terms []string, limit int) ([]Result, error) {
	out := []Result{}
	// Test cases (summary + preconditions + steps)
	q, args := likeClauses([]string{"tc.summary", "tc.preconditions", "(SELECT GROUP_CONCAT(action || ' ' || expected_result) FROM test_case_steps st WHERE st.test_case_id = tc.id)"}, terms, func(i int) string { return "?" })
	rows, err := s.db.QueryContext(ctx, `SELECT 'test_case', tc.id, tc.summary FROM test_cases tc WHERE tc.deleted_at IS NULL AND `+q+` ORDER BY tc.id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Type, &r.ID, &r.Title); err == nil {
			r.Snippet = r.Title
			out = append(out, r)
		}
	}
	rows.Close()
	// Plans
	q, args = likeClauses([]string{"name", "description"}, terms, func(i int) string { return "?" })
	rows, err = s.db.QueryContext(ctx, `SELECT 'test_plan', id, name FROM test_plans WHERE `+q+` ORDER BY id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Type, &r.ID, &r.Title); err == nil {
			r.Snippet = r.Title
			out = append(out, r)
		}
	}
	rows.Close()
	// Runs
	q, args = likeClauses([]string{"summary"}, terms, func(i int) string { return "?" })
	rows, err = s.db.QueryContext(ctx, `SELECT 'test_run', id, summary FROM test_runs WHERE `+q+` ORDER BY id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Type, &r.ID, &r.Title); err == nil {
			r.Snippet = r.Title
			out = append(out, r)
		}
	}
	rows.Close()
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) searchPG(ctx context.Context, terms []string, limit int) ([]Result, error) {
	out := []Result{}
	// PostgreSQL: use to_tsvector for real full-text search.
	vector := "to_tsvector('simple', coalesce(tc.summary,'') || ' ' || coalesce(tc.preconditions,'') || ' ' || coalesce((SELECT string_agg(st.action || ' ' || st.expected_result, ' ') FROM test_case_steps st WHERE st.test_case_id = tc.id), ''))"
	plainq := strings.Join(terms, " & ")
	rows, err := s.pg.Query(ctx, `SELECT 'test_case', tc.id, tc.summary FROM test_cases tc WHERE tc.deleted_at IS NULL AND `+vector+` @@ plainto_tsquery('simple', $1) ORDER BY tc.id DESC LIMIT $2`, plainq, limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Type, &r.ID, &r.Title); err == nil {
			r.Snippet = r.Title
			out = append(out, r)
		}
	}
	rows.Close()

	rows, err = s.pg.Query(ctx, `SELECT 'test_plan', id, name FROM test_plans WHERE to_tsvector('simple', coalesce(name,'') || ' ' || coalesce(description,'')) @@ plainto_tsquery('simple', $1) ORDER BY id DESC LIMIT $2`, plainq, limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Type, &r.ID, &r.Title); err == nil {
			r.Snippet = r.Title
			out = append(out, r)
		}
	}
	rows.Close()

	rows, err = s.pg.Query(ctx, `SELECT 'test_run', id, summary FROM test_runs WHERE to_tsvector('simple', coalesce(summary,'')) @@ plainto_tsquery('simple', $1) ORDER BY id DESC LIMIT $2`, plainq, limit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.Type, &r.ID, &r.Title); err == nil {
			r.Snippet = r.Title
			out = append(out, r)
		}
	}
	rows.Close()
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
