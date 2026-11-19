package app_test

import (
	"slices"
	"testing"

	"projectb/engine"
	"projectb/tabs"
)

// A phone's menu sheet opening starts the window's settings page out of sight: no tab to the
// shell, which lists the same tabs and shows the same one, and its page gets its snapshot as
// it starts. The menu's Settings then shows that tab, its page ready, beside the active one;
// a menu opened while it is shown makes no other.
func TestTheMenuOpeningStartsSettingsOutOfSight(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	var before []tabs.Tab
	var active engine.TabID
	r.main(func() { before, active = r.a.Tabs.AllIn(1), r.a.Tabs.ActiveIn(1) })

	r.shell(`{"t":"menuOpened"}`)
	var ahead engine.TabID
	r.main(func() { ahead = r.a.Tabs.Prepared(1, "settings") })
	if ahead == 0 || r.settingsIn(1) != 0 {
		t.Fatalf("the menu opened: settings made ahead %v, shown %v", ahead, r.settingsIn(1))
	}
	got, stop := r.e.PageFeed(ahead).Subscribe()
	defer stop()
	r.e.Post(func() { r.a.PageMessage(ahead, "settings", []byte(`{"t":"ready"}`)) })
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "snapshot" })
	var now []tabs.Tab
	r.main(func() { now, active = r.a.Tabs.AllIn(1), r.a.Tabs.ActiveIn(1) })
	if len(now) != len(before) || active != before[0].ID || r.e.Focused() == ahead || slices.ContainsFunc(r.e.Layout(1).Tabs, func(v engine.VisibleTab) bool { return v.Tab == ahead }) {
		t.Fatalf("made ahead, the window has tabs %v, active %v, focused %v, shown %v", now, active, r.e.Focused(), r.e.Layout(1).Tabs)
	}

	r.shell(`{"t":"menuOpened"}`)
	r.shell(`{"t":"run","text":"app.settings"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == len(before)+1 })
	if m["active"] != float64(ahead) || r.settingsIn(1) != ahead {
		t.Fatalf("Settings showed %v, not the tab made ahead %v", m["active"], ahead)
	}
	r.shell(`{"t":"menuOpened"}`)
	r.main(func() { ahead = r.a.Tabs.Prepared(1, "settings") })
	if ahead != 0 {
		t.Fatalf("a menu opened over Settings made another: %v", ahead)
	}
}
