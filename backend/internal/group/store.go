package group

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Group struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Members   []int64   `json:"members,omitempty"`
}

type Store struct {
	db  *sql.DB
	pg  *pgxpool.Pool
}

func NewStore(db *sql.DB, pg *pgxpool.Pool) *Store {
	return &Store{db: db, pg: pg}
}

func (s *Store) List(ctx context.Context) ([]Group, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT id, name, created_at FROM user_groups ORDER BY id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []Group
		for rows.Next() {
			var g Group
			if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
				return nil, err
			}
			out = append(out, g)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at FROM user_groups ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) Create(ctx context.Context, name string) (Group, error) {
	if s.pg != nil {
		var g Group
		err := s.pg.QueryRow(ctx, `INSERT INTO user_groups (name) VALUES ($1) RETURNING id, name, created_at`, name).
			Scan(&g.ID, &g.Name, &g.CreatedAt)
		return g, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO user_groups (name) VALUES (?)`, name)
	if err != nil {
		return Group{}, err
	}
	id, _ := res.LastInsertId()
	g := Group{ID: id, Name: name, CreatedAt: time.Now().UTC()}
	return g, nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM user_groups WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_groups WHERE id = ?`, id)
	return err
}

func (s *Store) Members(ctx context.Context, groupID int64) ([]int64, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT user_id FROM user_group_members WHERE group_id = $1 ORDER BY user_id`, groupID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err != nil {
				return nil, err
			}
			out = append(out, uid)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM user_group_members WHERE group_id = ? ORDER BY user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, rows.Err()
}

func (s *Store) AddMember(ctx context.Context, groupID, userID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `INSERT INTO user_group_members (group_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, groupID, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO user_group_members (group_id, user_id) VALUES (?, ?)`, groupID, userID)
	return err
}

func (s *Store) RemoveMember(ctx context.Context, groupID, userID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM user_group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM user_group_members WHERE group_id = ? AND user_id = ?`, groupID, userID)
	return err
}

// --- Group product permissions ---

type GroupProductPerm struct {
	GroupID   int64  `json:"group_id"`
	ProductID int64  `json:"product_id"`
	Role      string `json:"role"`
}

func (s *Store) ListProductPerms(ctx context.Context, groupID int64) ([]GroupProductPerm, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT group_id, product_id, role FROM group_product_permissions WHERE group_id = $1 ORDER BY product_id`, groupID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []GroupProductPerm
		for rows.Next() {
			var p GroupProductPerm
			if err := rows.Scan(&p.GroupID, &p.ProductID, &p.Role); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT group_id, product_id, role FROM group_product_permissions WHERE group_id = ? ORDER BY product_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupProductPerm
	for rows.Next() {
		var p GroupProductPerm
		if err := rows.Scan(&p.GroupID, &p.ProductID, &p.Role); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) SetProductPerm(ctx context.Context, groupID, productID int64, role string) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `
			INSERT INTO group_product_permissions (group_id, product_id, role) VALUES ($1, $2, $3)
			ON CONFLICT (group_id, product_id) DO UPDATE SET role = EXCLUDED.role`, groupID, productID, role)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO group_product_permissions (group_id, product_id, role) VALUES (?, ?, ?)
		ON CONFLICT (group_id, product_id) DO UPDATE SET role = excluded.role`, groupID, productID, role)
	return err
}

func (s *Store) DeleteProductPerm(ctx context.Context, groupID, productID int64) error {
	if s.pg != nil {
		_, err := s.pg.Exec(ctx, `DELETE FROM group_product_permissions WHERE group_id = $1 AND product_id = $2`, groupID, productID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM group_product_permissions WHERE group_id = ? AND product_id = ?`, groupID, productID)
	return err
}

// GroupsForUser returns IDs of groups the user belongs to.
func (s *Store) GroupsForUser(ctx context.Context, userID int64) ([]int64, error) {
	if s.pg != nil {
		rows, err := s.pg.Query(ctx, `SELECT group_id FROM user_group_members WHERE user_id = $1`, userID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []int64
		for rows.Next() {
			var gid int64
			if err := rows.Scan(&gid); err != nil {
				return nil, err
			}
			out = append(out, gid)
		}
		return out, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `SELECT group_id FROM user_group_members WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var gid int64
		if err := rows.Scan(&gid); err != nil {
			return nil, err
		}
		out = append(out, gid)
	}
	return out, rows.Err()
}
