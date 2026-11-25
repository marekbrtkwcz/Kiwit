package issuetracker

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Tracker struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Provider    string    `json:"provider" enum:"github,gitlab,jira"`
	BaseURL     string    `json:"base_url"`
	APIUsername string    `json:"api_username,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Bug struct {
	ID              int64      `json:"id"`
	TestExecutionID int64      `json:"test_execution_id"`
	TrackerID       *int64     `json:"tracker_id,omitempty"`
	ExternalKey     string     `json:"external_key"`
	URL             string     `json:"url"`
	Title           string     `json:"title,omitempty"`
	Status          string     `json:"status"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Repository interface {
	ListTrackers(ctx context.Context) ([]Tracker, error)
	GetTrackerByID(ctx context.Context, id int64) (Tracker, error)
	CreateTracker(ctx context.Context, name, provider, baseURL string) (Tracker, error)
	CreateTrackerWithAuth(ctx context.Context, name, provider, baseURL, apiUsername string) (Tracker, error)
	ListBugsByExecution(ctx context.Context, executionID int64) ([]Bug, error)
	GetBugByID(ctx context.Context, bugID int64) (Bug, error)
	LinkBug(ctx context.Context, executionID int64, trackerID *int64, externalKey, url, title string) (Bug, error)
	UpdateBugStatus(ctx context.Context, bugID int64, status string) (Bug, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) ListTrackers(ctx context.Context) ([]Tracker, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, provider, base_url, api_username, created_at FROM issue_trackers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query trackers: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) GetTrackerByID(ctx context.Context, id int64) (Tracker, error) {
	var t Tracker
	err := r.db.QueryRowContext(ctx, `SELECT id, name, provider, base_url, api_username, created_at FROM issue_trackers WHERE id = ?`, id).
		Scan(&t.ID, &t.Name, &t.Provider, &t.BaseURL, &t.APIUsername, &t.CreatedAt)
	if err != nil {
		return Tracker{}, err
	}
	return t, nil
}

func (r *SQLiteRepository) CreateTracker(ctx context.Context, name, provider, baseURL string) (Tracker, error) {
	return r.CreateTrackerWithAuth(ctx, name, provider, baseURL, "")
}

func (r *SQLiteRepository) CreateTrackerWithAuth(ctx context.Context, name, provider, baseURL, apiUsername string) (Tracker, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO issue_trackers (name, provider, base_url, api_username, created_at) VALUES (?, ?, ?, ?, ?)`,
		name, provider, baseURL, apiUsername, now,
	)
	if err != nil {
		return Tracker{}, fmt.Errorf("insert tracker: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Tracker{}, fmt.Errorf("last insert id: %w", err)
	}
	return Tracker{ID: id, Name: name, Provider: provider, BaseURL: baseURL, APIUsername: apiUsername, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListBugsByExecution(ctx context.Context, executionID int64) ([]Bug, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, test_execution_id, tracker_id, external_key, url, title, status, last_synced_at, created_at
		FROM bugs WHERE test_execution_id = ? ORDER BY id ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("query bugs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) GetBugByID(ctx context.Context, bugID int64) (Bug, error) {
	var b Bug
	var tid sql.NullInt64
	var synced sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, test_execution_id, tracker_id, external_key, url, title, status, last_synced_at, created_at
		FROM bugs WHERE id = ?`, bugID,
	).Scan(&b.ID, &b.TestExecutionID, &tid, &b.ExternalKey, &b.URL, &b.Title, &b.Status, &synced, &b.CreatedAt)
	if err != nil {
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

func (r *SQLiteRepository) LinkBug(ctx context.Context, executionID int64, trackerID *int64, externalKey, url, title string) (Bug, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO bugs (test_execution_id, tracker_id, external_key, url, title, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'open', ?)`,
		executionID, trackerID, externalKey, url, title, now,
	)
	if err != nil {
		return Bug{}, fmt.Errorf("insert bug: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Bug{}, fmt.Errorf("last insert id: %w", err)
	}
	return Bug{
		ID: id, TestExecutionID: executionID, TrackerID: trackerID,
		ExternalKey: externalKey, URL: url, Title: title, Status: "open", CreatedAt: now,
	}, nil
}

func (r *SQLiteRepository) UpdateBugStatus(ctx context.Context, bugID int64, status string) (Bug, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `UPDATE bugs SET status = ?, last_synced_at = ? WHERE id = ?`, status, now, bugID)
	if err != nil {
		return Bug{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Bug{}, sql.ErrNoRows
	}
	return r.GetBugByID(ctx, bugID)
}
