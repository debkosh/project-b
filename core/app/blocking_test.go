package app_test

import (
	"path/filepath"
	"testing"
	"time"

	"projectb/app"
	"projectb/blocking"
	"projectb/blocking/blockingtest"
	"projectb/profiles"
	"projectb/update"
)

// startWithLists starts the app on a release of the test catalogue's lists.
func startWithLists(t *testing.T, rel *blockingtest.Release) *rig {
	t.Helper()
	dir := t.TempDir()
	return startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Update: update.Config{StateDir: filepath.Join(dir, "updates")},
		Blocking: blocking.Setup{BaseURL: rel.URL(), Key: rel.Public, Catalogue: blockingtest.Catalogue()}})
}

func waitUntil(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func notice(tab map[string]any) map[string]any {
	n, _ := tab["notice"].(map[string]any)
	return n
}

// A site the Safe Browsing lists name doesn't open: the tab's notice says what it may do and
// which list said so, with Go back first and Open anyway, which opens it and lets its host
// through; Go back leaves another one. Warn before dangerous sites off, nothing is asked.
func TestADangerousSiteGetsTheUnsafeNotice(t *testing.T) {
	rel := blockingtest.New(t)
	rel.Put("urlhaus", "||harm.test^\n")
	rel.Put("phishing-filter", "|https://steal.test/login|$document\n")
	rel.Publish()
	r := startWithLists(t, rel)
	defer r.stop()
	waitUntil(t, "the safe lists", func() bool {
		_, _, a := r.a.Blocking.Dangerous("https://harm.test/")
		_, _, b := r.a.Blocking.Dangerous("https://steal.test/login")
		return a && b
	})
	r.visit("https://good.test/")
	r.visit("https://good.test/next")
	r.shell(`{"t":"navigate","text":"https://harm.test/download"}`)
	// With Go back once the tab can go back.
	tab := r.waitForTab(func(tab map[string]any) bool { return notice(tab) != nil && tab["canGoBack"] == true })
	n := notice(tab)
	actions := n["actions"].([]any)
	if n["kind"] != "unsafe" || n["title"] != "harm.test may harm your device" || n["detail"] != "URLhaus filter lists it as a malware site." || len(actions) != 2 {
		t.Fatalf("notice %v", n)
	}
	if back := actions[0].(map[string]any); back["id"] != "back" || back["title"] != "Go back" || back["primary"] != true {
		t.Fatalf("first action %v", back)
	}
	if open := actions[1].(map[string]any); open["id"] != "proceed" || open["title"] != "Open anyway" {
		t.Fatalf("second action %v", open)
	}
	r.shell(`{"t":"noticeAction","id":1,"action":"proceed"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://harm.test/download" && tab["loading"] == false && notice(tab) == nil
	})
	// The host passes from now on, until the app quits, and its pages say they are
	// dangerous, for the shells' Dangerous in the address field.
	danger := func(d any) bool {
		m, _ := d.(map[string]any)
		return m["kind"] == "malware" && m["list"] == "URLhaus filter"
	}
	r.shell(`{"t":"navigate","text":"https://harm.test/other"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://harm.test/other" && tab["loading"] == false && notice(tab) == nil && danger(tab["danger"])
	})
	// The site view's first row says which list named the site.
	got, snap := r.openSettings()
	if c, _ := snap["siteConnections"].(map[string]any)["https://harm.test"].(map[string]any); !danger(c["danger"]) {
		t.Fatalf("harm.test's site view %v", snap["siteConnections"])
	}
	r.main(func() { r.a.Tabs.Activate(1) })

	r.shell(`{"t":"navigate","text":"https://steal.test/login"}`)
	tab = r.waitForTab(func(tab map[string]any) bool { return notice(tab) != nil && len(notice(tab)["actions"].([]any)) == 2 })
	if n := notice(tab); n["title"] != "steal.test may be trying to steal your information" || n["detail"] != "Phishing URL Blocklist lists it as a phishing site." {
		t.Fatalf("phishing notice %v", n)
	}
	r.shell(`{"t":"noticeAction","id":1,"action":"back"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] != "https://steal.test/login" && notice(tab) == nil })

	r.settings(`{"t":"setPref","name":"warnDangerous","value":"false"}`)
	r.waitOn(got, func(m map[string]any) bool {
		return m["t"] == "prefs" && m["prefs"].(map[string]any)["warnDangerous"] == false
	})
	r.main(func() { r.a.Tabs.Activate(1) })
	r.shell(`{"t":"navigate","text":"https://steal.test/login"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://steal.test/login" && tab["loading"] == false && notice(tab) == nil
	})
}

// Settings has the Lists page's groups and lists, and switches a list and a group: a group
// on brings back its defaults, or all its lists where none is on by default, and off turns
// them all off; only the changes from the defaults are kept in the prefs.
func TestSettingsSwitchesTheBlockLists(t *testing.T) {
	rel := blockingtest.New(t)
	r := startWithLists(t, rel)
	defer r.stop()
	got, snap := r.openSettings()
	shown := snap["blocking"].(map[string]any)
	groups := shown["groups"].([]any)
	if len(groups) != 6 || shown["downloaded"] != false {
		t.Fatalf("groups %v", shown)
	}
	ads := groups[0].(map[string]any)
	if ads["id"] != "ads" || ads["on"] != true || ads["onCount"] != float64(2) || ads["total"] != float64(2) || len(ads["lists"].([]any)) != 2 {
		t.Fatalf("ads %v", ads)
	}
	group := func(m map[string]any, id string) map[string]any {
		for _, g := range m["blocking"].(map[string]any)["groups"].([]any) {
			if g := g.(map[string]any); g["id"] == id {
				return g
			}
		}
		return nil
	}
	r.settings(`{"t":"setBlockGroup","id":"annoyances","on":true}`)
	m := r.waitOn(got, func(m map[string]any) bool {
		return m["t"] == "blocking" && group(m, "annoyances")["onCount"] == float64(1)
	})
	if g := group(m, "annoyances"); g["on"] != true {
		t.Fatalf("annoyances %v", g)
	}
	r.settings(`{"t":"setBlockList","id":"easylist","on":false}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "blocking" && group(m, "ads")["onCount"] == float64(1) })
	r.settings(`{"t":"setBlockGroup","id":"ads","on":false}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "blocking" && group(m, "ads")["on"] == false })
	var lists map[string]bool
	r.main(func() { lists = r.a.Prefs.All().BlockLists })
	if len(lists) != 3 || !lists["cookies"] || lists["easylist"] || lists["unbreak"] {
		t.Fatalf("prefs %v", lists)
	}
	r.settings(`{"t":"setBlockGroup","id":"ads","on":true}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "blocking" && group(m, "ads")["onCount"] == float64(2) })
	r.main(func() { lists = r.a.Prefs.All().BlockLists })
	if len(lists) != 1 || !lists["cookies"] {
		t.Fatalf("prefs after ads on %v", lists)
	}
}
