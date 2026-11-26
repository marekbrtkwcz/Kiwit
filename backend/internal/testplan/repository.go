package testplan

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ListFilter defines SQL filter criteria for listing test plans.
type ListFilter struct {
	ProductID int64
	VersionID int64
	IsActive  *bool
	TagID     int64
	Q         string
	UserID    *int64
}

// CreateParams defines fields for creating a test plan.
type CreateParams struct {
	ProductID   int64
	VersionID   int64
	ParentID    *int64
	Name        string
	Description string
	PlanType    string
	ExtraLink   string
}

// UpdateParams defines fields for updating an existing test plan.
type UpdateParams struct {
	VersionID   *int64
	ParentID    *int64
	Name        string
	Description string
	PlanType    string
	ExtraLink   string
	IsActive    *bool
}

// PlanCase represents a test case attached to a test plan.
type PlanCase struct {
	TestCaseID int64  `json:"test_case_id"`
	Position   int    `json:"position"`
	Summary    string `json:"summary"`
	Priority   string `json:"priority"`
	Status     string `json:"status"`
}

// Repository defines persistence operations for test plans.
type Repository interface {
	List(ctx context.Context) ([]TestPlan, error)
	ListFiltered(ctx context.Context, f ListFilter) ([]TestPlan, error)
	GetByID(ctx context.Context, id int64) (TestPlan, error)
	Create(ctx context.Context, productID, versionID int64, parentID *int64, name, description string) (TestPlan, error)
	CreateWithMeta(ctx context.Context, p CreateParams) (TestPlan, error)
	Update(ctx context.Context, id int64, p UpdateParams) (TestPlan, error)
	ListCases(ctx context.Context, planID int64) ([]PlanCase, error)
	AddCases(ctx context.Context, planID int64, caseIDs []int64) error
	RemoveCase(ctx context.Context, planID, caseID int64) error
	ReorderCases(ctx context.Context, planID int64, caseIDs []int64) error
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
	return r.ListFiltered(ctx, ListFilter{})
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id int64) (TestPlan, error) {
	var tp TestPlan
	var parentID sql.NullInt64
	var active int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at
		FROM test_plans WHERE id = ?`, id,
	).Scan(&tp.ID, &tp.ProductID, &tp.VersionID, &parentID, &tp.Name, &tp.Description, &tp.PlanType, &tp.ExtraLink, &active, &tp.CreatedAt)
	if err != nil {
		return TestPlan{}, err
	}
	if parentID.Valid {
		tp.ParentID = &parentID.Int64
	}
	tp.IsActive = active == 1
	return tp, nil
}

func (r *SQLiteRepository) ListFiltered(ctx context.Context, f ListFilter) ([]TestPlan, error) {
	where := make([]string, 0, 6)
	args := make([]any, 0, 6)

	if f.ProductID > 0 {
		where = append(where, "tp.product_id = ?")
		args = append(args, f.ProductID)
	}
	if f.VersionID > 0 {
		where = append(where, "tp.version_id = ?")
		args = append(args, f.VersionID)
	}
	if f.IsActive != nil {
		if *f.IsActive {
			where = append(where, "tp.is_active = 1")
		} else {
			where = append(where, "tp.is_active = 0")
		}
	}
	if f.TagID > 0 {
		where = append(where, "EXISTS (SELECT 1 FROM test_plan_tags tpt WHERE tpt.test_plan_id = tp.id AND tpt.tag_id = ?)")
		args = append(args, f.TagID)
	}
	if trimmed := strings.TrimSpace(f.Q); trimmed != "" {
		where = append(where, "LOWER(tp.name) LIKE ?")
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

	rows, err := r.db.QueryContext(ctx, `
		SELECT tp.id, tp.product_id, tp.version_id, tp.parent_id, tp.name, tp.description, tp.plan_type, tp.extra_link, tp.is_active, tp.created_at
		FROM test_plans tp`+whereClause+`
		ORDER BY tp.id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("query test plans: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]TestPlan, 0)
	for rows.Next() {
		var tp TestPlan
		var parentID sql.NullInt64
		var active int
		if err := rows.Scan(&tp.ID, &tp.ProductID, &tp.VersionID, &parentID, &tp.Name, &tp.Description, &tp.PlanType, &tp.ExtraLink, &active, &tp.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan test plan: %w", err)
		}
		if parentID.Valid {
			tp.ParentID = &parentID.Int64
		}
		tp.IsActive = active == 1
		items = append(items, tp)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) Create(ctx context.Context, productID, versionID int64, parentID *int64, name, description string) (TestPlan, error) {
	return r.CreateWithMeta(ctx, CreateParams{
		ProductID: productID, VersionID: versionID, ParentID: parentID, Name: name, Description: description, PlanType: "regression",
	})
}

func (r *SQLiteRepository) CreateWithMeta(ctx context.Context, p CreateParams) (TestPlan, error) {
	planType := strings.TrimSpace(p.PlanType)
	if planType == "" {
		planType = "regression"
	}
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO test_plans (product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
		p.ProductID, p.VersionID, p.ParentID, p.Name, p.Description, planType, p.ExtraLink, now, now,
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
		ProductID:   p.ProductID,
		VersionID:   p.VersionID,
		ParentID:    p.ParentID,
		Name:        p.Name,
		Description: p.Description,
		PlanType:    planType,
		ExtraLink:   p.ExtraLink,
		IsActive:    true,
		CreatedAt:   now,
	}, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, id int64, p UpdateParams) (TestPlan, error) {
	var cur TestPlan
	var parentID sql.NullInt64
	var active int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at
		FROM test_plans WHERE id = ?`, id,
	).Scan(&cur.ID, &cur.ProductID, &cur.VersionID, &parentID, &cur.Name, &cur.Description, &cur.PlanType, &cur.ExtraLink, &active, &cur.CreatedAt)
	if err != nil {
		return TestPlan{}, err
	}
	if parentID.Valid {
		cur.ParentID = &parentID.Int64
	}
	cur.IsActive = active == 1

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
	activeInt := 0
	if cur.IsActive {
		activeInt = 1
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE test_plans
		SET version_id = ?, parent_id = ?, name = ?, description = ?, plan_type = ?, extra_link = ?, is_active = ?, updated_at = ?
		WHERE id = ?`,
		cur.VersionID, cur.ParentID, cur.Name, cur.Description, cur.PlanType, cur.ExtraLink, activeInt, time.Now().UTC(), id,
	)
	if err != nil {
		return TestPlan{}, fmt.Errorf("update test plan: %w", err)
	}
	return cur, nil
}

func (r *SQLiteRepository) ListCases(ctx context.Context, planID int64) ([]PlanCase, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tpc.test_case_id, tpc.position, tc.summary, tc.priority, tc.status
		FROM test_plan_cases tpc
		JOIN test_cases tc ON tc.id = tpc.test_case_id
		WHERE tpc.test_plan_id = ?
		ORDER BY tpc.position ASC, tpc.test_case_id ASC`, planID)
	if err != nil {
		return nil, fmt.Errorf("query plan cases: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) AddCases(ctx context.Context, planID int64, caseIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var maxPos int
	_ = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM test_plan_cases WHERE test_plan_id = ?`, planID).Scan(&maxPos)

	for i, caseID := range caseIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO test_plan_cases (test_plan_id, test_case_id, position)
			VALUES (?, ?, ?)`,
			planID, caseID, maxPos+i+1,
		); err != nil {
			return fmt.Errorf("attach case %d: %w", caseID, err)
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) RemoveCase(ctx context.Context, planID, caseID int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM test_plan_cases WHERE test_plan_id = ? AND test_case_id = ?`, planID, caseID)
	if err != nil {
		return fmt.Errorf("remove plan case: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SQLiteRepository) ReorderCases(ctx context.Context, planID int64, caseIDs []int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, cid := range caseIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE test_plan_cases SET position = ? WHERE test_plan_id = ? AND test_case_id = ?`, i+1, planID, cid); err != nil {
			return fmt.Errorf("reorder case %d: %w", cid, err)
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
	var parentID sql.NullInt64
	var desc, planType, extraLink string
	err = tx.QueryRowContext(ctx, `SELECT product_id, version_id, parent_id, description, plan_type, extra_link FROM test_plans WHERE id = ?`, sourcePlanID).
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
	res, err := tx.ExecContext(ctx, `
		INSERT INTO test_plans (product_id, version_id, parent_id, name, description, plan_type, extra_link, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`, productID, versionID, parentPtr, newName, desc, planType, extraLink, now, now)
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
		ID: newID, ProductID: productID, VersionID: versionID, ParentID: parentPtr, Name: newName, Description: desc, PlanType: planType, ExtraLink: extraLink, IsActive: true, CreatedAt: now,
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
