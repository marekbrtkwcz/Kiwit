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

	ghRes, err := issuetracker.ReportIssue(ctx, issuetracker.Tracker{Provider: "github", BaseURL: ghSrv.URL}, "gh-tok", "", "Bug title", "desc")
	require.NoError(t, err)
	require.Equal(t, "#99", ghRes.ExternalKey)

	jiraSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/2/issue", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{"key": "QA-12"})
	}))
	t.Cleanup(jiraSrv.Close)

	jiraRes, err := issuetracker.ReportIssue(ctx, issuetracker.Tracker{Provider: "jira", BaseURL: jiraSrv.URL}, "j-tok", "QA", "Bug title", "desc")
	require.NoError(t, err)
	require.Equal(t, "QA-12", jiraRes.ExternalKey)
}
