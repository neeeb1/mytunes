package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neeeb1/mytunes/internal/diff"
)

type rowKind int

const (
	artistRow rowKind = iota
	albumRow
)

type row struct {
	kind   rowKind
	artist *diff.Artist
	album  *diff.Album
}

// expandAll sets every artist's expansion state.
func (a *App) expandAll(v bool) {
	for _, ar := range a.tree.Artists {
		ar.Expanded = v
	}
}

// matches reports whether the artist (or any of its albums) matches the active
// filter. An empty filter matches everything.
func (a *App) artistMatches(ar *diff.Artist) bool {
	q := strings.ToLower(strings.TrimSpace(a.filterInput.Value()))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(ar.Name), q) {
		return true
	}
	for _, al := range ar.Albums {
		if strings.Contains(strings.ToLower(al.Name), q) {
			return true
		}
	}
	return false
}

func (a *App) albumMatches(al *diff.Album) bool {
	q := strings.ToLower(strings.TrimSpace(a.filterInput.Value()))
	if q == "" {
		return true
	}
	return strings.Contains(strings.ToLower(al.Name), q) ||
		strings.Contains(strings.ToLower(al.Artist), q)
}

// rebuildRows flattens the tree into the currently visible rows, honoring
// expansion and the active filter (matched artists auto-expand).
func (a *App) rebuildRows() {
	a.rows = a.rows[:0]
	filtering := strings.TrimSpace(a.filterInput.Value()) != ""
	for _, ar := range a.tree.Artists {
		if !a.artistMatches(ar) {
			continue
		}
		a.rows = append(a.rows, row{kind: artistRow, artist: ar})
		if ar.Expanded || filtering {
			for _, al := range ar.Albums {
				if a.albumMatches(al) {
					a.rows = append(a.rows, row{kind: albumRow, album: al})
				}
			}
		}
	}
	if a.cursor >= len(a.rows) {
		a.cursor = max(0, len(a.rows)-1)
	}
}

func (a *App) browseKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.filtering {
		switch m.String() {
		case "enter", "esc":
			a.filtering = false
			a.filterInput.Blur()
			a.rebuildRows()
			return a, nil
		default:
			var cmd tea.Cmd
			a.filterInput, cmd = a.filterInput.Update(m)
			a.rebuildRows()
			return a, cmd
		}
	}

	a.loadoutMsg = "" // clear any transient note on the next interaction

	switch m.String() {
	case "q", "esc":
		return a, tea.Quit
	case "ctrl+c":
		return a, tea.Quit
	case "down", "j":
		a.moveCursor(1)
	case "up", "k":
		a.moveCursor(-1)
	case "right", "l", "enter":
		a.setExpanded(true)
	case "left", "h":
		a.setExpanded(false)
	case " ":
		a.toggleCurrent()
	case "a":
		a.checkCurrentArtist(true)
	case "n":
		a.checkCurrentArtist(false)
	case "A":
		a.checkAll(true)
	case "N":
		a.checkAll(false)
	case "/":
		a.filtering = true
		a.filterInput.Focus()
		return a, textinput.Blink
	case "r":
		a.screen = screenLoading
		a.loadMsg = "Re-scanning…"
		return a, tea.Batch(a.spinner.Tick, a.scanCmd())
	case "s":
		a.screen = screenLoading
		a.loadMsg = "Computing transfer size (rsync dry-run)…"
		return a, tea.Batch(a.spinner.Tick, a.sizingCmd())
	case "S":
		a.beginNameEntry(loadoutSaveNew, "")
		return a, textinput.Blink
	case "L":
		a.loadoutCursor = 0
		a.screen = screenLoadoutPicker
		return a, nil
	}
	return a, nil
}

func (a *App) moveCursor(d int) {
	a.cursor += d
	if a.cursor < 0 {
		a.cursor = 0
	}
	if a.cursor >= len(a.rows) {
		a.cursor = len(a.rows) - 1
	}
}

func (a *App) currentRow() (row, bool) {
	if a.cursor < 0 || a.cursor >= len(a.rows) {
		return row{}, false
	}
	return a.rows[a.cursor], true
}

func (a *App) setExpanded(v bool) {
	r, ok := a.currentRow()
	if !ok {
		return
	}
	if r.kind == artistRow {
		r.artist.Expanded = v
		a.rebuildRows()
	}
}

func (a *App) toggleCurrent() {
	r, ok := a.currentRow()
	if !ok {
		return
	}
	if r.kind == albumRow {
		r.album.Checked = !r.album.Checked
		return
	}
	// Artist row: tri-state toggle — if all checked, clear; else check all.
	all, _ := r.artist.Tristate()
	r.artist.SetChecked(!all)
}

// currentArtist returns the artist the cursor is on: the artist itself for an
// artist row, or the owning artist for an album row.
func (a *App) currentArtist() (*diff.Artist, bool) {
	r, ok := a.currentRow()
	if !ok {
		return nil, false
	}
	if r.kind == artistRow {
		return r.artist, true
	}
	for _, ar := range a.tree.Artists {
		if ar.Name == r.album.Artist {
			return ar, true
		}
	}
	return nil, false
}

