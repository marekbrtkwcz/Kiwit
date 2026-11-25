package notify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/notify"
)

func TestWebhookClientSend(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := notify.NewWebhookClient(srv.URL)
	require.True(t, client.Enabled())
	require.NoError(t, client.Send(context.Background(), "TestRun #1 finished: 10 passed, 0 failed"))
	require.Equal(t, "TestRun #1 finished: 10 passed, 0 failed", received["text"])
}
