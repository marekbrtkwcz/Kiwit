package llm

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"log/slog"

	"github.com/kiwit/kiwit/internal/platform/secrets"
)

func TestPostLogsCallWithoutSecrets(t *testing.T) {
	// Same sink wiring as production: a JSON handler over a redacting writer,
	// so registered secrets cannot reach the output.
	var buf bytes.Buffer
	sink := secrets.DefaultVault().Writer(&buf)
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(sink, nil)))
	defer slog.SetDefault(prev)

	const key = "sk-test-1234abcd"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			// echo the offending value so it would leak into the error message
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"bad key ` + r.Header.Get("Authorization") + `"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	// Success: one "llm call" INFO entry, no key anywhere.
	out, err := Config{APIKey: key, BaseURL: srv.URL, Model: "test-model"}.Chat(context.Background(), "sys", "hello")
	if err != nil || out != "ok" {
		t.Fatalf("chat failed: %q %v", out, err)
	}
	logs := buf.String()
	if !strings.Contains(logs, `"msg":"llm call"`) || !strings.Contains(logs, `"ok":true`) || !strings.Contains(logs, "test-model") {
		t.Fatalf("success call not logged: %s", logs)
	}
	if strings.Contains(logs, key) {
		t.Fatalf("API key leaked into logs: %s", logs)
	}

	// Failure: WARN entry, and the key echoed by the server must be redacted.
	buf.Reset()
	_, err = Config{APIKey: key + "wrong", BaseURL: srv.URL, Model: "test-model"}.Chat(context.Background(), "sys", "hello")
	if err == nil {
		t.Fatal("expected failure")
	}
	logs = buf.String()
	if !strings.Contains(logs, `"msg":"llm call"`) || !strings.Contains(logs, `"ok":false`) {
		t.Fatalf("failed call not logged: %s", logs)
	}
	if strings.Contains(logs, key+"wrong") {
		t.Fatalf("API key leaked into error log: %s", logs)
	}
}
