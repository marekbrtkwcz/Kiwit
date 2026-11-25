package testrun

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) List(ctx context.Context) ([]TestRun, error) {
	items, _, err := r.ListFiltered(ctx, ListFilter{})
	return items, err
}

func (r *PostgresRepository) ListPaginated(ctx context.Context, limit, offset int) ([]TestRun, int64, error) {
	return r.ListFiltered(ctx, ListFilter{Limit: limit, Offset: offset})
}

func (r *PostgresRepository) ListFiltered(ctx context.Context, f ListFilter) ([]TestRun, int64, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 8)
	argIdx := 1

	if f.ProductID > 0 {
		where = append(where, fmt.Sprintf("tp.product_id = $%d", argIdx))
		args = append(args, f.ProductID)
		argIdx++
	}
	if f.TestPlanID > 0 {
		where = append(where, fmt.Sprintf("tr.test_plan_id = $%d", argIdx))
		args = append(args, f.TestPlanID)
		argIdx++
	}
	if f.BuildID > 0 {
		where = append(where, fmt.Sprintf("tr.build_id = $%d", argIdx))
		args = append(args, f.BuildID)
		argIdx++
	}
	switch f.Status {
	case "running":
		where = append(where, "tr.stopped_at IS NULL")
	case "stopped":
		where = append(where, "tr.stopped_at IS NOT NULL")
	}
	if trimmed := strings.TrimSpace(f.Q); trimmed != "" {
		where = append(where, fmt.Sprintf("tr.summary ILIKE $%d", argIdx))
		args = append(args, "%"+trimmed+"%")
		argIdx++
	}
	if f.UserID != nil {
		where = append(where, fmt.Sprintf("(NOT EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = tp.product_id) OR EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = tp.product_id AND pm.user_id = $%d))", argIdx))
		args = append(args, *f.UserID)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM test_runs tr JOIN test_plans tp ON tp.id = tr.test_plan_id`+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count test runs: %w", err)
	}
	query := `
		SELECT tr.id, tr.test_plan_id, tr.build_id, tr.environment_id, tr.manager_id, tr.default_tester_id,
		       tr.summary, tr.notes, tr.planned_start, tr.planned_stop, tr.started_at, tr.stopped_at
		FROM test_runs tr
		JOIN test_plans tp ON tp.id = tr.test_plan_id` + whereClause + ` ORDER BY tr.id DESC`
	queryArgs := append([]any(nil), args...)
	if f.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
		queryArgs = append(queryArgs, f.Limit, f.Offset)
	}
	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query test runs: %w", err)
	}
	defer rows.Close()

	items := make([]TestRun, 0)
	for rows.Next() {
		tr, err := scanTestRun(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan test run: %w", err)
		}
		items = append(items, tr)
	}
	return items, total, rows.Err()
}

func (r *PostgresRepository) GetByID(ctx context.Context, runID int64) (TestRun, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, test_plan_id, build_id, environment_id, manager_id, default_tester_id,
		       summary, notes, planned_start, planned_stop, started_at, stopped_at
		FROM test_runs WHERE id = $1`, runID)
	tr, err := scanTestRun(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TestRun{}, sql.ErrNoRows
		}
		return TestRun{}, err
	}
	return tr, nil
}

