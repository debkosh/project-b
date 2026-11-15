package app_test

import (
	"slices"
	"testing"

	"projectb/engine"
	"projectb/sites"
)

// main runs f on the app's thread and waits for it.
func (r *rig) main(f func()) {
	done := make(chan struct{})
	r.e.Post(func() { f(); close(done) })
	<-done
}

// The shell hears each open site's settings, in the table's order, its own kinds with their
// choices and defaults, and the sparks its page matches; and the panel's change comes back.
func TestTheShellHearsEachOpenSitesSettingsAndSetsThem(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.main(func() {
		if _, err := r.a.Sparks.Add("// ==UserScript==\n// @name Wide\n// @match https://news.test/*\n// ==/UserScript==\n"); err != nil {
			t.Error(err)
		}
	})
	r.shell(`{"t":"navigate","text":"https://news.test/today"}`)
	site := func(m map[string]any) map[string]any {
		all, _ := m["openSites"].(map[string]any)
		s, _ := all["https://news.test"].(map[string]any)
		return s
	}
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "openSites" && site(m) != nil })
	settings := site(m)["settings"].([]any)
	byKind := map[string]string{}
	for _, s := range settings {
		s := s.(map[string]any)
		byKind[s["kind"].(string)] = s["value"].(string)
	}
	if byKind["ads"] != "on" || byKind["links"] != "allow" || byKind["camera"] != "" {
		t.Fatalf("settings %v", settings)
	}
	sp := site(m)["sparks"].([]any)
	if len(sp) != 1 || sp[0].(map[string]any)["name"] != "Wide" || sp[0].(map[string]any)["on"] != true {
		t.Fatalf("sparks %v", sp)
	}
	id := sp[0].(map[string]any)["id"].(string)
	r.shell(`{"t":"setSiteSetting","origin":"https://news.test","kind":"spark:` + id + `","value":"off"}`)
	r.waitFor(func(m map[string]any) bool {
		s := site(m)
		if m["t"] != "openSites" || s == nil || len(s["sparks"].([]any)) == 0 {
			return false
		}
		return s["sparks"].([]any)[0].(map[string]any)["on"] == false
	})
	var scripts []engine.Script
	r.main(func() { scripts = r.a.SparksFor(1, "https://news.test/today", true) })
	if len(scripts) != 0 {
		t.Fatalf("a spark off for the site still runs: %v", scripts)
	}
}

// Links to other sites: Allow lets the page go; New tab opens a pressed link beside it and
// keeps the tab; Block, or New tab with no press, keeps the tab and opens nothing.
func TestLinksToOtherSitesFollowTheSitesSetting(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://shop.test/")
	ask := func(pressed bool) (n engine.Navigation, count int) {
		r.main(func() {
			n = r.a.LeavingSite(1, "https://shop.test/", "https://elsewhere.test/", pressed)
			count = len(r.a.Tabs.All())
		})
		return
	}
	if n, c := ask(true); n != engine.NavigationProceed || c != 1 {
		t.Fatalf("allow: %v, %d tabs", n, c)
	}
	r.main(func() { r.a.Sites.Set("https://shop.test", sites.Links, sites.NewTab) })
	if n, c := ask(false); n != engine.NavigationElsewhere || c != 1 {
		t.Fatalf("new tab with no press: %v, %d tabs", n, c)
	}
	if n, c := ask(true); n != engine.NavigationElsewhere || c != 2 {
		t.Fatalf("new tab: %v, %d tabs", n, c)
	}
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://elsewhere.test/" })
	r.main(func() { r.a.Sites.Set("https://shop.test", sites.Links, sites.Block) })
	if n, c := ask(true); n != engine.NavigationElsewhere || c != 2 {
		t.Fatalf("block: %v, %d tabs", n, c)
	}
}

// The engine has the ad blocker's hosts from the start, and the sites it is off for as each
// switches; a blocked request counts on the tab until its next page.
func TestTheAdBlockersRulesReachTheEngineAndItsCountTheTab(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var rules engine.AdRules
	r.main(func() { rules = r.e.AdRules() })
	if !slices.Contains(rules.Hosts, "doubleclick.net") || len(rules.Off) != 0 {
		t.Fatalf("at start %d hosts, off %v", len(rules.Hosts), rules.Off)
	}
	r.shell(`{"t":"setSiteSetting","origin":"https://news.test","kind":"ads","value":"off"}`)
	r.main(func() { rules = r.e.AdRules() })
	if !slices.Equal(rules.Off, []string{"https://news.test"}) {
		t.Fatalf("off %v", rules.Off)
	}
	r.visit("https://news.test/")
	r.main(func() { r.a.AdBlocked(1); r.a.AdBlocked(1) })
	r.waitForTab(func(tab map[string]any) bool { return tab["adsBlocked"] == float64(2) })
	r.shell(`{"t":"navigate","text":"https://news.test/next"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://news.test/next" && tab["adsBlocked"] == nil
	})
}
