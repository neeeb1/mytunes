package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// fits reports whether the queued work fits in free space. Valid because the
// sync runs deletes before copies: need = copy + update − delete.
func (a *App) fits() bool {
	return a.summary.NeedBytes() <= a.free
}

func (a *App) confirmKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "e", "esc":
		a.screen = screenBrowse
		return a, nil
	case "q":
		return a, tea.Quit
	case "u":
		if a.activeLoadout != "" && a.selectionChanged() {
			if l, ok := a.loadouts.Get(a.activeLoadout); ok {
				l.Albums = a.checkedRefs()
				a.loadouts.Put(*l)
				if err := a.loadouts.Save(); err != nil {
					a.err = err
				}
				a.loadoutBase = a.checkedSet() // clears drift
			}
		}
		return a, nil
	case "y":
		if !a.fits() {
			return a, nil // blocked; user must edit
		}
		if a.summary.CopyAlbums+a.summary.UpdateAlbums+a.summary.DeleteAlbums == 0 {
			return a, tea.Quit // nothing to do
		}
		a.screen = screenSyncing
		a.phase = "delete"
		return a, a.runSyncCmd()
	}
	return a, nil
}

func (a *App) viewConfirm() string {
	s := a.summary
	var b strings.Builder
	b.WriteString(titleStyle.Render("Confirm sync") + "\n\n")

	b.WriteString(tagCopy.Render("  copy   ") +
		fmtline(s.CopyAlbums, s.CopyBytes) + "\n")
	b.WriteString(tagUpdate.Render("  update ") +
		fmtline(s.UpdateAlbums, s.UpdateBytes) + "\n")
	b.WriteString(tagDelete.Render("  delete ") +
		fmtline(s.DeleteAlbums, s.DeleteBytes) + dimStyle.Render("  (local only)") + "\n\n")

	b.WriteString("  net space needed: " + humanBytes(s.NeedBytes()) + "\n")
	b.WriteString("  destination free: " + humanBytes(a.free) + "\n\n")

	if a.dryRun {
		b.WriteString(dimStyle.Render("  [dry-run: no files will change]") + "\n\n")
	}

	if !a.fits() {
		b.WriteString(errStyle.Render("  ✗ Not enough free space. Deselect items (e) to continue.") + "\n\n")
		b.WriteString(helpStyle.Render("e edit · q abort"))
		return b.String()
	}

	if s.CopyAlbums+s.UpdateAlbums+s.DeleteAlbums == 0 {
		b.WriteString(okStyle.Render("  Nothing queued — already up to date.") + "\n\n")
		b.WriteString(helpStyle.Render("e edit · q quit"))
		return b.String()
	}

	help := "y proceed (deletes first, then copies) · e edit · q abort"
	if a.activeLoadout != "" && a.selectionChanged() {
		b.WriteString(dimStyle.Render("  loadout '"+a.activeLoadout+"' has unsaved changes") + "\n\n")
		help += " · u update loadout"
	}
	b.WriteString(helpStyle.Render(help))
	return b.String()
}

func fmtline(albums int, bytes int64) string {
	return strings.TrimRight(
		strings.Join([]string{
			plural(albums, "album"),
			"(" + humanBytes(bytes) + ")",
		}, " "), " ")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}
