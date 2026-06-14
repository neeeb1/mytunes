package tui

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func mkdirs(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.Mkdir(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTabCompleteUnique(t *testing.T) {
	tmp := t.TempDir()
	mkdirs(t, tmp, "Movies", "Music")
	if err := os.WriteFile(filepath.Join(tmp, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	a := newApp()

	// Unique directory prefix completes and descends (trailing slash); files are
	// never offered.
	got := a.tabComplete(filepath.Join(tmp, "Mov"))
	if want := filepath.Join(tmp, "Movies") + "/"; got != want {
		t.Errorf("unique: got %q, want %q", got, want)
	}
	if a.pathCands != nil {
		t.Errorf("unique: candidates = %v, want nil", a.pathCands)
	}

	// No match leaves the input untouched.
	if got := a.tabComplete(filepath.Join(tmp, "Zzz")); got != filepath.Join(tmp, "Zzz") {
		t.Errorf("no-match: got %q, want unchanged", got)
	}
}

func TestTabCompleteCommonPrefix(t *testing.T) {
	tmp := t.TempDir()
	mkdirs(t, tmp, "album-one", "album-two", "single")

	a := newApp()
	// "al" is ambiguous but shares the prefix "album-"; extend to it and list.
	got := a.tabComplete(filepath.Join(tmp, "al"))
	if want := filepath.Join(tmp, "album-"); got != want {
		t.Errorf("prefix: got %q, want %q", got, want)
	}
	if !slices.Equal(a.pathCands, []string{"album-one", "album-two"}) {
		t.Errorf("prefix: candidates = %v", a.pathCands)
	}
	if a.pathActive != -1 {
		t.Errorf("prefix: active = %d, want -1 (listing, not cycling)", a.pathActive)
	}
}

func TestTabCompleteCycle(t *testing.T) {
	tmp := t.TempDir()
	mkdirs(t, tmp, "media", "mnt", "user") // share no prefix beyond ""

	a := newApp()
	base := tmp + "/"

	// First Tab on a trailing-slash dir cycles to the first candidate.
	got := a.tabComplete(base)
	if want := base + "media/"; got != want {
		t.Fatalf("cycle[0]: got %q, want %q", got, want)
	}
	if a.pathActive != 0 {
		t.Errorf("cycle[0]: active = %d, want 0", a.pathActive)
	}

	// Repeated Tab (value unchanged from last insert) advances through siblings.
	got = a.tabComplete(got)
	if want := base + "mnt/"; got != want {
		t.Errorf("cycle[1]: got %q, want %q", got, want)
	}
	got = a.tabComplete(got)
	if want := base + "user/"; got != want {
		t.Errorf("cycle[2]: got %q, want %q", got, want)
	}
	// Wraps back to the first.
	got = a.tabComplete(got)
	if want := base + "media/"; got != want {
		t.Errorf("cycle wrap: got %q, want %q", got, want)
	}

	// Typing (resetCompletion) ends the cycle.
	a.resetCompletion()
	if a.tabCycle != nil || a.pathActive != -1 {
		t.Errorf("after reset: tabCycle=%v active=%d", a.tabCycle, a.pathActive)
	}
}
