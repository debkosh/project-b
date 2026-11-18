package app_test

import (
	"fmt"
	"testing"

	"projectb/engine"
	"projectb/mockengine"
	"projectb/workspaces"
)

// Sites: as the page opens, Go has the engine count every workspace's storage and shows
// it by site, each with its hosts, cookies and bytes in every workspace and its hosts'
// permissions; an extension's storage isn't a site's. A site's Clear erases it in every
// workspace, and Clear all every site's; the permissions stay.
func TestSettingsListsAndClearsSiteData(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work workspaces.Workspace
	r.main(func() {
		work, _ = r.a.Workspaces.Create("Work", "#5b7fd6")
		r.e.AddSiteData("", engine.SiteData{Host: "mail.example.com", Cookies: 3, Bytes: 1000})
		r.e.AddSiteData("", engine.SiteData{Host: "example.com", Cookies: 2})
		r.e.AddSiteData(work.ID, engine.SiteData{Host: "mail.example.com", Bytes: 24})
		r.e.AddSiteData(work.ID, engine.SiteData{Host: "news.test", Cookies: 1, Bytes: 500})
		r.e.AddSiteData("", engine.SiteData{Host: "abcdefghijklmnopabcdefghijklmnop", Bytes: 100})
		r.a.Sites.Set("https://meet.example.com", "camera", "allow")
	})
	got, _ := r.openSettings()
	rows := func(m map[string]any) string {
		var s string
		for _, x := range m["sites"].([]any) {
			row := x.(map[string]any)
			s += fmt.Sprintf("%v %v %v %v %v; ", row["site"], row["hosts"], row["origins"], row["cookies"], row["bytes"])
		}
		return s
	}
	want := "example.com [example.com mail.example.com] [https://meet.example.com] 5 1024; news.test [news.test] [] 1 500; "
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sites" && rows(m) == want })
	if rows(m) != want {
		t.Fatalf("sites %s", rows(m))
	}

	r.settings(`{"t":"clearSiteData","site":"example.com"}`)
	want = "example.com [] [https://meet.example.com] 0 0; news.test [news.test] [] 1 500; "
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sites" && rows(m) == want })
	var erased []mockengine.Erase
	r.main(func() { erased = r.e.Erased() })
	if len(erased) != 2 || erased[0].Workspace != "" || erased[1].Workspace != work.ID ||
		fmt.Sprint(erased[0].Sites) != "[example.com]" || erased[0].Kinds != engine.EraseAll {
		t.Fatalf("erased %+v", erased)
	}

	r.settings(`{"t":"clearAllSiteData"}`)
	want = "example.com [] [https://meet.example.com] 0 0; "
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sites" && rows(m) == want })
	r.main(func() { erased = r.e.Erased() })
	if len(erased) != 4 || len(erased[2].Sites) != 0 || erased[2].Kinds != engine.EraseAll {
		t.Fatalf("erased %+v", erased)
	}
}
