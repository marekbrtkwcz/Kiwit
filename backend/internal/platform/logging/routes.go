package logging

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

type logsOutput struct {
	Body struct {
		Items []LogEntry `json:"items"`
	}
}

// RegisterRoutes mounts GET /api/v1/admin/logs — the in-app log viewer fed
// by the in-memory ring buffer (admin only).
func RegisterRoutes(api huma.API, ring *Ring) {
	huma.Register(api, huma.Operation{
		OperationID: "admin-logs",
		Method:      http.MethodGet,
		Path:        "/api/v1/admin/logs",
		Summary:     "Read recent in-memory log entries (admin)",
		Tags:        []string{"Admin"},
	}, func(ctx context.Context, input *struct {
		Limit int    `query:"limit" default:"200"`
		Level string `query:"level,omitempty" enum:"debug,info,warn,error" doc:"Minimum level"`
	}) (*logsOutput, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		out := &logsOutput{}
		out.Body.Items = ring.Snapshot(input.Level, input.Limit)
		return out, nil
	})
}
