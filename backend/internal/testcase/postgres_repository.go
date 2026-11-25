package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

func (r *PostgresRepository) List(ctx context.Context) ([]TestCase, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, summary, preconditions, format, gherkin_body, priority, status, created_at, updated_at
		FROM test_cases ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query test cases: %w", err)
	}
	defer rows.Close()

	items := make([]TestCase, 0)
	for rows.Next() {
		var tc TestCase
		if err := rows.Scan(
			&tc.ID, &tc.ProductID, &tc.Summary, &tc.Preconditions,
			&tc.Format, &tc.GherkinBody, &tc.Priority, &tc.Status,
			&tc.CreatedAt, &tc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan test case: %w", err)
		}
		tc.Steps = []Step{}
		items = append(items, tc)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Create(ctx context.Context, p UpsertParams) (TestCase, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var id int64
	err = tx.QueryRow(ctx, `
		INSERT INTO test_cases (product_id, summary, preconditions, format, gherkin_body, priority, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		p.ProductID, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, now, now,
	).Scan(&id)
	if err != nil {
		return TestCase{}, fmt.Errorf("insert test case: %w", err)
	}

	for i, s := range p.Steps {
		pos := s.Position
		if pos <= 0 {
			pos = i + 1
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_case_steps (test_case_id, position, action, expected_result)
			VALUES ($1, $2, $3, $4)`, id, pos, s.Action, s.ExpectedResult); err != nil {
			return TestCase{}, fmt.Errorf("insert step: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return TestCase{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestCase{
		ID: id, ProductID: p.ProductID, CategoryID: p.CategoryID, Summary: p.Summary,
		Preconditions: p.Preconditions, Format: p.Format, GherkinBody: p.GherkinBody,
		Priority: p.Priority, Status: p.Status, Steps: nonNilSteps(p.Steps),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int64, p UpsertParams) (TestCase, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `
		UPDATE test_cases
		SET product_id = $1, summary = $2, preconditions = $3, format = $4, gherkin_body = $5, priority = $6, status = $7, updated_at = $8
		WHERE id = $9`,
		p.ProductID, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, now, id,
	)
	if err != nil {
		return TestCase{}, fmt.Errorf("update test case: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return TestCase{}, sql.ErrNoRows
	}

	if _, err := tx.Exec(ctx, `DELETE FROM test_case_steps WHERE test_case_id = $1`, id); err != nil {
		return TestCase{}, fmt.Errorf("delete old steps: %w", err)
	}
	for i, s := range p.Steps {
		pos := s.Position
		if pos <= 0 {
			pos = i + 1
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_case_steps (test_case_id, position, action, expected_result)
			VALUES ($1, $2, $3, $4)`, id, pos, s.Action, s.ExpectedResult); err != nil {
			return TestCase{}, fmt.Errorf("insert step: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return TestCase{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestCase{
		ID: id, ProductID: p.ProductID, CategoryID: p.CategoryID, Summary: p.Summary,
		Preconditions: p.Preconditions, Format: p.Format, GherkinBody: p.GherkinBody,
		Priority: p.Priority, Status: p.Status, Steps: nonNilSteps(p.Steps),
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (r *PostgresRepository) ListRevisions(ctx context.Context, testCaseID int64) ([]Revision, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, test_case_id, version, summary, preconditions, format, gherkin_body, priority, status, steps_snapshot, created_at
		FROM test_case_revisions WHERE test_case_id = $1 ORDER BY version DESC`, testCaseID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []Revision{}, nil
		}
		return nil, fmt.Errorf("query revisions: %w", err)
	}
	defer rows.Close()

	items := make([]Revision, 0)
	for rows.Next() {
		var rev Revision
		var snap []byte
		if err := rows.Scan(
			&rev.ID, &rev.TestCaseID, &rev.Version, &rev.Summary, &rev.Preconditions,
			&rev.Format, &rev.GherkinBody, &rev.Priority, &rev.Status, &snap, &rev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan revision: %w", err)
		}
		_ = json.Unmarshal(snap, &rev.Steps)
		rev.Steps = nonNilSteps(rev.Steps)
		items = append(items, rev)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) BulkUpdate(ctx context.Context, ids []int64, status, priority string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var updated int64
	now := time.Now().UTC()
	for _, id := range ids {
		if status != "" {
			tag, err := tx.Exec(ctx, `UPDATE test_cases SET status = $1, updated_at = $2 WHERE id = $3`, status, now, id)
			if err != nil {
				return 0, err
			}
			updated += tag.RowsAffected()
		}
		if priority != "" {
			tag, err := tx.Exec(ctx, `UPDATE test_cases SET priority = $1, updated_at = $2 WHERE id = $3`, priority, now, id)
			if err != nil {
				return 0, err
			}
			if status == "" {
				updated += tag.RowsAffected()
			}
		}
	}
	return updated, tx.Commit(ctx)
}
