package testplan

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

func (r *PostgresRepository) List(ctx context.Context) ([]TestPlan, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, product_id, version_id, parent_id, name, description, is_active, created_at
		FROM test_plans ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query test plans: %w", err)
	}
	defer rows.Close()

	items := make([]TestPlan, 0)
	for rows.Next() {
		var tp TestPlan
		var parentID sql.NullInt64
		if err := rows.Scan(&tp.ID, &tp.ProductID, &tp.VersionID, &parentID, &tp.Name, &tp.Description, &tp.IsActive, &tp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan test plan: %w", err)
		}
		if parentID.Valid {
			tp.ParentID = &parentID.Int64
		}
		items = append(items, tp)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) Create(ctx context.Context, productID, versionID int64, parentID *int64, name, description string) (TestPlan, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO test_plans (product_id, version_id, parent_id, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7) RETURNING id`,
		productID, versionID, parentID, name, description, now, now,
	).Scan(&id)
	if err != nil {
		return TestPlan{}, fmt.Errorf("insert test plan: %w", err)
	}
	return TestPlan{
		ID: id, ProductID: productID, VersionID: versionID, ParentID: parentID, Name: name, Description: description, IsActive: true, CreatedAt: now,
	}, nil
}

func (r *PostgresRepository) ListCases(ctx context.Context, planID int64) ([]PlanCase, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tpc.test_case_id, tpc.position, tc.summary, tc.priority, tc.status
		FROM test_plan_cases tpc
		JOIN test_cases tc ON tc.id = tpc.test_case_id
		WHERE tpc.test_plan_id = $1
		ORDER BY tpc.position ASC, tpc.test_case_id ASC`, planID)
	if err != nil {
		return nil, fmt.Errorf("query plan cases: %w", err)
	}
	defer rows.Close()

	items := make([]PlanCase, 0)
	for rows.Next() {
		var pc PlanCase
		if err := rows.Scan(&pc.TestCaseID, &pc.Position, &pc.Summary, &pc.Priority, &pc.Status); err != nil {
			return nil, fmt.Errorf("scan plan case: %w", err)
		}
		items = append(items, pc)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) AddCases(ctx context.Context, planID int64, caseIDs []int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, caseID := range caseIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_plan_cases (test_plan_id, test_case_id, position)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, planID, caseID, i+1); err != nil {
			return fmt.Errorf("attach case %d: %w", caseID, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) Clone(ctx context.Context, sourcePlanID int64, newName string, newVersionID *int64) (TestPlan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return TestPlan{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productID, versionID int64
	var parentID sql.NullInt64
	var desc string
	err = tx.QueryRow(ctx, `SELECT product_id, version_id, parent_id, description FROM test_plans WHERE id = $1`, sourcePlanID).
		Scan(&productID, &versionID, &parentID, &desc)
	if err != nil {
		return TestPlan{}, fmt.Errorf("load source plan: %w", err)
	}
	if newVersionID != nil {
		versionID = *newVersionID
	}
	var parentPtr *int64
	if parentID.Valid {
		parentPtr = &parentID.Int64
	}

	now := time.Now().UTC()
	var newID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO test_plans (product_id, version_id, parent_id, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, TRUE, $6, $7) RETURNING id`, productID, versionID, parentPtr, newName, desc, now, now).Scan(&newID)
	if err != nil {
		return TestPlan{}, fmt.Errorf("insert cloned plan: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO test_plan_cases (test_plan_id, test_case_id, position)
		SELECT $1, test_case_id, position FROM test_plan_cases WHERE test_plan_id = $2`, newID, sourcePlanID); err != nil {
		return TestPlan{}, fmt.Errorf("clone plan cases: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return TestPlan{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestPlan{
		ID: newID, ProductID: productID, VersionID: versionID, ParentID: parentPtr, Name: newName, Description: desc, IsActive: true, CreatedAt: now,
	}, nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM test_plans WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}
