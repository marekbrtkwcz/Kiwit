package management

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwit/kiwit/internal/auth"
)

type RelationStore struct {
	db     *sql.DB
	pgPool *pgxpool.Pool
}

func NewRelationStore(db *sql.DB, pgPool *pgxpool.Pool) *RelationStore {
	return &RelationStore{db: db, pgPool: pgPool}
}

func (s *RelationStore) AttachTag(ctx context.Context, table, fkCol string, entityID, tagID int64) error {
	if s.pgPool != nil {
		_, err := s.pgPool.Exec(ctx,
			"INSERT INTO "+table+" ("+fkCol+", tag_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			entityID, tagID,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO "+table+" ("+fkCol+", tag_id) VALUES (?, ?)",
		entityID, tagID,
	)
	return err
}

func (s *RelationStore) ListTagsForEntity(ctx context.Context, table, fkCol string, entityID int64) ([]Tag, error) {
	items := make([]Tag, 0)
	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx,
			"SELECT t.id, t.name, t.created_at FROM tags t JOIN "+table+" m ON m.tag_id = t.id WHERE m."+fkCol+" = $1 ORDER BY t.name ASC",
			entityID,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var t Tag
			if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
				return nil, err
			}
			items = append(items, t)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT t.id, t.name, t.created_at FROM tags t JOIN "+table+" m ON m.tag_id = t.id WHERE m."+fkCol+" = ? ORDER BY t.name ASC",
		entityID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *RelationStore) AttachComponentToCase(ctx context.Context, caseID, componentID int64) error {
	if s.pgPool != nil {
		_, err := s.pgPool.Exec(ctx,
			`INSERT INTO test_case_components (test_case_id, component_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			caseID, componentID,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO test_case_components (test_case_id, component_id) VALUES (?, ?)`,
		caseID, componentID,
	)
	return err
}

func (s *RelationStore) ListComponentsForCase(ctx context.Context, caseID int64) ([]Component, error) {
	items := make([]Component, 0)
	if s.pgPool != nil {
		rows, err := s.pgPool.Query(ctx, `
			SELECT c.id, c.product_id, c.name, c.description, c.created_at
			FROM components c JOIN test_case_components m ON m.component_id = c.id
			WHERE m.test_case_id = $1 ORDER BY c.name ASC`, caseID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var c Component
			if err := rows.Scan(&c.ID, &c.ProductID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
				return nil, err
			}
			items = append(items, c)
		}
		return items, rows.Err()
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.product_id, c.name, c.description, c.created_at
		FROM components c JOIN test_case_components m ON m.component_id = c.id
		WHERE m.test_case_id = ? ORDER BY c.name ASC`, caseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c Component
		if err := rows.Scan(&c.ID, &c.ProductID, &c.Name, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func RegisterRelationRoutes(api huma.API, s *RelationStore) {
	registerTagPair := func(opPrefix, path, table, fkCol string) {
		huma.Register(api, huma.Operation{
			OperationID: "list-" + opPrefix + "-tags",
			Method:      http.MethodGet,
			Path:        path,
			Summary:     "List tags attached to " + opPrefix,
			Tags:        []string{"Management"},
		}, func(ctx context.Context, input *struct {
			ID int64 `path:"id"`
		}) (*ListTagsOutput, error) {
			items, err := s.ListTagsForEntity(ctx, table, fkCol, input.ID)
			if err != nil {
				return nil, huma.Error500InternalServerError("failed to list tags", err)
			}
			out := &ListTagsOutput{}
			out.Body.Items = items
			return out, nil
		})

		huma.Register(api, huma.Operation{
			OperationID: "attach-" + opPrefix + "-tag",
			Method:      http.MethodPost,
			Path:        path,
			Summary:     "Attach tag to " + opPrefix,
			Tags:        []string{"Management"},
		}, func(ctx context.Context, input *struct {
			ID   int64 `path:"id"`
			Body struct {
				TagID int64 `json:"tag_id" minimum:"1"`
			}
		}) (*struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}, error) {
			if err := auth.CheckRole(ctx, "tester"); err != nil {
				return nil, err
			}
			if err := s.AttachTag(ctx, table, fkCol, input.ID, input.Body.TagID); err != nil {
				return nil, huma.Error400BadRequest("failed to attach tag", err)
			}
			out := &struct {
				Body struct {
					OK bool `json:"ok"`
				}
			}{}
			out.Body.OK = true
			return out, nil
		})
	}

	registerTagPair("testcase", "/api/v1/testcases/{id}/tags", "test_case_tags", "test_case_id")
	registerTagPair("testplan", "/api/v1/testplans/{id}/tags", "test_plan_tags", "test_plan_id")
	registerTagPair("testrun", "/api/v1/testruns/{id}/tags", "test_run_tags", "test_run_id")

	huma.Register(api, huma.Operation{
		OperationID: "list-testcase-components",
		Method:      http.MethodGet,
		Path:        "/api/v1/testcases/{id}/components",
		Summary:     "List components attached to TestCase",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*ListComponentsOutput, error) {
		items, err := s.ListComponentsForCase(ctx, input.ID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list components", err)
		}
		out := &ListComponentsOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "attach-testcase-component",
		Method:      http.MethodPost,
		Path:        "/api/v1/testcases/{id}/components",
		Summary:     "Attach component to TestCase",
		Tags:        []string{"Management"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body struct {
			ComponentID int64 `json:"component_id" minimum:"1"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := s.AttachComponentToCase(ctx, input.ID, input.Body.ComponentID); err != nil {
			return nil, huma.Error400BadRequest("failed to attach component", err)
		}
		out := &struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}{}
		out.Body.OK = true
		return out, nil
	})
}
