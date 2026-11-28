package milestone

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Milestone struct {
	ID              int64      `json:"id"`
	VersionID       int64      `json:"version_id"`
	Name            string     `json:"name"`
	Description     string     `json:"description,omitempty"`
	DueDate         *string    `json:"due_date,omitempty"`
	Status          string     `json:"status" enum:"planned,in_progress,done"`
	SignoffDecision *string    `json:"signoff_decision,omitempty" enum:"go,no-go"`
	SignoffUserID   *int64     `json:"signoff_user_id,omitempty"`
	SignoffNote     string     `json:"signoff_note,omitempty"`
	SignoffAt       *time.Time `json:"signoff_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Progress struct {
	MilestoneID int64  `json:"milestone_id"`
	Total       int64  `json:"total"`
	Passed      int64  `json:"passed"`
	Failed      int64  `json:"failed"`
	Blocked     int64  `json:"blocked"`
	Skipped     int64  `json:"skipped"`
	Running     int64  `json:"running"`
	Idle        int64  `json:"idle"`
	PercentDone string `json:"percent_done"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const cols = `id, version_id, name, description, due_date, status, signoff_decision, signoff_user_id, signoff_note, signoff_at, created_at, updated_at`

func scanRow(row interface{ Scan(...any) error }) (Milestone, error) {
	var m Milestone
	err := row.Scan(&m.ID, &m.VersionID, &m.Name, &m.Description, &m.DueDate, &m.Status, &m.SignoffDecision, &m.SignoffUserID, &m.SignoffNote, &m.SignoffAt, &m.CreatedAt, &m.UpdatedAt)
	return m, err
}

type MilestoneInput struct {
	VersionID   int64   `json:"version_id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	DueDate     *string `json:"due_date,omitempty"`
	Status      string  `json:"status,omitempty"`
}

func (s *Store) List(ctx context.Context, versionID int64) ([]Milestone, error) {
	q := fmt.Sprintf(`SELECT %s FROM milestones WHERE ($1 = 0 OR version_id = $1) ORDER BY id`, cols)
	qs := fmt.Sprintf(`SELECT %s FROM milestones WHERE (? = 0 OR version_id = ?) ORDER BY id`, cols)
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, q, versionID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Milestone{}
		for rows.Next() {
			m, err := scanRow(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, qs, versionID, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Milestone{}
	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id int64) (Milestone, error) {
	if s.pg != nil {
		return scanRow(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM milestones WHERE id = $1", cols), id))
	}
	return scanRow(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM milestones WHERE id = ?", cols), id))
}

func (s *Store) Create(ctx context.Context, in MilestoneInput) (Milestone, error) {
	if in.Status == "" {
		in.Status = "planned"
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO milestones (version_id, name, description, due_date, status)
			VALUES ($1,$2,$3,$4,$5) RETURNING `+cols,
			in.VersionID, in.Name, in.Description, in.DueDate, in.Status)
		return scanRow(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO milestones (version_id, name, description, due_date, status) VALUES (?,?,?,?,?)`,
		in.VersionID, in.Name, in.Description, in.DueDate, in.Status)
	if err != nil {
		return Milestone{}, err
	}
	id, _ := res.LastInsertId()
	return s.Get(ctx, id)
}

func (s *Store) Update(ctx context.Context, id int64, in MilestoneInput) (Milestone, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE milestones SET version_id=$1, name=$2, description=$3, due_date=$4, status=$5, updated_at=NOW()
			WHERE id=$6 RETURNING `+cols,
			in.VersionID, in.Name, in.Description, in.DueDate, in.Status, id)
		return scanRow(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE milestones SET version_id=?, name=?, description=?, due_date=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		in.VersionID, in.Name, in.Description, in.DueDate, in.Status, id)
	if err != nil {
		return Milestone{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM milestones WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM milestones WHERE id = ?`, id)
	return err
}

func (s *Store) Signoff(ctx context.Context, id int64, userID int64, decision, note string) (Milestone, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE milestones SET signoff_decision=$1, signoff_user_id=$2, signoff_note=$3, signoff_at=NOW(), updated_at=NOW()
			WHERE id=$4 RETURNING `+cols, decision, userID, note, id)
		return scanRow(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE milestones SET signoff_decision=?, signoff_user_id=?, signoff_note=?, signoff_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		decision, userID, note, id)
	if err != nil {
		return Milestone{}, err
	}
	return s.Get(ctx, id)
}

func (s *Store) LinkRun(ctx context.Context, milestoneID, runID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `INSERT INTO milestone_runs (milestone_id, test_run_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, milestoneID, runID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO milestone_runs (milestone_id, test_run_id) VALUES (?, ?)`, milestoneID, runID)
	return err
}

func (s *Store) UnlinkRun(ctx context.Context, milestoneID, runID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM milestone_runs WHERE milestone_id = $1 AND test_run_id = $2`, milestoneID, runID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM milestone_runs WHERE milestone_id = ? AND test_run_id = ?`, milestoneID, runID)
	return err
}

func (s *Store) Runs(ctx context.Context, milestoneID int64) ([]int64, error) {
	q := `SELECT test_run_id FROM milestone_runs WHERE milestone_id = ? ORDER BY test_run_id`
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, milestoneID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []int64{}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			out = append(out, id)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, milestoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) Progress(ctx context.Context, milestoneID int64) (Progress, error) {
	p := Progress{MilestoneID: milestoneID}
	q := `SELECT status, COUNT(*) FROM test_executions te
		JOIN milestone_runs mr ON mr.test_run_id = te.test_run_id
		WHERE mr.milestone_id = ? GROUP BY status`
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, milestoneID)
		if err != nil {
			return p, err
		}
		defer rows.Close()
		for rows.Next() {
			var status string
			var n int64
			if err := rows.Scan(&status, &n); err != nil {
				return p, err
			}
			p.Total += n
			switch status {
			case "passed":
				p.Passed = n
			case "failed":
				p.Failed = n
			case "blocked":
				p.Blocked = n
			case "skipped":
				p.Skipped = n
			case "running":
				p.Running = n
			default:
				p.Idle = n
			}
		}
		if err := rows.Err(); err != nil {
			return p, err
		}
	} else {
		rows, err := s.db.QueryContext(ctx, q, milestoneID)
		if err != nil {
			return p, err
		}
		defer rows.Close()
		for rows.Next() {
			var status string
			var n int64
			if err := rows.Scan(&status, &n); err != nil {
				return p, err
			}
			p.Total += n
			switch status {
			case "passed":
				p.Passed = n
			case "failed":
				p.Failed = n
			case "blocked":
				p.Blocked = n
			case "skipped":
				p.Skipped = n
			case "running":
				p.Running = n
			default:
				p.Idle = n
			}
		}
		if err := rows.Err(); err != nil {
			return p, err
		}
	}
	if p.Total > 0 {
		pct := float64(p.Passed+p.Failed+p.Blocked+p.Skipped) / float64(p.Total) * 100
		p.PercentDone = fmt.Sprintf("%.0f%%", pct)
	} else {
		p.PercentDone = "0%"
	}
	return p, nil
}
