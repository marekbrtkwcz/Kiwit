package classification

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Classification struct {
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

func (s *Store) List(ctx context.Context) ([]Classification, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, name FROM classifications ORDER BY id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Classification{}
		for rows.Next() {
			var c Classification
			if err := rows.Scan(&c.ID, &c.Name); err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM classifications ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Classification{}
	for rows.Next() {
		var c Classification
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Create(ctx context.Context, name string) (Classification, error) {
	if s.pg != nil {
		var c Classification
		err := s.pg.QueryRow(ctx, `INSERT INTO classifications (name) VALUES ($1) RETURNING id, name`, name).Scan(&c.ID, &c.Name)
		return c, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO classifications (name) VALUES (?)`, name)
	if err != nil {
		return Classification{}, err
	}
	id, _ := res.LastInsertId()
	return Classification{ID: id, Name: name}, nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM classifications WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM classifications WHERE id = ?`, id)
	return err
}
