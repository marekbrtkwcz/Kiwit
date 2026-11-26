package audit

import (
	"fmt"
	"strings"
)

// UnifiedDiff produces a unified-style diff between old and new text values
// for a single field, using a simple LCS over lines.
func UnifiedDiff(field, oldVal, newVal string) string {
	if oldVal == newVal {
		return ""
	}
	oldLines := strings.Split(oldVal, "\n")
	newLines := strings.Split(newVal, "\n")
	lcs := longestCommonSubseq(oldLines, newLines)
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (old)\n+++ %s (new)\n", field, field)
	i, j, k := 0, 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if k < len(lcs) && i < len(oldLines) && j < len(newLines) && oldLines[i] == lcs[k] && newLines[j] == lcs[k] {
			fmt.Fprintf(&b, " %s\n", oldLines[i])
			i++
			j++
			k++
			continue
		}
		if i < len(oldLines) && (k >= len(lcs) || j >= len(newLines) || oldLines[i] != lcs[k]) {
			fmt.Fprintf(&b, "-%s\n", oldLines[i])
			i++
			continue
		}
		if j < len(newLines) {
			fmt.Fprintf(&b, "+%s\n", newLines[j])
			j++
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func longestCommonSubseq(a, b []string) []string {
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	out := []string{}
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			out = append(out, a[i])
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}
	return out
}

// DiffFields compares old and new field maps and returns:
// changed fields as old/new maps plus a human-readable unified diff string.
func DiffFields(oldVals, newVals map[string]any) (map[string]any, map[string]any, string) {
	changedOld := map[string]any{}
	changedNew := map[string]any{}
	var parts []string
	keys := map[string]bool{}
	for k := range oldVals {
		keys[k] = true
	}
	for k := range newVals {
		keys[k] = true
	}
	for k := range keys {
		o, n := oldVals[k], newVals[k]
		if fmt.Sprintf("%v", o) == fmt.Sprintf("%v", n) {
			continue
		}
		changedOld[k] = o
		changedNew[k] = n
		os, _ := o.(string)
		ns, _ := n.(string)
		if os != "" || ns != "" {
			if d := UnifiedDiff(k, os, ns); d != "" {
				parts = append(parts, d)
			}
		} else {
			parts = append(parts, fmt.Sprintf("%s: %v -> %v", k, o, n))
		}
	}
	return changedOld, changedNew, strings.Join(parts, "\n\n")
}
