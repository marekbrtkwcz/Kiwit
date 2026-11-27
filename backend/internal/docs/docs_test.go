package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestContentMatchesRepoDocs fails when the embedded copy drifts from the
// repository docs/ folder. Fix by running `make sync-docs` before committing.
func TestContentMatchesRepoDocs(t *testing.T) {
	repoDocs := filepath.Join("..", "..", "..", "docs")
	entries, err := os.ReadDir(repoDocs)
	if err != nil {
		t.Skipf("repo docs/ not available (running from an exported module): %v", err)
	}
	repoFiles := map[string]bool{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			repoFiles[e.Name()] = true
		}
	}
	if len(repoFiles) == 0 {
		t.Fatal("no markdown files found in repo docs/")
	}

	embedded, err := fs.ReadDir(contentFS, "content")
	if err != nil {
		t.Fatalf("read embedded content: %v", err)
	}
	embeddedFiles := map[string]bool{}
	for _, e := range embedded {
		if strings.HasSuffix(e.Name(), ".md") {
			embeddedFiles[e.Name()] = true
		}
	}

	for name := range repoFiles {
		if !embeddedFiles[name] {
			t.Errorf("%s exists in docs/ but is missing from the embedded copy — run `make sync-docs`", name)
			continue
		}
		want, err := os.ReadFile(filepath.Join(repoDocs, name))
		if err != nil {
			t.Fatalf("read repo %s: %v", name, err)
		}
		got, err := fs.ReadFile(contentFS, "content/"+name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		if string(want) != string(got) {
			t.Errorf("%s differs between docs/ and the embedded copy — run `make sync-docs`", name)
		}
	}
	for name := range embeddedFiles {
		if !repoFiles[name] {
			t.Errorf("%s exists in the embedded copy but not in docs/ — run `make sync-docs`", name)
		}
	}
}
