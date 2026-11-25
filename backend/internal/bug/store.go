package bug

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
)

// InternalBug represents a bug tracked natively inside Kiwit TCMS (similar to Kiwi TCMS tcms.bugs).
type InternalBug struct {
	ID              int64     `json:"id"`
	ProductID       int64     `json:"product_id"`
	BuildID         *int64    `json:"build_id,omitempty"`
	TestExecutionID *int64    `json:"test_execution_id,omitempty"`
	Summary         string    `json:"summary"`
	Description     string    `json:"description,omitempty"`
	Status          string    `json:"status" enum:"open,in_progress,resolved,closed"`
	Severity        string    `json:"severity" enum:"low,medium,high,critical"`
	ReporterID      *int64    `json:"reporter_id,omitempty"`
	AssigneeID      *int64    `json:"assignee_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Filter struct {
	ProductID       int64
	Status          string
	Severity        string
	TestExecutionID int64
	Q               string
	UserID          *int64
}

type Store struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

func NewStore(db *sql.DB, pgPool *pgxpool.Pool) *Store {
	return &Store{db: db, pgPool: pgPool}
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBug(s scanner) (InternalBug, error) {
	var b InternalBug
	var bldID, execID, repID, assID sql.NullInt64
	if err := s.Scan(
		&b.ID, &b.ProductID, &bldID, &execID, &b.Summary, &b.Description,
		&b.Status, &b.Severity, &repID, &assID, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		return InternalBug{}, err
	}
	if bldID.Valid {
		b.BuildID = &bldID.Int64
	}
	if execID.Valid {
		b.TestExecutionID = &execID.Int64
	}
	if repID.Valid {
		b.ReporterID = &repID.Int64
	}
	if assID.Valid {
		b.AssigneeID = &assID.Int64
	}
	return b, nil
}

func (s *Store) List(ctx context.Context, f Filter) ([]InternalBug, error) {
	pg := s.pgPool != nil
	cond := make([]string, 0, 6)
	args := make([]any, 0, 6)
	ph := func() string {
		if pg {
			return fmt.Sprintf("$%d", len(args)+1)
		}
		return "?"
	}

	if f.ProductID > 0 {
		cond = append(cond, "b.product_id = "+ph())
		args = append(args, f.ProductID)
	}
	if f.Status != "" {
		cond = append(cond, "b.status = "+ph())
		args = append(args, f.Status)
	}
	if f.Severity != "" {
		cond = append(cond, "b.severity = "+ph())
		args = append(args, f.Severity)
	}
	if f.TestExecutionID > 0 {
		cond = append(cond, "b.test_execution_id = "+ph())
		args = append(args, f.TestExecutionID)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		if pg {
			cond = append(cond, "b.summary ILIKE "+ph())
			args = append(args, "%"+q+"%")
		} else {
			cond = append(cond, "LOWER(b.summary) LIKE "+ph())
			args = append(args, "%"+strings.ToLower(q)+"%")
		}
	}
	if f.UserID != nil {
		cond = append(cond, "(NOT EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = b.product_id) OR EXISTS (SELECT 1 FROM product_members pm WHERE pm.product_id = b.product_id AND pm.user_id = "+ph()+"))")
		args = append(args, *f.UserID)
	}

	where := ""
	if len(cond) > 0 {
		where = " WHERE " + strings.Join(cond, " AND ")
	}
	q := `SELECT b.id, b.product_id, b.build_id, b.test_execution_id, b.summary, b.description,
	             b.status, b.severity, b.reporter_id, b.assignee_id, b.created_at, b.updated_at
	      FROM internal_bugs b` + where + ` ORDER BY b.id DESC`

	items := make([]InternalBug, 0)
	if pg {
		rows, err := s.pgPool.Query(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := scanBug(rows)
			if err != nil {
				return nil, err
			}
			items = append(items, b)
		}
		return items, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		b, err := scanBug(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

func (s *Store) GetByID(ctx context.Context, id int64) (InternalBug, error) {
	if s.pgPool != nil {
		row := s.pgPool.QueryRow(ctx, `
			SELECT id, product_id, build_id, test_execution_id, summary, description,
			       status, severity, reporter_id, assignee_id, created_at, updated_at
			FROM internal_bugs WHERE id = $1`, id)
		b, err := scanBug(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return InternalBug{}, sql.ErrNoRows
		}
		return b, err
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, product_id, build_id, test_execution_id, summary, description,
		       status, severity, reporter_id, assignee_id, created_at, updated_at
		FROM internal_bugs WHERE id = ?`, id)
	return scanBug(row)
}

