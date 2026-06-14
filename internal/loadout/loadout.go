// Package loadout persists named sets of album selections ("loadouts") to an
// XDG TOML file, alongside the mytunes config. A loadout records the exact set
// of albums a user wants present on the destination, so it can be quick-loaded
// into the browse screen later.
package loadout

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
)

// AlbumRef identifies an album by the same (Artist, Album) key diff.Build uses.
type AlbumRef struct {
	Artist string `toml:"artist"`
	Album  string `toml:"album"`
}

// Loadout is a named set of album references.
type Loadout struct {
	Name   string     `toml:"name"`
	Albums []AlbumRef `toml:"albums"`
}

// Store is the whole loadouts file: a TOML array of [[loadout]] tables.
type Store struct {
	Loadouts []Loadout `toml:"loadout"`

	path string // where this store was loaded from / will be saved
}

// storePath resolves $XDG_CONFIG_HOME/mytunes/loadouts.toml (falling back to
// ~/.config), matching config.configPath.
func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mytunes", "loadouts.toml"), nil
}

// Load reads the loadouts file. A missing file yields an empty store without
// writing one (unlike config.Load, we don't seed a file before the first save).
func Load() (Store, error) {
	p, err := storePath()
	if err != nil {
		return Store{}, err
	}

	s := Store{path: p}

	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if _, err := toml.Decode(string(data), &s); err != nil {
		return s, err
	}
	s.path = p
	s.sort()
	return s, nil
}

// Save writes all loadouts back to the file, creating parent dirs as needed.
func (s Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(s)
}

// Get returns the loadout with the given name.
func (s *Store) Get(name string) (*Loadout, bool) {
	for i := range s.Loadouts {
		if s.Loadouts[i].Name == name {
			return &s.Loadouts[i], true
		}
	}
	return nil, false
}

// Put adds a loadout, overwriting any existing one with the same name.
func (s *Store) Put(l Loadout) {
	if existing, ok := s.Get(l.Name); ok {
		*existing = l
		return
	}
	s.Loadouts = append(s.Loadouts, l)
	s.sort()
}

// Delete removes a loadout by name, reporting whether it existed.
func (s *Store) Delete(name string) bool {
	for i := range s.Loadouts {
		if s.Loadouts[i].Name == name {
			s.Loadouts = append(s.Loadouts[:i], s.Loadouts[i+1:]...)
			return true
		}
	}
	return false
}

// Rename changes a loadout's name. It fails if old is missing or new is already
// taken (and is a no-op success if old == new).
func (s *Store) Rename(old, name string) bool {
	if old == name {
		_, ok := s.Get(old)
		return ok
	}
	if _, taken := s.Get(name); taken {
		return false
	}
	l, ok := s.Get(old)
	if !ok {
		return false
	}
	l.Name = name
	s.sort()
	return true
}

// Names returns the loadout names, sorted.
func (s *Store) Names() []string {
	names := make([]string, len(s.Loadouts))
	for i, l := range s.Loadouts {
		names[i] = l.Name
	}
	sort.Strings(names)
	return names
}

func (s *Store) sort() {
	sort.Slice(s.Loadouts, func(i, j int) bool {
		return s.Loadouts[i].Name < s.Loadouts[j].Name
	})
}
