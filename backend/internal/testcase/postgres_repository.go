package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (r *PostgresRepository) List(ctx context.Context) ([]TestCase, error) {
	items, _, err := r.ListWithFilter(ctx, ListFilter{})
	return items, err
}

func (r *PostgresRepository) ListFiltered(ctx context.Context, q, priority string, limit, offset int) ([]TestCase, int64, error) {
	return r.ListWithFilter(ctx, ListFilter{Q: q, Priority: priority, Limit: limit, Offset: offset})
}

func (r *PostgresRepository) ListWithFilter(ctx context.Context, f ListFilter) ([]TestCase, int64, error) {
	where := make([]string, 0, 10)
	args := make([]any, 0, 12)
	argIdx := 1

	if trimmed := strings.TrimSpace(f.Q); trimmed != "" {
		where = append(where, fmt.Sprintf("tc.summary ILIKE $%d", argIdx))
		args = append(args, "%"+trimmed+"%")
		argIdx++
	}
	if f.ProductID > 0 {
		where = append(where, fmt.Sprintf("tc.product_id = $%d", argIdx))
		args = append(args, f.ProductID)
		argIdx++
	}
	if f.CategoryID > 0 {
		where = append(where, fmt.Sprintf("tc.category_id = $%d", argIdx))
		args = append(args, f.CategoryID)
		argIdx++
	}
	if f.ComponentID > 0 {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM test_case_components tcc WHERE tcc.test_case_id = tc.id AND tcc.component_id = $%d)", argIdx))
		args = append(args, f.ComponentID)
		argIdx++
	}
	if f.TagID > 0 {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM test_case_tags tct WHERE tct.test_case_id = tc.id AND tct.tag_id = $%d)", argIdx))
		args = append(args, f.TagID)
		argIdx++
	}
	if f.Priority != "" {
		where = append(where, fmt.Sprintf("tc.priority = $%d", argIdx))
		args = append(args, f.Priority)
		argIdx++
	}
	if f.Status != "" {
		where = append(where, fmt.Sprintf("tc.status = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	}
	if f.IsAutomated != nil {
		where = append(where, fmt.Sprintf("tc.is_automated = $%d", argIdx))
		args = append(args, *f.IsAutomated)
		argIdx++
	}
	if f.AuthorID > 0 {
		where = append(where, fmt.Sprintf("tc.author_id = $%d", argIdx))
		args = append(args, f.AuthorID)
		argIdx++
	}
	if f.UserID != nil {
		where = append(where, fmt.Sprintf("(NOT EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = tc.product_id) OR EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = tc.product_id AND pm.user_id = $%d))", argIdx))
		args = append(args, *f.UserID)
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_cases tc"+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count test cases: %w", err)
	}

	query := `
		SELECT tc.id, tc.product_id, tc.category_id, tc.author_id, tc.default_tester_id, tc.reviewer_id,
		       tc.summary, tc.preconditions, tc.requirement, tc.extra_link, tc.notes,
		       tc.format, tc.gherkin_body, tc.priority, tc.status, tc.is_automated, tc.script, tc.arguments,
		       tc.created_at, tc.updated_at
		FROM test_cases tc` + whereClause + ` ORDER BY tc.id ASC`
	queryArgs := append([]any(nil), args...)
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		queryArgs = append(queryArgs, f.Limit, f.Offset)
	}

	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query test cases: %w", err)
	}
	defer rows.Close()

	items := make([]TestCase, 0)
	for rows.Next() {
		tc, err := scanPostgresTestCase(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan test case: %w", err)
		}
		items = append(items, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if err := r.attachStepsBatch(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (TestCase, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, product_id, category_id, author_id, default_tester_id, reviewer_id,
		       summary, preconditions, requirement, extra_link, notes,
		       format, gherkin_body, priority, status, is_automated, script, arguments,
		       created_at, updated_at
		FROM test_cases WHERE id = $1`, id)
	tc, err := scanPostgresTestCase(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TestCase{}, sql.ErrNoRows
		}
		return TestCase{}, err
	}
	items := []TestCase{tc}
	if err := r.attachStepsBatch(ctx, items); err != nil {
		return TestCase{}, err
	}
	return items[0], nil
}

func scanPostgresTestCase(s rowScanner) (TestCase, error) {
	var tc TestCase
	var catID, authorID, testerID, reviewerID sql.NullInt64
	if err := s.Scan(
		&tc.ID, &tc.ProductID, &catID, &authorID, &testerID, &reviewerID,
		&tc.Summary, &tc.Preconditions, &tc.Requirement, &tc.ExtraLink, &tc.Notes,
		&tc.Format, &tc.GherkinBody, &tc.Priority, &tc.Status,
		&tc.IsAutomated, &tc.Script, &tc.Arguments,
		&tc.CreatedAt, &tc.UpdatedAt,
	); err != nil {
		return TestCase{}, err
	}
	if catID.Valid {
		tc.CategoryID = &catID.Int64
	}
	if authorID.Valid {
		tc.AuthorID = &authorID.Int64
	}
	if testerID.Valid {
		tc.DefaultTesterID = &testerID.Int64
	}
	if reviewerID.Valid {
		tc.ReviewerID = &reviewerID.Int64
	}
	tc.Steps = []Step{}
	return tc, nil
}

func (r *PostgresRepository) attachStepsBatch(ctx context.Context, items []TestCase) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(items))
	byID := make(map[int64]int, len(items))
	for i, tc := range items {
		ids[i] = tc.ID
		byID[tc.ID] = i
	}
	rows, err := r.pool.Query(ctx,
		`SELECT test_case_id, position, action, expected_result FROM test_case_steps WHERE test_case_id = ANY($1) ORDER BY test_case_id ASC, position ASC`,
		ids,
	)
	if err != nil {
		return fmt.Errorf("batch query steps: %w", err)
	}
	defer rows.Close()

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

func savePostgresRevision(ctx context.Context, tx pgx.Tx, testCaseID int64, p UpsertParams, now time.Time) error {
	var nextVer int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM test_case_revisions WHERE test_case_id = $1`, testCaseID).Scan(&nextVer); err != nil {
		return fmt.Errorf("next revision version: %w", err)
	}
	snap, err := json.Marshal(nonNilSteps(p.Steps))
	if err != nil {
		return fmt.Errorf("marshal revision steps: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO test_case_revisions (test_case_id, version, summary, preconditions, format, gherkin_body, priority, status, steps_snapshot, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		testCaseID, nextVer, p.Summary, p.Preconditions, p.Format, p.GherkinBody, p.Priority, p.Status, snap, now,
	); err != nil {
		return fmt.Errorf("insert revision: %w", err)
	}
	return nil
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
		INSERT INTO test_cases (
			product_id, category_id, author_id, default_tester_id, reviewer_id,
			summary, preconditions, requirement, extra_link, notes,
			format, gherkin_body, priority, status, is_automated, script, arguments, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19) RETURNING id`,
		p.ProductID, p.CategoryID, p.AuthorID, p.DefaultTesterID, p.ReviewerID,
		p.Summary, p.Preconditions, p.Requirement, p.ExtraLink, p.Notes,
		p.Format, p.GherkinBody, p.Priority, p.Status, p.IsAutomated, p.Script, p.Arguments, now, now,
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

	if err := savePostgresRevision(ctx, tx, id, p, now); err != nil {
		return TestCase{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TestCase{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestCase{
		ID: id, ProductID: p.ProductID, CategoryID: p.CategoryID, AuthorID: p.AuthorID, DefaultTesterID: p.DefaultTesterID, ReviewerID: p.ReviewerID,
		Summary: p.Summary, Preconditions: p.Preconditions, Requirement: p.Requirement, ExtraLink: p.ExtraLink, Notes: p.Notes,
		Format: p.Format, GherkinBody: p.GherkinBody,
		Priority: p.Priority, Status: p.Status, IsAutomated: p.IsAutomated, Script: p.Script, Arguments: p.Arguments,
		Steps: nonNilSteps(p.Steps), CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int64, p UpsertParams) (TestCase, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TestCase{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE test_cases
		SET product_id = $1, category_id = $2, default_tester_id = $3, reviewer_id = $4,
		    summary = $5, preconditions = $6, requirement = $7, extra_link = $8, notes = $9,
		    format = $10, gherkin_body = $11, priority = $12, status = $13, is_automated = $14, script = $15, arguments = $16, updated_at = $17
		WHERE id = $18 RETURNING created_at`,
		p.ProductID, p.CategoryID, p.DefaultTesterID, p.ReviewerID,
		p.Summary, p.Preconditions, p.Requirement, p.ExtraLink, p.Notes,
		p.Format, p.GherkinBody, p.Priority, p.Status, p.IsAutomated, p.Script, p.Arguments, now, id,
	).Scan(&createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TestCase{}, sql.ErrNoRows
		}
		return TestCase{}, fmt.Errorf("update test case: %w", err)
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

	if err := savePostgresRevision(ctx, tx, id, p, now); err != nil {
		return TestCase{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return TestCase{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestCase{
		ID: id, ProductID: p.ProductID, CategoryID: p.CategoryID, DefaultTesterID: p.DefaultTesterID, ReviewerID: p.ReviewerID,
		Summary: p.Summary, Preconditions: p.Preconditions, Requirement: p.Requirement, ExtraLink: p.ExtraLink, Notes: p.Notes,
		Format: p.Format, GherkinBody: p.GherkinBody,
		Priority: p.Priority, Status: p.Status, IsAutomated: p.IsAutomated, Script: p.Script, Arguments: p.Arguments,
		Steps: nonNilSteps(p.Steps), CreatedAt: createdAt, UpdatedAt: now,
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

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var paths []string
	if rows, err := tx.Query(ctx, `SELECT storage_path FROM attachments WHERE entity_type = 'test_case' AND entity_id = $1`, id); err == nil {
		for rows.Next() {
			var p string
			if scanErr := rows.Scan(&p); scanErr == nil && p != "" {
				paths = append(paths, p)
			}
		}
		rows.Close()
	}

	tag, err := tx.Exec(ctx, `DELETE FROM test_cases WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	_, _ = tx.Exec(ctx, `DELETE FROM attachments WHERE entity_type = 'test_case' AND entity_id = $1`, id)
	_, _ = tx.Exec(ctx, `DELETE FROM comments WHERE entity_type = 'test_case' AND entity_id = $1`, id)
	_, _ = tx.Exec(ctx, `DELETE FROM email_settings WHERE entity_type = 'test_case' AND entity_id = $1`, id)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	for _, p := range paths {
		_ = os.Remove(p)
	}
	return nil
}

func (r *PostgresRepository) Clone(ctx context.Context, id int64, newSummary string) (TestCase, error) {
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
