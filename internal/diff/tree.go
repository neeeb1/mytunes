// Package diff merges the remote (server) and local (destination) music
// libraries into an Artist→Album tree and derives, per album, both its current
// state and the action the user has queued for this session.
package diff

import "sort"

// State is what is true *now* about an album, before the user touches anything.
type State int

const (
	// Synced: every remote file is present locally with a matching size.
	Synced State = iota
	// RemoteOnly: album exists on the server but not on the destination.
	RemoteOnly
	// LocalOnly: album exists on the destination but not on the server (orphan).
	LocalOnly
	// Modified: album is on both sides but at least one file is missing
	// locally or differs in size.
	Modified
)

func (s State) String() string {
	switch s {
	case Synced:
		return "Synced"
	case RemoteOnly:
		return "RemoteOnly"
	case LocalOnly:
		return "LocalOnly"
	case Modified:
		return "Modified"
	}
	return "Unknown"
}

// Icon is the leading glyph shown for the state ("what's true now").
func (s State) Icon() string {
	switch s {
	case Synced:
		return "="
	case RemoteOnly:
		return "+"
	case LocalOnly:
		return "-"
	case Modified:
		return "~"
	}
	return "?"
}

// Action is what the user has queued for this album this session ("what they
// asked for"), derived from (State, Checked).
type Action int

const (
	None Action = iota
	Copy
	Update
	Delete
)

func (a Action) String() string {
	switch a {
	case Copy:
		return "COPY"
	case Update:
		return "UPDATE"
	case Delete:
		return "DELETE"
	}
	return ""
}

// Tag is the pending-action label, blank when there is no change.
func (a Action) Tag() string {
	if a == None {
		return ""
	}
	return "→" + a.String()
}

// FileEntry is one file discovered by a scan (remote or local). Path fields are
// split by the scanner from the verified 2-level layout
// <Artist>/<Album>/<File>.
type FileEntry struct {
	Artist string
	Album  string
	File   string
	Size   int64
}

// Album holds both sides' file→size maps plus the live state and the user's
// per-session checkbox.
type Album struct {
	Artist  string
	Name    string
	Remote  map[string]int64 // file name → size on server
	Local   map[string]int64 // file name → size on destination
	State   State
	Checked bool
}

// Artist groups albums and tracks UI expansion.
type Artist struct {
	Name     string
	Albums   []*Album
	Expanded bool
}

// Tree is the merged library, sorted by artist then album name.
type Tree struct {
	Artists []*Artist
}

func (al *Album) remoteSize() int64 { return sum(al.Remote) }

// LocalSize is the bytes the album currently occupies on the destination; this
// is what a delete frees.
func (al *Album) LocalSize() int64 { return sum(al.Local) }

// RemoteSize is the album's total size on the server (what a fresh copy costs).
func (al *Album) RemoteSize() int64 { return al.remoteSize() }

// UpdateSize approximates the bytes rsync would transfer to bring a Modified
// album up to date: remote files that are missing locally or differ in size.
// The Confirm step's dry-run computes the precise figure; this only feeds the
// in-browse summary.
func (al *Album) UpdateSize() int64 {
	var n int64
	for f, rs := range al.Remote {
		if ls, ok := al.Local[f]; !ok || ls != rs {
			n += rs
		}
	}
	return n
}

func sum(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}

// computeState classifies an album from its two file maps.
func computeState(remote, local map[string]int64) State {
	if len(local) == 0 {
		return RemoteOnly
	}
	if len(remote) == 0 {
		return LocalOnly
	}
	for f, rs := range remote {
		if ls, ok := local[f]; !ok || ls != rs {
			return Modified
		}
	}
	return Synced
}

// defaultChecked is the initial checkbox: checked means "should be present".
func defaultChecked(s State) bool {
	return s == Synced || s == Modified
}

// Action derives the queued action from the album's state and checkbox.
//
//	Synced     checked → None    unchecked → Delete
//	RemoteOnly checked → Copy    unchecked → None
//	LocalOnly  checked → Delete  unchecked → None
//	Modified   checked → Update  unchecked → Delete
func (al *Album) Action() Action {
	switch al.State {
	case Synced:
		if al.Checked {
			return None
		}
		return Delete
	case RemoteOnly:
		if al.Checked {
			return Copy
		}
		return None
	case LocalOnly:
		if al.Checked {
			return Delete
		}
		return None
	case Modified:
		if al.Checked {
			return Update
		}
		return Delete
	}
	return None
}

// Build merges remote and local file entries into a sorted Tree, computing each
// album's state and default checkbox.
func Build(remote, local []FileEntry) *Tree {
	type key struct{ artist, album string }
	albums := map[key]*Album{}

	get := func(artist, album string) *Album {
		k := key{artist, album}
		al, ok := albums[k]
		if !ok {
			al = &Album{
				Artist: artist,
				Name:   album,
				Remote: map[string]int64{},
				Local:  map[string]int64{},
			}
			albums[k] = al
		}
		return al
	}

	for _, e := range remote {
		get(e.Artist, e.Album).Remote[e.File] = e.Size
	}
	for _, e := range local {
		get(e.Artist, e.Album).Local[e.File] = e.Size
	}

	byArtist := map[string]*Artist{}
	for _, al := range albums {
		al.State = computeState(al.Remote, al.Local)
		al.Checked = defaultChecked(al.State)

		ar, ok := byArtist[al.Artist]
		if !ok {
			ar = &Artist{Name: al.Artist}
			byArtist[al.Artist] = ar
		}
		ar.Albums = append(ar.Albums, al)
	}

	t := &Tree{}
	for _, ar := range byArtist {
		sort.Slice(ar.Albums, func(i, j int) bool {
			return ar.Albums[i].Name < ar.Albums[j].Name
		})
		t.Artists = append(t.Artists, ar)
	}
	sort.Slice(t.Artists, func(i, j int) bool {
		return t.Artists[i].Name < t.Artists[j].Name
	})
	return t
}

// SetChecked sets every album under an artist to the given state.
func (ar *Artist) SetChecked(v bool) {
	for _, al := range ar.Albums {
		al.Checked = v
	}
}

// Tristate reports the artist's aggregate checkbox: all checked, none checked,
// or mixed (some).
func (ar *Artist) Tristate() (all, none bool) {
	all, none = true, true
	for _, al := range ar.Albums {
		if al.Checked {
			none = false
		} else {
			all = false
		}
	}
	return
}

// Summary totals the queued actions across the whole tree.
type Summary struct {
	CopyAlbums, UpdateAlbums, DeleteAlbums int
	CopyBytes, UpdateBytes, DeleteBytes    int64
}

// NeedBytes is the net space the destination must have, valid because deletes
// run before copies: copy + update − delete.
func (s Summary) NeedBytes() int64 {
	return s.CopyBytes + s.UpdateBytes - s.DeleteBytes
}

// Summarize walks the tree and tallies queued work.
func (t *Tree) Summarize() Summary {
	var s Summary
	for _, ar := range t.Artists {
		for _, al := range ar.Albums {
			switch al.Action() {
			case Copy:
				s.CopyAlbums++
				s.CopyBytes += al.RemoteSize()
			case Update:
				s.UpdateAlbums++
				s.UpdateBytes += al.UpdateSize()
			case Delete:
				s.DeleteAlbums++
				s.DeleteBytes += al.LocalSize()
			}
		}
	}
	return s
}
