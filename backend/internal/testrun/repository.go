package testrun

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ListFilter defines SQL filter and pagination criteria for listing test runs.
type ListFilter struct {
	ProductID  int64
	TestPlanID int64
	BuildID    int64
	Status     string // "running" or "stopped"
	Q          string
	UserID     *int64
	Limit      int
	Offset     int
}

type CreateParams struct {
	TestPlanID      int64
	BuildID         int64
	EnvironmentID   *int64
	ManagerID       *int64
	DefaultTesterID *int64
	Summary         string
	Notes           string
	PlannedStart    *time.Time
	PlannedStop     *time.Time
}

type UpdateParams struct {
	BuildID         *int64
	EnvironmentID   *int64
	ManagerID       *int64
	DefaultTesterID *int64
	Summary         string
	Notes           string
	PlannedStart    *time.Time
	PlannedStop     *time.Time
}

type ExecutionUpdateParams struct {
	Status        string
	Comment       string
	AssigneeID    *int64
	TestedByID    *int64
	EnvironmentID *int64
}

// ExecutionResult represents an updated test execution. PrevStatus is the
// status the execution had before the update (additive field used for
// idempotency guards in event emission).
type ExecutionResult struct {
	ID            int64  `json:"id"`
	Status        string `json:"status"`
	PrevStatus    string `json:"prev_status,omitempty"`
	Comment       string `json:"comment,omitempty"`
	AssigneeID    *int64 `json:"assignee_id,omitempty"`
	TestedByID    *int64 `json:"tested_by_id,omitempty"`
	EnvironmentID *int64 `json:"environment_id,omitempty"`
}

