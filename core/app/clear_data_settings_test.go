package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"projectb/engine"
	"projectb/history"
	"projectb/mockengine"
	"projectb/workspaces"
)

// Clear browsing data: what is ticked goes, and the page hears so. Site data and the cache
// are erased in every workspace and the profile's own storage, with the tabs left open;
// what isn't ticked stays.
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

	r.settings(`{"t":"clearData","since":"all","siteData":true}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dataCleared" })
	if m["siteData"] != true || m["history"] != false {
		t.Fatalf("the page heard %v", m)
	}
	var erased []mockengine.Erase
	r.main(func() { erased = r.e.Erased() })
	if len(erased) != 2 || erased[0].Workspace != "" || erased[1].Workspace != work.ID || erased[0].Open != 1 {
		t.Fatalf("erased %+v", erased)
	}
	if !erased[0].Since.IsZero() || erased[0].Kinds != engine.EraseSiteData {
		t.Fatalf("all time, site data alone: erased %+v", erased[0])
	}
	if visited() != 2 {
		t.Fatal("site data alone took the history")
	}

	r.settings(`{"t":"clearData","since":"all","history":true,"downloads":true}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dataCleared" && m["history"] == true })
	if visited() != 0 {
		t.Fatalf("history left %d", visited())
	}
	r.main(func() { erased = r.e.Erased() })
	if len(erased) != 2 {
		t.Fatalf("history alone erased %+v", erased)
	}
}

// From a time: the page hears how many pages each range would take, and a clear of the
// last hour takes the pages visited and downloads listed in it and erases from an hour
// ago, the cache alone when only it is ticked; what came before stays.
func TestSettingsClearsFromATime(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Unix()
	old := fmt.Sprintf(`{"version":1,"next":3,"downloads":[{"id":1,"name":"old.zip","state":"done","started":%d},{"id":2,"name":"unknown.zip","state":"done"}]}`, now-3*24*3600)
	if err := os.WriteFile(filepath.Join(dir, "downloads.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	r := start(t, dir)
	defer r.stop()
	src := filepath.Join(t.TempDir(), "new.txt")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.main(func() {
		r.a.History.Import([]history.Entry{
			{URL: "https://old.test/", Visits: 1, LastVisit: now - 3*24*3600},
			{URL: "https://older.test/", Visits: 1, LastVisit: now - 40*24*3600},
		})
	})
	r.visit("https://new.test/")
	r.main(func() {
		if _, err := r.a.Downloads.Keep(src, "test"); err != nil {
			t.Error(err)
		}
	})
	got, _ := r.openSettings()

	r.settings(`{"t":"clearDataCounts"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "clearDataCounts" })
	pages, _ := m["pages"].(map[string]any)
	want := map[string]float64{"hour": 1, "day": 1, "week": 2, "fourWeeks": 2, "all": 3}
	for k, n := range want {
		if pages[k] != n {
			t.Fatalf("pages %v, want %v", pages, want)
		}
	}

	r.settings(`{"t":"clearData","since":"hour","history":true,"cache":true,"downloads":true}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "dataCleared" })
	var left int
	var names []string
	r.main(func() {
		left = r.a.History.Count(0)
		for _, d := range r.a.Downloads.List() {
			names = append(names, d.Name)
		}
	})
	if left != 2 {
		t.Fatalf("the last hour left %d pages", left)
	}
	if fmt.Sprint(names) != "[old.zip unknown.zip]" {
		t.Fatalf("the last hour left the downloads %v", names)
	}
	var erased []mockengine.Erase
	r.main(func() { erased = r.e.Erased() })
	if len(erased) == 0 || erased[0].Kinds != engine.EraseCache {
		t.Fatalf("erased %+v", erased)
	}
	if ago := time.Since(erased[0].Since); ago < 59*time.Minute || ago > 61*time.Minute {
		t.Fatalf("erased from %v ago", ago)
	}
}
