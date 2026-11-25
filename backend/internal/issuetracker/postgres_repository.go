package issuetracker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func (r *PostgresRepository) ListTrackers(ctx context.Context) ([]Tracker, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, provider, base_url, api_username, created_at FROM issue_trackers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query trackers: %w", err)
	}
	defer rows.Close()

	items := make([]Tracker, 0)
	for rows.Next() {
		var t Tracker
		if err := rows.Scan(&t.ID, &t.Name, &t.Provider, &t.BaseURL, &t.APIUsername, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tracker: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetTrackerByID(ctx context.Context, id int64) (Tracker, error) {
	var t Tracker
	err := r.pool.QueryRow(ctx, `SELECT id, name, provider, base_url, api_username, created_at FROM issue_trackers WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.Provider, &t.BaseURL, &t.APIUsername, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tracker{}, sql.ErrNoRows
		}
		return Tracker{}, err
	}
	return t, nil
}

func (r *PostgresRepository) CreateTracker(ctx context.Context, name, provider, baseURL string) (Tracker, error) {
	return r.CreateTrackerWithAuth(ctx, name, provider, baseURL, "")
}

func (r *PostgresRepository) CreateTrackerWithAuth(ctx context.Context, name, provider, baseURL, apiUsername string) (Tracker, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO issue_trackers (name, provider, base_url, api_username, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		name, provider, baseURL, apiUsername, now,
	).Scan(&id)
	if err != nil {
		return Tracker{}, fmt.Errorf("insert tracker: %w", err)
	}
	return Tracker{ID: id, Name: name, Provider: provider, BaseURL: baseURL, APIUsername: apiUsername, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListBugsByExecution(ctx context.Context, executionID int64) ([]Bug, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, test_execution_id, tracker_id, external_key, url, title, status, last_synced_at, created_at
		FROM bugs WHERE test_execution_id = $1 ORDER BY id ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("query bugs: %w", err)
	}
	defer rows.Close()

	items := make([]Bug, 0)
	for rows.Next() {
		var b Bug
		var tid sql.NullInt64
		var synced sql.NullTime
		if err := rows.Scan(&b.ID, &b.TestExecutionID, &tid, &b.ExternalKey, &b.URL, &b.Title, &b.Status, &synced, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan bug: %w", err)
		}
		if tid.Valid {
			b.TrackerID = &tid.Int64
		}
		if synced.Valid {
			t := synced.Time
			b.LastSyncedAt = &t
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetBugByID(ctx context.Context, bugID int64) (Bug, error) {
	var b Bug
	var tid sql.NullInt64
	var synced sql.NullTime
	err := r.pool.QueryRow(ctx, `
		SELECT id, test_execution_id, tracker_id, external_key, url, title, status, last_synced_at, created_at
		FROM bugs WHERE id = $1`, bugID,
	).Scan(&b.ID, &b.TestExecutionID, &tid, &b.ExternalKey, &b.URL, &b.Title, &b.Status, &synced, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Bug{}, sql.ErrNoRows
		}
		return Bug{}, err
	}
	if tid.Valid {
		b.TrackerID = &tid.Int64
	}
	if synced.Valid {
		t := synced.Time
		b.LastSyncedAt = &t
	}
	return b, nil
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

func (r *PostgresRepository) UpdateBugStatus(ctx context.Context, bugID int64, status string) (Bug, error) {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `UPDATE bugs SET status = $1, last_synced_at = $2 WHERE id = $3`, status, now, bugID)
	if err != nil {
		return Bug{}, err
	}
	if tag.RowsAffected() == 0 {
		return Bug{}, sql.ErrNoRows
	}
	return r.GetBugByID(ctx, bugID)
}
