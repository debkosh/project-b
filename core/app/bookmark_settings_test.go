package app_test

import (
	"projectb/prefs"
	"testing"
)

// Settings' Bookmarks: the page's snapshot has the prefs and the bookmarks, with folders;
// a pref it sets by name reaches the page and the shells as prefs, and is kept; a new
// folder reaches the page, for the bar's folder list.
func TestSettingsSetsWhereBookmarksShow(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, snap := r.openSettings()
	if p, _ := snap["prefs"].(map[string]any); p == nil || p["bookmarksNewTab"] != true || p["bookmarksBar"] != false {
		t.Fatalf("the snapshot's prefs are %v", snap["prefs"])
	}
	if _, ok := snap["bookmarks"].([]any); !ok {
		t.Fatalf("the snapshot's bookmarks are %v", snap["bookmarks"])
	}
	r.settings(`{"t":"setPref","name":"bookmarksBar","value":"true"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "prefs" })
	if p := m["prefs"].(map[string]any); p["bookmarksBar"] != true {
		t.Fatalf("the page's prefs after setPref: %v", p)
	}
	r.waitFor(func(m map[string]any) bool {
		p, _ := m["prefs"].(map[string]any)
		return m["t"] == "prefs" && p["bookmarksBar"] == true
	})
	var folder string
	r.e.Post(func() {
		f, err := r.a.Bookmarks.NewFolder("", "Recipes")
		if err != nil {
			t.Error(err)
		}
		folder = f.ID
	})
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "bookmarks" })
	if list := m["bookmarks"].([]any); len(list) != 1 || list[0].(map[string]any)["kind"] != "folder" {
		t.Fatalf("the page's bookmarks: %v", list)
	}
	r.settings(`{"t":"setPref","name":"bookmarksBarFolder","value":"` + folder + `"}`)
	r.waitOn(got, func(m map[string]any) bool {
		p, _ := m["prefs"].(map[string]any)
		return m["t"] == "prefs" && p["bookmarksBarFolder"] == folder
	})
	r.flushPrefs()
	if p := prefs.New(dir).All(); !p.BookmarksBar || p.BookmarksBarFolder != folder {
		t.Fatalf("kept %+v", p)
	}
}
