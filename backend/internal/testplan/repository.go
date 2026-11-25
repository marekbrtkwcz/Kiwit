package testplan

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository defines persistence operations for test plans.
type Repository interface {
	List(ctx context.Context) ([]TestPlan, error)
	Create(ctx context.Context, productID, versionID int64, name, description string) (TestPlan, error)
	AddCases(ctx context.Context, planID int64, caseIDs []int64) error
	Clone(ctx context.Context, sourcePlanID int64, newName string, newVersionID *int64) (TestPlan, error)
	Delete(ctx context.Context, id int64) error
}

// SQLiteRepository implements Repository using database/sql (SQLite).
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a SQLite-backed test plan repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context) ([]TestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, product_id, version_id, name, description, is_active, created_at
		FROM test_plans
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query test plans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]TestPlan, 0)
	for rows.Next() {
		var tp TestPlan
		var active int
		if err := rows.Scan(&tp.ID, &tp.ProductID, &tp.VersionID, &tp.Name, &tp.Description, &active, &tp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan test plan: %w", err)
		}
		tp.IsActive = active == 1
		items = append(items, tp)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) Create(ctx context.Context, productID, versionID int64, name, description string) (TestPlan, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO test_plans (product_id, version_id, name, description, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)`,
		productID, versionID, name, description, now, now,
	)
	if err != nil {
		return TestPlan{}, fmt.Errorf("insert test plan: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return TestPlan{}, fmt.Errorf("last insert id: %w", err)
	}
	return TestPlan{
		ID:          id,
		ProductID:   productID,
		VersionID:   versionID,
		Name:        name,
		Description: description,
		IsActive:    true,
		CreatedAt:   now,
	}, nil
}

func (r *SQLiteRepository) AddCases(ctx context.Context, planID int64, caseIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, caseID := range caseIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO test_plan_cases (test_plan_id, test_case_id, position)
			VALUES (?, ?, ?)`,
			planID, caseID, i+1,
		); err != nil {
			return fmt.Errorf("attach case %d: %w", caseID, err)
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) Clone(ctx context.Context, sourcePlanID int64, newName string, newVersionID *int64) (TestPlan, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return TestPlan{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var productID, versionID int64
	var desc string
	err = tx.QueryRowContext(ctx, `SELECT product_id, version_id, description FROM test_plans WHERE id = ?`, sourcePlanID).
		Scan(&productID, &versionID, &desc)
	if err != nil {
		return TestPlan{}, fmt.Errorf("load source plan: %w", err)
	}
	if newVersionID != nil {
		versionID = *newVersionID
	}

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		INSERT INTO test_plans (product_id, version_id, name, description, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, ?, ?)`, productID, versionID, newName, desc, now, now)
	if err != nil {
		return TestPlan{}, fmt.Errorf("insert cloned plan: %w", err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return TestPlan{}, fmt.Errorf("last insert id: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO test_plan_cases (test_plan_id, test_case_id, position)
		SELECT ?, test_case_id, position FROM test_plan_cases WHERE test_plan_id = ?`, newID, sourcePlanID); err != nil {
		return TestPlan{}, fmt.Errorf("clone plan cases: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return TestPlan{}, fmt.Errorf("commit tx: %w", err)
	}
	return TestPlan{
		ID: newID, ProductID: productID, VersionID: versionID, Name: newName, Description: desc, IsActive: true, CreatedAt: now,
	}, nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM test_plans WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
