package testplan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func (r *PostgresRepository) List(ctx context.Context) ([]TestPlan, error) {
	return r.ListFiltered(ctx, ListFilter{})
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (TestPlan, error) {
	var tp TestPlan
	var parentID sql.NullInt64
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at
		FROM test_plans WHERE id = $1`, id,
	).Scan(&tp.ID, &tp.ProductID, &tp.VersionID, &parentID, &tp.Name, &tp.Description, &tp.PlanType, &tp.ExtraLink, &tp.IsActive, &tp.CreatedAt)
	if err != nil {
		return TestPlan{}, err
	}
	if parentID.Valid {
		tp.ParentID = &parentID.Int64
	}
	return tp, nil
}

func (r *PostgresRepository) ListFiltered(ctx context.Context, f ListFilter) ([]TestPlan, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 6)
	argIdx := 1

	if f.ProductID > 0 {
		where = append(where, fmt.Sprintf("tp.product_id = $%d", argIdx))
		args = append(args, f.ProductID)
		argIdx++
	}
	if f.VersionID > 0 {
		where = append(where, fmt.Sprintf("tp.version_id = $%d", argIdx))
		args = append(args, f.VersionID)
		argIdx++
	}
	if f.IsActive != nil {
		where = append(where, fmt.Sprintf("tp.is_active = $%d", argIdx))
		args = append(args, *f.IsActive)
		argIdx++
	}
	if f.TagID > 0 {
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM test_plan_tags tpt WHERE tpt.test_plan_id = tp.id AND tpt.tag_id = $%d)", argIdx))
		args = append(args, f.TagID)
		argIdx++
	}
	if trimmed := strings.TrimSpace(f.Q); trimmed != "" {
		where = append(where, fmt.Sprintf("tp.name ILIKE $%d", argIdx))
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

	rows, err := r.pool.Query(ctx, `
		SELECT tp.id, tp.product_id, tp.version_id, tp.parent_id, tp.name, tp.description, tp.plan_type, tp.extra_link, tp.is_active, tp.created_at
		FROM test_plans tp`+whereClause+` ORDER BY tp.id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query test plans: %w", err)
	}
	defer rows.Close()

	items := make([]TestPlan, 0)
	for rows.Next() {
		var tp TestPlan
		var parentID sql.NullInt64
		if err := rows.Scan(&tp.ID, &tp.ProductID, &tp.VersionID, &parentID, &tp.Name, &tp.Description, &tp.PlanType, &tp.ExtraLink, &tp.IsActive, &tp.CreatedAt); err != nil {
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
	return r.CreateWithMeta(ctx, CreateParams{
		ProductID: productID, VersionID: versionID, ParentID: parentID, Name: name, Description: description, PlanType: "regression",
	})
}

func (r *PostgresRepository) CreateWithMeta(ctx context.Context, p CreateParams) (TestPlan, error) {
	planType := strings.TrimSpace(p.PlanType)
	if planType == "" {
		planType = "regression"
	}
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO test_plans (product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, $8, $9) RETURNING id`,
		p.ProductID, p.VersionID, p.ParentID, p.Name, p.Description, planType, p.ExtraLink, now, now,
	).Scan(&id)
	if err != nil {
		return TestPlan{}, fmt.Errorf("insert test plan: %w", err)
	}
	return TestPlan{
		ID: id, ProductID: p.ProductID, VersionID: p.VersionID, ParentID: p.ParentID,
		Name: p.Name, Description: p.Description, PlanType: planType, ExtraLink: p.ExtraLink, IsActive: true, CreatedAt: now,
	}, nil
}

func (r *PostgresRepository) Update(ctx context.Context, id int64, p UpdateParams) (TestPlan, error) {
	var cur TestPlan
	var parentID sql.NullInt64
	err := r.pool.QueryRow(ctx, `
		SELECT id, product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at
		FROM test_plans WHERE id = $1`, id,
	).Scan(&cur.ID, &cur.ProductID, &cur.VersionID, &parentID, &cur.Name, &cur.Description, &cur.PlanType, &cur.ExtraLink, &cur.IsActive, &cur.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TestPlan{}, sql.ErrNoRows
		}
		return TestPlan{}, err
	}
	if parentID.Valid {
		cur.ParentID = &parentID.Int64
	}

	if p.VersionID != nil {
		cur.VersionID = *p.VersionID
	}
	if p.ParentID != nil {
		if *p.ParentID == 0 {
			cur.ParentID = nil
		} else {
			cur.ParentID = p.ParentID
		}
	}
	if strings.TrimSpace(p.Name) != "" {
		cur.Name = strings.TrimSpace(p.Name)
	}
	cur.Description = p.Description
	if strings.TrimSpace(p.PlanType) != "" {
		cur.PlanType = strings.TrimSpace(p.PlanType)
	}
	cur.ExtraLink = p.ExtraLink
	if p.IsActive != nil {
		cur.IsActive = *p.IsActive
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE test_plans
		SET version_id = $1, parent_id = $2, name = $3, description = $4, plan_type = $5, extra_link = $6, is_active = $7, updated_at = $8
		WHERE id = $9`,
		cur.VersionID, cur.ParentID, cur.Name, cur.Description, cur.PlanType, cur.ExtraLink, cur.IsActive, time.Now().UTC(), id,
	)
	if err != nil {
		return TestPlan{}, fmt.Errorf("update test plan: %w", err)
	}
	return cur, nil
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

	var maxPos int
	_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(position), 0) FROM test_plan_cases WHERE test_plan_id = $1`, planID).Scan(&maxPos)

	for i, caseID := range caseIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO test_plan_cases (test_plan_id, test_case_id, position)
			VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, planID, caseID, maxPos+i+1); err != nil {
			return fmt.Errorf("attach case %d: %w", caseID, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) RemoveCase(ctx context.Context, planID, caseID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM test_plan_cases WHERE test_plan_id = $1 AND test_case_id = $2`, planID, caseID)
	if err != nil {
		return fmt.Errorf("remove plan case: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) ReorderCases(ctx context.Context, planID int64, caseIDs []int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, cid := range caseIDs {
		if _, err := tx.Exec(ctx, `UPDATE test_plan_cases SET position = $1 WHERE test_plan_id = $2 AND test_case_id = $3`, i+1, planID, cid); err != nil {
			return fmt.Errorf("reorder case %d: %w", cid, err)
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
	var desc, planType, extraLink string
	err = tx.QueryRow(ctx, `SELECT product_id, version_id, parent_id, description, plan_type, extra_link FROM test_plans WHERE id = $1`, sourcePlanID).
		Scan(&productID, &versionID, &parentID, &desc, &planType, &extraLink)
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
		INSERT INTO test_plans (product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE, $8, $9) RETURNING id`, productID, versionID, parentPtr, newName, desc, planType, extraLink, now, now).Scan(&newID)
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
		ID: newID, ProductID: productID, VersionID: versionID, ParentID: parentPtr, Name: newName, Description: desc, PlanType: planType, ExtraLink: extraLink, IsActive: true, CreatedAt: now,
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
