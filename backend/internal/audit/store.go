package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ID         int64           `json:"id"`
	EntityType string          `json:"entity_type"`
	EntityID   int64           `json:"entity_id"`
	Action     string          `json:"action"`
	ActorID    *int64          `json:"actor_id,omitempty"`
	ActorName  string          `json:"actor_name"`
	OldValues  map[string]any  `json:"old_values"`
	NewValues  map[string]any  `json:"new_values"`
	CreatedAt  time.Time       `json:"created_at"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// Record stores an audit entry. old/new are the changed fields only.
func (s *Store) Record(ctx context.Context, entityType string, entityID int64, action string, actorID *int64, actorName string, oldVals, newVals map[string]any) error {
	oldJSON, err := json.Marshal(oldVals)
	if err != nil {
		return err
	}
	newJSON, err := json.Marshal(newVals)
	if err != nil {
		return err
	}
	if s.pg != nil {
		_, err = s.pg.Exec(ctx, `INSERT INTO audit_log (entity_type, entity_id, action, actor_id, actor_name, old_values, new_values) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			entityType, entityID, action, actorID, actorName, oldJSON, newJSON)
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO audit_log (entity_type, entity_id, action, actor_id, actor_name, old_values, new_values) VALUES (?,?,?,?,?,?,?)`,
		entityType, entityID, action, actorID, actorName, oldJSON, newJSON)
	return err
}

func (s *Store) List(ctx context.Context, entityType string, entityID int64, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, entity_type, entity_id, action, actor_id, actor_name, old_values, new_values, created_at FROM audit_log WHERE entity_type = $1 AND entity_id = $2 ORDER BY id DESC LIMIT $3`, entityType, entityID, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanRows(rows)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_type, entity_id, action, actor_id, actor_name, old_values, new_values, created_at FROM audit_log WHERE entity_type = ? AND entity_id = ? ORDER BY id DESC LIMIT ?`, entityType, entityID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSQLRows(rows)
}

// ListAll returns recent audit entries across all entities (for admins).
func (s *Store) ListAll(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, entity_type, entity_id, action, actor_id, actor_name, old_values, new_values, created_at FROM audit_log ORDER BY id DESC LIMIT $1`, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return scanRows(rows)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_type, entity_id, action, actor_id, actor_name, old_values, new_values, created_at FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSQLRows(rows)
}

func scanRows(rows pgx.Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		var e Entry
		var oldJSON, newJSON []byte
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.Action, &e.ActorID, &e.ActorName, &oldJSON, &newJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(oldJSON, &e.OldValues)
		_ = json.Unmarshal(newJSON, &e.NewValues)
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanSQLRows(rows *sql.Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		var e Entry
		var oldJSON, newJSON []byte
		if err := rows.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.Action, &e.ActorID, &e.ActorName, &oldJSON, &newJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(oldJSON, &e.OldValues)
		_ = json.Unmarshal(newJSON, &e.NewValues)
		out = append(out, e)
	}
	return out, rows.Err()
}
