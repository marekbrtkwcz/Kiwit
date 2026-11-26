package dataset

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Dataset struct {
	ID         int64     `json:"id"`
	TestCaseID int64     `json:"test_case_id"`
	Name       string    `json:"name"`
	Params     string    `json:"params"` // JSON object
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const cols = `id, test_case_id, name, params, created_at`

func scanRow(row interface{ Scan(...any) error }) (Dataset, error) {
	var d Dataset
	err := row.Scan(&d.ID, &d.TestCaseID, &d.Name, &d.Params, &d.CreatedAt)
	return d, err
}

func validParams(p string) bool {
	if p == "" {
		return true
	}
	var v any
	return json.Unmarshal([]byte(p), &v) == nil
}

func (s *Store) ListForCase(ctx context.Context, caseID int64) ([]Dataset, error) {
	q := fmt.Sprintf(`SELECT %s FROM test_case_datasets WHERE test_case_id = ? ORDER BY id`, cols)
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, caseID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []Dataset
		for rows.Next() {
			d, err := scanRow(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, d)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Dataset
	for rows.Next() {
		d, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int64) (Dataset, error) {
	if s.pg != nil {
		return scanRow(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM test_case_datasets WHERE id = $1", cols), id))
	}
	return scanRow(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM test_case_datasets WHERE id = ?", cols), id))
}

func (s *Store) Create(ctx context.Context, caseID int64, name, params string) (Dataset, error) {
	if !validParams(params) {
		return Dataset{}, fmt.Errorf("params must be valid JSON")
	}
	if params == "" {
		params = "{}"
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO test_case_datasets (test_case_id, name, params) VALUES ($1,$2,$3) RETURNING `+cols,
			caseID, name, params)
		return scanRow(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO test_case_datasets (test_case_id, name, params) VALUES (?,?,?)`, caseID, name, params)
	if err != nil {
		return Dataset{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, id)
}

func (s *Store) Update(ctx context.Context, id int64, name, params string) (Dataset, error) {
	if !validParams(params) {
		return Dataset{}, fmt.Errorf("params must be valid JSON")
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE test_case_datasets SET name=$1, params=$2 WHERE id=$3 RETURNING `+cols, name, params, id)
		return scanRow(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE test_case_datasets SET name=?, params=? WHERE id=?`, name, params, id)
	if err != nil {
		return Dataset{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM test_case_datasets WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM test_case_datasets WHERE id = ?`, id)
	return err
}

// ExpandDatasets creates one execution per dataset of the given test case in the
// run (skipping datasets that already have an execution). Returns created count.
func (s *Store) ExpandDatasets(ctx context.Context, runID, caseID int64) (int, error) {
	q := `INSERT INTO test_executions (test_run_id, test_case_id, dataset_id, status)
		SELECT ?, ?, d.id, 'idle' FROM test_case_datasets d
		WHERE d.test_case_id = ?
		AND NOT EXISTS (
			SELECT 1 FROM test_executions te
			WHERE te.test_run_id = ? AND te.test_case_id = ? AND te.dataset_id = d.id
		)`
	if s.pg != nil {
		q = strings.NewReplacer("?", "$1").Replace(q)
		// rebuild placeholders properly
		q = `INSERT INTO test_executions (test_run_id, test_case_id, dataset_id, status)
			SELECT $1, $2, d.id, 'idle' FROM test_case_datasets d
			WHERE d.test_case_id = $2
			AND NOT EXISTS (
				SELECT 1 FROM test_executions te
				WHERE te.test_run_id = $1 AND te.test_case_id = $2 AND te.dataset_id = d.id
			)`
		res, err := s.pg.Exec(ctx, q, runID, caseID)
		if err != nil {
			return 0, err
		}
		return int(res.RowsAffected()), nil
	}
	res, err := s.db.ExecContext(ctx, q, runID, caseID, caseID, runID, caseID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
