package sync

import "testing"

func TestSupportsInfoProgress2(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"upstream 3.2", "rsync  version 3.2.7  protocol version 31\n", true},
		{"upstream 3.1", "rsync  version 3.1.0  protocol version 31\n", true},
		{"upstream 4.0", "rsync  version 4.0.0  protocol version 32\n", true},
		{"upstream 3.0", "rsync  version 3.0.9  protocol version 30\n", false},
		{"apple 2.6.9", "rsync  version 2.6.9  protocol version 29\n", false},
		{"openrsync", "openrsync: protocol version 29\nrsync version 2.6.9 compatible\n", false},
		{"samba prefix", "rsync  version v3.2.7  protocol version 31\n", true},
		{"garbage", "not an rsync at all\n", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		if got := supportsInfoProgress2([]byte(c.out)); got != c.want {
			t.Errorf("%s: supportsInfoProgress2 = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLastLines(t *testing.T) {
	in := "a\n\nb\nc\nd\ne\nf\n"
	if got := lastLines(in, 5); got != "b; c; d; e; f" {
		t.Errorf("lastLines = %q", got)
	}
	if got := lastLines("", 5); got != "" {
		t.Errorf("lastLines(empty) = %q", got)
	}
}
