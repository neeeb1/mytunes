package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/neeeb1/mytunes/internal/config"
	"github.com/neeeb1/mytunes/internal/diff"
	"github.com/neeeb1/mytunes/internal/loadout"
)

func newApp() *App {
	return New(config.Config{}, false, false)
}

// validateMsg from a real (writable temp) dir should advance dest → loading.
func TestValidateAdvancesToLoading(t *testing.T) {
	a := newApp()
	tmp := t.TempDir()
	model, _ := a.Update(validateMsg{dest: tmp})
	got := model.(*App)
	if got.screen != screenLoading {
		t.Fatalf("screen = %v, want loading", got.screen)
	}
	if got.dest != tmp {
		t.Errorf("dest = %q, want %q", got.dest, tmp)
	}
}

func TestValidateErrorStaysOnDest(t *testing.T) {
	a := newApp()
	a.screen = screenDest
	model, _ := a.Update(validateMsg{err: errFake})
	got := model.(*App)
	if got.screen != screenDest || got.err == nil {
		t.Fatalf("screen=%v err=%v, want dest + error", got.screen, got.err)
	}
}

var errFake = fakeErr("bad path")

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

// Toggling an album, then rebuilding rows, reflects the queued action.
func TestBrowseToggleAndRows(t *testing.T) {
	a := newApp()
	a.tree = diff.Build(
		[]diff.FileEntry{
			{Artist: "A", Album: "New", File: "1.mp3", Size: 100}, // RemoteOnly
			{Artist: "A", Album: "Old", File: "1.mp3", Size: 100}, // Synced
		},
		[]diff.FileEntry{
			{Artist: "A", Album: "Old", File: "1.mp3", Size: 100},
		},
	)
	a.expandAll(true)
	a.rebuildRows()

	// rows: artist A, album New, album Old
	if len(a.rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(a.rows))
	}

	// Cursor on "New" album (RemoteOnly), toggle → Copy queued.
	a.cursor = 1
	a.toggleCurrent()
	if got := a.rows[1].album.Action(); got != diff.Copy {
		t.Errorf("after toggle, action = %v, want Copy", got)
	}

	// Collapse the artist → only the artist row remains.
	a.cursor = 0
	a.setExpanded(false)
	if len(a.rows) != 1 {
		t.Errorf("collapsed rows = %d, want 1", len(a.rows))
	}
}

// Free-space gate: need > free blocks; deletes reduce the need (delete-first).
func TestConfirmFitsGate(t *testing.T) {
	a := newApp()
	a.summary = diff.Summary{CopyBytes: 1000, DeleteBytes: 400}
	a.free = 700 // need = 1000-400 = 600 <= 700
	if !a.fits() {
		t.Errorf("expected fit: need=%d free=%d", a.summary.NeedBytes(), a.free)
	}
	a.free = 500 // 600 > 500
	if a.fits() {
		t.Errorf("expected blocked: need=%d free=%d", a.summary.NeedBytes(), a.free)
	}
}

// applyLoadout replaces the selection with the loadout's exact set, reports
// stale refs, and seeds drift detection.
func TestApplyLoadoutReplaceAndDrift(t *testing.T) {
	a := newApp()
	a.tree = diff.Build(
		[]diff.FileEntry{
			{Artist: "A", Album: "One", File: "1.mp3", Size: 100}, // RemoteOnly
			{Artist: "A", Album: "Two", File: "1.mp3", Size: 100}, // Synced
			{Artist: "B", Album: "Three", File: "1.mp3", Size: 100},
		},
		[]diff.FileEntry{
			{Artist: "A", Album: "Two", File: "1.mp3", Size: 100},
		},
	)
	a.rebuildRows()

	missing := a.applyLoadout(loadout.Loadout{Name: "trip", Albums: []loadout.AlbumRef{
		{Artist: "A", Album: "One"},
		{Artist: "B", Album: "Three"},
		{Artist: "Gone", Album: "Missing"}, // stale: not on server
	}})
	a.activeLoadout = "trip"

	if missing != 1 {
		t.Errorf("missing = %d, want 1", missing)
	}

	want := map[[2]string]bool{
		{"A", "One"}:   true,
		{"A", "Two"}:   false, // unchecked by replace even though it was Synced
		{"B", "Three"}: true,
	}
	for _, ar := range a.tree.Artists {
		for _, al := range ar.Albums {
			if al.Checked != want[albumKey(al.Artist, al.Name)] {
				t.Errorf("%s/%s checked=%v, want %v", al.Artist, al.Name, al.Checked, want[albumKey(al.Artist, al.Name)])
			}
		}
	}

	if a.selectionChanged() {
		t.Error("selectionChanged() true right after load, want false")
	}

	// Toggle one album → drift detected.
	a.tree.Artists[0].Albums[0].Checked = !a.tree.Artists[0].Albums[0].Checked
	if !a.selectionChanged() {
		t.Error("selectionChanged() false after edit, want true")
	}
}

