package sync

import "testing"

func mustContain(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args missing %q: %v", want, args)
}

func TestWithinDestGuard(t *testing.T) {
	dest := "/mnt/usb/Music"
	cases := []struct {
		target string
		want   bool
	}{
		{"/mnt/usb/Music/Artist/Album", true},
		{"/mnt/usb/Music/Artist", true},
		{"/mnt/usb/Music", false},            // the root itself
		{"/mnt/usb/Music/", false},           // cleans to root
		{"/mnt/usb", false},                  // parent
		{"/mnt/usb/Music/../../etc", false},  // escape via ..
		{"/etc/passwd", false},               // unrelated
		{"/mnt/usb/Music/A/../B", true},      // cleans to within
		{"/mnt/usb/MusicOther/Album", false}, // sibling prefix trap
	}
	for _, c := range cases {
		if got := withinDest(dest, c.target); got != c.want {
			t.Errorf("withinDest(%q, %q) = %v, want %v", dest, c.target, got, c.want)
		}
	}
}

func TestCheckDeletesRejectsEscape(t *testing.T) {
	j := Job{
		Dest:       "/mnt/usb/Music",
		DeleteDirs: []string{"/mnt/usb/Music/A/B", "/etc"},
	}
	if err := j.CheckDeletes(); err == nil {
		t.Fatal("expected CheckDeletes to reject /etc")
	}
}

func TestRsyncArgsLayout(t *testing.T) {
	j := Job{
		Remote:     "you@example.com",
		RemotePath: "/srv/music",
		Dest:       "/mnt/usb/Music",
		ExtraArgs:  []string{"--bwlimit=4M"},
	}
	args := j.rsyncArgs("/tmp/list.txt", "--info=progress2")

	// baseArgs preserved, in order, up front.
	for i, want := range baseArgs {
		if args[i] != want {
			t.Fatalf("baseArgs[%d] = %q, want %q", i, args[i], want)
		}
	}
	mustContain(t, args, "--bwlimit=4M")
	mustContain(t, args, "--info=progress2")
	mustContain(t, args, "--files-from=/tmp/list.txt")

	// Source has trailing slash and host prefix; dest trailing slash.
	if got := args[len(args)-2]; got != "you@example.com:/srv/music/" {
		t.Errorf("source = %q", got)
	}
	if got := args[len(args)-1]; got != "/mnt/usb/Music/" {
		t.Errorf("dest = %q", got)
	}
}

func TestParseTransferredBytes(t *testing.T) {
	out := []byte("Number of files: 10\nTotal transferred file size: 1,234,567 bytes\nfoo\n")
	n, err := parseTransferredBytes(out)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1234567 {
		t.Errorf("got %d, want 1234567", n)
	}
}

func TestParsePercent(t *testing.T) {
	cases := map[string]float64{
		"      1,234,567  45%   1.23MB/s    0:00:12":  45,
		"          32,768   0%    0.00kB/s   0:00:00": 0,
		"1000000000 100% 10MB/s 0:01:40 (xfr#1)":      100,
	}
	for line, want := range cases {
		got, ok := parsePercent(line)
		if !ok || got != want {
			t.Errorf("parsePercent(%q) = %v,%v want %v", line, got, ok, want)
		}
	}
	if _, ok := parsePercent("no percent here"); ok {
		t.Error("expected no match")
	}
}

func TestTransferredFile(t *testing.T) {
	files := []string{
		"A$AP Rocky/LONG.LIVE.A$AP/01 Palace.mp3",
		"Don't Be Dumb/Album/track.flac",
		"Artist/Album/Disc 1/01 Song.mp3", // deeper layout still has slashes
	}
	for _, f := range files {
		if name, ok := transferredFile(f); !ok || name != f {
			t.Errorf("transferredFile(%q) = %q,%v want it kept", f, name, ok)
		}
	}

	noise := []string{
		"",
		"sending incremental file list",
		"sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec",
		"total size is 43,000,000  speedup is 1.00",
	}
	for _, n := range noise {
		if name, ok := transferredFile(n); ok {
			t.Errorf("transferredFile(%q) = %q,true want it dropped", n, name)
		}
	}
}