// ExecutionItem represents a test execution with its test case summary and metadata.
type ExecutionItem struct {
	ID                int64      `json:"id"`
	TestRunID         int64      `json:"test_run_id"`
	TestCaseID        int64      `json:"test_case_id"`
	Summary           string     `json:"summary"`
	Status            string     `json:"status"`
	Comment           string     `json:"comment,omitempty"`
	AssigneeID        *int64     `json:"assignee_id,omitempty"`
	TestedByID        *int64     `json:"tested_by_id,omitempty"`
	EnvironmentID     *int64     `json:"environment_id,omitempty"`
	CaseTextVersion   int        `json:"case_text_version"`
	LatestCaseVersion int        `json:"latest_case_version"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	StoppedAt         *time.Time `json:"stopped_at,omitempty"`
}

// Repository defines persistence operations for test runs and executions.
type Repository interface {
	List(ctx context.Context) ([]TestRun, error)
	ListPaginated(ctx context.Context, limit, offset int) ([]TestRun, int64, error)
	ListFiltered(ctx context.Context, f ListFilter) ([]TestRun, int64, error)
	GetByID(ctx context.Context, runID int64) (TestRun, error)
	Create(ctx context.Context, testPlanID, buildID int64, summary, notes string) (TestRun, error)
	CreateWithMeta(ctx context.Context, p CreateParams) (TestRun, error)
	Update(ctx context.Context, runID int64, p UpdateParams) (TestRun, error)
	Stop(ctx context.Context, runID int64) error
	Delete(ctx context.Context, runID int64) error
	ListExecutions(ctx context.Context, runID int64) ([]ExecutionItem, error)
	// GetExecutionRun returns the TestRun owning the given execution.
	GetExecutionRun(ctx context.Context, executionID int64) (TestRun, error)
	AddCases(ctx context.Context, runID int64, caseIDs []int64, envID *int64) (int, error)
	DeleteExecution(ctx context.Context, runID, execID int64) error
	UpdateExecution(ctx context.Context, executionID int64, status, comment string) (ExecutionResult, error)
	UpdateExecutionFull(ctx context.Context, executionID int64, p ExecutionUpdateParams) (ExecutionResult, error)
	// GetExecutionStatuses returns the current status of each execution whose
	// ID is present in the database, keyed by execution ID.
	GetExecutionStatuses(ctx context.Context, ids []int64) (map[int64]string, error)
	BulkUpdateExecutions(ctx context.Context, ids []int64, status string, assigneeID, testedByID *int64) (int64, error)
	SyncExecutionCase(ctx context.Context, executionID int64) (ExecutionItem, error)
	ListLinks(ctx context.Context, executionID int64) ([]ExecutionLink, error)
	AddLink(ctx context.Context, executionID int64, name, url string) (ExecutionLink, error)
	DeleteLink(ctx context.Context, executionID, linkID int64) error
	ImportJUnitResults(ctx context.Context, testPlanID, buildID int64, summary string, results []ParsedCaseResult, autoCreate bool) (TestRun, int, error)
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
	items, _, err := r.ListFiltered(ctx, ListFilter{})
	return items, err
}

func (r *SQLiteRepository) ListPaginated(ctx context.Context, limit, offset int) ([]TestRun, int64, error) {
	return r.ListFiltered(ctx, ListFilter{Limit: limit, Offset: offset})
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTestRun(s rowScanner) (TestRun, error) {
	var tr TestRun
	var envID, mgrID, testerID sql.NullInt64
	var pStart, pStop, stopped sql.NullTime
	if err := s.Scan(
		&tr.ID, &tr.TestPlanID, &tr.BuildID, &envID, &mgrID, &testerID,
		&tr.Summary, &tr.Notes, &pStart, &pStop, &tr.StartedAt, &stopped,
	); err != nil {
		return TestRun{}, err
	}
	if envID.Valid {
		tr.EnvironmentID = &envID.Int64
	}
	if mgrID.Valid {
		tr.ManagerID = &mgrID.Int64
	}
	if testerID.Valid {
		tr.DefaultTesterID = &testerID.Int64
	}
	if pStart.Valid {
		t := pStart.Time
		tr.PlannedStart = &t
	}
	if pStop.Valid {
		t := pStop.Time
		tr.PlannedStop = &t
	}
	if stopped.Valid {
		t := stopped.Time
		tr.StoppedAt = &t
	}
	return tr, nil
}

func scanExecutionItem(s rowScanner) (ExecutionItem, error) {
	var ex ExecutionItem
	var assigneeID, testedByID, envID sql.NullInt64
	var started, stopped sql.NullTime
	if err := s.Scan(
		&ex.ID, &ex.TestRunID, &ex.TestCaseID, &ex.Summary, &ex.Status, &ex.Comment,
		&assigneeID, &testedByID, &envID, &ex.CaseTextVersion, &ex.LatestCaseVersion,
		&started, &stopped,
	); err != nil {
		return ExecutionItem{}, err
	}
	if assigneeID.Valid {
		ex.AssigneeID = &assigneeID.Int64
	}
	if testedByID.Valid {
		ex.TestedByID = &testedByID.Int64
	}
	if envID.Valid {
		ex.EnvironmentID = &envID.Int64
	}
	if started.Valid {
		t := started.Time
		ex.StartedAt = &t
	}
	if stopped.Valid {
		t := stopped.Time
		ex.StoppedAt = &t
	}
	return ex, nil
}

func (r *SQLiteRepository) ListFiltered(ctx context.Context, f ListFilter) ([]TestRun, int64, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 8)

	if f.ProductID > 0 {
		where = append(where, "tp.product_id = ?")
		args = append(args, f.ProductID)
	}
	if f.TestPlanID > 0 {
		where = append(where, "tr.test_plan_id = ?")
		args = append(args, f.TestPlanID)
	}
	if f.BuildID > 0 {
		where = append(where, "tr.build_id = ?")
		args = append(args, f.BuildID)
	}
	switch f.Status {
	case "running":
		where = append(where, "tr.stopped_at IS NULL")
	case "stopped":
		where = append(where, "tr.stopped_at IS NOT NULL")
	}
	if trimmed := strings.TrimSpace(f.Q); trimmed != "" {
		where = append(where, "LOWER(tr.summary) LIKE ?")
		args = append(args, "%"+strings.ToLower(trimmed)+"%")
	}
	if f.UserID != nil {
		where = append(where, "(NOT EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = tp.product_id) OR EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = tp.product_id AND pm.user_id = ?))")
		args = append(args, *f.UserID)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_runs tr JOIN test_plans tp ON tp.id = tr.test_plan_id`+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count test runs: %w", err)
	}
	query := `
		SELECT tr.id, tr.test_plan_id, tr.build_id, tr.environment_id, tr.manager_id, tr.default_tester_id,
		       tr.summary, tr.notes, tr.planned_start, tr.planned_stop, tr.started_at, tr.stopped_at
		FROM test_runs tr
		JOIN test_plans tp ON tp.id = tr.test_plan_id` + whereClause + ` ORDER BY tr.id DESC`
	queryArgs := append([]any(nil), args...)
	if f.Limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		queryArgs = append(queryArgs, f.Limit, f.Offset)
	}
	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query test runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) GetByID(ctx context.Context, runID int64) (TestRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, test_plan_id, build_id, environment_id, manager_id, default_tester_id,
		       summary, notes, planned_start, planned_stop, started_at, stopped_at
		FROM test_runs WHERE id = ?`, runID)
	return scanTestRun(row)
}

func (r *SQLiteRepository) GetExecutionRun(ctx context.Context, executionID int64) (TestRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT tr.id, tr.test_plan_id, tr.build_id, tr.environment_id, tr.manager_id, tr.default_tester_id,
		       tr.summary, tr.notes, tr.planned_start, tr.planned_stop, tr.started_at, tr.stopped_at
		FROM test_runs tr
		JOIN test_executions te ON te.test_run_id = tr.id
		WHERE te.id = ?`, executionID)
	return scanTestRun(row)
}

