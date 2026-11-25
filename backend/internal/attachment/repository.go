package attachment

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Attachment struct {
	ID          int64     `json:"id"`
	EntityType  string    `json:"entity_type"`
	EntityID    int64     `json:"entity_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	StoragePath string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, a Attachment) (Attachment, error)
	ListByEntity(ctx context.Context, entityType string, entityID int64) ([]Attachment, error)
	GetByID(ctx context.Context, id int64) (Attachment, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Create(ctx context.Context, a Attachment) (Attachment, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO attachments (entity_type, entity_id, filename, content_type, size_bytes, storage_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.EntityType, a.EntityID, a.Filename, a.ContentType, a.SizeBytes, a.StoragePath, now,
	)
	if err != nil {
		return Attachment{}, fmt.Errorf("insert attachment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Attachment{}, fmt.Errorf("last insert id: %w", err)
	}
	a.ID = id
	a.CreatedAt = now
	return a, nil
}

func (r *SQLiteRepository) ListByEntity(ctx context.Context, entityType string, entityID int64) ([]Attachment, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, size_bytes, storage_path, created_at
		FROM attachments
		WHERE entity_type = ? AND entity_id = ?
		ORDER BY id ASC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("query attachments: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) GetByID(ctx context.Context, id int64) (Attachment, error) {
	var a Attachment
	err := r.db.QueryRowContext(ctx, `
		SELECT id, entity_type, entity_id, filename, content_type, size_bytes, storage_path, created_at
		FROM attachments WHERE id = ?`, id,
	).Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Filename, &a.ContentType, &a.SizeBytes, &a.StoragePath, &a.CreatedAt)
	if err != nil {
		return Attachment{}, err
	}
	return a, nil
}
