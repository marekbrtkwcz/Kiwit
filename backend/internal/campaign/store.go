package campaign

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Campaign struct {
	ID          int64     `json:"id"`
	ProductID   int64     `json:"product_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status" enum:"active,completed,archived"`
	StartDate   *string   `json:"start_date,omitempty"`
	EndDate     *string   `json:"end_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Iteration struct {
	ID         int64     `json:"id"`
	CampaignID int64     `json:"campaign_id"`
	Name       string    `json:"name"`
	Status     string    `json:"status" enum:"planned,in_progress,done"`
	StartDate  *string   `json:"start_date,omitempty"`
	EndDate    *string   `json:"end_date,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PlanNode struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	IsActive    bool   `json:"is_active"`
	IterationID *int64 `json:"iteration_id,omitempty"`
}

type IterationNode struct {
	Iteration
	Plans []PlanNode `json:"plans"`
}

type CampaignTree struct {
	Campaign
	Iterations []IterationNode `json:"iterations"`
	Plans      []PlanNode      `json:"plans"` // plans directly under campaign (no iteration)
}

type Store struct {
	db *sql.DB
	pg *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

const campaignCols = `id, product_id, name, description, status, start_date, end_date, created_at, updated_at`
const iterationCols = `id, campaign_id, name, status, start_date, end_date, created_at, updated_at`

func scanCampaign(row interface{ Scan(...any) error }) (Campaign, error) {
	var c Campaign
	err := row.Scan(&c.ID, &c.ProductID, &c.Name, &c.Description, &c.Status, &c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func scanIteration(row interface{ Scan(...any) error }) (Iteration, error) {
	var it Iteration
	err := row.Scan(&it.ID, &it.CampaignID, &it.Name, &it.Status, &it.StartDate, &it.EndDate, &it.CreatedAt, &it.UpdatedAt)
	return it, err
}

type CampaignInput struct {
	ProductID   int64   `json:"product_id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status,omitempty"`
	StartDate   *string `json:"start_date,omitempty"`
	EndDate     *string `json:"end_date,omitempty"`
}

type IterationInput struct {
	Name      string  `json:"name"`
	Status    string  `json:"status,omitempty"`
	StartDate *string `json:"start_date,omitempty"`
	EndDate   *string `json:"end_date,omitempty"`
}

func (s *Store) ListCampaigns(ctx context.Context, productID int64) ([]Campaign, error) {
	q := fmt.Sprintf(`SELECT %s FROM campaigns WHERE ($1 = 0 OR product_id = $1) ORDER BY id`, campaignCols)
	qs := fmt.Sprintf(`SELECT %s FROM campaigns WHERE (? = 0 OR product_id = ?) ORDER BY id`, campaignCols)
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, q, productID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Campaign{}
		for rows.Next() {
			c, err := scanCampaign(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, qs, productID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Campaign{}
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetCampaign(ctx context.Context, id int64) (Campaign, error) {
	if s.pg != nil {
		return scanCampaign(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM campaigns WHERE id = $1", campaignCols), id))
	}
	return scanCampaign(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM campaigns WHERE id = ?", campaignCols), id))
}

func (s *Store) CreateCampaign(ctx context.Context, in CampaignInput) (Campaign, error) {
	if in.Status == "" {
		in.Status = "active"
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO campaigns (product_id, name, description, status, start_date, end_date) VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+campaignCols,
			in.ProductID, in.Name, in.Description, in.Status, in.StartDate, in.EndDate)
		return scanCampaign(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO campaigns (product_id, name, description, status, start_date, end_date) VALUES (?,?,?,?,?,?)`,
		in.ProductID, in.Name, in.Description, in.Status, in.StartDate, in.EndDate)
	if err != nil {
		return Campaign{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetCampaign(ctx, id)
}

func (s *Store) UpdateCampaign(ctx context.Context, id int64, in CampaignInput) (Campaign, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE campaigns SET product_id=$1, name=$2, description=$3, status=$4, start_date=$5, end_date=$6, updated_at=NOW() WHERE id=$7 RETURNING `+campaignCols,
			in.ProductID, in.Name, in.Description, in.Status, in.StartDate, in.EndDate, id)
		return scanCampaign(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE campaigns SET product_id=?, name=?, description=?, status=?, start_date=?, end_date=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		in.ProductID, in.Name, in.Description, in.Status, in.StartDate, in.EndDate, id)
	if err != nil {
		return Campaign{}, err
	}
	return s.GetCampaign(ctx, id)
}

func (s *Store) DeleteCampaign(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM campaigns WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM campaigns WHERE id = ?`, id)
	return err
}

func (s *Store) ListIterations(ctx context.Context, campaignID int64) ([]Iteration, error) {
	q := fmt.Sprintf(`SELECT %s FROM campaign_iterations WHERE campaign_id = ? ORDER BY id`, iterationCols)
	if s.pg != nil {
		q = strings.ReplaceAll(q, "?", "$1")
		rows, err := s.pg.Query(ctx, q, campaignID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Iteration{}
		for rows.Next() {
			it, err := scanIteration(rows)
			if err != nil {
				return nil, err
			}
			out = append(out, it)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, q, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Iteration{}
	for rows.Next() {
		it, err := scanIteration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) GetIteration(ctx context.Context, id int64) (Iteration, error) {
	if s.pg != nil {
		return scanIteration(s.pg.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM campaign_iterations WHERE id = $1", iterationCols), id))
	}
	return scanIteration(s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM campaign_iterations WHERE id = ?", iterationCols), id))
}

func (s *Store) CreateIteration(ctx context.Context, campaignID int64, in IterationInput) (Iteration, error) {
	if in.Status == "" {
		in.Status = "planned"
	}
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `INSERT INTO campaign_iterations (campaign_id, name, status, start_date, end_date) VALUES ($1,$2,$3,$4,$5) RETURNING `+iterationCols,
			campaignID, in.Name, in.Status, in.StartDate, in.EndDate)
		return scanIteration(row)
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO campaign_iterations (campaign_id, name, status, start_date, end_date) VALUES (?,?,?,?,?)`,
		campaignID, in.Name, in.Status, in.StartDate, in.EndDate)
	if err != nil {
		return Iteration{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetIteration(ctx, id)
}

func (s *Store) UpdateIteration(ctx context.Context, id int64, in IterationInput) (Iteration, error) {
	if s.pg != nil {
		row := s.pg.QueryRow(ctx, `UPDATE campaign_iterations SET name=$1, status=$2, start_date=$3, end_date=$4, updated_at=NOW() WHERE id=$5 RETURNING `+iterationCols,
			in.Name, in.Status, in.StartDate, in.EndDate, id)
		return scanIteration(row)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE campaign_iterations SET name=?, status=?, start_date=?, end_date=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		in.Name, in.Status, in.StartDate, in.EndDate, id)
	if err != nil {
		return Iteration{}, err
	}
	return s.GetIteration(ctx, id)
}

func (s *Store) DeleteIteration(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM campaign_iterations WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM campaign_iterations WHERE id = ?`, id)
	return err
}

// AssignPlan links a test plan to a campaign and/or iteration.
func (s *Store) AssignPlan(ctx context.Context, planID int64, campaignID, iterationID *int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `UPDATE test_plans SET campaign_id=$1, iteration_id=$2 WHERE id=$3`, campaignID, iterationID, planID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE test_plans SET campaign_id=?, iteration_id=? WHERE id=?`, campaignID, iterationID, planID)
	return err
}

// Tree builds campaign → iterations → plans hierarchy.
func (s *Store) Tree(ctx context.Context, campaignID int64) (CampaignTree, error) {
	c, err := s.GetCampaign(ctx, campaignID)
	if err != nil {
		return CampaignTree{}, err
	}
	tree := CampaignTree{Campaign: c, Iterations: []IterationNode{}, Plans: []PlanNode{}}
	iters, err := s.ListIterations(ctx, campaignID)
	if err != nil {
		return tree, err
	}
	for _, it := range iters {
		tree.Iterations = append(tree.Iterations, IterationNode{Iteration: it, Plans: []PlanNode{}})
	}
	planQ := `SELECT id, name, is_active, iteration_id FROM test_plans WHERE campaign_id = ? ORDER BY id`
	if s.pg != nil {
		planQ = strings.ReplaceAll(planQ, "?", "$1")
		r, err := s.pg.Query(ctx, planQ, campaignID)
		if err != nil {
			return tree, err
		}
		defer r.Close()
		for r.Next() {
			var p PlanNode
			if err := r.Scan(&p.ID, &p.Name, &p.IsActive, &p.IterationID); err != nil {
				return tree, err
			}
			addPlan(&tree, p)
		}
		return tree, r.Err()
	}
	r, err := s.db.QueryContext(ctx, planQ, campaignID)
	if err != nil {
		return tree, err
	}
	defer r.Close()
	for r.Next() {
		var p PlanNode
		if err := r.Scan(&p.ID, &p.Name, &p.IsActive, &p.IterationID); err != nil {
			return tree, err
		}
		addPlan(&tree, p)
	}
	return tree, r.Err()
}

func addPlan(tree *CampaignTree, p PlanNode) {
	if p.IterationID != nil {
		for i := range tree.Iterations {
			if tree.Iterations[i].ID == *p.IterationID {
				tree.Iterations[i].Plans = append(tree.Iterations[i].Plans, p)
				return
			}
		}
	}
	tree.Plans = append(tree.Plans, p)
}

// CampaignProductID resolves product of a campaign for access control.
func (s *Store) CampaignProductID(ctx context.Context, id int64) (int64, error) {
	var pid int64
	if s.pg != nil {
		err := s.pg.QueryRow(ctx, `SELECT product_id FROM campaigns WHERE id = $1`, id).Scan(&pid)
		return pid, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT product_id FROM campaigns WHERE id = ?`, id).Scan(&pid)
	return pid, err
}
