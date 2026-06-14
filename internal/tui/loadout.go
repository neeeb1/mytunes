package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/neeeb1/mytunes/internal/loadout"
)

// albumKey is the stable (Artist, Album) identity used to match albums across a
// fresh scan, matching diff.Build's key.
func albumKey(artist, album string) [2]string { return [2]string{artist, album} }

// checkedRefs collects a ref for every currently-checked album.
func (a *App) checkedRefs() []loadout.AlbumRef {
	var refs []loadout.AlbumRef
	for _, ar := range a.tree.Artists {
		for _, al := range ar.Albums {
			if al.Checked {
				refs = append(refs, loadout.AlbumRef{Artist: al.Artist, Album: al.Name})
			}
		}
	}
	return refs
}

// checkedSet snapshots the current checked albums as a set for drift detection.
func (a *App) checkedSet() map[[2]string]bool {
	set := map[[2]string]bool{}
	for _, ar := range a.tree.Artists {
		for _, al := range ar.Albums {
			if al.Checked {
				set[albumKey(al.Artist, al.Name)] = true
			}
		}
	}
	return set
}

// applyLoadout replaces the current selection with the loadout's exact set:
// its albums become checked, all others unchecked. It returns how many of the
// loadout's albums no longer exist on the server (stale refs). It also records
// the resulting checked set as the drift baseline.
func (a *App) applyLoadout(l loadout.Loadout) (missing int) {
	want := map[[2]string]bool{}
	for _, ref := range l.Albums {
		want[albumKey(ref.Artist, ref.Album)] = true
	}

	matched := 0
	for _, ar := range a.tree.Artists {
		for _, al := range ar.Albums {
			if want[albumKey(al.Artist, al.Name)] {
				al.Checked = true
				matched++
			} else {
				al.Checked = false
			}
		}
	}
	a.rebuildRows()
	a.loadoutBase = a.checkedSet()
	return len(want) - matched
}

// selectionChanged reports whether the current selection has drifted from the
// active loadout's baseline.
func (a *App) selectionChanged() bool {
	if a.activeLoadout == "" {
		return false
	}
	cur := a.checkedSet()
	if len(cur) != len(a.loadoutBase) {
		return true
	}
	for k := range cur {
		if !a.loadoutBase[k] {
			return true
		}
	}
	return false
}

// ---- picker screen ----

func (a *App) loadoutPickerKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	names := a.loadouts.Names()
	switch m.String() {
	case "esc", "q", "L":
		a.screen = screenBrowse
		return a, nil
	case "down", "j":
		if a.loadoutCursor < len(names)-1 {
			a.loadoutCursor++
		}
	case "up", "k":
		if a.loadoutCursor > 0 {
			a.loadoutCursor--
		}
	case "enter":
		if name, ok := pickName(names, a.loadoutCursor); ok {
			if l, ok := a.loadouts.Get(name); ok {
				missing := a.applyLoadout(*l)
				a.activeLoadout = name
				if missing > 0 {
					a.loadoutMsg = fmt.Sprintf("%s no longer on server", plural(missing, "album"))
				} else {
					a.loadoutMsg = "loaded loadout '" + name + "'"
				}
				a.screen = screenBrowse
			}
		}
	case "d":
		if name, ok := pickName(names, a.loadoutCursor); ok {
			a.loadouts.Delete(name)
			if err := a.loadouts.Save(); err != nil {
				a.err = err
			}
			if a.activeLoadout == name {
				a.activeLoadout = ""
				a.loadoutBase = nil
			}
			if a.loadoutCursor >= len(a.loadouts.Names()) {
				a.loadoutCursor = max(0, len(a.loadouts.Names())-1)
			}
		}
	case "r":
		if name, ok := pickName(names, a.loadoutCursor); ok {
			a.beginNameEntry(loadoutRename, name)
			return a, textinput.Blink
		}
	case "c":
		if name, ok := pickName(names, a.loadoutCursor); ok {
			a.beginNameEntry(loadoutDuplicate, name)
			return a, textinput.Blink
		}
	}
	return a, nil
}

func pickName(names []string, i int) (string, bool) {
	if i < 0 || i >= len(names) {
		return "", false
	}
	return names[i], true
}

