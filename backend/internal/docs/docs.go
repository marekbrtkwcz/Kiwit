// Package docs serves the embedded documentation (repo docs/ folder) at
// /docs/<name>.md so the Help menu links work offline in self-hosted
// instances. The content copy is refreshed by `make sync-docs`.
package docs

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed all:content
var contentFS embed.FS

// Register mounts GET /docs/{name}.md on the router.
func Register(r chi.Router) {
	sub, err := fs.Sub(contentFS, "content")
	if err != nil {
		panic(err)
	}
	r.Get("/docs/{name}", func(w http.ResponseWriter, req *http.Request) {
		name := chi.URLParam(req, "name")
		if name == "" || strings.Contains(name, "/") || !strings.HasSuffix(name, ".md") {
			http.NotFound(w, req)
			return
		}
		b, err := fs.ReadFile(sub, name)
		if err != nil {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = w.Write(b)
	})
}