// checkCurrentArtist checks or unchecks every album of the highlighted artist,
// whether or not that artist is expanded.
func (a *App) checkCurrentArtist(v bool) {
	if ar, ok := a.currentArtist(); ok {
		ar.SetChecked(v)
	}
}

// checkAll checks or unchecks every album in the whole tree.
func (a *App) checkAll(v bool) {
	for _, ar := range a.tree.Artists {
		ar.SetChecked(v)
	}
}

func (a *App) viewBrowse() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("myTunes") + dimStyle.Render("  "+a.dest))
	if a.dryRun {
		b.WriteString(dimStyle.Render("  [dry-run]"))
	}
	if a.activeLoadout != "" {
		b.WriteString(dimStyle.Render("  loadout: " + a.activeLoadout))
	}
	b.WriteByte('\n')
	if a.err != nil {
		b.WriteString(errStyle.Render("error: "+a.err.Error()) + "\n")
	}
	if a.loadoutMsg != "" {
		b.WriteString(okStyle.Render(a.loadoutMsg) + "\n")
	}
	b.WriteString(a.sizeLine() + "\n")
	b.WriteByte('\n')

	visible := a.height - 6
	if visible < 3 {
		visible = 3
	}
	a.scrollInto(visible)

	end := min(a.top+visible, len(a.rows))
	for i := a.top; i < end; i++ {
		b.WriteString(a.renderRow(i) + "\n")
	}

	b.WriteByte('\n')
	if a.filtering {
		b.WriteString(a.filterInput.View() + "\n")
	}
	b.WriteString(helpStyle.Render(
		"↑↓/jk move · space toggle · a/n artist · A/N all · / filter · S save · L loadouts · s summary · q quit"))
	return b.String()
}

// sizeLine renders a live, rsync-free estimate of the selection's space cost and
// warns when it exceeds the destination's free space. a.free is 0 when df failed
// at scan time, in which case the warning is omitted.
func (a *App) sizeLine() string {
	s := a.tree.Summarize()
	add := s.CopyBytes + s.UpdateBytes
	line := "selected: +" + humanBytes(add) +
		" · frees " + humanBytes(s.DeleteBytes) +
		" · net " + humanBytes(s.NeedBytes())
	if a.free <= 0 {
		return dimStyle.Render(line)
	}
	line += " / " + humanBytes(a.free) + " free"
	if s.NeedBytes() > a.free {
		return dimStyle.Render("selected: +"+humanBytes(add)+" · frees "+humanBytes(s.DeleteBytes)+" · ") +
			errStyle.Render("net "+humanBytes(s.NeedBytes())+" / "+humanBytes(a.free)+" free ⚠ exceeds free space")
	}
	return dimStyle.Render(line)
}

// scrollInto keeps the cursor within the visible window.
func (a *App) scrollInto(visible int) {
	if a.cursor < a.top {
		a.top = a.cursor
	}
	if a.cursor >= a.top+visible {
		a.top = a.cursor - visible + 1
	}
	if a.top < 0 {
		a.top = 0
	}
}

func (a *App) renderRow(i int) string {
	r := a.rows[i]
	cursor := "  "
	if i == a.cursor {
		cursor = cursorStyle.Render("› ")
	}

	if r.kind == artistRow {
		ar := r.artist
		arrow := "▶"
		if ar.Expanded || strings.TrimSpace(a.filterInput.Value()) != "" {
			arrow = "▼"
		}
		synced, queued := 0, 0
		for _, al := range ar.Albums {
			if al.State == diff.Synced {
				synced++
			}
			if al.Action() != diff.None {
				queued++
			}
		}
		stats := dimStyle.Render(fmt.Sprintf("[%d/%d synced · %d queued]", synced, len(ar.Albums), queued))
		return fmt.Sprintf("%s%s %s %s %s", cursor, checkbox(ar), arrow, artistStyle.Render(ar.Name), stats)
	}

	al := r.album
	st := stateStyle(al.State)
	name := st.Render(fmt.Sprintf("%s %s", al.State.Icon(), al.Name))
	size := dimStyle.Render(humanBytes(al.RemoteSize()))
	if al.State == diff.LocalOnly {
		size = dimStyle.Render(humanBytes(al.LocalSize()) + " local")
	}
	tag := actionTag(al.Action())
	return fmt.Sprintf("%s   %s %s  %s  %s", cursor, albumBox(al), name, size, tag)
}

// checkbox renders an artist tri-state box.
func checkbox(ar *diff.Artist) string {
	all, none := ar.Tristate()
	switch {
	case all:
		return "[x]"
	case none:
		return "[ ]"
	default:
		return "[~]"
	}
}

// albumBox renders an album checkbox; LocalOnly is shown distinctly since its
// only meaningful intent is delete.
func albumBox(al *diff.Album) string {
	if al.Checked {
		if al.State == diff.LocalOnly {
			return "[✗]" // marked for deletion
		}
		return "[x]"
	}
	return "[ ]"
}
