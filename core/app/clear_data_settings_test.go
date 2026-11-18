package app_test

import (
	"testing"

	"projectb/mockengine"
	"projectb/workspaces"
)

// Clear browsing data: what is ticked goes, all of it, and the page hears so. Site data is
// erased in every workspace and the profile's own storage, with the tabs left open; what
// isn't ticked stays.
func TestSettingsClearsBrowsingData(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work workspaces.Workspace
	r.main(func() { work, _ = r.a.Workspaces.Create("Work", "#5b7fd6") })
	r.visit("https://a.test/")
	r.visit("https://b.test/")
	visited := func() (n int) {
		r.main(func() { n = len(r.a.History.Search("test", 10)) })
		return n
	}
	if visited() != 2 {
		t.Fatalf("visited %d", visited())
	}
	got, _ := r.openSettings()

	r.settings(`{"t":"clearData","siteData":true}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dataCleared" })
	if m["siteData"] != true || m["history"] != false {
		t.Fatalf("the page heard %v", m)
	}
	var erased []mockengine.Erase
	r.main(func() { erased = r.e.Erased() })
	if len(erased) != 2 || erased[0].Workspace != "" || erased[1].Workspace != work.ID || erased[0].Open != 1 {
		t.Fatalf("erased %+v", erased)
	}
	if visited() != 2 {
		t.Fatal("site data alone took the history")
	}

	r.settings(`{"t":"clearData","history":true,"downloads":true}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dataCleared" && m["history"] == true })
	if visited() != 0 {
		t.Fatalf("history left %d", visited())
	}
	r.main(func() { erased = r.e.Erased() })
	if len(erased) != 2 {
		t.Fatalf("history alone erased %+v", erased)
	}
}
