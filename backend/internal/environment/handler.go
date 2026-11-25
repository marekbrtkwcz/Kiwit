package environment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Environment struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]string `json:"properties"`
	CreatedAt   time.Time         `json:"created_at"`
}

type Store struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

func NewStore(db *sql.DB, pgPool *pgxpool.Pool) *Store {
	return &Store{db: db, pgPool: pgPool}
}

func RegisterRoutes(api huma.API, s *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "list-environments",
		Method:      http.MethodGet,
		Path:        "/api/v1/environments",
		Summary:     "List test environments with properties",
		Tags:        []string{"Environments"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Environment `json:"items"`
		}
	}, error) {
		items, err := s.list(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list environments", err)
		}
		out := &struct {
			Body struct {
				Items []Environment `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-environment",
		Method:      http.MethodPost,
		Path:        "/api/v1/environments",
		Summary:     "Create test environment",
		Tags:        []string{"Environments"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name        string            `json:"name" minLength:"1"`
			Description string            `json:"description,omitempty"`
			Properties  map[string]string `json:"properties,omitempty"`
		}
	}) (*struct{ Body Environment }, error) {
		env, err := s.create(ctx, input.Body.Name, input.Body.Description, input.Body.Properties)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create environment", err)
		}
		return &struct{ Body Environment }{Body: env}, nil
	})
}

func (s *Store) list(ctx context.Context) ([]Environment, error) {
	items := make([]Environment, 0)
	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx, `SELECT id, name, description, properties, created_at FROM environments ORDER BY id ASC`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var e Environment
			var raw []byte
			if err := rows.Scan(&e.ID, &e.Name, &e.Description, &raw, &e.CreatedAt); err != nil {
				return nil, err
			}
			_ = json.Unmarshal(raw, &e.Properties)
			if e.Properties == nil {
				e.Properties = map[string]string{}
			}
			items = append(items, e)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, properties, created_at FROM environments ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var e Environment
		var raw string
		if err := rows.Scan(&e.ID, &e.Name, &e.Description, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &e.Properties)
		if e.Properties == nil {
			e.Properties = map[string]string{}
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

func (s *Store) create(ctx context.Context, name, desc string, props map[string]string) (Environment, error) {
	if props == nil {
		props = map[string]string{}
	}
	raw, _ := json.Marshal(props)
	now := time.Now().UTC()
	if s.pgPool != nil {
		var id int64
		err := s.pgPool.QueryRow(ctx,
			`INSERT INTO environments (name, description, properties, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
			name, desc, raw, now,
		).Scan(&id)
		if err != nil {
			return Environment{}, fmt.Errorf("insert environment: %w", err)
		}
		return Environment{ID: id, Name: name, Description: desc, Properties: props, CreatedAt: now}, nil
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO environments (name, description, properties, created_at) VALUES (?, ?, ?, ?)`,
		name, desc, string(raw), now,
	)
	if err != nil {
		return Environment{}, fmt.Errorf("insert environment: %w", err)
	}
	id, _ := res.LastInsertId()
	return Environment{ID: id, Name: name, Description: desc, Properties: props, CreatedAt: now}, nil
}
