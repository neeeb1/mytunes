package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// humanBytes formats a byte count as IEC units (e.g. 5.8 GiB), mirroring the
// original script's `numfmt --to=iec-i`.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// truncateMiddle shortens s to at most width runes, collapsing the middle with
// an ellipsis so both the leading album dir and the trailing file name stay
// visible. Width is counted in runes, which is good enough for the path display.
func truncateMiddle(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}
	keep := width - 1 // room for the ellipsis
	head := keep / 2
	tail := keep - head
	return string(r[:head]) + "…" + string(r[len(r)-tail:])
}

// expandHome resolves a leading ~ to the user's home directory.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// splitPath divides a typed path into a directory portion (kept verbatim, so a
// leading ~ or a relative path is preserved) and the final segment to complete.
func splitPath(in string) (dir, base string) {
	if i := strings.LastIndex(in, "/"); i >= 0 {
		return in[:i+1], in[i+1:]
	}
	return "", in
}

// dirMatches lists the subdirectories of dir whose names start with base,
// sorted. dir may contain a leading ~. Returns nil if dir can't be read.
func dirMatches(dir, base string) []string {
	readDir := expandHome(dir)
	if readDir == "" {
		readDir = "."
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), base) {
			matches = append(matches, e.Name())
		}
	}
	return matches // os.ReadDir already returns entries sorted by name
}

// completionHint renders the candidate directory names under a path input,
// highlighting the active one while cycling. The list is windowed so a large
// directory doesn't flood the line.
func completionHint(cands []string, active int) string {
	if len(cands) == 0 {
		return ""
	}
	const win = 10
	start := 0
	if active >= 0 {
		start = active - win/2
	}
	if start < 0 {
		start = 0
	}
	end := start + win
	if end > len(cands) {
		end = len(cands)
		start = max(0, end-win)
	}

	var parts []string
	if start > 0 {
		parts = append(parts, dimStyle.Render("…"))
	}
	for i := start; i < end; i++ {
		if i == active {
			parts = append(parts, cursorStyle.Render(cands[i]))
		} else {
			parts = append(parts, dimStyle.Render(cands[i]))
		}
	}
	if end < len(cands) {
		parts = append(parts, dimStyle.Render(fmt.Sprintf("…(+%d)", len(cands)-end)))
	}
	return "  " + strings.Join(parts, "  ")
}

func commonPrefix(a, b string) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}
