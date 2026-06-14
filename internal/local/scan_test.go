package local

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRoundTripsNames(t *testing.T) {
	root := t.TempDir()
	// Artist/Album/File with awkward names.
	files := map[string]string{
		filepath.Join(root, "A$AP Rocky", "Don't Be Dumb", "01 Intro.mp3"): "abc",
		filepath.Join(root, "Sigur Rós", "( )", "track 1.flac"):            "de",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(entries), entries)
	}

	byArtist := map[string]int64{}
	for _, e := range entries {
		byArtist[e.Artist] = e.Size
	}
	if byArtist["A$AP Rocky"] != 3 {
		t.Errorf("A$AP Rocky size = %d, want 3", byArtist["A$AP Rocky"])
	}
	if byArtist["Sigur Rós"] != 2 {
		t.Errorf("Sigur Rós size = %d, want 2", byArtist["Sigur Rós"])
	}
}

func TestScanMissingDestIsEmpty(t *testing.T) {
	entries, err := Scan(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries for missing dest, got %d", len(entries))
	}
}

func TestFreeBytesNonNegative(t *testing.T) {
	n, err := FreeBytes(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Errorf("FreeBytes = %d, want > 0", n)
	}
}
