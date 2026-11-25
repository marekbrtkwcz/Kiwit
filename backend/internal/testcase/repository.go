package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
	Create(ctx context.Context, p UpsertParams) (TestCase, error)
	Update(ctx context.Context, id int64, p UpsertParams) (TestCase, error)
	ListRevisions(ctx context.Context, testCaseID int64) ([]Revision, error)
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
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, product_id, category_id, summary, preconditions, format, gherkin_body, priority, status, created_at, updated_at
		FROM test_cases
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query test cases: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]TestCase, 0)
	for rows.Next() {
		var tc TestCase
		var catID sql.NullInt64
		if err := rows.Scan(
			&tc.ID, &tc.ProductID, &catID, &tc.Summary, &tc.Preconditions,
			&tc.Format, &tc.GherkinBody, &tc.Priority, &tc.Status,
			&tc.CreatedAt, &tc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan test case: %w", err)
		}
		if catID.Valid {
			tc.CategoryID = &catID.Int64
		}
		tc.Steps = []Step{}
		items = append(items, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate test cases: %w", err)
	}

	for i := range items {
		steps, err := r.loadSteps(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
		items[i].Steps = steps
	}
	return items, nil
}

func (r *SQLiteRepository) Create(ctx context.Context, p UpsertParams) (TestCase, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO test_cases (product_id, category_id, summary, preconditions, format, gherkin_body, priority, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProductID, p.CategoryID, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, now, now,
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

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE test_cases
		SET product_id = ?, category_id = ?, summary = ?, preconditions = ?, format = ?, gherkin_body = ?, priority = ?, status = ?, updated_at = ?
		WHERE id = ?`,
		p.ProductID, p.CategoryID, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, now, id,
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
		Steps:         nonNilSteps(p.Steps),
		CreatedAt:     now,
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