func (r *SQLiteRepository) Update(ctx context.Context, runID int64, p UpdateParams) (TestRun, error) {
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
	_, err = r.db.ExecContext(ctx, `
		UPDATE test_runs
		SET build_id = ?, environment_id = ?, manager_id = ?, default_tester_id = ?, summary = ?, notes = ?, planned_start = ?, planned_stop = ?
		WHERE id = ?`,
		cur.BuildID, cur.EnvironmentID, cur.ManagerID, cur.DefaultTesterID, cur.Summary, cur.Notes, cur.PlannedStart, cur.PlannedStop, runID,
	)
	if err != nil {
		return TestRun{}, fmt.Errorf("update test run: %w", err)
	}
	return cur, nil
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

func (r *SQLiteRepository) Delete(ctx context.Context, runID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var paths []string
	if rows, err := tx.QueryContext(ctx, `
		SELECT storage_path FROM attachments
		WHERE entity_type = 'test_execution'
		  AND entity_id IN (SELECT id FROM test_executions WHERE test_run_id = ?)`, runID); err == nil {
		for rows.Next() {
			var p string
			if scanErr := rows.Scan(&p); scanErr == nil && p != "" {
				paths = append(paths, p)
			}
		}
		_ = rows.Close()
	}

	_, _ = tx.ExecContext(ctx, `
		DELETE FROM attachments
		WHERE entity_type = 'test_execution'
		  AND entity_id IN (SELECT id FROM test_executions WHERE test_run_id = ?)`, runID)
	_, _ = tx.ExecContext(ctx, `
		DELETE FROM comments
		WHERE entity_type = 'test_execution'
		  AND entity_id IN (SELECT id FROM test_executions WHERE test_run_id = ?)`, runID)

	res, err := tx.ExecContext(ctx, `DELETE FROM test_runs WHERE id = ?`, runID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
	return nil
}

func (r *SQLiteRepository) ListExecutions(ctx context.Context, runID int64) ([]ExecutionItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.test_run_id, e.test_case_id, tc.summary, e.status, e.comment,
		       e.assignee_id, e.tested_by_id, e.environment_id, e.case_text_version,
		       COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = e.test_case_id), 1),
		       e.started_at, e.stopped_at
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
		ex, err := scanExecutionItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}
		items = append(items, ex)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) Create(ctx context.Context, testPlanID, buildID int64, summary, notes string) (TestRun, error) {
	return r.CreateWithMeta(ctx, CreateParams{
		TestPlanID: testPlanID, BuildID: buildID, Summary: summary, Notes: notes,
	})
}

func (r *SQLiteRepository) CreateWithMeta(ctx context.Context, p CreateParams) (TestRun, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TestRun{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO test_runs (test_plan_id, build_id, environment_id, manager_id, default_tester_id, summary, notes, planned_start, planned_stop, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.TestPlanID, p.BuildID, p.EnvironmentID, p.ManagerID, p.DefaultTesterID, p.Summary, p.Notes, p.PlannedStart, p.PlannedStop, now,
	)
	if err != nil {
		return TestRun{}, fmt.Errorf("insert test run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return TestRun{}, fmt.Errorf("last insert id: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT test_case_id FROM test_plan_cases WHERE test_plan_id = ? ORDER BY position ASC`, p.TestPlanID)
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
		var revVer int
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 1) FROM test_case_revisions WHERE test_case_id = ?`, cid).Scan(&revVer)
		if revVer <= 0 {
			revVer = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO test_executions (test_run_id, test_case_id, status, assignee_id, environment_id, case_text_version, steps_snapshot, created_at, updated_at)
			VALUES (?, ?, 'idle', ?, ?, ?, ?, ?, ?)`,
			runID, cid, p.DefaultTesterID, p.EnvironmentID, revVer, snap, now, now,
		); err != nil {
			return TestRun{}, fmt.Errorf("insert execution for case %d: %w", cid, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return TestRun{}, fmt.Errorf("commit tx: %w", err)
	}

	return TestRun{
		ID: runID, TestPlanID: p.TestPlanID, BuildID: p.BuildID, EnvironmentID: p.EnvironmentID,
		ManagerID: p.ManagerID, DefaultTesterID: p.DefaultTesterID, Summary: p.Summary, Notes: p.Notes,
		PlannedStart: p.PlannedStart, PlannedStop: p.PlannedStop, StartedAt: now,
	}, nil
}

func (r *SQLiteRepository) AddCases(ctx context.Context, runID int64, caseIDs []int64, envID *int64) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var runEnv, runTester sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT environment_id, default_tester_id FROM test_runs WHERE id = ?`, runID).Scan(&runEnv, &runTester); err != nil {
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
		snap, err := loadStepsSnapshot(ctx, tx, cid)
		if err != nil {
			return 0, err
		}
		var revVer int
		_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 1) FROM test_case_revisions WHERE test_case_id = ?`, cid).Scan(&revVer)
		if revVer <= 0 {
			revVer = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO test_executions (test_run_id, test_case_id, status, assignee_id, environment_id, case_text_version, steps_snapshot, created_at, updated_at)
			VALUES (?, ?, 'idle', ?, ?, ?, ?, ?, ?)`,
			runID, cid, assignee, targetEnv, revVer, snap, now, now,
		); err != nil {
			return 0, fmt.Errorf("add case %d to run: %w", cid, err)
		}
		added++
	}
	return added, tx.Commit()
}

