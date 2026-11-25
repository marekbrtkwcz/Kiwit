package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// UpsertParams holds fields for creating or updating a test case and its steps.
type UpsertParams struct {
	ProductID     int64
	CategoryID    *int64
	Summary       string
	Preconditions string
	Format        string
	GherkinBody   string
	Priority      string
	Status        string
	IsAutomated   bool
	Script        string
	Arguments     string
	Steps         []Step
}

// Revision represents a historical snapshot of a test case.
type Revision struct {
	ID            int64     `json:"id"`
	TestCaseID    int64     `json:"test_case_id"`
	Version       int       `json:"version"`
	Summary       string    `json:"summary"`
	Preconditions string    `json:"preconditions,omitempty"`
	Format        string    `json:"format"`
	GherkinBody   string    `json:"gherkin_body,omitempty"`
	Priority      string    `json:"priority"`
	Status        string    `json:"status"`
	Steps         []Step    `json:"steps"`
	CreatedAt     time.Time `json:"created_at"`
}

// Repository defines persistence operations for test cases.
type Repository interface {
	List(ctx context.Context) ([]TestCase, error)
	ListFiltered(ctx context.Context, q, priority string, limit, offset int) ([]TestCase, int64, error)
	GetByID(ctx context.Context, id int64) (TestCase, error)
	Create(ctx context.Context, p UpsertParams) (TestCase, error)
	Update(ctx context.Context, id int64, p UpsertParams) (TestCase, error)
	ListRevisions(ctx context.Context, testCaseID int64) ([]Revision, error)
	BulkUpdate(ctx context.Context, ids []int64, status, priority string) (int64, error)
	Delete(ctx context.Context, id int64) error
	Clone(ctx context.Context, id int64, newSummary string) (TestCase, error)
}

// SQLiteRepository implements Repository using database/sql (SQLite).
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a SQLite-backed test case repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]TestCase, error) {
	items, _, err := r.ListFiltered(ctx, "", "", 0, 0)
	return items, err
}

