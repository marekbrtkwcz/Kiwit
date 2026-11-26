package logging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/kiwit/kiwit/internal/platform/secrets"
)

// LogEntry is one log record kept for the in-app log viewer.
type LogEntry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Attrs   map[string]string `json:"attrs,omitempty"`
}

// Ring is an slog.Handler keeping the most recent entries in memory so the
// admin UI can show them even when stdout is invisible (desktop exe).
type Ring struct {
	mu      sync.Mutex
	entries []LogEntry
	max     int
	min     slog.Level
}

func NewRing(max int, min slog.Level) *Ring {
	if max <= 0 {
		max = 2000
	}
	return &Ring{entries: []LogEntry{}, max: max, min: min}
}

func (r *Ring) Enabled(_ context.Context, l slog.Level) bool { return l >= r.min }

func (r *Ring) Handle(_ context.Context, rec slog.Record) error {
	attrs := map[string]string{}
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	e := LogEntry{Time: rec.Time, Level: strings.ToLower(rec.Level.String()), Message: secrets.DefaultVault().Redact(rec.Message), Attrs: attrs}
	for k, val := range attrs {
		attrs[k] = secrets.DefaultVault().Redact(val)
	}
	if len(attrs) == 0 {
		e.Attrs = nil
	}
	r.mu.Lock()
	r.entries = append(r.entries, e)
	if len(r.entries) > r.max {
		r.entries = r.entries[len(r.entries)-r.max:]
	}
	r.mu.Unlock()
	return nil
}

func (r *Ring) WithAttrs(_ []slog.Attr) slog.Handler { return r }
func (r *Ring) WithGroup(_ string) slog.Handler      { return r }

var levelRank = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// Snapshot returns up to limit entries at or above the given level,
// newest first. Empty level means all levels.
func (r *Ring) Snapshot(level string, limit int) []LogEntry {
	if limit <= 0 || limit > r.max {
		limit = 200
	}
	min := slog.LevelDebug
	if l, ok := levelRank[level]; ok {
		min = l
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []LogEntry{}
	for i := len(r.entries) - 1; i >= 0 && len(out) < limit; i-- {
		e := r.entries[i]
		if lvl, ok := levelRank[e.Level]; ok && lvl < min {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Fanout forwards each record to every handler that has it enabled.
type Fanout struct {
	handlers []slog.Handler
}

func NewFanout(handlers ...slog.Handler) *Fanout { return &Fanout{handlers: handlers} }

func (f *Fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f *Fanout) Handle(ctx context.Context, rec slog.Record) error {
	for _, h := range f.handlers {
		if h.Enabled(ctx, rec.Level) {
			if err := h.Handle(ctx, rec); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return NewFanout(hs...)
}

func (f *Fanout) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		hs[i] = h.WithGroup(name)
	}
	return NewFanout(hs...)
}

// RotateWriter is an io.Writer that appends to a file and rotates it by size:
// when path exceeds maxSize bytes it is renamed to path.1 (shifting older
// suffixes up to maxFiles, deleting the oldest).
type RotateWriter struct {
	mu       sync.Mutex
	path     string
	maxSize  int64
	maxFiles int
	f        *os.File
	size     int64
}

func NewRotateWriter(path string, maxSize int64, maxFiles int) (*RotateWriter, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	if maxFiles < 1 {
		maxFiles = 1
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	size := int64(0)
	if st, err := f.Stat(); err == nil {
		size = st.Size()
	}
	return &RotateWriter{path: path, maxSize: maxSize, maxFiles: maxFiles, f: f, size: size}, nil
}

func (w *RotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

func (w *RotateWriter) rotate() error {
	_ = w.f.Close()
	_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.maxFiles))
	for i := w.maxFiles - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	if err := os.Rename(w.path, w.path+".1"); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.f = f
	w.size = 0
	return nil
}
