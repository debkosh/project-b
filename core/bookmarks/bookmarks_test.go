package bookmarks

import "testing"

func TestBookmarksAreKeptEditedAndRemoved(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	heard := 0
	m.OnChanged(func() { heard++ })
	m.Add("A", "https://a.test/")
	m.Add("B", "https://b.test/")
	m.Add("A again", "https://a.test/")
	if l := m.List(); len(l) != 2 || l[0].URL != "https://b.test/" || heard != 2 {
		t.Fatalf("list = %+v, heard %d", l, heard)
	}
	a := m.List()[1]
	if err := m.Edit(a.ID, "", "https://b.test/"); err == nil {
		t.Fatal("an address bookmarked already should be refused")
	}
	if err := m.Edit(a.ID, "", "https://a.test/home"); err != nil || m.List()[1].Title != "https://a.test/home" {
		t.Fatalf("edit: %v %+v", err, m.List())
	}
	if got := m.Search("home", 10); len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("search = %+v", got)
	}
	m.RemoveURL("https://b.test/")
	again := New(dir)
	if l := again.List(); len(l) != 1 || l[0].URL != "https://a.test/home" || !again.Has("https://a.test/home") {
		t.Fatalf("after a restart: %+v", l)
	}
	again.Add("C", "https://c.test/")
	if l := again.List(); l[0].ID == a.ID || l[0].ID == l[1].ID {
		t.Fatalf("ids should stay unique: %+v", l)
	}
}