func (r *SQLiteRepository) ListFiltered(ctx context.Context, q, priority string, limit, offset int) ([]TestCase, int64, error) {
	where := make([]string, 0, 2)
	args := make([]any, 0, 4)

	if trimmed := strings.TrimSpace(q); trimmed != "" {
		where = append(where, "LOWER(summary) LIKE ?")
		args = append(args, "%"+strings.ToLower(trimmed)+"%")
	}
	if priority != "" {
		where = append(where, "priority = ?")
		args = append(args, priority)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test_cases"+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count test cases: %w", err)
	}

	query := `
		SELECT id, product_id, category_id, summary, preconditions, format, gherkin_body, priority, status, is_automated, script, arguments, created_at, updated_at
		FROM test_cases` + whereClause + ` ORDER BY id ASC`
	queryArgs := append([]any(nil), args...)
	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		queryArgs = append(queryArgs, limit, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query test cases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]TestCase, 0)
	for rows.Next() {
		tc, err := scanSQLiteTestCase(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate test cases: %w", err)
	}

	if err := r.attachStepsBatch(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id int64) (TestCase, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, category_id, summary, preconditions, format, gherkin_body, priority, status, is_automated, script, arguments, created_at, updated_at
		FROM test_cases WHERE id = ?`, id)
	tc, err := scanSQLiteTestCase(row)
	if err != nil {
		return TestCase{}, err
	}
	steps, err := r.loadSteps(ctx, tc.ID)
	if err != nil {
		return TestCase{}, err
	}
	tc.Steps = steps
	return tc, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSQLiteTestCase(s rowScanner) (TestCase, error) {
	var tc TestCase
	var catID sql.NullInt64
	var isAuto int
	if err := s.Scan(
		&tc.ID, &tc.ProductID, &catID, &tc.Summary, &tc.Preconditions,
		&tc.Format, &tc.GherkinBody, &tc.Priority, &tc.Status,
		&isAuto, &tc.Script, &tc.Arguments,
		&tc.CreatedAt, &tc.UpdatedAt,
	); err != nil {
		return TestCase{}, err
	}
	tc.IsAutomated = isAuto == 1
	if catID.Valid {
		tc.CategoryID = &catID.Int64
	}
	tc.Steps = []Step{}
	return tc, nil
}

func (r *SQLiteRepository) attachStepsBatch(ctx context.Context, items []TestCase) error {
	if len(items) == 0 {
		return nil
	}
	placeholders := make([]string, len(items))
	args := make([]any, len(items))
	byID := make(map[int64]int, len(items))
	for i, tc := range items {
		placeholders[i] = "?"
		args[i] = tc.ID
		byID[tc.ID] = i
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT test_case_id, position, action, expected_result FROM test_case_steps WHERE test_case_id IN (`+strings.Join(placeholders, ",")+`) ORDER BY test_case_id ASC, position ASC`,
		args...,
	)
	if err != nil {
		return fmt.Errorf("batch query steps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var tcID int64
		var st Step
		if err := rows.Scan(&tcID, &st.Position, &st.Action, &st.ExpectedResult); err != nil {
			return fmt.Errorf("scan step: %w", err)
		}
		if idx, ok := byID[tcID]; ok {
			items[idx].Steps = append(items[idx].Steps, st)
		}
	}
	return rows.Err()
}

func (r *SQLiteRepository) Create(ctx context.Context, p UpsertParams) (TestCase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	isAutoInt := 0
	if p.IsAutomated {
		isAutoInt = 1
	}
	res, err := tx.ExecContext(ctx, `
		INSERT INTO test_cases (product_id, category_id, summary, preconditions, format, gherkin_body, priority, status, is_automated, script, arguments, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProductID, p.CategoryID, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, isAutoInt, p.Script, p.Arguments, now, now,
	)
	if err != nil {
		return TestCase{}, fmt.Errorf("insert test case: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return TestCase{}, fmt.Errorf("last insert id: %w", err)
	}

	if err := insertSteps(ctx, tx, id, p.Steps); err != nil {
		return TestCase{}, err
	}
	var nextVer int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM test_case_revisions WHERE test_case_id = ?`, id).Scan(&nextVer); err != nil {
		return TestCase{}, fmt.Errorf("next revision version: %w", err)
	}
	if err := saveRevision(ctx, tx, id, nextVer, p, now); err != nil {
		return TestCase{}, err
	}
	if err := tx.Commit(); err != nil {
		return TestCase{}, fmt.Errorf("commit tx: %w", err)
	}

	return TestCase{
		ID:            id,
		ProductID:     p.ProductID,
		CategoryID:    p.CategoryID,
		Summary:       p.Summary,
		Preconditions: p.Preconditions,
		Format:        p.Format,
		GherkinBody:   p.GherkinBody,
		Priority:      p.Priority,
		Status:        p.Status,
		IsAutomated:   p.IsAutomated,
		Script:        p.Script,
		Arguments:     p.Arguments,
		Steps:         nonNilSteps(p.Steps),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, id int64, p UpsertParams) (TestCase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var createdAt time.Time
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM test_cases WHERE id = ?`, id).Scan(&createdAt); err != nil {
		return TestCase{}, err
	}

	now := time.Now().UTC()
	isAutoInt := 0
	if p.IsAutomated {
		isAutoInt = 1
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE test_cases
		SET product_id = ?, category_id = ?, summary = ?, preconditions = ?, format = ?, gherkin_body = ?, priority = ?, status = ?, is_automated = ?, script = ?, arguments = ?, updated_at = ?
		WHERE id = ?`,
		p.ProductID, p.CategoryID, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, isAutoInt, p.Script, p.Arguments, now, id,
	)
	if err != nil {
		return TestCase{}, fmt.Errorf("update test case: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return TestCase{}, fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return TestCase{}, sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM test_case_steps WHERE test_case_id = ?`, id); err != nil {
		return TestCase{}, fmt.Errorf("delete old steps: %w", err)
	}
	if err := insertSteps(ctx, tx, id, p.Steps); err != nil {
		return TestCase{}, err
	}
	var nextVer int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM test_case_revisions WHERE test_case_id = ?`, id).Scan(&nextVer); err != nil {
		return TestCase{}, fmt.Errorf("next revision version: %w", err)
	}
	if err := saveRevision(ctx, tx, id, nextVer, p, now); err != nil {
		return TestCase{}, err
	}
	if err := tx.Commit(); err != nil {
		return TestCase{}, fmt.Errorf("commit tx: %w", err)
	}

	return TestCase{
		ID:            id,
		ProductID:     p.ProductID,
		CategoryID:    p.CategoryID,
		Summary:       p.Summary,
		Preconditions: p.Preconditions,
		Format:        p.Format,
		GherkinBody:   p.GherkinBody,
		Priority:      p.Priority,
		Status:        p.Status,
		IsAutomated:   p.IsAutomated,
		Script:        p.Script,
		Arguments:     p.Arguments,
		Steps:         nonNilSteps(p.Steps),
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}, nil
}

func (r *SQLiteRepository) loadSteps(ctx context.Context, testCaseID int64) ([]Step, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT position, action, expected_result
		FROM test_case_steps
		WHERE test_case_id = ?
		ORDER BY position ASC`, testCaseID)
	if err != nil {
		return nil, fmt.Errorf("query steps: %w", err)
	}
	defer func() { _ = rows.Close() }()

	steps := make([]Step, 0)
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.Position, &s.Action, &s.ExpectedResult); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		steps = append(steps, s)
	}
	return steps, rows.Err()
}

