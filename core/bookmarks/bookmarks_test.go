package bookmarks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// in is the titles in folder parent, in their places.
func in(m *Manager, parent string) string {
	var t []string
	for _, e := range m.All() {
		if e.Parent == parent {
			t = append(t, e.Title)
		}
	}
	return strings.Join(t, " ")
}

// A bookmark goes first in the folder last saved in; an address is bookmarked once; an
// edit keeps the id; and it all comes back after a restart.
func TestBookmarksAreKeptEditedAndRemoved(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	heard := 0
	m.OnChanged(func() { heard++ })
	a, _ := m.Add("A", "https://a.test/")
	m.Add("B", "https://b.test/")
	if again, _ := m.Add("A again", "https://a.test/"); again.ID != a.ID || heard != 2 {
		t.Fatalf("a second Add of an address: %+v, heard %d", again, heard)
	}
	if got := in(m, Top); got != "B A" {
		t.Fatalf("the top holds %q", got)
	}
	if err := m.Edit(a.ID, "", "https://b.test/"); err == nil {
		t.Fatal("an address bookmarked already should be refused")
	}
	if err := m.Edit(a.ID, "", "https://a.test/home"); err != nil {
		t.Fatal(err)
	}
	if e, _ := m.Get(a.ID); e.Title != "https://a.test/home" || e.Modified < e.Added {
		t.Fatalf("after the edit: %+v", e)
	}
	m.RemoveURL("https://b.test/")
	again := New(dir)
	if l := again.List(); len(l) != 1 || l[0].ID != a.ID || !again.Has("https://a.test/home") {
		t.Fatalf("after a restart: %+v", l)
	}
	if len(a.ID) != 16 {
		t.Fatalf("id %q", a.ID)
	}
}

// Folders hold bookmarks and folders; a move puts an entry first in its new folder and
// changes that entry alone; a folder doesn't go into itself; removing a folder takes all
// it holds.
func TestFolders(t *testing.T) {
	m := New(t.TempDir())
	work, _ := m.NewFolder(Top, "Work")
	design, _ := m.NewFolder(work.ID, "Design")
	a, _ := m.Add("A", "https://a.test/")
	b, _ := m.Add("B", "https://b.test/")
	if _, err := m.NewFolder("nope", "X"); err == nil {
		t.Fatal("a folder in no folder")
	}
	before, _ := m.Get(b.ID)
	if err := m.Move(a.ID, design.ID); err != nil {
		t.Fatal(err)
	}
	m.Move(b.ID, design.ID)
	if got := in(m, design.ID); got != "B A" {
		t.Fatalf("Design holds %q", got)
	}
	if after, _ := m.Get(b.ID); after.Place == before.Place && after.Parent == before.Parent {
		t.Fatal("the move changed nothing")
	}
	if err := m.Move(work.ID, design.ID); err == nil {
		t.Fatal("a folder went into one it holds")
	}
	if err := m.Move(work.ID, work.ID); err == nil {
		t.Fatal("a folder went into itself")
	}
	if err := m.Edit(design.ID, "", ""); err == nil {
		t.Fatal("a folder with no name")
	}
	m.Remove(work.ID)
	if all := m.All(); len(all) != 0 {
		t.Fatalf("left after removing Work: %+v", all)
	}
}

// The star's panel saves a title and a folder, and the next bookmark goes into that folder;
// once the folder is gone, the next goes to the top.
func TestTheFolderLastSavedInTakesTheNext(t *testing.T) {
	m := New(t.TempDir())
	read, _ := m.NewFolder(Top, "Reading")
	a, _ := m.Add("A", "https://a.test/")
	if err := m.Save(a.ID, "Article", read.ID); err != nil {
		t.Fatal(err)
	}
	b, _ := m.Add("B", "https://b.test/")
	if b.Parent != read.ID || in(m, read.ID) != "B Article" {
		t.Fatalf("B is in %q; Reading holds %q", b.Parent, in(m, read.ID))
	}
	m.Remove(read.ID)
	if c, _ := m.Add("C", "https://c.test/"); c.Parent != Top {
		t.Fatalf("with Reading gone, C went to %q", c.Parent)
	}
}

// The list from before folders becomes the top folder's, newest first, with new ids, the
// times in milliseconds, and the file written as version 2.
func TestTheListBeforeFoldersMovesToTheTop(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bookmarks.json"), []byte(`{"version":1,"next":3,"bookmarks":[
		{"id":2,"title":"New","url":"https://new.test/","added":1700000200},
		{"id":1,"title":"Old","url":"https://old.test/","added":1700000100}]}`), 0o600)
	m := New(dir)
	if got := in(m, Top); got != "New Old" {
		t.Fatalf("the top holds %q", got)
	}
	if l := m.List(); l[0].Added != 1700000200000 || l[0].ID == l[1].ID || len(l[0].ID) != 16 {
		t.Fatalf("list %+v", l)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "bookmarks.json"))
	var f file
	json.Unmarshal(b, &f)
	if f.Version != 2 || len(f.Entries) != 2 {
		t.Fatalf("written: %s", b)
	}
}

// A key between two sorts between them, at either end too, and spread's keys are in order.
func TestPlaces(t *testing.T) {
	keys := spread(1000)
	if !slices.IsSorted(keys) || len(keys[999]) > 2 {
		t.Fatalf("spread: %v … %v", keys[:3], keys[997:])
	}
	pairs := [][2]string{{"", ""}, {"", "1"}, {"", "01"}, {"z", ""}, {"a", "b"}, {"a", "a1"}, {"az", "b"}, {"azz", "b0z"}, {keys[0], keys[1]}}
	for _, p := range pairs {
		k := between(p[0], p[1])
		if !(p[0] < k) || !(p[1] == "" || k < p[1]) || strings.HasSuffix(k, "0") {
			t.Errorf("between(%q, %q) = %q", p[0], p[1], k)
		}
	}
	// Always first, or always last: each key past the one before, and short still.
	first, last := "", ""
	for range 1000 {
		f, l := between("", first), between(last, "")
		if (first != "" && f >= first) || l <= last || strings.HasSuffix(f, "0") {
			t.Fatalf("between(\"\", %q) = %q, between(%q, \"\") = %q", first, f, last, l)
		}
		first, last = f, l
	}
	if len(first) > 40 || len(last) > 40 {
		t.Errorf("1000 firsts make a key %d long, 1000 lasts %d", len(first), len(last))
	}
}
