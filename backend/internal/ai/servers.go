package ai

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/llm"
)

// resolve picks the LLM config: explicit server_id, else default server, else env.
func resolve(ctx context.Context, servers *llm.ServerStore, serverID int64) (llm.Config, string, error) {
	if serverID != 0 {
		srv, err := servers.Get(ctx, serverID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return llm.Config{}, "", huma.Error404NotFound("AI server not found")
			}
			return llm.Config{}, "", err
		}
		cfg := srv.ToConfig()
		if !cfg.Enabled() {
			return llm.Config{}, "", huma.Error503ServiceUnavailable("AI server has no API key")
		}
		return cfg, srv.Name, nil
	}
	cfg, name, err := servers.Default(ctx)
	if err != nil {
		return llm.Config{}, "", huma.Error503ServiceUnavailable("LLM not configured — add an AI server in admin or set KIWIT_LLM_API_KEY")
	}
	return cfg, name, nil
}

type serversOutput struct {
	Body struct {
		Items []llm.Server `json:"items"`
	}
}

type serverOutput struct {
	Body llm.Server
}

// RegisterServerRoutes mounts admin CRUD for AI servers (SquashTM-style, per instance).
func RegisterServerRoutes(api huma.API, servers *llm.ServerStore) {
	huma.Register(api, huma.Operation{
		OperationID: "list-ai-servers",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/ai-servers",
		Summary:     "List configured AI servers (admin)",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct{}) (*serversOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		items, err := servers.List(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list AI servers", err)
		}
		out := &serversOutput{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-ai-server",
		Method:      http.MethodPost,
		Path:        "/api/v1/admin/ai-servers",
		Summary:     "Add an AI server (admin)",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body llm.ServerInput
	}) (*serverOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		srv, err := servers.Create(ctx, input.Body)
		if err != nil {
			return nil, huma.Error409Conflict("server name already exists", err)
		}
		return &serverOutput{Body: srv}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-ai-server",
		Method:      http.MethodPut,
		Path:        "/api/v1/admin/ai-servers/{id}",
		Summary:     "Update an AI server (admin)",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		ID   int64 `path:"id"`
		Body llm.ServerInput
	}) (*serverOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		srv, err := servers.Update(ctx, input.ID, input.Body)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("AI server not found")
			}
			return nil, huma.Error409Conflict("server name already exists", err)
		}
		return &serverOutput{Body: srv}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "delete-ai-server",
		Method:      http.MethodDelete,
		Path:        "/api/v1/admin/ai-servers/{id}",
		Summary:     "Delete an AI server (admin)",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		ID int64 `path:"id"`
	}) (*struct{}, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := servers.Delete(ctx, input.ID); err != nil {
			return nil, huma.Error500InternalServerError("failed to delete AI server", err)
		}
		return nil, nil
	})
}
