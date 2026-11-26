package dashboard

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MyRun struct {
	ID        int64      `json:"id"`
	Summary   string     `json:"summary"`
	PlanID    int64      `json:"plan_id"`
	StoppedAt *time.Time `json:"stopped_at,omitempty"`
	StartedAt time.Time  `json:"started_at"`
}

type Activity struct {
	ID         int64     `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Action     string    `json:"action"`
	ActorName  string    `json:"actor_name"`
	CreatedAt  time.Time `json:"created_at"`
}

type Snapshot struct {
	MyRuns         []MyRun    `json:"my_runs"`
	RecentActivity []Activity `json:"recent_activity"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// Get returns the personalized dashboard snapshot for a user: runs they manage
// or test plus the most recent audit activity.
func (s *Store) Get(ctx context.Context, userID int64) (Snapshot, error) {
	var snap Snapshot
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `
			SELECT id, summary, test_plan_id, stopped_at, started_at FROM test_runs
			WHERE manager_id = $1 OR default_tester_id = $1
			ORDER BY id DESC LIMIT 10`, userID)
		if err != nil {
			return snap, err
		}
		for rows.Next() {
			var r MyRun
			if err := rows.Scan(&r.ID, &r.Summary, &r.PlanID, &r.StoppedAt, &r.StartedAt); err != nil {
				rows.Close()
				return snap, err
			}
			snap.MyRuns = append(snap.MyRuns, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return snap, err
		}
		arows, err := s.pg.Query(ctx, `
			SELECT id, entity_type, entity_id, action, actor_name, created_at FROM audit_log
			ORDER BY id DESC LIMIT 15`)
		if err != nil {
			return snap, err
		}
		defer arows.Close()
		for arows.Next() {
			var a Activity
			if err := arows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Action, &a.ActorName, &a.CreatedAt); err != nil {
				return snap, err
			}
			snap.RecentActivity = append(snap.RecentActivity, a)
		}
		return snap, arows.Err()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, summary, test_plan_id, stopped_at, started_at FROM test_runs
		WHERE manager_id = ? OR default_tester_id = ?
		ORDER BY id DESC LIMIT 10`, userID, userID)
	if err != nil {
		return snap, err
	}
	for rows.Next() {
		var r MyRun
		if err := rows.Scan(&r.ID, &r.Summary, &r.PlanID, &r.StoppedAt, &r.StartedAt); err != nil {
			rows.Close()
			return snap, err
		}
		snap.MyRuns = append(snap.MyRuns, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return snap, err
	}
	arows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, action, actor_name, created_at FROM audit_log
		ORDER BY id DESC LIMIT 15`)
	if err != nil {
		return snap, err
	}
	defer arows.Close()
	for arows.Next() {
		var a Activity
		if err := arows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Action, &a.ActorName, &a.CreatedAt); err != nil {
			return snap, err
		}
		snap.RecentActivity = append(snap.RecentActivity, a)
	}
	return snap, arows.Err()
}
