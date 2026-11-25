package management

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ProductRepository defines persistence operations for products, versions, builds, components, and tags.
type ProductRepository interface {
	ListProducts(ctx context.Context) ([]Product, error)
	CreateProduct(ctx context.Context, name, description string) (Product, error)
	ListVersions(ctx context.Context, productID int64) ([]Version, error)
	CreateVersion(ctx context.Context, productID int64, value string) (Version, error)
	ListBuilds(ctx context.Context, versionID int64) ([]Build, error)
	CreateBuild(ctx context.Context, versionID int64, name string) (Build, error)
	ListComponents(ctx context.Context, productID int64) ([]Component, error)
	CreateComponent(ctx context.Context, productID int64, name, description string) (Component, error)
	ListTags(ctx context.Context) ([]Tag, error)
	CreateTag(ctx context.Context, name string) (Tag, error)
	ListCategories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, name string) (Category, error)
}

// SQLiteRepository implements ProductRepository using database/sql (SQLite).
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a SQLite-backed ProductRepository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, description, created_at FROM products ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query products: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate products: %w", err)
	}
	return items, nil
}

func (r *SQLiteRepository) CreateProduct(ctx context.Context, name, description string) (Product, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO products (name, description, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		name, description, now, now,
	)
	if err != nil {
		return Product{}, fmt.Errorf("insert product: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Product{}, fmt.Errorf("last insert id: %w", err)
	}
	return Product{
		ID:          id,
		Name:        name,
		Description: description,
		CreatedAt:   now,
	}, nil
}

func (r *SQLiteRepository) ListVersions(ctx context.Context, productID int64) ([]Version, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, value, created_at FROM versions WHERE product_id = ? ORDER BY id ASC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) CreateVersion(ctx context.Context, productID int64, value string) (Version, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO versions (product_id, value, created_at) VALUES (?, ?, ?)`,
		productID, value, now,
	)
	if err != nil {
		return Version{}, fmt.Errorf("insert version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Version{}, fmt.Errorf("last insert id: %w", err)
	}
	return Version{ID: id, ProductID: productID, Value: value, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListBuilds(ctx context.Context, versionID int64) ([]Build, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, version_id, name, is_active, created_at FROM builds WHERE version_id = ? ORDER BY id ASC`,
		versionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query builds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]Build, 0)
	for rows.Next() {
		var b Build
		var active int
		if err := rows.Scan(&b.ID, &b.VersionID, &b.Name, &active, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan build: %w", err)
		}
		b.IsActive = active == 1
		items = append(items, b)
	}
	return items, rows.Err()
}

func (r *SQLiteRepository) CreateBuild(ctx context.Context, versionID int64, name string) (Build, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO builds (version_id, name, is_active, created_at) VALUES (?, ?, 1, ?)`,
		versionID, name, now,
	)
	if err != nil {
		return Build{}, fmt.Errorf("insert build: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Build{}, fmt.Errorf("last insert id: %w", err)
	}
	return Build{ID: id, VersionID: versionID, Name: name, IsActive: true, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListComponents(ctx context.Context, productID int64) ([]Component, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, name, description, created_at FROM components WHERE product_id = ? ORDER BY id ASC`,
		productID,
	)
	if err != nil {
		return nil, fmt.Errorf("query components: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) CreateComponent(ctx context.Context, productID int64, name, description string) (Component, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO components (product_id, name, description, created_at) VALUES (?, ?, ?, ?)`,
		productID, name, description, now,
	)
	if err != nil {
		return Component{}, fmt.Errorf("insert component: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Component{}, fmt.Errorf("last insert id: %w", err)
	}
	return Component{ID: id, ProductID: productID, Name: name, Description: description, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, created_at FROM tags ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) CreateTag(ctx context.Context, name string) (Tag, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `INSERT INTO tags (name, created_at) VALUES (?, ?)`, name, now)
	if err != nil {
		return Tag{}, fmt.Errorf("insert tag: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Tag{}, fmt.Errorf("last insert id: %w", err)
	}
	return Tag{ID: id, Name: name, CreatedAt: now}, nil
}

func (r *SQLiteRepository) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, created_at FROM categories ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *SQLiteRepository) CreateCategory(ctx context.Context, name string) (Category, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `INSERT INTO categories (name, created_at) VALUES (?, ?)`, name, now)
	if err != nil {
		return Category{}, fmt.Errorf("insert category: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, fmt.Errorf("last insert id: %w", err)
	}
	return Category{ID: id, Name: name, CreatedAt: now}, nil
}
