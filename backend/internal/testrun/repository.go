package testrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ExecutionResult represents an updated test execution.
type ExecutionResult struct {
	ID      int64  `json:"id"`
	Status  string `json:"status"`
	Comment string `json:"comment,omitempty"`
}

// ExecutionItem represents a test execution with its test case summary.
type ExecutionItem struct {
	ID          int64  `json:"id"`
	TestRunID   int64  `json:"test_run_id"`
	TestCaseID  int64  `json:"test_case_id"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
	Comment     string `json:"comment,omitempty"`
}

// Repository defines persistence operations for test runs and executions.
type Repository interface {
	List(ctx context.Context) ([]TestRun, error)
	Create(ctx context.Context, testPlanID, buildID int64, summary, notes string) (TestRun, error)
	Stop(ctx context.Context, runID int64) error
	ListExecutions(ctx context.Context, runID int64) ([]ExecutionItem, error)
	UpdateExecution(ctx context.Context, executionID int64, status, comment string) (ExecutionResult, error)
}

// SQLiteRepository implements Repository using database/sql (SQLite).
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a SQLite-backed test run repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]TestRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, test_plan_id, build_id, summary, notes, started_at
		FROM test_runs
		ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query test runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]TestRun, 0)
	for rows.Next() {
		var tr TestRun
		if err := rows.Scan(&tr.ID, &tr.TestPlanID, &tr.BuildID, &tr.Summary, &tr.Notes, &tr.StartedAt); err != nil {
			return nil, fmt.Errorf("scan test run: %w", err)
		}
		items = append(items, tr)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) Stop(ctx context.Context, runID int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE test_runs SET stopped_at = ? WHERE id = ?`, time.Now().UTC(), runID)
	if err != nil {
		return fmt.Errorf("stop test run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) ListExecutions(ctx context.Context, runID int64) ([]ExecutionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.test_run_id, e.test_case_id, tc.summary, e.status, e.comment
		FROM test_executions e
		JOIN test_cases tc ON tc.id = e.test_case_id
		WHERE e.test_run_id = ?
		ORDER BY e.id ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query executions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]ExecutionItem, 0)
	for rows.Next() {
		var ex ExecutionItem
		if err := rows.Scan(&ex.ID, &ex.TestRunID, &ex.TestCaseID, &ex.Summary, &ex.Status, &ex.Comment); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}
		items = append(items, ex)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) Stop(ctx context.Context, runID int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE test_runs SET stopped_at = ? WHERE id = ?`, time.Now().UTC(), runID)
	if err != nil {
		return fmt.Errorf("stop test run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) Create(ctx context.Context, testPlanID, buildID int64, summary, notes string) (TestRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TestRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO test_runs (test_plan_id, build_id, summary, notes, started_at)
		VALUES (?, ?, ?, ?, ?)`,
		testPlanID, buildID, summary, notes, now,
	)
	if err != nil {
		return TestRun{}, fmt.Errorf("insert test run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return TestRun{}, fmt.Errorf("last insert id: %w", err)
	}

	// Snapshot test case steps for all cases attached to the test plan.
	rows, err := tx.QueryContext(ctx, `
		SELECT test_case_id FROM test_plan_cases WHERE test_plan_id = ? ORDER BY position ASC`, testPlanID)
	if err != nil {
		return TestRun{}, fmt.Errorf("query plan cases: %w", err)
	}
	var caseIDs []int64
	for rows.Next() {
		var cid int64
		if err := rows.Scan(&cid); err != nil {
			_ = rows.Close()
			return TestRun{}, fmt.Errorf("scan plan case: %w", err)
		}
		caseIDs = append(caseIDs, cid)
	}
	_ = rows.Close()

	for _, cid := range caseIDs {
		snap, err := loadStepsSnapshot(ctx, tx, cid)
		if err != nil {
			return TestRun{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO test_executions (test_run_id, test_case_id, status, steps_snapshot, created_at, updated_at)
			VALUES (?, ?, 'idle', ?, ?, ?)`,
			runID, cid, snap, now, now,
		); err != nil {
			return TestRun{}, fmt.Errorf("insert execution for case %d: %w", cid, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return TestRun{}, fmt.Errorf("commit tx: %w", err)
	}

	return TestRun{
		ID:         runID,
		TestPlanID: testPlanID,
		BuildID:    buildID,
		Summary:    summary,
		Notes:      notes,
		StartedAt:  now,
	}, nil
}

func (r *SQLiteRepository) UpdateExecution(ctx context.Context, executionID int64, status, comment string) (ExecutionResult, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		UPDATE test_executions
		SET status = ?, comment = ?, executed_at = ?, updated_at = ?
		WHERE id = ?`,
		status, comment, now, now, executionID,
	)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("update execution: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return ExecutionResult{}, sql.ErrNoRows
	}
	return ExecutionResult{
		ID:      executionID,
		Status:  status,
		Comment: comment,
	}, nil
}

type stepSnapshot struct {
	Position       int    `json:"position"`
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result,omitempty"`
}

func loadStepsSnapshot(ctx context.Context, tx *sql.Tx, testCaseID int64) (string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT position, action, expected_result
		FROM test_case_steps
		WHERE test_case_id = ?
		ORDER BY position ASC`, testCaseID)
	if err != nil {
		return "", fmt.Errorf("query steps snapshot: %w", err)
	}
	defer func() { _ = rows.Close() }()

	steps := make([]stepSnapshot, 0)
	for rows.Next() {
		var s stepSnapshot
		if err := rows.Scan(&s.Position, &s.Action, &s.ExpectedResult); err != nil {
			return "", fmt.Errorf("scan step snapshot: %w", err)
		}
		steps = append(steps, s)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	b, err := json.Marshal(steps)
	if err != nil {
		return "", fmt.Errorf("marshal steps snapshot: %w", err)
	}
	return string(b), nil
}