func (r *SQLiteRepository) DeleteExecution(ctx context.Context, runID, execID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM test_executions WHERE id = ? AND test_run_id = ?`, execID, runID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) UpdateExecution(ctx context.Context, executionID int64, status, comment string) (ExecutionResult, error) {
	return r.UpdateExecutionFull(ctx, executionID, ExecutionUpdateParams{Status: status, Comment: comment})
}

func (r *SQLiteRepository) UpdateExecutionFull(ctx context.Context, executionID int64, p ExecutionUpdateParams) (ExecutionResult, error) {
	var curStatus, curComment string
	var curAssignee, curTestedBy, curEnv sql.NullInt64
	var curStarted sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT status, comment, assignee_id, tested_by_id, environment_id, started_at
		FROM test_executions WHERE id = ?`, executionID,
	).Scan(&curStatus, &curComment, &curAssignee, &curTestedBy, &curEnv, &curStarted)
	if err != nil {
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

	_, err = r.db.ExecContext(ctx, `
		UPDATE test_executions
		SET status = ?, comment = ?, assignee_id = ?, tested_by_id = ?, environment_id = ?,
		    started_at = ?, stopped_at = ?, executed_at = ?, updated_at = ?
		WHERE id = ?`,
		nextStatus, nextComment, nextAssignee, nextTestedBy, nextEnv,
		startedAt, stoppedAt, now, now, executionID,
	)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("update execution: %w", err)
	}
	return ExecutionResult{
		ID:            executionID,
		Status:        nextStatus,
		PrevStatus:    curStatus,
		Comment:       nextComment,
		AssigneeID:    nextAssignee,
		TestedByID:    nextTestedBy,
		EnvironmentID: nextEnv,
	}, nil
}