func (r *PostgresRepository) Update(ctx context.Context, runID int64, p UpdateParams) (TestRun, error) {
	cur, err := r.GetByID(ctx, runID)
	if err != nil {
		return TestRun{}, err
	}
	if p.BuildID != nil {
		cur.BuildID = *p.BuildID
	}
	if p.EnvironmentID != nil {
		if *p.EnvironmentID == 0 {
			cur.EnvironmentID = nil
		} else {
			cur.EnvironmentID = p.EnvironmentID
		}
	}
	if p.ManagerID != nil {
		cur.ManagerID = p.ManagerID
	}
	if p.DefaultTesterID != nil {
		cur.DefaultTesterID = p.DefaultTesterID
	}
	if strings.TrimSpace(p.Summary) != "" {
		cur.Summary = strings.TrimSpace(p.Summary)
	}
	cur.Notes = p.Notes
	if p.PlannedStart != nil {
		cur.PlannedStart = p.PlannedStart
	}
	if p.PlannedStop != nil {
		cur.PlannedStop = p.PlannedStop
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE test_runs
		SET build_id = $1, environment_id = $2, manager_id = $3, default_tester_id = $4,
		    summary = $5, notes = $6, planned_start = $7, planned_stop = $8
		WHERE id = $9`,
		cur.BuildID, cur.EnvironmentID, cur.ManagerID, cur.DefaultTesterID,
		cur.Summary, cur.Notes, cur.PlannedStart, cur.PlannedStop, runID,
	)
	if err != nil {
		return TestRun{}, fmt.Errorf("update test run: %w", err)
	}
	return cur, nil
}

func (r *PostgresRepository) Create(ctx context.Context, testPlanID, buildID int64, summary, notes string) (TestRun, error) {
	return r.CreateWithMeta(ctx, CreateParams{
		TestPlanID: testPlanID, BuildID: buildID, Summary: summary, Notes: notes,
	})
}

func (r *PostgresRepository) CreateWithMeta(ctx context.Context, p CreateParams) (TestRun, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TestRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var runID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO test_runs (test_plan_id, build_id, environment_id, manager_id, default_tester_id, summary, notes, planned_start, planned_stop, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		p.TestPlanID, p.BuildID, p.EnvironmentID, p.ManagerID, p.DefaultTesterID, p.Summary, p.Notes, p.PlannedStart, p.PlannedStop, now,
	).Scan(&runID)
	if err != nil {
		return TestRun{}, fmt.Errorf("insert test run: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO test_executions (test_run_id, test_case_id, status, assignee_id, environment_id, case_text_version, steps_snapshot, created_at, updated_at)
		SELECT $1, tpc.test_case_id, 'idle', $2, $3,
			COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = tpc.test_case_id), 1),
			COALESCE((SELECT jsonb_agg(jsonb_build_object('position', s.position, 'action', s.action, 'expected_result', s.expected_result) ORDER BY s.position)
			          FROM test_case_steps s WHERE s.test_case_id = tpc.test_case_id), '[]'::jsonb),
			$4, $5
		FROM test_plan_cases tpc WHERE tpc.test_plan_id = $6 ORDER BY tpc.position ASC`,
		runID, p.DefaultTesterID, p.EnvironmentID, now, now, p.TestPlanID,
	); err != nil {
		return TestRun{}, fmt.Errorf("insert executions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return TestRun{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestRun{
		ID: runID, TestPlanID: p.TestPlanID, BuildID: p.BuildID, EnvironmentID: p.EnvironmentID,
		ManagerID: p.ManagerID, DefaultTesterID: p.DefaultTesterID, Summary: p.Summary, Notes: p.Notes,
		PlannedStart: p.PlannedStart, PlannedStop: p.PlannedStop, StartedAt: now,
	}, nil
}

func (r *PostgresRepository) AddCases(ctx context.Context, runID int64, caseIDs []int64, envID *int64) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var runEnv, runTester sql.NullInt64
	if err := tx.QueryRow(ctx, `SELECT environment_id, default_tester_id FROM test_runs WHERE id = $1`, runID).Scan(&runEnv, &runTester); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, sql.ErrNoRows
		}
		return 0, err
	}
	targetEnv := envID
	if targetEnv == nil && runEnv.Valid {
		v := runEnv.Int64
		targetEnv = &v
	}
	var assignee *int64
	if runTester.Valid {
		v := runTester.Int64
		assignee = &v
	}

	now := time.Now().UTC()
	added := 0
	for _, cid := range caseIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_executions (test_run_id, test_case_id, status, assignee_id, environment_id, case_text_version, steps_snapshot, created_at, updated_at)
			VALUES ($1, $2, 'idle', $3, $4,
				COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = $2), 1),
				COALESCE((SELECT jsonb_agg(jsonb_build_object('position', s.position, 'action', s.action, 'expected_result', s.expected_result) ORDER BY s.position)
				          FROM test_case_steps s WHERE s.test_case_id = $2), '[]'::jsonb),
				$5, $6)`,
			runID, cid, assignee, targetEnv, now, now,
		); err != nil {
			return 0, fmt.Errorf("add case %d to run: %w", cid, err)
		}
		added++
	}
	return added, tx.Commit(ctx)
}

func (r *PostgresRepository) DeleteExecution(ctx context.Context, runID, execID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM test_executions WHERE id = $1 AND test_run_id = $2`, execID, runID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) ListExecutions(ctx context.Context, runID int64) ([]ExecutionItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.test_run_id, e.test_case_id, tc.summary, e.status, e.comment,
		       e.assignee_id, e.tested_by_id, e.environment_id, e.case_text_version,
		       COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = e.test_case_id), 1),
		       e.started_at, e.stopped_at
		FROM test_executions e
		JOIN test_cases tc ON tc.id = e.test_case_id
		WHERE e.test_run_id = $1 ORDER BY e.id ASC`, runID)
	if err != nil {
		return nil, fmt.Errorf("query executions: %w", err)
	}
	defer rows.Close()

	items := make([]ExecutionItem, 0)
	for rows.Next() {
		ex, err := scanExecutionItem(rows)
		if err != nil {
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var paths []string
	if rows, err := tx.Query(ctx, `
		SELECT storage_path FROM attachments
		WHERE entity_type = 'test_execution'
		  AND entity_id IN (SELECT id FROM test_executions WHERE test_run_id = $1)`, runID); err == nil {
		for rows.Next() {
			var p string
			if scanErr := rows.Scan(&p); scanErr == nil && p != "" {
				paths = append(paths, p)
			}
		}
		rows.Close()
	}

	_, _ = tx.Exec(ctx, `
		DELETE FROM attachments
		WHERE entity_type = 'test_execution'
		  AND entity_id IN (SELECT id FROM test_executions WHERE test_run_id = $1)`, runID)
	_, _ = tx.Exec(ctx, `
		DELETE FROM comments
		WHERE entity_type = 'test_execution'
		  AND entity_id IN (SELECT id FROM test_executions WHERE test_run_id = $1)`, runID)

	tag, err := tx.Exec(ctx, `DELETE FROM test_runs WHERE id = $1`, runID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
	return nil
}

func (r *PostgresRepository) UpdateExecution(ctx context.Context, executionID int64, status, comment string) (ExecutionResult, error) {
	return r.UpdateExecutionFull(ctx, executionID, ExecutionUpdateParams{Status: status, Comment: comment})
}

func (r *PostgresRepository) UpdateExecutionFull(ctx context.Context, executionID int64, p ExecutionUpdateParams) (ExecutionResult, error) {
	var curStatus, curComment string
	var curAssignee, curTestedBy, curEnv sql.NullInt64
	var curStarted sql.NullTime
	err := r.pool.QueryRow(ctx, `
		SELECT status, comment, assignee_id, tested_by_id, environment_id, started_at
		FROM test_executions WHERE id = $1`, executionID,
	).Scan(&curStatus, &curComment, &curAssignee, &curTestedBy, &curEnv, &curStarted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ExecutionResult{}, sql.ErrNoRows
		}
		return ExecutionResult{}, err
	}

	now := time.Now().UTC()
	nextStatus := curStatus
	if p.Status != "" {
		nextStatus = p.Status
	}
	nextComment := curComment
	if p.Comment != "" || p.Status != "" {
		nextComment = p.Comment
	}
	var nextAssignee *int64
	if curAssignee.Valid {
		v := curAssignee.Int64
		nextAssignee = &v
	}
	if p.AssigneeID != nil {
		if *p.AssigneeID == 0 {
			nextAssignee = nil
		} else {
			nextAssignee = p.AssigneeID
		}
	}
	var nextTestedBy *int64
	if curTestedBy.Valid {
		v := curTestedBy.Int64
		nextTestedBy = &v
	}
	if p.TestedByID != nil {
		nextTestedBy = p.TestedByID
	}
	var nextEnv *int64
	if curEnv.Valid {
		v := curEnv.Int64
		nextEnv = &v
	}
	if p.EnvironmentID != nil {
		if *p.EnvironmentID == 0 {
			nextEnv = nil
		} else {
			nextEnv = p.EnvironmentID
		}
	}

	var startedAt, stoppedAt *time.Time
	if curStarted.Valid {
		t := curStarted.Time
		startedAt = &t
	}
	if nextStatus != "idle" && startedAt == nil {
		startedAt = &now
	}
	if nextStatus != "idle" && nextStatus != "running" {
		stoppedAt = &now
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE test_executions
		SET status = $1, comment = $2, assignee_id = $3, tested_by_id = $4, environment_id = $5,
		    started_at = $6, stopped_at = $7, executed_at = $8, updated_at = $9
		WHERE id = $10`,
		nextStatus, nextComment, nextAssignee, nextTestedBy, nextEnv,
		startedAt, stoppedAt, now, now, executionID,
	)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("update execution: %w", err)
	}
	return ExecutionResult{
		ID:            executionID,
		Status:        nextStatus,
		Comment:       nextComment,
		AssigneeID:    nextAssignee,
		TestedByID:    nextTestedBy,
		EnvironmentID: nextEnv,
	}, nil
}

