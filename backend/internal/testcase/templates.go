package testcase

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Template struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func RegisterTemplateRoutes(api huma.API, db *sql.DB, pgPool *pgxpool.Pool) {
	huma.Register(api, huma.Operation{
		OperationID: "list-templates",
		Method:      http.MethodGet,
		Path:        "/api/v1/templates",
		Summary:     "List test case templates",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Template `json:"items"`
		}
	}, error) {
		items := make([]Template, 0)
		if pgPool != nil {
			rows, err := pgPool.Query(ctx, `SELECT id, name, body, created_at FROM templates ORDER BY id ASC`)
			if err != nil {
				return nil, huma.Error500InternalServerError("failed to list templates", err)
			}
			defer rows.Close()
			for rows.Next() {
				var t Template
				if err := rows.Scan(&t.ID, &t.Name, &t.Body, &t.CreatedAt); err != nil {
					return nil, huma.Error500InternalServerError("failed to scan template", err)
				}
				items = append(items, t)
			}
		} else {
			rows, err := db.QueryContext(ctx, `SELECT id, name, body, created_at FROM templates ORDER BY id ASC`)
			if err != nil {
				return nil, huma.Error500InternalServerError("failed to list templates", err)
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				var t Template
				if err := rows.Scan(&t.ID, &t.Name, &t.Body, &t.CreatedAt); err != nil {
					return nil, huma.Error500InternalServerError("failed to scan template", err)
				}
				items = append(items, t)
			}
		}
		out := &struct {
			Body struct {
				Items []Template `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-template",
		Method:      http.MethodPost,
		Path:        "/api/v1/templates",
		Summary:     "Create test case template",
		Tags:        []string{"TestCases"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name string `json:"name" minLength:"1"`
			Body string `json:"body" minLength:"1"`
		}
	}) (*struct{ Body Template }, error) {
		now := time.Now().UTC()
		var id int64
		if pgPool != nil {
			if err := pgPool.QueryRow(ctx, `INSERT INTO templates (name, body, created_at) VALUES ($1, $2, $3) RETURNING id`, input.Body.Name, input.Body.Body, now).Scan(&id); err != nil {
				return nil, huma.Error409Conflict("failed to create template", err)
			}
		} else {
			res, err := db.ExecContext(ctx, `INSERT INTO templates (name, body, created_at) VALUES (?, ?, ?)`, input.Body.Name, input.Body.Body, now)
			if err != nil {
				return nil, huma.Error409Conflict("failed to create template", err)
			}
			id, _ = res.LastInsertId()
		}
		return &struct{ Body Template }{Body: Template{ID: id, Name: input.Body.Name, Body: input.Body.Body, CreatedAt: now}}, nil
	})
}
