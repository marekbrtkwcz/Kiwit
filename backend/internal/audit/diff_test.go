package audit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnifiedDiff(t *testing.T) {
	d := UnifiedDiff("notes", "line1\nline2", "line1\nline3")
	require.Contains(t, d, "--- notes (old)")
	require.Contains(t, d, "+++ notes (new)")
	require.Contains(t, d, "-line2")
	require.Contains(t, d, "+line3")
	require.Contains(t, d, " line1")
	require.Empty(t, UnifiedDiff("notes", "same", "same"))
}

func TestDiffFields(t *testing.T) {
	oldV := map[string]any{"summary": "old", "status": "draft", "notes": "a\nb"}
	newV := map[string]any{"summary": "new", "status": "draft", "notes": "a\nc"}
	co, cn, text := DiffFields(oldV, newV)
	require.Equal(t, map[string]any{"summary": "old", "notes": "a\nb"}, co)
	require.Equal(t, map[string]any{"summary": "new", "notes": "a\nc"}, cn)
	require.Contains(t, text, "-old")
	require.Contains(t, text, "+new")
	require.Contains(t, text, "-b")
	require.Contains(t, text, "+c")
}
