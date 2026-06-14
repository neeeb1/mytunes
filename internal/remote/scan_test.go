package remote

import "testing"

// Adversarial names must round-trip: the scan is NUL-delimited and parsed as
// bytes, never split on whitespace.
func TestParseAdversarialNames(t *testing.T) {
	root := "/srv/music"
	// Records: "<size>\t<path>\0"
	rec := func(size, path string) []byte {
		return append([]byte(size+"\t"+path), 0)
	}
	var out []byte
	out = append(out, rec("100", "/srv/music/A$AP Rocky/Don't Be Dumb/A$AP_Don't_01_Intro.mp3")...)
	out = append(out, rec("200", "/srv/music/Sigur Rós/( )/track 1.flac")...)
	out = append(out, rec("300", "/srv/music/Artist/Album/disc1/05 Song.mp3")...) // deeper than 3

	entries, err := parse(out, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	if e := entries[0]; e.Artist != "A$AP Rocky" || e.Album != "Don't Be Dumb" || e.Size != 100 {
		t.Errorf("entry0 = %+v", e)
	}
	if e := entries[1]; e.Artist != "Sigur Rós" || e.Album != "( )" || e.File != "track 1.flac" {
		t.Errorf("entry1 = %+v", e)
	}
	// Deeper layout keeps the subdir as part of the file name.
	if e := entries[2]; e.Album != "Album" || e.File != "disc1/05 Song.mp3" {
		t.Errorf("entry2 = %+v", e)
	}
}

func TestParseSkipsShallow(t *testing.T) {
	out := append([]byte("100\t/srv/music/LooseFile.mp3"), 0)
	entries, err := parse(out, "/srv/music")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected shallow path skipped, got %v", entries)
	}
}