// a/n act on the highlighted artist's whole album set even when the artist is
// collapsed; when an album row is highlighted, they act on that album's artist.
func TestCheckCurrentArtist(t *testing.T) {
	a := newApp()
	a.tree = diff.Build(
		[]diff.FileEntry{
			{Artist: "A", Album: "One", File: "1.mp3", Size: 100},
			{Artist: "A", Album: "Two", File: "1.mp3", Size: 100},
			{Artist: "B", Album: "Solo", File: "1.mp3", Size: 100},
		},
		nil,
	)
	a.expandAll(false) // all artists collapsed
	a.rebuildRows()

	// rows: artist A, artist B (no album rows while collapsed)
	if len(a.rows) != 2 {
		t.Fatalf("rows = %d, want 2 (collapsed)", len(a.rows))
	}

	// Highlight artist A and check all — both A albums get checked despite being
	// collapsed/not in the row list.
	a.cursor = 0
	a.checkCurrentArtist(true)
	for _, al := range a.tree.Artists[0].Albums {
		if !al.Checked {
			t.Errorf("A/%s not checked", al.Name)
		}
	}
	if a.tree.Artists[1].Albums[0].Checked {
		t.Error("B/Solo checked, should be untouched")
	}

	// n on artist A clears them again.
	a.checkCurrentArtist(false)
	for _, al := range a.tree.Artists[0].Albums {
		if al.Checked {
			t.Errorf("A/%s still checked after none", al.Name)
		}
	}

	// Highlight an album row of artist A → acts on artist A.
	a.tree.Artists[0].Expanded = true
	a.rebuildRows()
	a.cursor = 1 // first album under A
	if a.rows[a.cursor].kind != albumRow {
		t.Fatalf("row %d not an album row", a.cursor)
	}
	a.checkCurrentArtist(true)
	for _, al := range a.tree.Artists[0].Albums {
		if !al.Checked {
			t.Errorf("A/%s not checked from album-row scope", al.Name)
		}
	}
}

// The setup wizard collects remote/path/destination across three steps, saves
// the config, and lands on the destination screen.
func TestSetupWizard(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load() // empty config with a valid (temp) save path
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	a := New(cfg, false, true)
	if a.screen != screenSetup {
		t.Fatalf("screen = %v, want setup", a.screen)
	}

	enter := tea.KeyMsg{Type: tea.KeyEnter}

	// Step 1 requires user@host.
	a.setupInput.SetValue("no-at-sign")
	a.Update(enter)
	if a.setupErr == "" || a.setupStep != 0 {
		t.Fatalf("expected validation error on step 1, got err=%q step=%d", a.setupErr, a.setupStep)
	}
	a.setupInput.SetValue("alice@example.com")
	a.Update(enter)

	// Step 2: remote path.
	a.setupInput.SetValue("/srv/music")
	a.Update(enter)

	// Step 3: optional destination.
	a.setupInput.SetValue("/tmp/dest")
	a.Update(enter)

	if a.screen != screenDest {
		t.Fatalf("after wizard screen = %v, want dest", a.screen)
	}
	if a.cfg.RemoteUser != "alice" || a.cfg.RemoteHost != "example.com" {
		t.Errorf("remote = %q@%q, want alice@example.com", a.cfg.RemoteUser, a.cfg.RemoteHost)
	}
	if a.cfg.RemotePath != "/srv/music" {
		t.Errorf("path = %q, want /srv/music", a.cfg.RemotePath)
	}
	if a.cfg.LastDest != "/tmp/dest" {
		t.Errorf("dest = %q, want /tmp/dest", a.cfg.LastDest)
	}

	// Config was persisted: a fresh load sees the saved values.
	reloaded, err := config.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.NeedsSetup() {
		t.Error("reloaded config still NeedsSetup, want fully configured")
	}
}
