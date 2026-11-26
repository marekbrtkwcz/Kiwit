package property

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Property struct {
	ID         int64  `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Name       string `json:"name"`
	Value      string `json:"value"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

func validEntity(t string) bool {
	return t == "test_case" || t == "test_run" || t == "environment"
}

func (s *Store) List(ctx context.Context, entityType string, entityID int64) ([]Property, error) {
	if !validEntity(entityType) {
		return nil, fmt.Errorf("invalid entity_type")
	}
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, entity_type, entity_id, name, value FROM entity_properties WHERE entity_type = $1 AND entity_id = $2 ORDER BY name`, entityType, entityID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []Property
		for rows.Next() {
			var p Property
			if err := rows.Scan(&p.ID, &p.EntityType, &p.EntityID, &p.Name, &p.Value); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_type, entity_id, name, value FROM entity_properties WHERE entity_type = ? AND entity_id = ? ORDER BY name`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Property
	for rows.Next() {
		var p Property
		if err := rows.Scan(&p.ID, &p.EntityType, &p.EntityID, &p.Name, &p.Value); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Set(ctx context.Context, entityType string, entityID int64, name, value string) (Property, error) {
	if !validEntity(entityType) {
		return Property{}, fmt.Errorf("invalid entity_type")
	}
	if s.pg != nil {
		var p Property
		err := s.pg.QueryRow(ctx, `
			INSERT INTO entity_properties (entity_type, entity_id, name, value) VALUES ($1, $2, $3, $4)
			ON CONFLICT (entity_type, entity_id, name) DO UPDATE SET value = EXCLUDED.value
			RETURNING id, entity_type, entity_id, name, value`,
			entityType, entityID, name, value,
		).Scan(&p.ID, &p.EntityType, &p.EntityID, &p.Name, &p.Value)
		return p, err
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO entity_properties (entity_type, entity_id, name, value) VALUES (?, ?, ?, ?)
		ON CONFLICT (entity_type, entity_id, name) DO UPDATE SET value = excluded.value`,
		entityType, entityID, name, value)
	if err != nil {
		return Property{}, err
	}
	id, _ := res.LastInsertId()
	return Property{ID: id, EntityType: entityType, EntityID: entityID, Name: name, Value: value}, nil
}

func (s *Store) Delete(ctx context.Context, entityType string, entityID int64, name string) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM entity_properties WHERE entity_type = $1 AND entity_id = $2 AND name = $3`, entityType, entityID, name)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM entity_properties WHERE entity_type = ? AND entity_id = ? AND name = ?`, entityType, entityID, name)
	return err
}

// --- TestRun CC ---

func (s *Store) ListRunCC(ctx context.Context, runID int64) ([]int64, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT user_id FROM test_run_cc WHERE run_id = $1 ORDER BY user_id`, runID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM test_run_cc WHERE run_id = ? ORDER BY user_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) AddRunCC(ctx context.Context, runID, userID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `INSERT INTO test_run_cc (run_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, runID, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO test_run_cc (run_id, user_id) VALUES (?, ?)`, runID, userID)
	return err
}

func (s *Store) RemoveRunCC(ctx context.Context, runID, userID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM test_run_cc WHERE run_id = $1 AND user_id = $2`, runID, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM test_run_cc WHERE run_id = ? AND user_id = ?`, runID, userID)
	return err
}
