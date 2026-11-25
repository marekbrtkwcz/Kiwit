package attachment

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

func (r *PostgresRepository) Create(ctx context.Context, a Attachment) (Attachment, error) {
	now := time.Now().UTC()
	err := r.pool.QueryRow(ctx, `
		INSERT INTO attachments (entity_type, entity_id, filename, content_type, size_bytes, storage_path, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		a.EntityType, a.EntityID, a.Filename, a.ContentType, a.SizeBytes, a.StoragePath, now,
	).Scan(&a.ID)
	if err != nil {
		return Attachment{}, fmt.Errorf("insert attachment: %w", err)
	}
	a.CreatedAt = now
	return a, nil
}

func (r *PostgresRepository) ListByEntity(ctx context.Context, entityType string, entityID int64) ([]Attachment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, size_bytes, storage_path, created_at
		FROM attachments WHERE entity_type = $1 AND entity_id = $2 ORDER BY id ASC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer rows.Close()

	items := make([]Attachment, 0)
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.StoragePath, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (Attachment, error) {
	var a Attachment
	err := r.pool.QueryRow(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, size_bytes, storage_path, created_at
		FROM attachments WHERE id = $1`, id,
	).Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.StoragePath, &a.CreatedAt)
	return a, err
}

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM attachments WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}
	return nil
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
