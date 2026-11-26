package linkref

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Link struct {
	ID         int64     `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

func validEntity(t string) bool {
	return t == "test_run" || t == "test_execution"
}

func (s *Store) List(ctx context.Context, entityType string, entityID int64) ([]Link, error) {
	if !validEntity(entityType) {
		return nil, fmt.Errorf("invalid entity_type")
	}
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, entity_type, entity_id, name, url, created_at FROM link_references WHERE entity_type = $1 AND entity_id = $2 ORDER BY id`, entityType, entityID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Link{}
		for rows.Next() {
			var l Link
			if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.Name, &l.URL, &l.CreatedAt); err != nil {
				return nil, err
			}
			out = append(out, l)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, entity_type, entity_id, name, url, created_at FROM link_references WHERE entity_type = ? AND entity_id = ? ORDER BY id`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.Name, &l.URL, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) Add(ctx context.Context, entityType string, entityID int64, name, url string) (Link, error) {
	if !validEntity(entityType) {
		return Link{}, fmt.Errorf("invalid entity_type")
	}
	if s.pg != nil {
		var l Link
		err := s.pg.QueryRow(ctx, `INSERT INTO link_references (entity_type, entity_id, name, url) VALUES ($1, $2, $3, $4) RETURNING id, entity_type, entity_id, name, url, created_at`,
			entityType, entityID, name, url).Scan(&l.ID, &l.EntityType, &l.EntityID, &l.Name, &l.URL, &l.CreatedAt)
		return l, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO link_references (entity_type, entity_id, name, url) VALUES (?, ?, ?, ?)`, entityType, entityID, name, url)
	if err != nil {
		return Link{}, err
	}
	id, _ := res.LastInsertId()
	return Link{ID: id, EntityType: entityType, EntityID: entityID, Name: name, URL: url, CreatedAt: time.Now().UTC()}, nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM link_references WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM link_references WHERE id = ?`, id)
	return err
}
