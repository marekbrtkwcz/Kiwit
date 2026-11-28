package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewLocal(dir)
	ctx := context.Background()

	path, err := s.Put(ctx, "attachments/2026/report.pdf", bytes.NewReader([]byte("hello kiwit")), 11)
	if err != nil {
		t.Fatal(err)
	}
	// The stored path must stay inside dir and use the base name only.
	if filepath.Dir(path) != dir {
		t.Errorf("path %q escapes storage dir %q", path, dir)
	}
	if filepath.Base(path) != "report.pdf" {
		t.Errorf("expected base name report.pdf, got %q", filepath.Base(path))
	}

	rc, err := s.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil || string(data) != "hello kiwit" {
		t.Errorf("round trip mismatch: %q err=%v", data, err)
	}

	if err := s.Delete(ctx, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone after Delete")
	}
	// Deleting a missing object is not an error.
	if err := s.Delete(ctx, path); err != nil {
		t.Errorf("delete of missing object should be nil, got %v", err)
	}
}

func TestLocalStoragePutSanitizesKey(t *testing.T) {
	dir := t.TempDir()
	s := NewLocal(dir)
	path, err := s.Put(context.Background(), "../../etc/evil.txt", strings.NewReader("x"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("path traversal not sanitized: %q", path)
	}
}

func TestLocalStorageOpenMissing(t *testing.T) {
	s := NewLocal(t.TempDir())
	if _, err := s.Open(context.Background(), filepath.Join(t.TempDir(), "nope.bin")); err == nil {
		t.Error("expected error opening missing file")
	}
}
