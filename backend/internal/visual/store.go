package visual

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Baseline struct {
	ID           int64     `json:"id"`
	TestCaseID   int64     `json:"test_case_id"`
	AttachmentID int64     `json:"attachment_id"`
	Label        string    `json:"label,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Comparison struct {
	ID           int64     `json:"id"`
	ExecutionID  int64     `json:"execution_id"`
	BaselineID   int64     `json:"baseline_id"`
	AttachmentID int64     `json:"attachment_id"`
	DiffPercent  float64   `json:"diff_percent"`
	Verdict      string    `json:"verdict" enum:"match,mismatch,no-baseline"`
	CreatedAt    time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const baselineCols = `id, test_case_id, attachment_id, label, created_at`
const comparisonCols = `id, execution_id, baseline_id, attachment_id, diff_percent, verdict, created_at`

func scanBaseline(row interface{ Scan(...any) error }) (Baseline, error) {
	var b Baseline
	err := row.Scan(&b.ID, &b.TestCaseID, &b.AttachmentID, &b.Label, &b.CreatedAt)
	return b, err
}

func scanComparison(row interface{ Scan(...any) error }) (Comparison, error) {
	var c Comparison
	err := row.Scan(&c.ID, &c.ExecutionID, &c.BaselineID, &c.AttachmentID, &c.DiffPercent, &c.Verdict, &c.CreatedAt)
	return c, err
}

func (s *Store) AddBaseline(ctx context.Context, caseID, attachmentID int64, label string) (Baseline, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO visual_baselines (test_case_id, attachment_id, label) VALUES ($1,$2,$3) RETURNING `+baselineCols,
			caseID, attachmentID, label)
		return scanBaseline(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO visual_baselines (test_case_id, attachment_id, label) VALUES (?,?,?)`, caseID, attachmentID, label)
	if err != nil {
		return Baseline{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetBaseline(ctx, id)
}

func (s *Store) GetBaseline(ctx context.Context, id int64) (Baseline, error) {
	if s.pg != nil {
		return scanBaseline(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM visual_baselines WHERE id = $1", baselineCols), id))
	}
	return scanBaseline(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM visual_baselines WHERE id = ?", baselineCols), id))
}

func (s *Store) ListBaselines(ctx context.Context, caseID int64) ([]Baseline, error) {
	q := fmt.Sprintf(`SELECT %s FROM visual_baselines WHERE test_case_id = ? ORDER BY id DESC`, baselineCols)
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, caseID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Baseline{}
		for rows.Next() {
			b, err := scanBaseline(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, b)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Baseline{}
	for rows.Next() {
		b, err := scanBaseline(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) LatestBaseline(ctx context.Context, caseID int64) (Baseline, error) {
	q := fmt.Sprintf(`SELECT %s FROM visual_baselines WHERE test_case_id = ? ORDER BY id DESC LIMIT 1`, baselineCols)
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		return scanBaseline(s.pg.QueryRow(ctx, q, caseID))
	}
	return scanBaseline(s.db.QueryRowContext(ctx, q, caseID))
}

func (s *Store) DeleteBaseline(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM visual_baselines WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM visual_baselines WHERE id = ?`, id)
	return err
}

// BaselineCaseID resolves the test case of a baseline (for access control).
func (s *Store) BaselineCaseID(ctx context.Context, id int64) (int64, error) {
	var cid int64
	if s.pg != nil {
		err := s.pg.QueryRow(ctx, `SELECT test_case_id FROM visual_baselines WHERE id = $1`, id).Scan(&cid)
		return cid, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT test_case_id FROM visual_baselines WHERE id = ?`, id).Scan(&cid)
	return cid, err
}

func (s *Store) SaveComparison(ctx context.Context, executionID, baselineID, attachmentID int64, diffPercent float64, verdict string) (Comparison, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO visual_comparisons (execution_id, baseline_id, attachment_id, diff_percent, verdict) VALUES ($1,$2,$3,$4,$5) RETURNING `+comparisonCols,
			executionID, baselineID, attachmentID, diffPercent, verdict)
		return scanComparison(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO visual_comparisons (execution_id, baseline_id, attachment_id, diff_percent, verdict) VALUES (?,?,?,?,?)`,
		executionID, baselineID, attachmentID, diffPercent, verdict)
	if err != nil {
		return Comparison{}, err
	}
	id, _ := res.LastInsertId()
	if s.pg != nil {
		return scanComparison(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM visual_comparisons WHERE id = $1", comparisonCols), id))
	}
	return scanComparison(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM visual_comparisons WHERE id = ?", comparisonCols), id))
}

func (s *Store) ListComparisons(ctx context.Context, executionID int64) ([]Comparison, error) {
	q := fmt.Sprintf(`SELECT %s FROM visual_comparisons WHERE execution_id = ? ORDER BY id DESC`, comparisonCols)
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, executionID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Comparison{}
		for rows.Next() {
			c, err := scanComparison(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comparison{}
	for rows.Next() {
		c, err := scanComparison(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ExecutionCaseID resolves the test case of an execution (for baseline lookup).
func (s *Store) ExecutionCaseID(ctx context.Context, executionID int64) (int64, error) {
	var cid int64
	if s.pg != nil {
		err := s.pg.QueryRow(ctx, `SELECT test_case_id FROM test_executions WHERE id = $1`, executionID).Scan(&cid)
		return cid, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT test_case_id FROM test_executions WHERE id = ?`, executionID).Scan(&cid)
	return cid, err
}
