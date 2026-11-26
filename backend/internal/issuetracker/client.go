package issuetracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

type ReportedIssue struct {
	ExternalKey string
	URL         string
}

func isBlockedIP(ip net.IP) bool {
	return ip == nil ||
		ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// isHostAllowlisted checks KIWIT_TRACKER_ALLOW_PRIVATE_HOSTS (comma-separated hosts/IPs or "*").
func isHostAllowlisted(host string) bool {
	raw := strings.TrimSpace(os.Getenv("KIWIT_TRACKER_ALLOW_PRIVATE_HOSTS"))
	if raw == "" {
		return false
	}
	for _, entry := range strings.Split(raw, ",") {
		e := strings.TrimSpace(entry)
		if e == "" {
			continue
		}
		if e == "*" || strings.EqualFold(e, host) {
			return true
		}
	}
	return false
}

// ValidateTrackerURL ensures the tracker base URL uses http(s) and respects SSRF protection unless allowlisted.
func ValidateTrackerURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid tracker base_url: must be http or https with host")
	}
	host := u.Hostname()
	if isHostAllowlisted(host) {
		return nil
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return fmt.Errorf("tracker host %q is not allowed", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return fmt.Errorf("tracker IP %q is not allowed", host)
		}
	}
	return nil
}

func safeHTTPClient(targetHost string) *http.Client {
	allowPrivate := isHostAllowlisted(targetHost)
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if isHostAllowlisted(host) {
				return nil
			}
			if ip := net.ParseIP(host); isBlockedIP(ip) {
				return fmt.Errorf("blocked connection to private/local address %s", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return ValidateTrackerURL(req.URL.String())
		},
	}
}

func setJiraAuth(req *http.Request, tracker Tracker, apiToken string) {
	user := strings.TrimSpace(tracker.APIUsername)
	if user == "" {
		user = strings.TrimSpace(os.Getenv("KIWIT_TRACKER_USERNAME"))
	}
	if user != "" {
		// Jira Cloud requires Basic Auth (email:api_token)
		req.SetBasicAuth(user, apiToken)
		return
	}
	// Jira Data Center / Server PAT uses Bearer auth
	req.Header.Set("Authorization", "Bearer "+apiToken)
}

func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// ReportIssue creates a remote issue on GitHub, GitLab, or Jira via HTTP REST API with SSRF protection.
func ReportIssue(ctx context.Context, tracker Tracker, apiToken, projectKey, title, body string) (ReportedIssue, error) {
	if err := ValidateTrackerURL(tracker.BaseURL); err != nil {
		return ReportedIssue{}, err
	}
	return ReportIssueWithClient(ctx, safeHTTPClient(hostFromURL(tracker.BaseURL)), tracker, apiToken, projectKey, title, body)
}

