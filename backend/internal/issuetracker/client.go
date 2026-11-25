package issuetracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ReportedIssue struct {
	ExternalKey string
	URL         string
}

// ReportIssue creates a remote issue on GitHub, GitLab, or Jira via HTTP REST API.
func ReportIssue(ctx context.Context, tracker Tracker, apiToken, projectKey, title, body string) (ReportedIssue, error) {
	base := strings.TrimRight(tracker.BaseURL, "/")
	client := &http.Client{Timeout: 10 * time.Second}

	switch tracker.Provider {
	case "github":
		payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/issues", bytes.NewReader(payload))
		if err != nil {
			return ReportedIssue{}, err
		}
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return ReportedIssue{}, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return ReportedIssue{}, fmt.Errorf("github api returned %d", res.StatusCode)
		}
		var out struct {
			Number  int    `json:"number"`
			HTMLURL string `json:"html_url"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return ReportedIssue{}, err
		}
		return ReportedIssue{ExternalKey: fmt.Sprintf("#%d", out.Number), URL: out.HTMLURL}, nil

	case "gitlab":
		payload, _ := json.Marshal(map[string]string{"title": title, "description": body})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/issues", bytes.NewReader(payload))
		if err != nil {
			return ReportedIssue{}, err
		}
		req.Header.Set("PRIVATE-TOKEN", apiToken)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return ReportedIssue{}, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return ReportedIssue{}, fmt.Errorf("gitlab api returned %d", res.StatusCode)
		}
		var out struct {
			IID    int    `json:"iid"`
			WebURL string `json:"web_url"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return ReportedIssue{}, err
		}
		return ReportedIssue{ExternalKey: fmt.Sprintf("#%d", out.IID), URL: out.WebURL}, nil

	case "jira":
		payload, _ := json.Marshal(map[string]any{
			"fields": map[string]any{
				"project":     map[string]string{"key": projectKey},
				"summary":     title,
				"description": body,
				"issuetype":   map[string]string{"name": "Bug"},
			},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/rest/api/2/issue", bytes.NewReader(payload))
		if err != nil {
			return ReportedIssue{}, err
		}
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return ReportedIssue{}, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return ReportedIssue{}, fmt.Errorf("jira api returned %d", res.StatusCode)
		}
		var out struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return ReportedIssue{}, err
		}
		return ReportedIssue{ExternalKey: out.Key, URL: base + "/browse/" + out.Key}, nil

	default:
		return ReportedIssue{}, fmt.Errorf("unsupported provider: %s", tracker.Provider)
	}
}
