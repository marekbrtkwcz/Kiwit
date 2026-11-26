package markdown

import (
	"bytes"
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/yuin/goldmark"
)

// RegisterRoutes mounts POST /api/v1/markdown/render — renders Markdown to sanitized HTML.
func RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "markdown-render",
		Method:      http.MethodPost,
		Path:        "/api/v1/markdown/render",
		Summary:     "Render Markdown text to HTML (raw HTML escaped)",
		Tags:        []string{"Utils"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Text string `json:"text" maxLength:"100000"`
		}
	}) (*struct {
		Body struct {
			HTML string `json:"html"`
		}
	}, error) {
		// goldmark with default options escapes raw HTML in the source (Unsafe=false),
		// so the output is safe to embed.
		var buf bytes.Buffer
		if err := goldmark.Convert([]byte(input.Body.Text), &buf); err != nil {
			return nil, huma.Error400BadRequest("failed to render markdown")
		}
		out := &struct {
			Body struct {
				HTML string `json:"html"`
			}
		}{}
		out.Body.HTML = buf.String()
		return out, nil
	})
}
