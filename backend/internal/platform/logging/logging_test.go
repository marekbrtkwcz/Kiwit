package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRingKeepsRecentEntries(t *testing.T) {
	r := NewRing(3, slog.LevelInfo)
	logger := slog.New(r)
	logger.Info("one")
	logger.Warn("two")
	logger.Error("three")
	logger.Info("four") // evicts "one"

	got := r.Snapshot("", 10)
	if len(got) != 3 {
		t.Fatalf("want 3 entries, got %d", len(got))
	}
	if got[0].Message != "four" || got[1].Message != "three" || got[2].Message != "two" {
		t.Fatalf("newest-first order broken: %+v", got)
	}
}

func TestRingLevelFilterAndDebugExcluded(t *testing.T) {
	r := NewRing(10, slog.LevelInfo)
	logger := slog.New(r)
	logger.Debug("hidden")
	logger.Info("info entry")
	logger.Error("err entry")

	got := r.Snapshot("warn", 10)
	if len(got) != 1 || got[0].Message != "err entry" {
		t.Fatalf("level filter failed: %+v", got)
	}
	if len(r.Snapshot("debug", 10)) != 2 {
		t.Fatal("debug entries must not enter the ring")
	}
}

func TestRingAttrs(t *testing.T) {
	r := NewRing(2, slog.LevelInfo)
	slog.New(r).Error("boom", "err", "disk full", "status", 500)
	got := r.Snapshot("", 10)
	if got[0].Attrs["err"] != "disk full" || got[0].Attrs["status"] != "500" {
		t.Fatalf("attrs not captured: %+v", got[0])
	}
}

func TestFanoutWritesToAllHandlers(t *testing.T) {
	var buf bytes.Buffer
	r := NewRing(5, slog.LevelInfo)
	logger := slog.New(NewFanout(slog.NewJSONHandler(&buf, nil), r))
	logger.Info("hello", "k", "v")

	if !strings.Contains(buf.String(), `"msg":"hello"`) {
		t.Fatalf("primary handler output missing: %s", buf.String())
	}
	if r.Snapshot("", 5)[0].Message != "hello" {
		t.Fatal("ring handler did not receive the record")
	}
}

func TestRotateWriterRotatesBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kiwit.log")
	w, err := NewRotateWriter(path, 64, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Three writes of 40 bytes: second crosses 64B -> rotate, third too.
	for i := 0; i < 3; i++ {
		if _, err := w.Write([]byte(strings.Repeat("x", 40))); err != nil {
			t.Fatal(err)
		}
	}
	_ = w.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("active log missing: %v", err)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("first rotation missing: %v", err)
	}
	st, err := os.Stat(path + ".2")
	if err != nil {
		t.Fatalf("second rotation missing: %v", err)
	}
	if st.Size() == 0 {
		t.Fatal("shifted file must keep content")
	}
	if _, err := os.Stat(path + ".3"); !os.IsNotExist(err) {
		t.Fatal("maxFiles=2 must delete the oldest file")
	}
}