// ReportIssueWithClient creates a remote issue using the provided HTTP client (useful for tests).
func ReportIssueWithClient(ctx context.Context, client *http.Client, tracker Tracker, apiToken, projectKey, title, body string) (ReportedIssue, error) {
	base := strings.TrimRight(tracker.BaseURL, "/")
	if client == nil {
		client = safeHTTPClient(hostFromURL(tracker.BaseURL))
	}

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
		setJiraAuth(req, tracker, apiToken)
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

	case "bugzilla":
		payload, _ := json.Marshal(map[string]string{
			"product":     projectKey,
			"summary":     title,
			"description": body,
			"version":     "unspecified",
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/rest/bug", bytes.NewReader(payload))
		if err != nil {
			return ReportedIssue{}, err
		}
		setBugzillaAuth(req, tracker, apiToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return ReportedIssue{}, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return ReportedIssue{}, fmt.Errorf("bugzilla api returned %d", res.StatusCode)
		}
		var out struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return ReportedIssue{}, err
		}
		key := fmt.Sprintf("%d", out.ID)
		return ReportedIssue{ExternalKey: key, URL: base + "/show_bug.cgi?id=" + key}, nil

	case "azure":
		// BaseURL already contains the project: https://dev.azure.com/{org}/{project}
		patch, _ := json.Marshal([]map[string]string{
			{"op": "add", "path": "/fields/System.Title", "value": title},
			{"op": "add", "path": "/fields/Microsoft.VSTS.TCM.ReproSteps", "value": body},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			base+"/_apis/wit/workitems/$Bug?api-version=7.1", bytes.NewReader(patch))
		if err != nil {
			return ReportedIssue{}, err
		}
		setAzureAuth(req, apiToken)
		req.Header.Set("Content-Type", "application/json-patch+json")
		res, err := client.Do(req)
		if err != nil {
			return ReportedIssue{}, err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return ReportedIssue{}, fmt.Errorf("azure boards api returned %d", res.StatusCode)
		}
		var out struct {
			ID    int `json:"id"`
			Links struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"_links"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return ReportedIssue{}, err
		}
		key := fmt.Sprintf("%d", out.ID)
		issueURL := out.Links.HTML.Href
		if issueURL == "" {
			issueURL = base + "/_workitems/edit/" + key
		}
		return ReportedIssue{ExternalKey: key, URL: issueURL}, nil

	default:
		return ReportedIssue{}, fmt.Errorf("unsupported provider: %s", tracker.Provider)
	}
}

// FetchIssueStatus queries the remote tracker for the current status of externalKey.
func FetchIssueStatus(ctx context.Context, tracker Tracker, apiToken, externalKey string) (string, error) {
	if err := ValidateTrackerURL(tracker.BaseURL); err != nil {
		return "", err
	}
	return FetchIssueStatusWithClient(ctx, safeHTTPClient(hostFromURL(tracker.BaseURL)), tracker, apiToken, externalKey)
}

func FetchIssueStatusWithClient(ctx context.Context, client *http.Client, tracker Tracker, apiToken, externalKey string) (string, error) {
	base := strings.TrimRight(tracker.BaseURL, "/")
	key := strings.TrimPrefix(strings.TrimSpace(externalKey), "#")
	if client == nil {
		client = safeHTTPClient(hostFromURL(tracker.BaseURL))
	}

	switch tracker.Provider {
	case "github":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/issues/"+url.PathEscape(key), nil)
		if err != nil {
			return "", err
		}
		if apiToken != "" {
			req.Header.Set("Authorization", "Bearer "+apiToken)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return "", fmt.Errorf("github api returned %d", res.StatusCode)
		}
		var out struct {
			State string `json:"state"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return "", err
		}
		return out.State, nil

	case "gitlab":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/issues/"+url.PathEscape(key), nil)
		if err != nil {
			return "", err
		}
		if apiToken != "" {
			req.Header.Set("PRIVATE-TOKEN", apiToken)
		}
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return "", fmt.Errorf("gitlab api returned %d", res.StatusCode)
		}
		var out struct {
			State string `json:"state"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return "", err
		}
		return out.State, nil

	case "jira":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/rest/api/2/issue/"+url.PathEscape(key)+"?fields=status", nil)
		if err != nil {
			return "", err
		}
		setJiraAuth(req, tracker, apiToken)
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return "", fmt.Errorf("jira api returned %d", res.StatusCode)
		}
		var out struct {
			Fields struct {
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return "", err
		}
		return out.Fields.Status.Name, nil

	case "bugzilla":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/rest/bug/"+url.PathEscape(key), nil)
		if err != nil {
			return "", err
		}
		setBugzillaAuth(req, tracker, apiToken)
		req.Header.Set("Accept", "application/json")
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return "", fmt.Errorf("bugzilla api returned %d", res.StatusCode)
		}
		var out struct {
			Bugs []struct {
				Status string `json:"status"`
			} `json:"bugs"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return "", err
		}
		if len(out.Bugs) == 0 {
			return "", fmt.Errorf("bugzilla bug %s not found", key)
		}
		return out.Bugs[0].Status, nil

	case "azure":
		// BaseURL already contains the project: https://dev.azure.com/{org}/{project}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			base+"/_apis/wit/workitems/"+url.PathEscape(key)+"?api-version=7.1&$expand=fields", nil)
		if err != nil {
			return "", err
		}
		setAzureAuth(req, apiToken)
		res, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode >= 400 {
			return "", fmt.Errorf("azure boards api returned %d", res.StatusCode)
		}
		var out struct {
			Fields map[string]any `json:"fields"`
		}
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			return "", err
		}
		if s, ok := out.Fields["System.State"].(string); ok {
			return s, nil
		}
		return "", fmt.Errorf("azure work item %s has no state", key)

	default:
		return "", fmt.Errorf("unsupported provider: %s", tracker.Provider)
	}
}

// PostIssueComment posts a comment back to an existing remote issue on GitHub, GitLab, or Jira.
func PostIssueComment(ctx context.Context, tracker Tracker, apiToken, externalKey, body string) error {
	if err := ValidateTrackerURL(tracker.BaseURL); err != nil {
		return err
	}
	return PostIssueCommentWithClient(ctx, safeHTTPClient(hostFromURL(tracker.BaseURL)), tracker, apiToken, externalKey, body)
}

func PostIssueCommentWithClient(ctx context.Context, client *http.Client, tracker Tracker, apiToken, externalKey, body string) error {
	base := strings.TrimRight(tracker.BaseURL, "/")
	key := strings.TrimPrefix(strings.TrimSpace(externalKey), "#")
	if client == nil {
		client = safeHTTPClient(hostFromURL(tracker.BaseURL))
	}
	payload, _ := json.Marshal(map[string]string{"body": body})

	var req *http.Request
	var err error
	switch tracker.Provider {
	case "github":
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, base+"/issues/"+url.PathEscape(key)+"/comments", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+apiToken)
		req.Header.Set("Accept", "application/vnd.github+json")
	case "gitlab":
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, base+"/issues/"+url.PathEscape(key)+"/notes", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("PRIVATE-TOKEN", apiToken)
	case "jira":
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, base+"/rest/api/2/issue/"+url.PathEscape(key)+"/comment", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		setJiraAuth(req, tracker, apiToken)
	case "bugzilla":
		comment, _ := json.Marshal(map[string]string{"comment": body})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, base+"/rest/bug/"+url.PathEscape(key)+"/comment", bytes.NewReader(comment))
		if err != nil {
			return err
		}
		setBugzillaAuth(req, tracker, apiToken)
	case "azure":
		// BaseURL already contains the project: https://dev.azure.com/{org}/{project}
		comment, _ := json.Marshal(map[string]string{"text": body})
		req, err = http.NewRequestWithContext(ctx, http.MethodPost,
			base+"/_apis/wit/workitems/"+url.PathEscape(key)+"/comments?api-version=7.1-preview.3", bytes.NewReader(comment))
		if err != nil {
			return err
		}
		setAzureAuth(req, apiToken)
	default:
		return fmt.Errorf("unsupported provider: %s", tracker.Provider)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s api returned %d", tracker.Provider, res.StatusCode)
	}
	return nil
}

// setBugzillaAuth sets Bugzilla REST auth: X-BUGZILLA-API-KEY when no username is
// configured, otherwise HTTP Basic (login:api_key).
func setBugzillaAuth(req *http.Request, tracker Tracker, apiToken string) {
	user := strings.TrimSpace(tracker.APIUsername)
	if user == "" {
		user = strings.TrimSpace(os.Getenv("KIWIT_TRACKER_USERNAME"))
	}
	if user != "" {
		req.SetBasicAuth(user, apiToken)
		return
	}
	req.Header.Set("X-BUGZILLA-API-KEY", apiToken)
}

// setAzureAuth sets Azure DevOps PAT auth: Basic with empty username and the PAT
// as password (base64 of ":<PAT>").
func setAzureAuth(req *http.Request, apiToken string) {
	req.SetBasicAuth("", apiToken)
}
