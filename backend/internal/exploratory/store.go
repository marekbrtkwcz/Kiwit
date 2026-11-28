package exploratory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID             int64     `json:"id"`
	ProductID      int64     `json:"product_id"`
	TestRunID      *int64    `json:"test_run_id,omitempty"`
	Charter        string    `json:"charter"`
	TimeboxMinutes int       `json:"timebox_minutes"`
	TesterID       *int64    `json:"tester_id,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	Findings       string    `json:"findings"` // JSON array
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const cols = `id, product_id, test_run_id, charter, timebox_minutes, tester_id, notes, findings, created_at, updated_at`

func scanRow(row interface{ Scan(...any) error }) (Session, error) {
	var s Session
	err := row.Scan(&s.ID, &s.ProductID, &s.TestRunID, &s.Charter, &s.TimeboxMinutes, &s.TesterID, &s.Notes, &s.Findings, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

func (s *Store) List(ctx context.Context, productID int64) ([]Session, error) {
	q := fmt.Sprintf(`SELECT %s FROM exploratory_sessions WHERE ($1 = 0 OR product_id = $1) ORDER BY id DESC`, cols)
	qs := fmt.Sprintf(`SELECT %s FROM exploratory_sessions WHERE (? = 0 OR product_id = ?) ORDER BY id DESC`, cols)
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, q, productID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Session{}
		for rows.Next() {
			x, err := scanRow(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, x)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, qs, productID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		x, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int64) (Session, error) {
	if s.pg != nil {
		return scanRow(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM exploratory_sessions WHERE id = $1", cols), id))
	}
	return scanRow(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM exploratory_sessions WHERE id = ?", cols), id))
}

type SessionInput struct {
	ProductID      int64  `json:"product_id"`
	TestRunID      *int64 `json:"test_run_id,omitempty"`
	Charter        string `json:"charter"`
	TimeboxMinutes int    `json:"timebox_minutes,omitempty"`
	TesterID       *int64 `json:"tester_id,omitempty"`
	Notes          string `json:"notes,omitempty"`
	Findings       string `json:"findings,omitempty"` // JSON array
}

func (s *Store) Create(ctx context.Context, in SessionInput) (Session, error) {
	if in.TimeboxMinutes <= 0 {
		in.TimeboxMinutes = 60
	}
	if in.Findings == "" {
		in.Findings = "[]"
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO exploratory_sessions (product_id, test_run_id, charter, timebox_minutes, tester_id, notes, findings)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING `+cols,
			in.ProductID, in.TestRunID, in.Charter, in.TimeboxMinutes, in.TesterID, in.Notes, in.Findings)
		return scanRow(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO exploratory_sessions (product_id, test_run_id, charter, timebox_minutes, tester_id, notes, findings)
		VALUES (?,?,?,?,?,?,?)`, in.ProductID, in.TestRunID, in.Charter, in.TimeboxMinutes, in.TesterID, in.Notes, in.Findings)
	if err != nil {
		return Session{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, id)
}

func (s *Store) Update(ctx context.Context, id int64, in SessionInput) (Session, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE exploratory_sessions SET product_id=$1, test_run_id=$2, charter=$3, timebox_minutes=$4, tester_id=$5, notes=$6, findings=$7, updated_at=NOW()
			WHERE id=$8 RETURNING `+cols,
			in.ProductID, in.TestRunID, in.Charter, in.TimeboxMinutes, in.TesterID, in.Notes, in.Findings, id)
		return scanRow(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE exploratory_sessions SET product_id=?, test_run_id=?, charter=?, timebox_minutes=?, tester_id=?, notes=?, findings=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`, in.ProductID, in.TestRunID, in.Charter, in.TimeboxMinutes, in.TesterID, in.Notes, in.Findings, id)
	if err != nil {
		return Session{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM exploratory_sessions WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM exploratory_sessions WHERE id = ?`, id)
	return err
}

// ProductIDOf resolves the product of a session for access control.
func (s *Store) ProductIDOf(ctx context.Context, id int64) (int64, error) {
	var pid int64
	if s.pg != nil {
		err := s.pg.QueryRow(ctx, `SELECT product_id FROM exploratory_sessions WHERE id = $1`, id).Scan(&pid)
		return pid, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT product_id FROM exploratory_sessions WHERE id = ?`, id).Scan(&pid)
	return pid, err
}

var _ = strings.TrimSpace
