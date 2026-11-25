package testrun

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]TestRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, test_plan_id, build_id, summary, notes, started_at
		FROM test_runs ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("query test runs: %w", err)
	}
	defer rows.Close()

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

func (r *PostgresRepository) Create(ctx context.Context, testPlanID, buildID int64, summary, notes string) (TestRun, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TestRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var runID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO test_runs (test_plan_id, build_id, summary, notes, started_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		testPlanID, buildID, summary, notes, now,
	).Scan(&runID)
	if err != nil {
		return TestRun{}, fmt.Errorf("insert test run: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO test_executions (test_run_id, test_case_id, status, steps_snapshot, created_at, updated_at)
		SELECT $1, tpc.test_case_id, 'idle',
			COALESCE((SELECT jsonb_agg(jsonb_build_object('position', s.position, 'action', s.action, 'expected_result', s.expected_result) ORDER BY s.position)
			          FROM test_case_steps s WHERE s.test_case_id = tpc.test_case_id), '[]'::jsonb),
			$2, $3
		FROM test_plan_cases tpc WHERE tpc.test_plan_id = $4 ORDER BY tpc.position ASC`,
		runID, now, now, testPlanID,
	); err != nil {
		return TestRun{}, fmt.Errorf("insert executions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return TestRun{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestRun{ID: runID, TestPlanID: testPlanID, BuildID: buildID, Summary: summary, Notes: notes, StartedAt: now}, nil
}

func (r *PostgresRepository) ListExecutions(ctx context.Context, runID int64) ([]ExecutionItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.test_run_id, e.test_case_id, tc.summary, e.status, e.comment
		FROM test_executions e
		JOIN test_cases tc ON tc.id = e.test_case_id
		WHERE e.test_run_id = $1 ORDER BY e.id ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query executions: %w", err)
	}
	defer rows.Close()

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

func (r *PostgresRepository) Stop(ctx context.Context, runID int64) error {
	tag, err := r.pool.Exec(ctx, `UPDATE test_runs SET stopped_at = $1 WHERE id = $2`, time.Now().UTC(), runID)
	if err != nil {
		return fmt.Errorf("stop test run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, runID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM test_runs WHERE id = $1`, runID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) UpdateExecution(ctx context.Context, executionID int64, status, comment string) (ExecutionResult, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
		UPDATE test_executions SET status = $1, comment = $2, executed_at = $3, updated_at = $4 WHERE id = $5`,
		status, comment, now, now, executionID,
	)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("update execution: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ExecutionResult{}, sql.ErrNoRows
	}
	return ExecutionResult{ID: executionID, Status: status, Comment: comment}, nil
}
