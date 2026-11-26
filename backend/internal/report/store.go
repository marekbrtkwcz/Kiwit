package report

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MilestoneReport struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Version         string  `json:"version"`
	ProductName     string  `json:"product_name"`
	Status          string  `json:"status"`
	DueDate         *string `json:"due_date,omitempty"`
	SignoffDecision *string `json:"signoff_decision,omitempty"`
	SignoffAt       *string `json:"signoff_at,omitempty"`
	Total           int64   `json:"total_executions"`
	Passed          int64   `json:"passed"`
	Failed          int64   `json:"failed"`
	Blocked         int64   `json:"blocked"`
	Skipped         int64   `json:"skipped"`
	Idle            int64   `json:"idle"`
	Running         int64   `json:"running"`
	OpenBugs        int64   `json:"open_bugs"`
	Runs            []int64 `json:"runs"`
}

type MultiReport struct {
	GeneratedAt string            `json:"generated_at"`
	Milestones  []MilestoneReport `json:"milestones"`
	Totals      MilestoneReport   `json:"totals"`
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

// Build aggregates a multi-milestone (multi-campaign) report.
func (s *Store) Build(ctx context.Context, milestoneIDs []int64) (MultiReport, error) {
	rep := MultiReport{Milestones: []MilestoneReport{}}
	if len(milestoneIDs) == 0 {
		return rep, nil
	}
	ph := func(i int) string { return "?" }
	if s.pg != nil {
		ph = func(i int) string { return fmt.Sprintf("$%d", i) }
	}
	args := make([]any, len(milestoneIDs))
	placeholders := make([]string, len(milestoneIDs))
	for i, id := range milestoneIDs {
		args[i] = id
		placeholders[i] = ph(i + 1)
	}
	in := strings.Join(placeholders, ",")

	q := fmt.Sprintf(`SELECT m.id, m.name, COALESCE(v.value, ''), COALESCE(p.name, ''), m.status, m.due_date,
			m.signoff_decision, m.signoff_at,
			COALESCE(SUM(CASE WHEN te.status = 'passed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN te.status = 'failed' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN te.status = 'blocked' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN te.status = 'skipped' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN te.status = 'idle' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN te.status = 'running' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN te.status IS NULL THEN 0 ELSE 1 END), 0),
			COALESCE((SELECT COUNT(*) FROM bugs b JOIN test_executions te2 ON te2.id = b.test_execution_id
			          JOIN milestone_runs mr2 ON mr2.test_run_id = te2.test_run_id
			          WHERE mr2.milestone_id = m.id AND b.status != 'closed'), 0)
		FROM milestones m
		JOIN versions v ON v.id = m.version_id
		JOIN products p ON p.id = v.product_id
		LEFT JOIN milestone_runs mr ON mr.milestone_id = m.id
		LEFT JOIN test_executions te ON te.test_run_id = mr.test_run_id
		WHERE m.id IN (%s)
		GROUP BY m.id, m.name, v.value, p.name, m.status, m.due_date, m.signoff_decision, m.signoff_at
		ORDER BY m.id`, in)

	rows := map[int64]*MilestoneReport{}

	if s.pg != nil {
		rs, err := s.pg.Query(ctx, q, args...)
		if err != nil {
			return rep, err
		}
		for rs.Next() {
			var m MilestoneReport
			var signoffAt any
			if err := rs.Scan(&m.ID, &m.Name, &m.Version, &m.ProductName, &m.Status, &m.DueDate, &m.SignoffDecision, &signoffAt,
				&m.Passed, &m.Failed, &m.Blocked, &m.Skipped, &m.Idle, &m.Running, &m.Total, &m.OpenBugs); err != nil {
				rs.Close()
				return rep, err
			}
			if t, ok := signoffAt.(interface{ String() string }); ok && signoffAt != nil {
				s := t.String()
				m.SignoffAt = &s
			}
			rows[m.ID] = &m
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return rep, err
		}
		// simpler: per-milestone run lists
		for _, id := range milestoneIDs {
			r2, err := s.pg.Query(ctx, `SELECT test_run_id FROM milestone_runs WHERE milestone_id = $1 ORDER BY test_run_id`, id)
			if err != nil {
				return rep, err
			}
			for r2.Next() {
				var rid int64
				if err := r2.Scan(&rid); err == nil && rows[id] != nil {
					rows[id].Runs = append(rows[id].Runs, rid)
				}
			}
			r2.Close()
		}
	} else {
		rs, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return rep, err
		}
		for rs.Next() {
			var m MilestoneReport
			var signoffAt any
			if err := rs.Scan(&m.ID, &m.Name, &m.Version, &m.ProductName, &m.Status, &m.DueDate, &m.SignoffDecision, &signoffAt,
				&m.Passed, &m.Failed, &m.Blocked, &m.Skipped, &m.Idle, &m.Running, &m.Total, &m.OpenBugs); err != nil {
				rs.Close()
				return rep, err
			}
			if signoffAt != nil {
				if s2, ok := signoffAt.(string); ok {
					m.SignoffAt = &s2
				}
			}
			rows[m.ID] = &m
		}
		rs.Close()
		if err := rs.Err(); err != nil {
			return rep, err
		}
		for _, id := range milestoneIDs {
			r2, err := s.db.QueryContext(ctx, `SELECT test_run_id FROM milestone_runs WHERE milestone_id = ? ORDER BY test_run_id`, id)
			if err != nil {
				return rep, err
			}
			for r2.Next() {
				var rid int64
				if err := r2.Scan(&rid); err == nil && rows[id] != nil {
					rows[id].Runs = append(rows[id].Runs, rid)
				}
			}
			r2.Close()
		}
	}

	for _, id := range milestoneIDs {
		if m, ok := rows[id]; ok {
			rep.Milestones = append(rep.Milestones, *m)
			rep.Totals.Total += m.Total
			rep.Totals.Passed += m.Passed
			rep.Totals.Failed += m.Failed
			rep.Totals.Blocked += m.Blocked
			rep.Totals.Skipped += m.Skipped
			rep.Totals.Idle += m.Idle
			rep.Totals.Running += m.Running
			rep.Totals.OpenBugs += m.OpenBugs
		}
	}
	return rep, nil
}