func (r *PostgresRepository) BulkUpdateExecutions(ctx context.Context, ids []int64, status string, assigneeID, testedByID *int64) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var updated int64
	for _, id := range ids {
		sets := []string{"updated_at = $1"}
		args := []any{now}
		idx := 2
		if status != "" {
			sets = append(sets, fmt.Sprintf("status = $%d", idx), fmt.Sprintf("executed_at = $%d", idx+1))
			args = append(args, status, now)
			idx += 2
			if testedByID != nil {
				sets = append(sets, fmt.Sprintf("tested_by_id = $%d", idx))
				args = append(args, *testedByID)
				idx++
			}
		}
		if assigneeID != nil {
			if *assigneeID == 0 {
				sets = append(sets, "assignee_id = NULL")
			} else {
				sets = append(sets, fmt.Sprintf("assignee_id = $%d", idx))
				args = append(args, *assigneeID)
				idx++
			}
		}
		args = append(args, id)
		tag, err := tx.Exec(ctx, fmt.Sprintf("UPDATE test_executions SET %s WHERE id = $%d", strings.Join(sets, ", "), idx), args...)
		if err != nil {
			return 0, err
		}
		updated += tag.RowsAffected()
	}
	return updated, tx.Commit(ctx)
}

func (r *PostgresRepository) SyncExecutionCase(ctx context.Context, executionID int64) (ExecutionItem, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE test_executions e
		SET steps_snapshot = COALESCE((SELECT jsonb_agg(jsonb_build_object('position', s.position, 'action', s.action, 'expected_result', s.expected_result) ORDER BY s.position)
		                               FROM test_case_steps s WHERE s.test_case_id = e.test_case_id), '[]'::jsonb),
		    case_text_version = COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = e.test_case_id), 1),
		    updated_at = $1
		WHERE e.id = $2`, time.Now().UTC(), executionID)
	if err != nil {
		return ExecutionItem{}, err
	}
	if tag.RowsAffected() == 0 {
		return ExecutionItem{}, sql.ErrNoRows
	}
	row := r.pool.QueryRow(ctx, `
		SELECT e.id, e.test_run_id, e.test_case_id, tc.summary, e.status, e.comment,
		       e.assignee_id, e.tested_by_id, e.environment_id, e.case_text_version,
		       COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = e.test_case_id), 1),
		       e.started_at, e.stopped_at
		FROM test_executions e JOIN test_cases tc ON tc.id = e.test_case_id
		WHERE e.id = $1`, executionID)
	return scanExecutionItem(row)
}

func (r *PostgresRepository) ListLinks(ctx context.Context, executionID int64) ([]ExecutionLink, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, test_execution_id, name, url, created_at
		FROM execution_links WHERE test_execution_id = $1 ORDER BY id ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ExecutionLink, 0)
	for rows.Next() {
		var l ExecutionLink
		if err := rows.Scan(&l.ID, &l.TestExecutionID, &l.Name, &l.URL, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) AddLink(ctx context.Context, executionID int64, name, url string) (ExecutionLink, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO execution_links (test_execution_id, name, url, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		executionID, name, url, now,
	).Scan(&id)
	if err != nil {
		return ExecutionLink{}, err
	}
	return ExecutionLink{ID: id, TestExecutionID: executionID, Name: name, URL: url, CreatedAt: now}, nil
}

func (r *PostgresRepository) DeleteLink(ctx context.Context, executionID, linkID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM execution_links WHERE id = $1 AND test_execution_id = $2`, linkID, executionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}
