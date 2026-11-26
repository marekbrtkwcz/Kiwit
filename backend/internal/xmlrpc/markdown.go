package xmlrpc

import (
	"bytes"

	"github.com/yuin/goldmark"
)

// renderMarkdown converts Markdown to HTML (raw HTML escaped by default).
func renderMarkdown(text string) string {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(text), &buf); err != nil {
		return ""
	}
	return buf.String()
}
