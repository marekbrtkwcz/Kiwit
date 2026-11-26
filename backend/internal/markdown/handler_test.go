package markdown

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

func TestRender(t *testing.T) {
	_, api := humatest.New(t, huma.DefaultConfig("test", "0.0.1"))
	RegisterRoutes(api)

	resp := api.Post("/api/v1/markdown/render", strings.NewReader(`{"text":"**bold** <script>x</script>"}`))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "<strong>bold</strong>") {
		t.Fatalf("expected rendered strong tag, got: %s", body)
	}
	if strings.Contains(body, "<script>") {
		t.Fatalf("raw HTML must be escaped, got: %s", body)
	}
}

var _ = context.Background
