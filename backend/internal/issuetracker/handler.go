package issuetracker

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func RegisterRoutes(api huma.API, repo Repository) {
	huma.Register(api, huma.Operation{
		OperationID: "list-issue-trackers",
		Method:      http.MethodGet,
		Path:        "/api/v1/issue-trackers",
		Summary:     "List configured issue trackers",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Items []Tracker `json:"items"`
		}
	}, error) {
		items, err := repo.ListTrackers(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list trackers", err)
		}
		out := &struct {
			Body struct {
				Items []Tracker `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-issue-tracker",
		Method:      http.MethodPost,
		Path:        "/api/v1/issue-trackers",
		Summary:     "Register an issue tracker (GitHub, GitLab, Jira)",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name     string `json:"name" minLength:"1"`
			Provider string `json:"provider" enum:"github,gitlab,jira"`
			BaseURL  string `json:"base_url" minLength:"1"`
		}
	}) (*struct{ Body Tracker }, error) {
		t, err := repo.CreateTracker(ctx, input.Body.Name, input.Body.Provider, input.Body.BaseURL)
		if err != nil {
			return nil, huma.Error409Conflict("failed to create tracker", err)
		}
		return &struct{ Body Tracker }{Body: t}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-execution-bugs",
		Method:      http.MethodGet,
		Path:        "/api/v1/executions/{execution_id}/bugs",
		Summary:     "List bugs linked to a test execution",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct {
		ExecutionID int64 `path:"execution_id"`
	}) (*struct {
		Body struct {
			Items []Bug `json:"items"`
		}
	}, error) {
		items, err := repo.ListBugsByExecution(ctx, input.ExecutionID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to list bugs", err)
		}
		out := &struct {
			Body struct {
				Items []Bug `json:"items"`
			}
		}{}
		out.Body.Items = items
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "link-execution-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{execution_id}/bugs",
		Summary:     "Link an external bug (Jira/GitHub/GitLab) to a test execution",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct {
		ExecutionID int64 `path:"execution_id"`
		Body        struct {
			TrackerID   *int64 `json:"tracker_id,omitempty"`
			ExternalKey string `json:"external_key" minLength:"1"`
			URL         string `json:"url" minLength:"1"`
			Title       string `json:"title,omitempty"`
		}
	}) (*struct{ Body Bug }, error) {
		b, err := repo.LinkBug(ctx, input.ExecutionID, input.Body.TrackerID, input.Body.ExternalKey, input.Body.URL, input.Body.Title)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to link bug", err)
		}
		return &struct{ Body Bug }{Body: b}, nil
	})
}
