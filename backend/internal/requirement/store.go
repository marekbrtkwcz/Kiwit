package requirement

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Requirement struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version"`
	Status      string    `json:"status" enum:"draft,approved,obsolete"`
	Priority    string    `json:"priority"`
	Category    string    `json:"category,omitempty"`
	AuthorID    *int64    `json:"author_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CoverageEntry struct {
	TestCaseID        int64      `json:"test_case_id"`
	Summary           string     `json:"summary"`
	Status            string     `json:"status"`
	LatestExecutionID *int64     `json:"latest_execution_id,omitempty"`
	LatestExecStatus  string     `json:"latest_execution_status,omitempty"`
	LatestRunID       *int64     `json:"latest_run_id,omitempty"`
	LinkedAt          time.Time  `json:"linked_at"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const requirementCols = `id, product_id, key, name, description, version, status, priority, category, author_id, created_at, updated_at`

func scanRequirement(row interface{ Scan(...any) error }) (Requirement, error) {
	var r Requirement
	err := row.Scan(&r.ID, &r.ProductID, &r.Key, &r.Name, &r.Description, &r.Version, &r.Status, &r.Priority, &r.Category, &r.AuthorID, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

type filter struct {
	q         string
	productID int64
	status    string
	priority  string
	category  string
}

func (s *Store) List(ctx context.Context, f filter, limit, offset int) ([]Requirement, int64, error) {
	var conds []string
	var args []any
	if f.q != "" {
		conds = append(conds, "(key ILIKE ? OR name ILIKE ? OR description ILIKE ?)")
		pat := "%" + f.q + "%"
		args = append(args, pat, pat, pat)
	}
	if f.productID != 0 {
		conds = append(conds, "product_id = ?")
		args = append(args, f.productID)
	}
	if f.status != "" {
		conds = append(conds, "status = ?")
		args = append(args, f.status)
	}
	if f.priority != "" {
		conds = append(conds, "priority = ?")
		args = append(args, f.priority)
	}
	if f.category != "" {
		conds = append(conds, "category = ?")
		args = append(args, f.category)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	if limit <= 0 {
		limit = 100
	}

	if s.pg != nil {
		q, vals := pgWhere(where, args)
		q = fmt.Sprintf("SELECT %s FROM requirements %s ORDER BY id LIMIT $%d OFFSET $%d", requirementCols, q, len(vals)+1, len(vals)+2)
		vals = append(vals, limit, offset)
		rows, err := s.pg.Query(ctx, q, vals...)
		if err != nil {
			return nil, 0, err
		}
		var out []Requirement
		for rows.Next() {
			r, err := scanRequirement(rows)
			if err != nil {
				rows.Close()
				return nil, 0, err
			}
			out = append(out, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, 0, err
		}
		cq, cvals := pgWhere(where, args)
		cq = "SELECT COUNT(*) FROM requirements " + cq
		var total int64
		if err := s.pg.QueryRow(ctx, cq, cvals...).Scan(&total); err != nil {
			return nil, 0, err
		}
		return out, total, nil
	}

	q := fmt.Sprintf("SELECT %s FROM requirements %s ORDER BY id LIMIT ? OFFSET ?", requirementCols, where)
	rows, err := s.db.QueryContext(ctx, q, append(append([]any{}, args...), limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	var out []Requirement
	for rows.Next() {
		r, err := scanRequirement(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, r)
	}
	rows.Close()
	cq := fmt.Sprintf("SELECT COUNT(*) FROM requirements %s", where)
	var total int64
	if err := s.db.QueryRowContext(ctx, cq, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// pgWhere converts ? placeholders to $n and returns the WHERE clause with values.
func pgWhere(where string, args []any) (string, []any) {
	if where == "" {
		return "", args
	}
	parts := strings.Split(where, "?")
	var b strings.Builder
	b.WriteString("WHERE ")
	for i, p := range parts {
		b.WriteString(p)
		if i < len(parts)-1 {
			b.WriteString(fmt.Sprintf("$%d", i+1))
		}
	}
	return b.String(), args
}

func (s *Store) Get(ctx context.Context, id int64) (Requirement, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM requirements WHERE id = $1", requirementCols), id)
		return scanRequirement(row)
	}
	row := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM requirements WHERE id = ?", requirementCols), id)
	return scanRequirement(row)
}

type RequirementInput struct {
	ProductID   int64  `json:"product_id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version,omitempty"`
	Status      string `json:"status,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Category    string `json:"category,omitempty"`
	AuthorID    *int64 `json:"author_id,omitempty"`
}

func (s *Store) Create(ctx context.Context, in RequirementInput) (Requirement, error) {
	if in.Version == "" {
		in.Version = "1"
	}
	if in.Status == "" {
		in.Status = "draft"
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO requirements (product_id, key, name, description, version, status, priority, category, author_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+requirementCols,
			in.ProductID, in.Key, in.Name, in.Description, in.Version, in.Status, in.Priority, in.Category, in.AuthorID)
		return scanRequirement(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO requirements (product_id, key, name, description, version, status, priority, category, author_id)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		in.ProductID, in.Key, in.Name, in.Description, in.Version, in.Status, in.Priority, in.Category, in.AuthorID)
	if err != nil {
		return Requirement{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, id)
}

func (s *Store) Update(ctx context.Context, id int64, in RequirementInput) (Requirement, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE requirements SET product_id=$1, key=$2, name=$3, description=$4, version=$5, status=$6, priority=$7, category=$8, updated_at=NOW()
			WHERE id=$9 RETURNING `+requirementCols,
			in.ProductID, in.Key, in.Name, in.Description, in.Version, in.Status, in.Priority, in.Category, id)
		return scanRequirement(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE requirements SET product_id=?, key=?, name=?, description=?, version=?, status=?, priority=?, category=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`,
		in.ProductID, in.Key, in.Name, in.Description, in.Version, in.Status, in.Priority, in.Category, id)
	if err != nil {
		return Requirement{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM requirements WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM requirements WHERE id = ?`, id)
	return err
}

func (s *Store) LinkCase(ctx context.Context, requirementID, testCaseID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `INSERT INTO requirement_coverage (requirement_id, test_case_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, requirementID, testCaseID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO requirement_coverage (requirement_id, test_case_id) VALUES (?, ?)`, requirementID, testCaseID)
	return err
}

func (s *Store) UnlinkCase(ctx context.Context, requirementID, testCaseID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM requirement_coverage WHERE requirement_id = $1 AND test_case_id = $2`, requirementID, testCaseID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM requirement_coverage WHERE requirement_id = ? AND test_case_id = ?`, requirementID, testCaseID)
	return err
}

func (s *Store) Coverage(ctx context.Context, requirementID int64) ([]CoverageEntry, error) {
	q := `SELECT rc.test_case_id, tc.summary, tc.status, rc.created_at,
		(SELECT te.id FROM test_executions te WHERE te.test_case_id = tc.id ORDER BY te.id DESC LIMIT 1) AS latest_exec_id,
		(SELECT te.status FROM test_executions te WHERE te.test_case_id = tc.id ORDER BY te.id DESC LIMIT 1) AS latest_exec_status,
		(SELECT te.test_run_id FROM test_executions te WHERE te.test_case_id = tc.id ORDER BY te.id DESC LIMIT 1) AS latest_run_id
		FROM requirement_coverage rc JOIN test_cases tc ON tc.id = rc.test_case_id
		WHERE rc.requirement_id = ? ORDER BY tc.id`
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, requirementID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []CoverageEntry
		for rows.Next() {
			var c CoverageEntry
			if err := rows.Scan(&c.TestCaseID, &c.Summary, &c.Status, &c.LinkedAt, &c.LatestExecutionID, &c.LatestExecStatus, &c.LatestRunID); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, requirementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoverageEntry
	for rows.Next() {
		var c CoverageEntry
		if err := rows.Scan(&c.TestCaseID, &c.Summary, &c.Status, &c.LinkedAt, &c.LatestExecutionID, &c.LatestExecStatus, &c.LatestRunID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) RequirementsForCase(ctx context.Context, testCaseID int64) ([]Requirement, error) {
	q := fmt.Sprintf(`SELECT %s FROM requirements r JOIN requirement_coverage rc ON rc.requirement_id = r.id WHERE rc.test_case_id = ? ORDER BY r.id`, requirementCols)
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, testCaseID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []Requirement
		for rows.Next() {
			r, err := scanRequirement(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, testCaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Requirement
	for rows.Next() {
		r, err := scanRequirement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
