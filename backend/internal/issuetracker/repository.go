package issuetracker

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Tracker struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider" enum:"github,gitlab,jira"`
	BaseURL   string    `json:"base_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Bug struct {
	ID              int64     `json:"id"`
	TestExecutionID int64     `json:"test_execution_id"`
	TrackerID       *int64    `json:"tracker_id,omitempty"`
	ExternalKey     string    `json:"external_key"`
	URL             string    `json:"url"`
	Title           string    `json:"title,omitempty"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

type Repository interface {
	ListTrackers(ctx context.Context) ([]Tracker, error)
	CreateTracker(ctx context.Context, name, provider, baseURL string) (Tracker, error)
	ListBugsByExecution(ctx context.Context, executionID int64) ([]Bug, error)
	LinkBug(ctx context.Context, executionID int64, trackerID *int64, externalKey, url, title string) (Bug, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) ListTrackers(ctx context.Context) ([]Tracker, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, provider, base_url, created_at FROM issue_trackers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query trackers: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) CreateTracker(ctx context.Context, name, provider, baseURL string) (Tracker, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO issue_trackers (name, provider, base_url, created_at) VALUES (?, ?, ?, ?)`,
		name, provider, baseURL, now,
	)
	if err != nil {
		return Tracker{}, fmt.Errorf("insert tracker: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Tracker{}, fmt.Errorf("last insert id: %w", err)
	}
	return Tracker{ID: id, Name: name, Provider: provider, BaseURL: baseURL, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListBugsByExecution(ctx context.Context, executionID int64) ([]Bug, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, test_execution_id, tracker_id, external_key, url, title, status, created_at
		FROM bugs WHERE test_execution_id = ? ORDER BY id ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("query bugs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
		ID:              id,
		TestExecutionID: executionID,
		TrackerID:       trackerID,
		ExternalKey:     externalKey,
		URL:             url,
		Title:           title,
		Status:          "open",
		CreatedAt:       now,
	}, nil
}