// GetExecutionStatuses returns the status of every listed execution that
// exists, keyed by ID. An empty ID list returns an empty map without querying.
func (r *SQLiteRepository) GetExecutionStatuses(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, status FROM test_executions WHERE id IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, fmt.Errorf("query execution statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("scan execution status: %w", err)
		}
		out[id] = status
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) BulkUpdateExecutions(ctx context.Context, ids []int64, status string, assigneeID, testedByID *int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	var updated int64
	for _, id := range ids {
		sets := []string{"updated_at = ?"}
		args := []any{now}
		if status != "" {
			sets = append(sets, "status = ?", "executed_at = ?")
			args = append(args, status, now)
			if testedByID != nil {
				sets = append(sets, "tested_by_id = ?")
				args = append(args, *testedByID)
			}
		}
		if assigneeID != nil {
			if *assigneeID == 0 {
				sets = append(sets, "assignee_id = NULL")
			} else {
				sets = append(sets, "assignee_id = ?")
				args = append(args, *assigneeID)
			}
		}
		args = append(args, id)
		res, err := tx.ExecContext(ctx, "UPDATE test_executions SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		updated += n
	}
	return updated, tx.Commit()
}

func (r *SQLiteRepository) SyncExecutionCase(ctx context.Context, executionID int64) (ExecutionItem, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ExecutionItem{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var caseID int64
	if err := tx.QueryRowContext(ctx, `SELECT test_case_id FROM test_executions WHERE id = ?`, executionID).Scan(&caseID); err != nil {
		return ExecutionItem{}, err
	}
	snap, err := loadStepsSnapshot(ctx, tx, caseID)
	if err != nil {
		return ExecutionItem{}, err
	}
	var latestVer int
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 1) FROM test_case_revisions WHERE test_case_id = ?`, caseID).Scan(&latestVer)
	if latestVer <= 0 {
		latestVer = 1
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE test_executions SET steps_snapshot = ?, case_text_version = ?, updated_at = ? WHERE id = ?`,
		snap, latestVer, time.Now().UTC(), executionID,
	); err != nil {
		return ExecutionItem{}, err
	}
	if err := tx.Commit(); err != nil {
		return ExecutionItem{}, err
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT e.id, e.test_run_id, e.test_case_id, tc.summary, e.status, e.comment,
		       e.assignee_id, e.tested_by_id, e.environment_id, e.case_text_version,
		       COALESCE((SELECT MAX(rev.version) FROM test_case_revisions rev WHERE rev.test_case_id = e.test_case_id), 1),
		       e.started_at, e.stopped_at
		FROM test_executions e JOIN test_cases tc ON tc.id = e.test_case_id
		WHERE e.id = ?`, executionID)
	return scanExecutionItem(row)
}

func (r *SQLiteRepository) ListLinks(ctx context.Context, executionID int64) ([]ExecutionLink, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, test_execution_id, name, url, created_at
		FROM execution_links WHERE test_execution_id = ? ORDER BY id ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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

func (r *SQLiteRepository) AddLink(ctx context.Context, executionID int64, name, url string) (ExecutionLink, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO execution_links (test_execution_id, name, url, created_at) VALUES (?, ?, ?, ?)`,
		executionID, name, url, now,
	)
	if err != nil {
		return ExecutionLink{}, err
	}
	id, _ := res.LastInsertId()
	return ExecutionLink{ID: id, TestExecutionID: executionID, Name: name, URL: url, CreatedAt: now}, nil
}

func (r *SQLiteRepository) DeleteLink(ctx context.Context, executionID, linkID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM execution_links WHERE id = ? AND test_execution_id = ?`, linkID, executionID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) ImportJUnitResults(ctx context.Context, testPlanID, buildID int64, summary string, results []ParsedCaseResult, autoCreate bool) (TestRun, int, error) {
	tr, err := r.Create(ctx, testPlanID, buildID, summary, "Imported from JUnit XML")
	if err != nil {
		return TestRun{}, 0, err
	}
	execs, err := r.ListExecutions(ctx, tr.ID)
	if err != nil {
		return tr, 0, err
	}
	byName := make(map[string]ExecutionItem, len(execs))
	for _, ex := range execs {
		byName[ex.Summary] = ex
	}

	var productID int64
	if autoCreate {
		_ = r.db.QueryRowContext(ctx, `SELECT product_id FROM test_plans WHERE id = ?`, testPlanID).Scan(&productID)
	}

	createdCases := 0
	now := time.Now().UTC()
	for _, res := range results {
		if ex, ok := byName[res.Name]; ok {
			_, _ = r.UpdateExecution(ctx, ex.ID, res.Status, res.Comment)
			continue
		}
		if !autoCreate || productID == 0 || strings.TrimSpace(res.Name) == "" {
			continue
		}
		ins, err := r.db.ExecContext(ctx, `
			INSERT INTO test_cases (product_id, summary, format, priority, status, is_automated, created_at, updated_at)
			VALUES (?, ?, 'steps', 'medium', 'confirmed', 1, ?, ?)`,
			productID, res.Name, now, now,
		)
		if err != nil {
			continue
		}
		caseID, _ := ins.LastInsertId()
		_, _ = r.db.ExecContext(ctx, `
			INSERT INTO test_case_revisions (test_case_id, version, summary, preconditions, format, gherkin_body, priority, status, steps_snapshot, created_at)
			VALUES (?, 1, ?, '', 'steps', '', 'medium', 'confirmed', '[]', ?)`,
			caseID, res.Name, now,
		)
		_, _ = r.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO test_plan_cases (test_plan_id, test_case_id, position)
			VALUES (?, ?, COALESCE((SELECT MAX(position) + 1 FROM test_plan_cases WHERE test_plan_id = ?), 1))`,
			testPlanID, caseID, testPlanID,
		)
		_, _ = r.db.ExecContext(ctx, `
			INSERT INTO test_executions (test_run_id, test_case_id, status, comment, case_text_version, steps_snapshot, executed_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, 1, '[]', ?, ?, ?)`,
			tr.ID, caseID, res.Status, res.Comment, now, now, now,
		)
		createdCases++
	}
	return tr, createdCases, nil
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