func insertSteps(ctx context.Context, tx *sql.Tx, testCaseID int64, steps []Step) error {
	for i, s := range steps {
		pos := s.Position
		if pos <= 0 {
			pos = i + 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO test_case_steps (test_case_id, position, action, expected_result)
			VALUES (?, ?, ?, ?)`,
			testCaseID, pos, s.Action, s.ExpectedResult,
		); err != nil {
			return fmt.Errorf("insert step %d: %w", pos, err)
		}
	}
	return nil
}

func nonNilSteps(steps []Step) []Step {
	if steps == nil {
		return []Step{}
	}
	return steps
}

func saveRevision(ctx context.Context, tx *sql.Tx, testCaseID int64, version int, p UpsertParams, now time.Time) error {
	snap, err := json.Marshal(nonNilSteps(p.Steps))
	if err != nil {
		return fmt.Errorf("marshal revision steps: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO test_case_revisions (test_case_id, version, summary, preconditions, format, gherkin_body, priority, status, steps_snapshot, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		testCaseID, version, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, string(snap), now,
	); err != nil {
		return fmt.Errorf("insert revision: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListRevisions(ctx context.Context, testCaseID int64) ([]Revision, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, test_case_id, version, summary, preconditions, format, gherkin_body, priority, status, steps_snapshot, created_at
		FROM test_case_revisions
		WHERE test_case_id = ?
		ORDER BY version DESC`, testCaseID)
	if err != nil {
		return nil, fmt.Errorf("query revisions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]Revision, 0)
	for rows.Next() {
		var rev Revision
		var snap string
		if err := rows.Scan(
			&rev.ID, &rev.TestCaseID, &rev.Version, &rev.Summary, &rev.Preconditions,
			&rev.Format, &rev.GherkinBody, &rev.Priority, &rev.Status, &snap, &rev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan revision: %w", err)
		}
		_ = json.Unmarshal([]byte(snap), &rev.Steps)
		rev.Steps = nonNilSteps(rev.Steps)
		items = append(items, rev)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) BulkUpdate(ctx context.Context, ids []int64, status, priority string) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var updated int64
	now := time.Now().UTC()
	for _, id := range ids {
		if status != "" {
			res, err := tx.ExecContext(ctx, `UPDATE test_cases SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
			if err != nil {
				return 0, err
			}
			n, _ := res.RowsAffected()
			updated += n
		}
		if priority != "" {
			res, err := tx.ExecContext(ctx, `UPDATE test_cases SET priority = ?, updated_at = ? WHERE id = ?`, priority, now, id)
			if err != nil {
				return 0, err
			}
			if status == "" {
				n, _ := res.RowsAffected()
				updated += n
			}
		}
	}
	return updated, tx.Commit()
}

func (r *SQLiteRepository) Delete(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var paths []string
	if rows, err := tx.QueryContext(ctx, `SELECT storage_path FROM attachments WHERE entity_type = 'test_case' AND entity_id = ?`, id); err == nil {
		for rows.Next() {
			var p string
			if scanErr := rows.Scan(&p); scanErr == nil && p != "" {
				paths = append(paths, p)
			}
		}
		_ = rows.Close()
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM test_cases WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM attachments WHERE entity_type = 'test_case' AND entity_id = ?`, id)
	_, _ = tx.ExecContext(ctx, `DELETE FROM comments WHERE entity_type = 'test_case' AND entity_id = ?`, id)
	_, _ = tx.ExecContext(ctx, `DELETE FROM email_settings WHERE entity_type = 'test_case' AND entity_id = ?`, id)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
	return nil
}

func (r *SQLiteRepository) Clone(ctx context.Context, id int64, newSummary string) (TestCase, error) {
	tc, err := r.GetByID(ctx, id)
	if err != nil {
		return TestCase{}, err
	}
	summary := newSummary
	if summary == "" {
		summary = tc.Summary + " (copy)"
	}
	return r.Create(ctx, UpsertParams{
		ProductID:     tc.ProductID,
		CategoryID:    tc.CategoryID,
		Summary:       summary,
		Preconditions: tc.Preconditions,
		Format:        tc.Format,
		GherkinBody:   tc.GherkinBody,
		Priority:      tc.Priority,
		Status:        tc.Status,
		IsAutomated:   tc.IsAutomated,
		Script:        tc.Script,
		Arguments:     tc.Arguments,
		Steps:         tc.Steps,
	})
}
