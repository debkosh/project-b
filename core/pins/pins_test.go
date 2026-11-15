package pins

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// A new profile has no pins and no file.
func TestANewProfileHasNoPins(t *testing.T) {
	dir := t.TempDir()
	if l := New(dir).List(); l == nil || len(l) != 0 {
		t.Fatalf("list %#v", l)
	}
	if _, err := os.Stat(filepath.Join(dir, "pins.json")); !os.IsNotExist(err) {
		t.Fatalf("a file before any pin: %v", err)
	}
}

// A pin gets an id Go makes and goes after its workspace's others; the list is the file
// beside workspaces.json, read again by the next start.
func TestPinsAreKeptInTheProfileInTheirOrder(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	changes := 0
	m.OnChanged(func() { changes++ })
	a1 := m.Add("", "https://a.test/", "A", "data:image/png;base64,AA==")
	b1 := m.Add("0a1b2c3d", "https://b.test/", "B", "")
	a2 := m.Add("", "https://a2.test/", "A2", "")
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(a1.ID) || a1.ID == b1.ID || a1.ID == a2.ID || changes != 3 {
		t.Fatalf("made %+v %+v %+v, %d changes", a1, b1, a2, changes)
	}
	want := []Pin{a1, a2, b1}
	if l := m.List(); !slices.Equal(l, want) || m.Index(b1.ID) != 2 || m.Index("nothere") != -1 {
		t.Fatalf("list %+v", l)
	}
	if got := New(dir).List(); !slices.Equal(got, want) {
		t.Fatalf("read again %+v", got)
	}
	if p, ok := m.Get(a2.ID); !ok || p != a2 {
		t.Fatalf("get %+v %v", p, ok)
	}
}

// Removing a pin takes it out, an unknown one is refused, and deleting a workspace takes
// its pins.
func TestRemovingPins(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	a := m.Add("", "https://a.test/", "A", "")
	b := m.Add("0a1b2c3d", "https://b.test/", "B", "")
	c := m.Add("0a1b2c3d", "https://c.test/", "C", "")
	changes := 0
	m.OnChanged(func() { changes++ })
	if err := m.Remove(b.ID); err != nil || m.Remove(b.ID) == nil {
		t.Fatalf("remove: %v", err)
	}
	m.RemoveWorkspace("nothere")
	m.RemoveWorkspace("0a1b2c3d")
	if l := New(dir).List(); !slices.Equal(l, []Pin{a}) || changes != 2 {
		t.Fatalf("left %+v after %d changes; c was %+v", l, changes, c)
	}
}
