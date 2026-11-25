package issuetracker

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

func (r *PostgresRepository) ListTrackers(ctx context.Context) ([]Tracker, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, provider, base_url, created_at FROM issue_trackers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query trackers: %w", err)
	}
	defer rows.Close()

	items := make([]Tracker, 0)
	for rows.Next() {
		var t Tracker
		if err := rows.Scan(&t.ID, &t.Name, &t.Provider, &t.BaseURL, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tracker: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateTracker(ctx context.Context, name, provider, baseURL string) (Tracker, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO issue_trackers (name, provider, base_url, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		name, provider, baseURL, now,
	).Scan(&id)
	if err != nil {
		return Tracker{}, fmt.Errorf("insert tracker: %w", err)
	}
	return Tracker{ID: id, Name: name, Provider: provider, BaseURL: baseURL, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListBugsByExecution(ctx context.Context, executionID int64) ([]Bug, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, test_execution_id, tracker_id, external_key, url, title, status, created_at
		FROM bugs WHERE test_execution_id = $1 ORDER BY id ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("query bugs: %w", err)
	}
	defer rows.Close()

	items := make([]Bug, 0)
	for rows.Next() {
		var b Bug
		var tid sql.NullInt64
		if err := rows.Scan(&b.ID, &b.TestExecutionID, &tid, &b.ExternalKey, &b.URL, &b.Title, &b.Status, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan bug: %w", err)
		}
		if tid.Valid {
			b.TrackerID = &tid.Int64
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) LinkBug(ctx context.Context, executionID int64, trackerID *int64, externalKey, url, title string) (Bug, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO bugs (test_execution_id, tracker_id, external_key, url, title, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'open', $6) RETURNING id`,
		executionID, trackerID, externalKey, url, title, now,
	).Scan(&id)
	if err != nil {
		return Bug{}, fmt.Errorf("insert bug: %w", err)
	}
	return Bug{
		ID: id, TestExecutionID: executionID, TrackerID: trackerID,
		ExternalKey: externalKey, URL: url, Title: title, Status: "open", CreatedAt: now,
	}, nil
}
