package plantype

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PlanType struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

func (s *Store) List(ctx context.Context) ([]PlanType, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, name FROM test_plan_types ORDER BY id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []PlanType{}
		for rows.Next() {
			var t PlanType
			if err := rows.Scan(&t.ID, &t.Name); err != nil {
				return nil, err
			}
			out = append(out, t)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM test_plan_types ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PlanType{}
	for rows.Next() {
		var t PlanType
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Create(ctx context.Context, name string) (PlanType, error) {
	if s.pg != nil {
		var t PlanType
		err := s.pg.QueryRow(ctx, `INSERT INTO test_plan_types (name) VALUES ($1) RETURNING id, name`, name).Scan(&t.ID, &t.Name)
		return t, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO test_plan_types (name) VALUES (?)`, name)
	if err != nil {
		return PlanType{}, err
	}
	id, _ := res.LastInsertId()
	return PlanType{ID: id, Name: name}, nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM test_plan_types WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM test_plan_types WHERE id = ?`, id)
	return err
}
