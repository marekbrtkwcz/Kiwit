package comment

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

func (r *PostgresRepository) List(ctx context.Context, entityType string, entityID int64) ([]Comment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, author_id, body, created_at
		FROM comments WHERE entity_type = $1 AND entity_id = $2 ORDER BY id ASC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("query comments: %w", err)
	}
	defer rows.Close()

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

func (r *PostgresRepository) Create(ctx context.Context, entityType string, entityID int64, authorID *int64, body string) (Comment, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO comments (entity_type, entity_id, author_id, body, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`, entityType, entityID, authorID, body, now).Scan(&id)
	if err != nil {
		return Comment{}, fmt.Errorf("insert comment: %w", err)
	}
	return Comment{ID: id, EntityType: entityType, EntityID: entityID, AuthorID: authorID, Body: body, CreatedAt: now}, nil
}

func (r *PostgresRepository) EntityExists(ctx context.Context, entityType string, entityID int64) (bool, error) {
	var q string
	switch entityType {
	case "test_case":
		q = `SELECT COUNT(*) FROM test_cases WHERE id = $1`
	case "test_execution":
		q = `SELECT COUNT(*) FROM test_executions WHERE id = $1`
	default:
		return false, nil
	}
	var count int
	if err := r.pool.QueryRow(ctx, q, entityID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
