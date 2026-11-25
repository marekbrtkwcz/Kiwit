package issuetracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

// ValidateTrackerURL ensures the tracker base URL uses http(s) and does not point to local/private networks.
func ValidateTrackerURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid tracker base_url: must be http or https with host")
	}
	host := u.Hostname()
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

func safeHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
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

// ReportIssue creates a remote issue on GitHub, GitLab, or Jira via HTTP REST API with SSRF protection.
func ReportIssue(ctx context.Context, tracker Tracker, apiToken, projectKey, title, body string) (ReportedIssue, error) {
	if err := ValidateTrackerURL(tracker.BaseURL); err != nil {
		return ReportedIssue{}, err
	}
	return ReportIssueWithClient(ctx, safeHTTPClient(), tracker, apiToken, projectKey, title, body)
}

// ReportIssueWithClient creates a remote issue using the provided HTTP client (useful for tests).
func ReportIssueWithClient(ctx context.Context, client *http.Client, tracker Tracker, apiToken, projectKey, title, body string) (ReportedIssue, error) {
	base := strings.TrimRight(tracker.BaseURL, "/")
	if client == nil {
		client = safeHTTPClient()
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
