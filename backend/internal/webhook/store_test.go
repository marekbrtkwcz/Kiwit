package webhook_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/kiwit/kiwit/internal/platform/db"
	"github.com/kiwit/kiwit/internal/webhook"
)

func newStore(t *testing.T) *webhook.Store {
	t.Helper()
	ctx := context.Background()
	sqlDB, err := db.OpenSQLite(ctx, "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return webhook.NewStore(sqlDB, nil)
}

func TestWebhookSign(t *testing.T) {
	ts := int64(1730000000)
	body := []byte(`{"a":1}`)

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(fmt.Sprintf("%d.%s", ts, string(body))))
	expected := fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))

	require.Equal(t, expected, webhook.Sign("secret", ts, body))
	require.Equal(t, "", webhook.Sign("", ts, body))
}

func TestWebhookSecretMasking(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	created, err := store.Create(ctx, webhook.Webhook{
		Name: "masked", URL: "http://example.com/hook", Secret: "s3cr3t",
		Events: []string{"run.stopped"}, Active: true,
	})
	require.NoError(t, err)

	b, err := json.Marshal(created)
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(b, &raw))
	_, hasSecret := raw["secret"]
	require.False(t, hasSecret, "secret must never appear in serialized webhook responses")

	items, err := store.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, items)
	b, err = json.Marshal(items)
	require.NoError(t, err)
	require.NotContains(t, string(b), "secret")
}

func TestWebhookCRUDFlow(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	created, err := store.Create(ctx, webhook.Webhook{
		Name: "ci", URL: "http://example.com/hook", Secret: "s3cr3t",
		Events: []string{"run.stopped", "execution.failed"}, Active: true,
	})
	require.NoError(t, err)
	require.Positive(t, created.ID)
	require.Equal(t, []string{"run.stopped", "execution.failed"}, created.Events)
	require.WithinDuration(t, time.Now().UTC(), created.CreatedAt, time.Minute)

	got, err := store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "ci", got.Name)
	require.Equal(t, "s3cr3t", got.Secret)
	require.True(t, got.Active)

	updated, err := store.Update(ctx, created.ID, webhook.Webhook{
		Name: "ci2", URL: "http://example.com/hook2", Secret: "",
		Events: []string{"execution.assigned"}, Active: false,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "ci2", updated.Name)
	require.False(t, updated.Active)
	require.Equal(t, []string{"execution.assigned"}, updated.Events)

	// Empty secret on update keeps the stored secret (signing stays enabled).
	items, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	kept, err := store.Get(ctx, items[0].ID)
	require.NoError(t, err)
	require.Equal(t, "s3cr3t", kept.Secret, "update with empty secret must keep the current secret")

	require.NoError(t, store.Delete(ctx, created.ID))
	_, err = store.Get(ctx, created.ID)
	require.ErrorIs(t, err, webhook.ErrNotFound)
	require.ErrorIs(t, store.Delete(ctx, created.ID), webhook.ErrNotFound)
	_, err = store.Update(ctx, created.ID, got, true)
	require.ErrorIs(t, err, webhook.ErrNotFound)
}