func (a *App) viewLoadoutPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Loadouts") + "\n\n")

	names := a.loadouts.Names()
	if len(names) == 0 {
		b.WriteString(dimStyle.Render("  No loadouts saved yet.") + "\n\n")
		b.WriteString(helpStyle.Render("esc back"))
		return b.String()
	}

	for i, name := range names {
		cursor := "  "
		if i == a.loadoutCursor {
			cursor = cursorStyle.Render("› ")
		}
		count := 0
		if l, ok := a.loadouts.Get(name); ok {
			count = len(l.Albums)
		}
		line := name
		if name == a.activeLoadout {
			line += okStyle.Render(" (active)")
		}
		b.WriteString(cursor + line + dimStyle.Render("  "+plural(count, "album")) + "\n")
	}

	b.WriteByte('\n')
	b.WriteString(helpStyle.Render(
		"↑↓/jk move · enter load · r rename · c duplicate · d delete · esc back"))
	return b.String()
}

// ---- name-entry screen ----

func (a *App) beginNameEntry(action loadoutAction, source string) {
	a.loadoutAction = action
	a.loadoutSource = source
	a.loadoutNameErr = ""
	switch action {
	case loadoutSaveNew:
		a.loadoutName.SetValue("")
	case loadoutRename:
		a.loadoutName.SetValue(source)
	case loadoutDuplicate:
		a.loadoutName.SetValue(source + " copy")
	}
	a.loadoutName.CursorEnd()
	a.loadoutName.Focus()
	a.screen = screenLoadoutName
}

func (a *App) loadoutNameKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.String() {
	case "esc":
		a.loadoutName.Blur()
		a.screen = a.nameReturnScreen()
		return a, nil
	case "enter":
		return a.commitNameEntry()
	default:
		var cmd tea.Cmd
		a.loadoutName, cmd = a.loadoutName.Update(m)
		return a, cmd
	}
}

// nameReturnScreen is where the name-entry screen returns to: browse for a new
// save, the picker for rename/duplicate.
func (a *App) nameReturnScreen() screen {
	if a.loadoutAction == loadoutSaveNew {
		return screenBrowse
	}
	return screenLoadoutPicker
}

func (a *App) commitNameEntry() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(a.loadoutName.Value())
	if name == "" {
		a.loadoutNameErr = "name cannot be empty"
		return a, nil
	}

	switch a.loadoutAction {
	case loadoutSaveNew:
		a.loadouts.Put(loadout.Loadout{Name: name, Albums: a.checkedRefs()})
		a.activeLoadout = name
		a.loadoutBase = a.checkedSet()
		a.loadoutMsg = "saved loadout '" + name + "'"
	case loadoutRename:
		if !a.loadouts.Rename(a.loadoutSource, name) {
			a.loadoutNameErr = "name already in use"
			return a, nil
		}
		if a.activeLoadout == a.loadoutSource {
			a.activeLoadout = name
		}
	case loadoutDuplicate:
		if _, taken := a.loadouts.Get(name); taken {
			a.loadoutNameErr = "name already in use"
			return a, nil
		}
		src, ok := a.loadouts.Get(a.loadoutSource)
		if !ok {
			a.loadoutNameErr = "source loadout missing"
			return a, nil
		}
		albums := append([]loadout.AlbumRef(nil), src.Albums...)
		a.loadouts.Put(loadout.Loadout{Name: name, Albums: albums})
	}

	if err := a.loadouts.Save(); err != nil {
		a.err = err
	}
	a.loadoutName.Blur()
	a.screen = a.nameReturnScreen()
	return a, nil
}

func (a *App) viewLoadoutName() string {
	var b strings.Builder
	var title string
	switch a.loadoutAction {
	case loadoutSaveNew:
		title = "Save loadout"
	case loadoutRename:
		title = "Rename loadout '" + a.loadoutSource + "'"
	case loadoutDuplicate:
		title = "Duplicate loadout '" + a.loadoutSource + "'"
	}
	b.WriteString(titleStyle.Render(title) + "\n\n")

	if a.loadoutAction == loadoutSaveNew {
		b.WriteString(dimStyle.Render("  "+plural(len(a.checkedRefs()), "album")+" selected") + "\n\n")
	}

	b.WriteString("  " + a.loadoutName.View() + "\n\n")
	if a.loadoutNameErr != "" {
		b.WriteString(errStyle.Render("  "+a.loadoutNameErr) + "\n\n")
	}
	b.WriteString(helpStyle.Render("enter save · esc cancel"))
	return b.String()
}
