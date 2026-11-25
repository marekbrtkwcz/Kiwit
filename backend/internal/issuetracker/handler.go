package issuetracker

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/auth"
)

func RegisterRoutes(api huma.API, repo Repository, guard *auth.AccessGuard) {
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
			Name        string `json:"name" minLength:"1"`
			Provider    string `json:"provider" enum:"github,gitlab,jira"`
			BaseURL     string `json:"base_url" minLength:"1"`
			APIUsername string `json:"api_username,omitempty" doc:"Optional username/email for Jira Cloud Basic Auth"`
		}
	}) (*struct{ Body Tracker }, error) {
		if err := auth.CheckRole(ctx, "admin"); err != nil {
			return nil, err
		}
		if err := ValidateTrackerURL(input.Body.BaseURL); err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		t, err := repo.CreateTrackerWithAuth(ctx, input.Body.Name, input.Body.Provider, input.Body.BaseURL, input.Body.APIUsername)
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
		if err := guard.EnforceExecution(ctx, input.ExecutionID, false); err != nil {
			return nil, err
		}
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
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ExecutionID, true); err != nil {
			return nil, err
		}
		b, err := repo.LinkBug(ctx, input.ExecutionID, input.Body.TrackerID, input.Body.ExternalKey, input.Body.URL, input.Body.Title)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to link bug", err)
		}
		return &struct{ Body Bug }{Body: b}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "report-execution-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{execution_id}/bugs/report",
		Summary:     "Create an issue in GitHub/GitLab/Jira via REST API and link it to TestExecution",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct {
		ExecutionID int64 `path:"execution_id"`
		Body        struct {
			TrackerID  int64  `json:"tracker_id" minimum:"1"`
			APIToken   string `json:"api_token,omitempty"`
			ProjectKey string `json:"project_key,omitempty"`
			Title      string `json:"title" minLength:"1"`
			Body       string `json:"body,omitempty"`
		}
	}) (*struct{ Body Bug }, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ExecutionID, true); err != nil {
			return nil, err
		}
		target, err := repo.GetTrackerByID(ctx, input.Body.TrackerID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("issue tracker not found")
			}
			return nil, huma.Error500InternalServerError("failed to load tracker", err)
		}
		token := input.Body.APIToken
		if token == "" {
			token = os.Getenv("KIWIT_TRACKER_TOKEN")
		}
		reported, err := ReportIssue(ctx, target, token, input.Body.ProjectKey, input.Body.Title, input.Body.Body)
		if err != nil {
			return nil, huma.Error502BadGateway("failed to report issue to tracker", err)
		}
		tid := target.ID
		b, err := repo.LinkBug(ctx, input.ExecutionID, &tid, reported.ExternalKey, reported.URL, input.Body.Title)
		if err != nil {
			return nil, huma.Error400BadRequest("failed to link reported bug", err)
		}
		return &struct{ Body Bug }{Body: b}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sync-execution-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{execution_id}/bugs/{bug_id}/sync",
		Summary:     "Fetch live status of a linked bug from GitHub/GitLab/Jira",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct {
		ExecutionID int64 `path:"execution_id"`
		BugID       int64 `path:"bug_id"`
		Body        struct {
			APIToken string `json:"api_token,omitempty"`
		}
	}) (*struct{ Body Bug }, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ExecutionID, false); err != nil {
			return nil, err
		}
		b, err := repo.GetBugByID(ctx, input.BugID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, huma.Error404NotFound("bug not found")
			}
			return nil, huma.Error500InternalServerError("failed to load bug", err)
		}
		if b.TrackerID == nil {
			return nil, huma.Error400BadRequest("bug is not associated with a configured issue tracker")
		}
		target, err := repo.GetTrackerByID(ctx, *b.TrackerID)
		if err != nil {
			return nil, huma.Error404NotFound("issue tracker not found")
		}
		token := input.Body.APIToken
		if token == "" {
			token = os.Getenv("KIWIT_TRACKER_TOKEN")
		}
		status, err := FetchIssueStatus(ctx, target, token, b.ExternalKey)
		if err != nil {
			return nil, huma.Error502BadGateway("failed to fetch remote issue status", err)
		}
		updated, err := repo.UpdateBugStatus(ctx, b.ID, status)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to persist synced status", err)
		}
		return &struct{ Body Bug }{Body: updated}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "comment-execution-bug",
		Method:      http.MethodPost,
		Path:        "/api/v1/executions/{execution_id}/bugs/{bug_id}/comment",
		Summary:     "Post a comment to a linked remote issue on GitHub/GitLab/Jira",
		Tags:        []string{"IssueTrackers"},
	}, func(ctx context.Context, input *struct {
		ExecutionID int64 `path:"execution_id"`
		BugID       int64 `path:"bug_id"`
		Body        struct {
			APIToken string `json:"api_token,omitempty"`
			Body     string `json:"body" minLength:"1"`
		}
	}) (*struct {
		Body struct {
			OK bool `json:"ok"`
		}
	}, error) {
		if err := auth.CheckRole(ctx, "tester"); err != nil {
			return nil, err
		}
		if err := guard.EnforceExecution(ctx, input.ExecutionID, true); err != nil {
			return nil, err
		}
		b, err := repo.GetBugByID(ctx, input.BugID)
		if err != nil {
			return nil, huma.Error404NotFound("bug not found")
		}
		if b.TrackerID == nil {
			return nil, huma.Error400BadRequest("bug is not associated with a configured issue tracker")
		}
		target, err := repo.GetTrackerByID(ctx, *b.TrackerID)
		if err != nil {
			return nil, huma.Error404NotFound("issue tracker not found")
		}
		token := input.Body.APIToken
		if token == "" {
			token = os.Getenv("KIWIT_TRACKER_TOKEN")
		}
		if err := PostIssueComment(ctx, target, token, b.ExternalKey, input.Body.Body); err != nil {
			return nil, huma.Error502BadGateway("failed to post remote comment", err)
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
