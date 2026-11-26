package issuetracker_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/issuetracker"
)

func TestReportIssueProviders(t *testing.T) {
	ctx := context.Background()

	ghSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/issues", r.URL.Path)
		require.Equal(t, "Bearer gh-tok", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{"number": 99, "html_url": "https://github.com/o/r/issues/99"})
	}))
	t.Cleanup(ghSrv.Close)

	ghRes, err := issuetracker.ReportIssueWithClient(ctx, ghSrv.Client(), issuetracker.Tracker{Provider: "github", BaseURL: ghSrv.URL}, "gh-tok", "", "Bug title", "desc")
	require.NoError(t, err)
	require.Equal(t, "#99", ghRes.ExternalKey)

	jiraSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/2/issue", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{"key": "QA-12"})
	}))
	t.Cleanup(jiraSrv.Close)

	jiraRes, err := issuetracker.ReportIssueWithClient(ctx, jiraSrv.Client(), issuetracker.Tracker{Provider: "jira", BaseURL: jiraSrv.URL}, "j-tok", "QA", "Bug title", "desc")
	require.NoError(t, err)
	require.Equal(t, "QA-12", jiraRes.ExternalKey)

	// SSRF protection rejects loopback and metadata URLs
	_, err = issuetracker.ReportIssue(ctx, issuetracker.Tracker{Provider: "github", BaseURL: "http://127.0.0.1:8080"}, "tok", "", "t", "b")
	require.Error(t, err)
	_, err = issuetracker.ReportIssue(ctx, issuetracker.Tracker{Provider: "github", BaseURL: "http://169.254.169.254/latest"}, "tok", "", "t", "b")
	require.Error(t, err)
}

func TestReportIssueBugzillaAndAzure(t *testing.T) {
	ctx := context.Background()

	bzSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/bug", r.URL.Path)
		require.Equal(t, "bz-key", r.Header.Get("X-BUGZILLA-API-KEY"))
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "ProductX", body["product"])
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 4242})
	}))
	t.Cleanup(bzSrv.Close)

	bzRes, err := issuetracker.ReportIssueWithClient(ctx, bzSrv.Client(), issuetracker.Tracker{Provider: "bugzilla", BaseURL: bzSrv.URL}, "bz-key", "ProductX", "Bug title", "desc")
	require.NoError(t, err)
	require.Equal(t, "4242", bzRes.ExternalKey)
	require.Contains(t, bzRes.URL, "show_bug.cgi?id=4242")

	azSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/_apis/wit/workitems/$Bug", r.URL.Path)
		require.Equal(t, "application/json-patch+json", r.Header.Get("Content-Type"))
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Empty(t, user)
		require.Equal(t, "az-pat", pass)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 77, "_links": map[string]any{"html": map[string]string{"href": "https://dev.azure.com/org/proj/_workitems/edit/77"}}})
	}))
	t.Cleanup(azSrv.Close)

	azRes, err := issuetracker.ReportIssueWithClient(ctx, azSrv.Client(), issuetracker.Tracker{Provider: "azure", BaseURL: azSrv.URL}, "az-pat", "proj", "Bug title", "desc")
	require.NoError(t, err)
	require.Equal(t, "77", azRes.ExternalKey)
	require.Contains(t, azRes.URL, "_workitems/edit/77")

	// status: bugzilla
	bzStatusSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/bug/4242", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"bugs": []map[string]string{{"status": "RESOLVED"}}})
	}))
	t.Cleanup(bzStatusSrv.Close)
	st, err := issuetracker.FetchIssueStatusWithClient(ctx, bzStatusSrv.Client(), issuetracker.Tracker{Provider: "bugzilla", BaseURL: bzStatusSrv.URL}, "bz-key", "4242")
	require.NoError(t, err)
	require.Equal(t, "RESOLVED", st)

	// status: azure
	azStatusSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/_apis/wit/workitems/77", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"fields": map[string]string{"System.State": "Closed"}})
	}))
	t.Cleanup(azStatusSrv.Close)
	st, err = issuetracker.FetchIssueStatusWithClient(ctx, azStatusSrv.Client(), issuetracker.Tracker{Provider: "azure", BaseURL: azStatusSrv.URL}, "az-pat", "77")
	require.NoError(t, err)
	require.Equal(t, "Closed", st)

	// comment: bugzilla + azure
	bzCommentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/bug/4242/comment", r.URL.Path)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "hello", body["comment"])
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	t.Cleanup(bzCommentSrv.Close)
	require.NoError(t, issuetracker.PostIssueCommentWithClient(ctx, bzCommentSrv.Client(), issuetracker.Tracker{Provider: "bugzilla", BaseURL: bzCommentSrv.URL}, "bz-key", "4242", "hello"))

	azCommentSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/_apis/wit/workitems/77/comments", r.URL.Path)
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "hello", body["text"])
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
	}))
	t.Cleanup(azCommentSrv.Close)
	require.NoError(t, issuetracker.PostIssueCommentWithClient(ctx, azCommentSrv.Client(), issuetracker.Tracker{Provider: "azure", BaseURL: azCommentSrv.URL}, "az-pat", "77", "hello"))
}