func (s *Store) Create(ctx context.Context, b InternalBug) (InternalBug, error) {
	now := time.Now().UTC()
	if b.Status == "" {
		b.Status = "open"
	}
	if b.Severity == "" {
		b.Severity = "medium"
	}
	b.CreatedAt = now
	b.UpdatedAt = now
	if s.pgPool != nil {
		err := s.pgPool.QueryRow(ctx, `
			INSERT INTO internal_bugs (product_id, build_id, test_execution_id, summary, description, status, severity, reporter_id, assignee_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
			b.ProductID, b.BuildID, b.TestExecutionID, b.Summary, b.Description, b.Status, b.Severity, b.ReporterID, b.AssigneeID, now, now,
		).Scan(&b.ID)
		return b, err
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO internal_bugs (product_id, build_id, test_execution_id, summary, description, status, severity, reporter_id, assignee_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ProductID, b.BuildID, b.TestExecutionID, b.Summary, b.Description, b.Status, b.Severity, b.ReporterID, b.AssigneeID, now, now,
	)
	if err != nil {
		return InternalBug{}, err
	}
	b.ID, _ = res.LastInsertId()
	return b, nil
}

func (s *Store) Update(ctx context.Context, id int64, summary, description, status, severity string, assigneeID *int64) (InternalBug, error) {
	cur, err := s.GetByID(ctx, id)
	if err != nil {
		return InternalBug{}, err
	}
	if strings.TrimSpace(summary) != "" {
		cur.Summary = strings.TrimSpace(summary)
	}
	cur.Description = description
	if status != "" {
		cur.Status = status
	}
	if severity != "" {
		cur.Severity = severity
	}
	if assigneeID != nil {
		if *assigneeID == 0 {
			cur.AssigneeID = nil
		} else {
			cur.AssigneeID = assigneeID
		}
	}
	cur.UpdatedAt = time.Now().UTC()
	if s.pgPool != nil {
		_, err = s.pgPool.Exec(ctx, `
			UPDATE internal_bugs SET summary = $1, description = $2, status = $3, severity = $4, assignee_id = $5, updated_at = $6 WHERE id = $7`,
			cur.Summary, cur.Description, cur.Status, cur.Severity, cur.AssigneeID, cur.UpdatedAt, id,
		)
		return cur, err
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE internal_bugs SET summary = ?, description = ?, status = ?, severity = ?, assignee_id = ?, updated_at = ? WHERE id = ?`,
		cur.Summary, cur.Description, cur.Status, cur.Severity, cur.AssigneeID, cur.UpdatedAt, id,
	)
	return cur, err
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pgPool != nil {
		tag, err := s.pgPool.Exec(ctx, `DELETE FROM internal_bugs WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return sql.ErrNoRows
		}
		return nil
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM internal_bugs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func RegisterRoutes(api huma.API, store *Store, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "list-internal-bugs",
		Method:      http.MethodGet,
		Path:        "/api/v1/bugs",
		Summary:     "List bugs in the built-in Bug Tracker",
		Tags:        []string{"Bugs"},
	}, func(ctx context.Context, input *struct {
		ProductID       int64  `query:"product_id"`
		Status          string `query:"status" enum:",open,in_progress,resolved,closed"`
		Severity        string `query:"severity" enum:",low,medium,high,critical"`
		TestExecutionID int64  `query:"test_execution_id"`
		Q               string `query:"q"`
	}) (*struct {
		Body struct {
			Items []InternalBug `json:"items"`
		}
	}, error) {
		items, err := store.List(ctx, Filter{
			ProductID:       input.ProductID,
			Status:          input.Status,
			Severity:        input.Severity,
			TestExecutionID: input.TestExecutionID,
			Q:               input.Q,
			UserID:          auth.FilterUserID(ctx),
		})
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list bugs", err)
		}
		out := &struct {
			Body struct {
				Items []InternalBug `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-internal-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/bugs",
		Summary:     "Create a bug in the built-in Bug Tracker",
		Tags:        []string{"Bugs"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			ProductID       int64  `json:"product_id" minimum:"1"`
			BuildID         *int64 `json:"build_id,omitempty"`
			TestExecutionID *int64 `json:"test_execution_id,omitempty"`
			Summary         string `json:"summary" minLength:"1" maxLength:"255"`
			Description     string `json:"description,omitempty"`
			Severity        string `json:"severity,omitempty" default:"medium" enum:"low,medium,high,critical"`
			AssigneeID      *int64 `json:"assignee_id,omitempty"`
		}
	}) (*struct{ Body InternalBug }, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceProduct(ctx, input.Body.ProductID, true); err != nil {
			return nil, err
		}
		var reporterID *int64
		if u, ok := auth.UserFromContext(ctx); ok {
			uid := u.ID
			reporterID = &uid
		}
		b, err := store.Create(ctx, InternalBug{
			ProductID:       input.Body.ProductID,
			BuildID:         input.Body.BuildID,
			TestExecutionID: input.Body.TestExecutionID,
			Summary:         input.Body.Summary,
			Description:     input.Body.Description,
			Status:          "open",
			Severity:        input.Body.Severity,
			ReporterID:      reporterID,
			AssigneeID:      input.Body.AssigneeID,
		})
		if err != nil {
			return nil, huma.Error400BadRequest("failed to create bug", err)
		}
		return &struct{ Body InternalBug }{Body: b}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-internal-bug",
		Method:      http.MethodPut,
		Path:        "/api/v1/bugs/{id}",
		Summary:     "Update a bug in the built-in Bug Tracker",
		Tags:        []string{"Bugs"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			Summary     string `json:"summary" minLength:"1" maxLength:"255"`
			Description string `json:"description,omitempty"`
			Status      string `json:"status,omitempty" enum:",open,in_progress,resolved,closed"`
			Severity    string `json:"severity,omitempty" enum:",low,medium,high,critical"`
			AssigneeID  *int64 `json:"assignee_id,omitempty"`
		}
	}) (*struct{ Body InternalBug }, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		existing, err := store.GetByID(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("bug not found")
			}
			return nil, huma.Error500InternalServerError("failed to load bug", err)
		}
		if err := guard.EnforceProduct(ctx, existing.ProductID, true); err != nil {
			return nil, err
		}
		updated, err := store.Update(ctx, input.ID, input.Body.Summary, input.Body.Description, input.Body.Status, input.Body.Severity, input.Body.AssigneeID)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to update bug", err)
		}
		return &struct{ Body InternalBug }{Body: updated}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-internal-bug",
		Method:      http.MethodDelete,
		Path:        "/api/v1/bugs/{id}",
		Summary:     "Delete a bug from the built-in Bug Tracker",
		Tags:        []string{"Bugs"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		existing, err := store.GetByID(ctx, input.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("bug not found")
			}
			return nil, huma.Error500InternalServerError("failed to load bug", err)
		}
		if err := guard.EnforceProduct(ctx, existing.ProductID, true); err != nil {
			return nil, err
		}
		if err := store.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete bug", err)
		}
		return nil, nil
	})
}
