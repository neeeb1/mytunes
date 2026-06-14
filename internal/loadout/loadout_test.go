package loadout

import (
	"reflect"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Missing file loads as empty.
	s, err := Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(s.Loadouts) != 0 {
		t.Fatalf("want empty store, got %d", len(s.Loadouts))
	}

	s.Put(Loadout{Name: "road-trip", Albums: []AlbumRef{
		{Artist: "A$AP Rocky", Album: "LONG.LIVE.A$AP"},
		{Artist: "Boards of Canada", Album: "Music Has the Right to Children"},
	}})
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got.Loadouts, s.Loadouts) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got.Loadouts, s.Loadouts)
	}
}

func TestPutOverwrite(t *testing.T) {
	var s Store
	s.Put(Loadout{Name: "party", Albums: []AlbumRef{{Artist: "X", Album: "1"}}})
	s.Put(Loadout{Name: "party", Albums: []AlbumRef{{Artist: "Y", Album: "2"}}})

	if len(s.Loadouts) != 1 {
		t.Fatalf("want 1 loadout after overwrite, got %d", len(s.Loadouts))
	}
	l, ok := s.Get("party")
	if !ok || len(l.Albums) != 1 || l.Albums[0].Artist != "Y" {
		t.Fatalf("overwrite did not replace albums: %+v", l)
	}
}

func TestDelete(t *testing.T) {
	var s Store
	s.Put(Loadout{Name: "a"})
	s.Put(Loadout{Name: "b"})

	if !s.Delete("a") {
		t.Fatal("Delete(a) = false, want true")
	}
	if s.Delete("a") {
		t.Fatal("Delete(a) again = true, want false")
	}
	if _, ok := s.Get("a"); ok {
		t.Fatal("a still present after delete")
	}
	if _, ok := s.Get("b"); !ok {
		t.Fatal("b removed unexpectedly")
	}
}

func TestRename(t *testing.T) {
	var s Store
	s.Put(Loadout{Name: "old"})
	s.Put(Loadout{Name: "taken"})

	if s.Rename("missing", "x") {
		t.Fatal("rename of missing = true, want false")
	}
	if s.Rename("old", "taken") {
		t.Fatal("rename into taken name = true, want false")
	}
	if !s.Rename("old", "new") {
		t.Fatal("rename old->new = false, want true")
	}
	if _, ok := s.Get("new"); !ok {
		t.Fatal("new name missing after rename")
	}
	if _, ok := s.Get("old"); ok {
		t.Fatal("old name still present after rename")
	}
}

func TestNamesSorted(t *testing.T) {
	var s Store
	s.Put(Loadout{Name: "zebra"})
	s.Put(Loadout{Name: "alpha"})
	s.Put(Loadout{Name: "mango"})

	got := s.Names()
	want := []string{"alpha", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
}
