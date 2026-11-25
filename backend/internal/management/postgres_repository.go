package management

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements ProductRepository using pgxpool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, description, created_at FROM products ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer rows.Close()

	items := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateProduct(ctx context.Context, name, description string) (Product, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO products (name, description, created_at, updated_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		name, description, now, now,
	).Scan(&id)
	if err != nil {
		return Product{}, fmt.Errorf("insert product: %w", err)
	}
	return Product{ID: id, Name: name, Description: description, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, product_id, value, created_at FROM versions WHERE product_id = $1 ORDER BY id ASC`, productID)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	items := make([]Version, 0)
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Value, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		items = append(items, v)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateVersion(ctx context.Context, productID int64, value string) (Version, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO versions (product_id, value, created_at) VALUES ($1, $2, $3) RETURNING id`,
		productID, value, now,
	).Scan(&id)
	if err != nil {
		return Version{}, fmt.Errorf("insert version: %w", err)
	}
	return Version{ID: id, ProductID: productID, Value: value, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListBuilds(ctx context.Context, versionID int64) ([]Build, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, version_id, name, is_active, created_at FROM builds WHERE version_id = $1 ORDER BY id ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("query builds: %w", err)
	}
	defer rows.Close()

	items := make([]Build, 0)
	for rows.Next() {
		var b Build
		if err := rows.Scan(&b.ID, &b.VersionID, &b.Name, &b.IsActive, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan build: %w", err)
		}
		items = append(items, b)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateBuild(ctx context.Context, versionID int64, name string) (Build, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO builds (version_id, name, is_active, created_at) VALUES ($1, $2, TRUE, $3) RETURNING id`,
		versionID, name, now,
	).Scan(&id)
	if err != nil {
		return Build{}, fmt.Errorf("insert build: %w", err)
	}
	return Build{ID: id, VersionID: versionID, Name: name, IsActive: true, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListComponents(ctx context.Context, productID int64) ([]Component, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, product_id, name, description, created_at FROM components WHERE product_id = $1 ORDER BY id ASC`, productID)
	if err != nil {
		return nil, fmt.Errorf("query components: %w", err)
	}
	defer rows.Close()

	items := make([]Component, 0)
	for rows.Next() {
		var c Component
		if err := rows.Scan(&c.ID, &c.ProductID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan component: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateComponent(ctx context.Context, productID int64, name, description string) (Component, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO components (product_id, name, description, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		productID, name, description, now,
	).Scan(&id)
	if err != nil {
		return Component{}, fmt.Errorf("insert component: %w", err)
	}
	return Component{ID: id, ProductID: productID, Name: name, Description: description, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, created_at FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	items := make([]Tag, 0)
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateTag(ctx context.Context, name string) (Tag, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO tags (name, created_at) VALUES ($1, $2) RETURNING id`, name, now).Scan(&id)
	if err != nil {
		return Tag{}, fmt.Errorf("insert tag: %w", err)
	}
	return Tag{ID: id, Name: name, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, created_at FROM categories ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer rows.Close()

	items := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateCategory(ctx context.Context, name string) (Category, error) {
	now := time.Now().UTC()
	var id int64
	err := r.pool.QueryRow(ctx, `INSERT INTO categories (name, created_at) VALUES ($1, $2) RETURNING id`, name, now).Scan(&id)
	if err != nil {
		return Category{}, fmt.Errorf("insert category: %w", err)
	}
	return Category{ID: id, Name: name, CreatedAt: now}, nil
}

func (r *PostgresRepository) AddMember(ctx context.Context, productID, userID int64, role string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO product_members (product_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (product_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		productID, userID, role, time.Now().UTC(),
	)
	return err
}

func (r *PostgresRepository) HasAccess(ctx context.Context, productID, userID int64) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM product_members WHERE product_id = $1 AND user_id = $2`,
		productID, userID,
	).Scan(&count)
	return count > 0, err
}

func (r *PostgresRepository) ListPriorities(ctx context.Context) ([]Priority, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, value, is_active, created_at FROM priorities ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Priority, 0)
	for rows.Next() {
		var p Priority
		if err := rows.Scan(&p.ID, &p.Value, &p.IsActive, &p.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreatePriority(ctx context.Context, value string) (Priority, error) {
	now := time.Now().UTC()
	var id int64
	if err := r.pool.QueryRow(ctx, `INSERT INTO priorities (value, is_active, created_at) VALUES ($1, TRUE, $2) RETURNING id`, value, now).Scan(&id); err != nil {
		return Priority{}, err
	}
	return Priority{ID: id, Value: value, IsActive: true, CreatedAt: now}, nil
}

func (r *PostgresRepository) ListExecutionStatuses(ctx context.Context) ([]ExecutionStatus, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, color, weight, created_at FROM execution_statuses ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ExecutionStatus, 0)
	for rows.Next() {
		var s ExecutionStatus
		if err := rows.Scan(&s.ID, &s.Name, &s.Color, &s.Weight, &s.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) CreateExecutionStatus(ctx context.Context, name, color string, weight int) (ExecutionStatus, error) {
	now := time.Now().UTC()
	var id int64
	if err := r.pool.QueryRow(ctx, `INSERT INTO execution_statuses (name, color, weight, created_at) VALUES ($1, $2, $3, $4) RETURNING id`, name, color, weight, now).Scan(&id); err != nil {
		return ExecutionStatus{}, err
	}
	return ExecutionStatus{ID: id, Name: name, Color: color, Weight: weight, CreatedAt: now}, nil
}
