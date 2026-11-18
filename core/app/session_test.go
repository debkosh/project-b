package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"projectb/engine"
)

// onMain runs f on the engine's thread and waits for it.
func (r *rig) onMain(f func()) {
	done := make(chan struct{})
	r.e.Post(func() { f(); close(done) })
	<-done
}

// urls is every tab's address, in the windows' order.
func (r *rig) urls() []string {
	var out []string
	r.onMain(func() {
		for _, tab := range r.a.Tabs.All() {
			out = append(out, tab.URL)
		}
	})
	return out
}

func TestTabsComeBackAtEveryStartUnlessTheSettingSaysANewTabButAlwaysAfterACrash(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.visit("https://a.test/")
	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	r.visit("https://b.test/")
	r.stop()
	if _, err := os.Stat(filepath.Join(dir, "session.json")); err != nil {
		t.Fatal(err)
	}

	// A clean quit, and the default: they come back, the one in the background unloaded.
	again := start(t, dir)
	if got := again.urls(); len(got) != 2 || got[0] != "https://a.test/" || got[1] != "https://b.test/" {
		t.Fatalf("after a quit the tabs are %v", got)
	}
	var active, waiting engine.TabID
	again.onMain(func() {
		for _, tab := range again.a.Tabs.All() {
			if tab.URL == "https://a.test/" {
				waiting = tab.ID
			}
		}
		active = again.a.Tabs.ActiveIn(1)
	})
	again.e.Settle()
	again.onMain(func() {
		if got, _ := again.a.Tabs.Get(active); got.URL != "https://b.test/" {
			t.Errorf("the active tab is %s, want b.test's", got.URL)
		}
		if got := again.e.Loaded(waiting); got == "https://a.test/" {
			t.Error("a background tab loaded before it was shown")
		}
		again.a.Tabs.Activate(waiting)
	})
	again.waitForTab(func(tab map[string]any) bool {
		return tab["id"] == float64(waiting) && tab["loading"] == false && tab["title"] == "Mock a.test"
	})
	again.onMain(func() {
		if got := again.e.Loaded(waiting); got != "https://a.test/" {
			t.Errorf("shown, it loaded %q", got)
		}
		again.a.Prefs.SetStart("fresh")
	})
	again.stop()

	// A crash brings them back whatever the setting says.
	os.WriteFile(filepath.Join(dir, "crash.log"), []byte("pid 1\n"), 0o600)
	crashed := start(t, dir)
	if got := crashed.urls(); len(got) != 2 {
		t.Fatalf("after a crash the tabs are %v", got)
	}
	crashed.onMain(func() {
		for _, tab := range crashed.a.Tabs.All() {
			if tab.URL == "https://b.test/" && tab.Title != "Mock b.test" {
				t.Errorf("after a crash b.test's tab is titled %q", tab.Title)
			}
		}
	})
	crashed.stop()

	// A clean quit with On start set to start fresh: none of them.
	fresh := start(t, dir)
	defer fresh.stop()
	for _, u := range fresh.urls() {
		if u != "" {
			t.Fatalf("starting fresh the tabs are %v", fresh.urls())
		}
	}
}

func TestAClosedTabAndAClosedWindowReopenNewestFirst(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.shell(`{"t":"newTab"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	b := m["active"].(float64)
	r.visit("https://b.test/")
	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	r.visit("https://c.test/")
	r.shell(fmt.Sprintf(`{"t":"closeTab","id":%v}`, b))
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })

	var w2 engine.WindowID
	r.onMain(func() { w2, _ = r.a.NewWindow("https://d.test/") })
	r.e.Settle()
	r.e.Fire(w2, 0, "close", "")
	r.e.Settle()

	var titles []string
	r.onMain(func() {
		for _, it := range r.a.Commands.ListItems("recentlyClosed") {
			titles = append(titles, it.Title)
		}
	})
	if len(titles) != 2 || titles[0] != "1 tab" || titles[1] != "Mock b.test" {
		t.Fatalf("Recently Closed lists %v", titles)
	}

	// The newest, the window, comes back with its tab.
	r.onMain(func() { r.a.Commands.Run("tab.reopenClosed") })
	r.e.Settle()
	if got := r.urls(); len(got) != 3 || got[2] != "https://d.test/" {
		t.Fatalf("after reopening the window the tabs are %v", got)
	}
	// Then the tab, into its window at its place, active.
	r.onMain(func() {
		r.a.Window.Request(1, engine.WindowFront)
		r.a.Commands.Run("tab.reopenClosed")
	})
	r.e.Settle()
	r.onMain(func() {
		in1 := r.a.Tabs.AllIn(1)
		if len(in1) != 3 || in1[1].URL != "https://b.test/" || r.a.Tabs.ActiveIn(1) != in1[1].ID {
			t.Errorf("window 1 after reopening the tab: %+v, active %v", in1, r.a.Tabs.ActiveIn(1))
		}
		if len(r.a.Commands.ListItems("recentlyClosed")) != 0 {
			t.Error("Recently Closed still lists what came back")
		}
	})
}