// TestWebhookUpdateKeepsStoredSecretWhenKeepSecretSet verifies that an update
// with an empty secret and keepSecret=true preserves signing.
func TestWebhookUpdateKeepsStoredSecretWhenKeepSecretSet(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	created, err := store.Create(ctx, webhook.Webhook{
		Name: "keep", URL: "http://example.com/hook", Secret: "s3cr3t",
		Events: []string{"run.stopped"}, Active: true,
	})
	require.NoError(t, err)

	updated, err := store.Update(ctx, created.ID, webhook.Webhook{
		Name: "keep2", URL: "http://example.com/hook", Secret: "",
		Events: []string{"run.stopped"}, Active: true,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "s3cr3t", updated.Secret, "keepSecret=true with an empty secret must keep the current secret")
}

// TestWebhookUpdateClearsSecretWhenKeepSecretUnset verifies that an update with
// an empty secret and keepSecret=false disables signing.
func TestWebhookUpdateClearsSecretWhenKeepSecretUnset(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	created, err := store.Create(ctx, webhook.Webhook{
		Name: "clear", URL: "http://example.com/hook", Secret: "s3cr3t",
		Events: []string{"run.stopped"}, Active: true,
	})
	require.NoError(t, err)

	updated, err := store.Update(ctx, created.ID, webhook.Webhook{
		Name: "clear2", URL: "http://example.com/hook", Secret: "",
		Events: []string{"run.stopped"}, Active: true,
	}, false)
	require.NoError(t, err)
	require.Empty(t, updated.Secret, "keepSecret=false with an empty secret must disable signing")

	stored, err := store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Empty(t, stored.Secret)
}

// TestWebhookUpdateOverwritesSecretWhenProvided verifies that a non-empty
// secret overwrites the stored one in both keepSecret modes.
func TestWebhookUpdateOverwritesSecretWhenProvided(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	created, err := store.Create(ctx, webhook.Webhook{
		Name: "rotate", URL: "http://example.com/hook", Secret: "old",
		Events: []string{"run.stopped"}, Active: true,
	})
	require.NoError(t, err)

	updated, err := store.Update(ctx, created.ID, webhook.Webhook{
		Name: "rotate", URL: "http://example.com/hook", Secret: "new",
		Events: []string{"run.stopped"}, Active: true,
	}, true)
	require.NoError(t, err)
	require.Equal(t, "new", updated.Secret)

	updated, err = store.Update(ctx, created.ID, webhook.Webhook{
		Name: "rotate", URL: "http://example.com/hook", Secret: "newest",
		Events: []string{"run.stopped"}, Active: true,
	}, false)
	require.NoError(t, err)
	require.Equal(t, "newest", updated.Secret)
}

// TestWebhookEmitFanOut verifies that only active webhooks subscribed to the
// event receive a signed payload.
func TestWebhookEmitFanOut(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	// httptest servers listen on 127.0.0.1: private targets must be allowed.
	prevPrivate := webhook.AllowPrivateTargets
	webhook.AllowPrivateTargets = true
	t.Cleanup(func() { webhook.AllowPrivateTargets = prevPrivate })

	var subCount, unsubCount, offCount int64
	subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&subCount, 1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	unsubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&unsubCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	offSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&offCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(subSrv.Close)
	t.Cleanup(unsubSrv.Close)
	t.Cleanup(offSrv.Close)

	_, err := store.Create(ctx, webhook.Webhook{
		Name: "subscribed", URL: subSrv.URL, Secret: "k", Events: []string{"run.stopped"}, Active: true,
	})
	require.NoError(t, err)
	_, err = store.Create(ctx, webhook.Webhook{
		Name: "other-event", URL: unsubSrv.URL, Events: []string{"execution.failed"}, Active: true,
	})
	require.NoError(t, err)
	_, err = store.Create(ctx, webhook.Webhook{
		Name: "inactive", URL: offSrv.URL, Events: []string{"run.stopped"}, Active: false,
	})
	require.NoError(t, err)

	store.EmitEvent("run.stopped", "test_run", 42, "Test run #42 stopped", "")

	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt64(&subCount) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	require.EqualValues(t, 1, atomic.LoadInt64(&subCount))
	require.EqualValues(t, 0, atomic.LoadInt64(&unsubCount))
	require.EqualValues(t, 0, atomic.LoadInt64(&offCount))

	// Nil store is a safe no-op.
	var nilStore *webhook.Store
	require.NotPanics(t, func() { nilStore.EmitEvent("run.stopped", "test_run", 1, "t", "") })
}

// TestWebhookSSRFBlock verifies the delivery client refuses loopback/private
// targets unless AllowPrivateTargets is explicitly enabled.
func TestWebhookSSRFBlock(t *testing.T) {
	prevPrivate := webhook.AllowPrivateTargets
	webhook.AllowPrivateTargets = false
	t.Cleanup(func() { webhook.AllowPrivateTargets = prevPrivate })

	ctx := context.Background()
	payload := webhook.Payload{Event: "webhook.test", Title: "blocked", Timestamp: time.Now().UTC().Format(time.RFC3339)}

	// A loopback target is blocked before the dial even when the port is closed.
	_, err := webhook.Deliver(ctx, webhook.Webhook{
		URL: "http://127.0.0.1:9/hook", Secret: "s",
	}, payload)
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked")

	// With private targets allowed, delivery to a local httptest server works.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	webhook.AllowPrivateTargets = true
	status, err := webhook.Deliver(ctx, webhook.Webhook{
		URL: srv.URL, Secret: "s",
	}, payload)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)

	webhook.AllowPrivateTargets = false
}

func TestWebhookEmitEventWithData(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	prevPrivate := webhook.AllowPrivateTargets
	webhook.AllowPrivateTargets = true
	t.Cleanup(func() { webhook.AllowPrivateTargets = prevPrivate })

	var gotPayload webhook.Payload
	var gotSig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotPayload)
		gotSig = r.Header.Get("X-Kiwit-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	_, err := store.Create(ctx, webhook.Webhook{
		Name: "bug-hook", URL: srv.URL, Secret: "test-secret", Events: []string{"bug.created"}, Active: true,
	})
	require.NoError(t, err)

	data := map[string]any{
		"bug_id":       int64(7),
		"execution_id": int64(3),
		"tracker":      "JIRA",
		"url":          "https://jira.example.com/browse/PROJ-42",
		"summary":      "Login fails",
		"reporter":     "admin",
	}
	store.EmitEventWithData("bug.created", "bug", 7, "Login fails", "desc", data)

	deadline := time.Now().Add(3 * time.Second)
	for gotPayload.Event == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, "bug.created", gotPayload.Event)
	require.Equal(t, "bug", gotPayload.EntityType)
	require.EqualValues(t, 7, gotPayload.EntityID)
	require.Equal(t, "Login fails", gotPayload.Title)
	require.Equal(t, "JIRA", gotPayload.Data["tracker"])
	require.Equal(t, "https://jira.example.com/browse/PROJ-42", gotPayload.Data["url"])
	require.Equal(t, "admin", gotPayload.Data["reporter"])
	require.NotEmpty(t, gotSig, "expected HMAC signature header")

	// Nil store no-op
	var nilStore *webhook.Store
	require.NotPanics(t, func() { nilStore.EmitEventWithData("bug.created", "bug", 1, "t", "", nil) })
}
