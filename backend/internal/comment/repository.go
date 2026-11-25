package comment

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Comment struct {
	ID         int64     `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	AuthorID   *int64    `json:"author_id,omitempty"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

type Repository interface {
	List(ctx context.Context, entityType string, entityID int64) ([]Comment, error)
	Create(ctx context.Context, entityType string, entityID int64, authorID *int64, body string) (Comment, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) List(ctx context.Context, entityType string, entityID int64) ([]Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, author_id, body, created_at
		FROM comments WHERE entity_type = ? AND entity_id = ? ORDER BY id ASC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]Comment, 0)
	for rows.Next() {
		var c Comment
		var aid sql.NullInt64
		if err := rows.Scan(&c.ID, &c.EntityType, &c.EntityID, &aid, &c.Body, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		if aid.Valid {
			c.AuthorID = &aid.Int64
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) Create(ctx context.Context, entityType string, entityID int64, authorID *int64, body string) (Comment, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO comments (entity_type, entity_id, author_id, body, created_at)
		VALUES (?, ?, ?, ?, ?)`, entityType, entityID, authorID, body, now)
	if err != nil {
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Comment{}, fmt.Errorf("last insert id: %w", err)
	}
	return Comment{ID: id, EntityType: entityType, EntityID: entityID, AuthorID: authorID, Body: body, CreatedAt: now}, nil
}
