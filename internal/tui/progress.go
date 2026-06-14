package tui

import (
	"fmt"
	"strings"
)

func (a *App) viewProgress() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Syncing") + "\n\n")

	delLine := "  free: "
	if a.delTotal > 0 {
		delLine += fmt.Sprintf("%d/%d albums", a.delDone, a.delTotal)
	} else {
		delLine += dimStyle.Render("none")
	}
	if a.phase != "delete" && (a.delTotal == 0 || a.delDone == a.delTotal) {
		delLine += okStyle.Render("  ✓")
	}
	b.WriteString(delLine + "\n\n")

	b.WriteString("  copy:\n  " + a.prog.View() + "\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("  %.0f%%", a.copyPercent)) + "\n")
	if a.curFile != "" {
		b.WriteString(dimStyle.Render("  "+truncateMiddle(a.curFile, max(a.width-4, 20))) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("transferring… ctrl+c to abort"))
	return b.String()
}

func (a *App) viewDone() string {
	var b strings.Builder
	if a.err != nil {
		b.WriteString(errStyle.Render("Sync failed") + "\n\n")
		b.WriteString("  " + a.err.Error() + "\n\n")
		b.WriteString(helpStyle.Render("any key to quit"))
		return b.String()
	}
	b.WriteString(okStyle.Render("Done") + "\n\n")
	s := a.summary
	b.WriteString(fmt.Sprintf("  copied  %s (%s)\n", plural(s.CopyAlbums, "album"), humanBytes(s.CopyBytes)))
	b.WriteString(fmt.Sprintf("  updated %s\n", plural(s.UpdateAlbums, "album")))
	b.WriteString(fmt.Sprintf("  freed   %s (%s, local)\n\n", plural(s.DeleteAlbums, "album"), humanBytes(s.DeleteBytes)))
	if a.dryRun {
		b.WriteString(dimStyle.Render("  [dry-run: nothing was actually changed]") + "\n\n")
	}
	b.WriteString(helpStyle.Render("any key to quit"))
	return b.String()
}
