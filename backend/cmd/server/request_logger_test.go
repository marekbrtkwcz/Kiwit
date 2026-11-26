package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"log/slog"
)

func TestRequestLoggerPromotesSlowRequestsToWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	slow := requestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1100 * time.Millisecond)
	}))
	slow.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/slow", nil))
	if out := buf.String(); !strings.Contains(out, `"msg":"slow request"`) || !strings.Contains(out, `"level":"WARN"`) {
		t.Fatalf("slow request not logged at WARN: %s", out)
	}

	buf.Reset()
	fast := requestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	fast.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/fast", nil))
	if out := buf.String(); !strings.Contains(out, `"msg":"http request"`) || !strings.Contains(out, `"level":"INFO"`) {
		t.Fatalf("fast request not logged at INFO: %s", out)
	}
}
