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

// ask runs a question on the app's thread and waits for the answer it posts back, as
// history's do.
func ask[T any](r *rig, question func(answer func(T))) T {
	got := make(chan T, 1)
	r.main(func() { question(func(v T) { got <- v }) })
	return <-got
}

// The shell hears each open site's settings, in the table's order, its own kinds with their
// choices and defaults, and the sparks that run on its page; and the panel's change comes
// back.
func TestTheShellHearsEachOpenSitesSettingsAndSetsThem(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.main(func() {
		for _, s := range r.a.Sparks.Builtins() {
			r.a.Sparks.Remove(s.ID)
		}
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
	if sp := site(m)["sparks"].([]any); len(sp) != 1 || sp[0] != "Wide" || byKind["sparks"] != "on" {
		t.Fatalf("sparks %v, %q", sp, byKind["sparks"])
	}
	r.shell(`{"t":"setSiteSetting","origin":"https://news.test","kind":"sparks","value":"off"}`)
	r.waitFor(func(m map[string]any) bool {
		s := site(m)
		if m["t"] != "openSites" || s == nil {
			return false
		}
		for _, x := range s["settings"].([]any) {
			if x := x.(map[string]any); x["kind"] == "sparks" {
				return x["value"] == "off"
			}
		}
		return false
	})
	var scripts []engine.Script
	r.main(func() { scripts = r.a.SparksFor(1, "https://news.test/today", true) })
	if len(scripts) != 0 {
		t.Fatalf("a spark runs where sparks are off: %v", scripts)
	}
}

// Links to other sites holds a page's own navigations, not the person's: under Block, a
// link or script of the page's to another site goes nowhere and the page stays; one to its
// own site goes; the address field goes anywhere; a tab the page opens on another site is
// closed and counted on its opener; and Allow lets the page go anywhere.
func TestLinksToOtherSitesHoldThePagesOwnNavigations(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://shop.test/")
	r.main(func() { r.a.Sites.Set("https://shop.test", sites.Links, sites.Block) })
	tabNow := func() (u string, n int, popups int) {
		r.main(func() {
			tb, _ := r.a.Tabs.Get(1)
			u, n, popups = tb.URL, len(r.a.Tabs.All()), tb.PopupsBlocked
		})
		return
	}
	settle := func() { r.e.Settle(); r.main(func() {}); r.e.Settle() }

	r.e.Fire(1, 1, "navigate", "https://elsewhere.test/")
	settle()
	if u, _, _ := tabNow(); u != "https://shop.test/" {
		t.Fatalf("a page's link to another site under Block took the tab to %q", u)
	}
	r.e.Fire(1, 1, "navigate", "https://shop.test/cart")
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://shop.test/cart" && tab["loading"] == false })

	r.e.Fire(1, 1, "open", "https://ads.test/")
	settle()
	if _, n, popups := tabNow(); n != 1 || popups != 1 {
		t.Fatalf("a popup to another site under Block: %d tabs, %d counted", n, popups)
	}
	r.e.Fire(1, 1, "open", "https://shop.test/help")
	settle()
	if _, n, _ := tabNow(); n != 2 {
		t.Fatalf("a popup to the page's own site under Block: %d tabs", n)
	}
	r.main(func() {
		for _, x := range r.a.Tabs.All() {
			if x.ID != 1 {
				r.a.Tabs.Close(x.ID)
			}
		}
		r.a.Tabs.Activate(1)
	})

	// The person's own way goes anywhere.
	r.visit("https://elsewhere.test/")

	r.visit("https://shop.test/")
	r.main(func() { r.a.Sites.Set("https://shop.test", sites.Links, sites.Allow) })
	r.e.Fire(1, 1, "navigate", "https://elsewhere.test/")
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://elsewhere.test/" })
}

// A site's Pop-ups at Block, the default, a tab its page opens with no tap first closes as
// it opens, counted on the page with its address; one after a tap opens. The site panel's
// Open it opens it, and Pop-ups at Allow lets the next one open.
func TestASitesPopupsAreBlockedUntilAllowed(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://news.test/")
	settle := func() { r.e.Settle(); r.main(func() {}); r.e.Settle() }
	now := func() (n int, tb tabsTab) {
		r.main(func() {
			x, _ := r.a.Tabs.Get(1)
			n, tb = len(r.a.Tabs.All()), tabsTab{x.PopupsBlocked, x.BlockedPopup}
		})
		return
	}

	r.e.Fire(1, 1, "popup", "https://ads.test/win")
	settle()
	if n, tb := now(); n != 1 || tb != (tabsTab{1, "https://ads.test/win"}) {
		t.Fatalf("a pop-up under Block: %d tabs, %v", n, tb)
	}
	r.e.Fire(1, 1, "open", "https://ads.test/tapped")
	settle()
	if n, _ := now(); n != 2 {
		t.Fatalf("a tab opened after a tap: %d tabs", n)
	}
	r.main(func() { r.a.Tabs.Activate(1) })

	r.shell(`{"t":"openBlockedPopup"}`)
	settle()
	if n, tb := now(); n != 3 || tb != (tabsTab{0, ""}) {
		t.Fatalf("after Open it: %d tabs, %v", n, tb)
	}
	r.main(func() { r.a.Tabs.Activate(1) })

	r.main(func() { r.a.Sites.Set("https://news.test", sites.Popups, sites.Allow) })
	r.e.Fire(1, 1, "popup", "https://ads.test/again")
	settle()
	if n, tb := now(); n != 4 || tb.count != 0 {
		t.Fatalf("a pop-up under Allow: %d tabs, %v", n, tb)
	}
}

// tabsTab is what a tab says of the pop-ups its page opened that Go closed.
type tabsTab struct {
	count int
	last  string
}

// The engine has the ad blocker's hosts from the start, and the sites it is off for as each
// switches; a blocked request counts on the tab until its next page. Off on every site, the
// engine has no hosts and the site panel no switch.
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

	// Off on every site, Settings' Block ads: the rules have no hosts, and the panel no
	// switch; on again, the hosts and each site's own come back.
	got, _ := r.openSettings()
	r.settings(`{"t":"setPref","name":"blockAds","value":"false"}`)
	r.waitOn(got, func(m map[string]any) bool {
		return m["t"] == "prefs" && m["prefs"].(map[string]any)["blockAds"] == false
	})
	r.main(func() { rules = r.e.AdRules() })
	if len(rules.Hosts) != 0 {
		t.Fatalf("off on every site, %d hosts", len(rules.Hosts))
	}
	hasAds := func(m map[string]any) bool {
		site, _ := m["openSites"].(map[string]any)["https://news.test"].(map[string]any)
		for _, s := range site["settings"].([]any) {
			if s.(map[string]any)["kind"] == "ads" {
				return true
			}
		}
		return false
	}
	r.waitFor(func(m map[string]any) bool { return m["t"] == "openSites" && !hasAds(m) })
	r.settings(`{"t":"setPref","name":"blockAds","value":"true"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "openSites" && hasAds(m) })
	r.main(func() { rules = r.e.AdRules() })
	if !slices.Contains(rules.Hosts, "doubleclick.net") || !slices.Equal(rules.Off, []string{"https://news.test"}) {
		t.Fatalf("on again: %d hosts, off %v", len(rules.Hosts), rules.Off)
	}
}
