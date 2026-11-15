package app_test

import (
	"fmt"
	"testing"

	"projectb/engine"
	"projectb/pins"
	"projectb/tabs"
)

func isPins(n int) func(map[string]any) bool {
	return func(m map[string]any) bool { l, ok := m["pins"].([]any); return m["t"] == "pins" && ok && len(l) == n }
}

// tabsNow is window w's tabs as Go has them.
func (r *rig) tabsNow(w engine.WindowID) []tabs.Tab {
	var list []tabs.Tab
	r.e.Post(func() { list = r.a.Tabs.AllIn(w) })
	r.e.Settle()
	return list
}

// A shell pins a tab: every shell hears the pin, the tab carries it, and the profile keeps
// it. Closing the tab keeps the pin, openPin opens a tab for it, or shows the one open, and
// unpin removes it, its tab staying. A tab with no address, and one of ours, aren't pinned.
func TestAShellPinsATabAndThePinOutlivesItsTab(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	if snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); fmt.Sprint(snap["pins"]) != "[]" {
		t.Fatalf("a new profile's pins: %v", snap["pins"])
	}
	r.shell(`{"t":"pinTab","id":1}`)
	r.visit("https://a.test/")
	r.shell(`{"t":"pinTab","id":1}`)
	p := r.waitFor(isPins(1))["pins"].([]any)[0].(map[string]any)
	pin := p["id"].(string)
	if p["url"] != "https://a.test/" || p["workspace"] != "" || p["title"] != "Mock a.test" {
		t.Fatalf("the pin is %v", p)
	}
	r.waitFor(func(m map[string]any) bool {
		l, _ := m["tabs"].([]any)
		return m["t"] == "tabs" && len(l) == 1 && l[0].(map[string]any)["pin"] == pin
	})
	if l := pins.New(dir).List(); len(l) != 1 || l[0].ID != pin {
		t.Fatalf("the profile keeps %+v", l)
	}

	r.shell(`{"t":"closeTab","id":1}`)
	r.waitFor(isTabs(0))
	r.shell(`{"t":"openPin","pin":"` + pin + `"}`)
	r.waitForTab(func(t map[string]any) bool {
		return t["pin"] == pin && t["title"] == "Mock a.test" && t["loading"] == false
	})
	r.shell(`{"t":"newTab"}`)
	r.waitFor(isTabs(2))
	r.shell(`{"t":"openPin","pin":"` + pin + `"}`)
	r.e.Settle()
	open := r.tabsNow(1)
	if len(open) != 2 || open[0].Pin != pin || open[0].URL != "https://a.test/" || r.activeID() != open[0].ID {
		t.Fatalf("after openPin twice: %+v, active %v", open, r.activeID())
	}

	r.shell(`{"t":"unpin","pin":"` + pin + `"}`)
	r.waitFor(isPins(0))
	if now := r.tabsNow(1); len(now) != 2 || now[0].ID != open[0].ID || now[0].Pin != "" {
		t.Fatalf("after unpin: %+v", now)
	}

	r.openSettings()
	r.shell(fmt.Sprintf(`{"t":"pinTab","id":%d}`, r.settingsTab))
	r.e.Settle()
	if l := pins.New(dir).List(); len(l) != 0 {
		t.Fatalf("our tab was pinned: %+v", l)
	}
}

func (r *rig) activeID() engine.TabID {
	var id engine.TabID
	r.e.Post(func() { id = r.a.Tabs.ActiveID() })
	r.e.Settle()
	return id
}

// Pin Tab is in the Tab menu: off for a new tab, Pin Tab on a web tab, and Unpin Tab once
// it is pinned.
func TestPinTabFromTheMenu(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	item := func() engine.MenuItem {
		var found engine.MenuItem
		var walk func([]engine.MenuItem)
		walk = func(items []engine.MenuItem) {
			for _, it := range items {
				if it.Action == "tab.pin" {
					found = it
				}
				walk(it.Items)
			}
		}
		r.e.Post(func() { walk(r.e.Menu()) })
		r.e.Settle()
		return found
	}
	if it := item(); it.Title != "Pin Tab" || it.Enabled {
		t.Fatalf("on a new tab: %+v", it)
	}
	r.visit("https://a.test/")
	if it := item(); it.Title != "Pin Tab" || !it.Enabled {
		t.Fatalf("on a web tab: %+v", it)
	}
	r.e.Post(func() { r.a.MenuChosen("tab.pin") })
	r.waitFor(isPins(1))
	r.e.Settle()
	if it := item(); it.Title != "Unpin Tab" || !it.Enabled {
		t.Fatalf("on a pinned tab: %+v", it)
	}
	r.e.Post(func() { r.a.MenuChosen("tab.pin") })
	r.waitFor(isPins(0))
}

// Deleting a workspace deletes its pins, and leaves the others.
func TestDeletingAWorkspaceDeletesItsPins(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.shell(`{"t":"pinTab","id":1}`)
	r.shell(`{"t":"createWorkspace","text":"Work","color":"#5b7fd6"}`)
	m := r.waitFor(func(m map[string]any) bool {
		l, _ := m["workspaces"].([]any)
		return m["t"] == "workspaces" && len(l) == 2
	})
	work := m["workspaces"].([]any)[1].(map[string]any)["id"].(string)
	r.shell(`{"t":"newTab","workspace":"` + work + `","text":"https://b.test/"}`)
	r.waitForTab(func(t map[string]any) bool { return t["title"] == "Mock b.test" && t["loading"] == false })
	r.shell(fmt.Sprintf(`{"t":"pinTab","id":%d}`, r.activeID()))
	r.waitFor(isPins(2))
	r.shell(`{"t":"deleteWorkspace","workspace":"` + work + `"}`)
	r.waitFor(isPins(1))
	if l := pins.New(dir).List(); len(l) != 1 || l[0].Workspace != "" {
		t.Fatalf("the profile keeps %+v", l)
	}
}
