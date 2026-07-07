package diff

import "testing"

func remote(artist, album, file string, size int64) FileEntry {
	return FileEntry{Artist: artist, Album: album, File: file, Size: size}
}

func TestStateClassification(t *testing.T) {
	cases := []struct {
		name           string
		remote, local  []FileEntry
		wantState      State
		wantChecked    bool
		wantDefaultAct Action
	}{
		{
			name:           "remote only",
			remote:         []FileEntry{remote("A", "X", "1.mp3", 100)},
			wantState:      RemoteOnly,
			wantChecked:    false,
			wantDefaultAct: None,
		},
		{
			name:           "local only orphan",
			local:          []FileEntry{remote("A", "X", "1.mp3", 100)},
			wantState:      LocalOnly,
			wantChecked:    true, // on the device → starts checked (keep)
			wantDefaultAct: None,
		},
		{
			name:           "synced",
			remote:         []FileEntry{remote("A", "X", "1.mp3", 100)},
			local:          []FileEntry{remote("A", "X", "1.mp3", 100)},
			wantState:      Synced,
			wantChecked:    true,
			wantDefaultAct: None,
		},
		{
			name:           "modified differing size",
			remote:         []FileEntry{remote("A", "X", "1.mp3", 100)},
			local:          []FileEntry{remote("A", "X", "1.mp3", 90)},
			wantState:      Modified,
			wantChecked:    true,
			wantDefaultAct: Update,
		},
		{
			name:           "modified missing file",
			remote:         []FileEntry{remote("A", "X", "1.mp3", 100), remote("A", "X", "2.mp3", 50)},
			local:          []FileEntry{remote("A", "X", "1.mp3", 100)},
			wantState:      Modified,
			wantChecked:    true,
			wantDefaultAct: Update,
		},
		{
			name:           "synced ignores extra local file",
			remote:         []FileEntry{remote("A", "X", "1.mp3", 100)},
			local:          []FileEntry{remote("A", "X", "1.mp3", 100), remote("A", "X", "extra.mp3", 7)},
			wantState:      Synced,
			wantChecked:    true,
			wantDefaultAct: None,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := Build(c.remote, c.local)
			al := tree.Artists[0].Albums[0]
			if al.State != c.wantState {
				t.Errorf("state = %v, want %v", al.State, c.wantState)
			}
			if al.Checked != c.wantChecked {
				t.Errorf("default checked = %v, want %v", al.Checked, c.wantChecked)
			}
			if got := al.Action(); got != c.wantDefaultAct {
				t.Errorf("default action = %v, want %v", got, c.wantDefaultAct)
			}
		})
	}
}

// Every (state, checked) → action transition from the plan's table.
func TestActionTransitions(t *testing.T) {
	cases := []struct {
		state   State
		checked bool
		want    Action
	}{
		{Synced, true, None},
		{Synced, false, Delete},
		{RemoteOnly, true, Copy},
		{RemoteOnly, false, None},
		{LocalOnly, true, None},
		{LocalOnly, false, Delete},
		{Modified, true, Update},
		{Modified, false, Delete},
	}
	for _, c := range cases {
		al := &Album{State: c.state, Checked: c.checked}
		if got := al.Action(); got != c.want {
			t.Errorf("(%v, checked=%v) = %v, want %v", c.state, c.checked, got, c.want)
		}
	}
}

func TestSummarizeAndNeedBytes(t *testing.T) {
	tree := Build(
		[]FileEntry{
			remote("A", "New", "1.mp3", 1000),    // RemoteOnly → copy 1000
			remote("A", "Changed", "1.mp3", 500), // Modified
			remote("B", "Same", "1.mp3", 200),    // Synced
		},
		[]FileEntry{
			remote("A", "Changed", "1.mp3", 400), // size differs → update 500
			remote("B", "Same", "1.mp3", 200),    // synced
			remote("C", "Orphan", "1.mp3", 300),  // LocalOnly
		},
	)

	// Queue the orphan for deletion, and the RemoteOnly album for copy.
	for _, ar := range tree.Artists {
		switch ar.Name {
		case "C":
			ar.Albums[0].Checked = false // orphan unchecked → delete
		case "A":
			for _, al := range ar.Albums {
				if al.Name == "New" {
					al.Checked = true // RemoteOnly → copy
				}
			}
		}
	}

	s := tree.Summarize()
	if s.CopyAlbums != 1 || s.CopyBytes != 1000 {
		t.Errorf("copy = %d albums / %d bytes", s.CopyAlbums, s.CopyBytes)
	}
	if s.UpdateAlbums != 1 || s.UpdateBytes != 500 {
		t.Errorf("update = %d albums / %d bytes", s.UpdateAlbums, s.UpdateBytes)
	}
	if s.DeleteAlbums != 1 || s.DeleteBytes != 300 {
		t.Errorf("delete = %d albums / %d bytes", s.DeleteAlbums, s.DeleteBytes)
	}
	if got, want := s.NeedBytes(), int64(1000+500-300); got != want {
		t.Errorf("NeedBytes = %d, want %d", got, want)
	}
}

func TestArtistState(t *testing.T) {
	tree := Build(
		[]FileEntry{
			remote("A", "X", "1.mp3", 1), // A: all RemoteOnly
			remote("A", "Y", "1.mp3", 1),
			remote("B", "X", "1.mp3", 1), // B: Synced + RemoteOnly → mixed
			remote("B", "Y", "1.mp3", 1),
			remote("C", "X", "1.mp3", 1), // C: all Synced
		},
		[]FileEntry{
			remote("B", "X", "1.mp3", 1),
			remote("C", "X", "1.mp3", 1),
			remote("D", "X", "1.mp3", 1), // D: all LocalOnly
		},
	)
	want := map[string]State{
		"A": RemoteOnly,
		"B": Modified, // mixed states aggregate to Modified
		"C": Synced,
		"D": LocalOnly,
	}
	for _, ar := range tree.Artists {
		if got := ar.State(); got != want[ar.Name] {
			t.Errorf("artist %s state = %v, want %v", ar.Name, got, want[ar.Name])
		}
	}
}

func TestArtistTristateAndSetChecked(t *testing.T) {
	tree := Build([]FileEntry{
		remote("A", "X", "1.mp3", 1), // RemoteOnly → unchecked
		remote("A", "Y", "1.mp3", 1), // RemoteOnly → unchecked
	}, nil)
	ar := tree.Artists[0]

	if all, none := ar.Tristate(); all || !none {
		t.Errorf("fresh RemoteOnly artist: all=%v none=%v, want all=false none=true", all, none)
	}
	ar.Albums[0].Checked = true
	if all, none := ar.Tristate(); all || none {
		t.Errorf("mixed: all=%v none=%v, want both false", all, none)
	}
	ar.SetChecked(true)
	if all, none := ar.Tristate(); !all || none {
		t.Errorf("all set: all=%v none=%v, want all=true none=false", all, none)
	}
}
