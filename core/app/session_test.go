package app_test

import (
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

func TestTabsComeBackAfterACrashAndNotAfterAQuit(t *testing.T) {
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

	// A crash leaves crash.log behind.
	os.WriteFile(filepath.Join(dir, "crash.log"), []byte("pid 1\n"), 0o600)
	again := start(t, dir)
	var tabs []string
	var active, waiting engine.TabID
	again.onMain(func() {
		for _, tab := range again.a.Tabs.All() {
			tabs = append(tabs, tab.URL)
			if tab.URL == "https://a.test/" {
				waiting = tab.ID
			}
		}
		active = again.a.Tabs.ActiveIn(1)
	})
	if len(tabs) != 2 || tabs[0] != "https://a.test/" || tabs[1] != "https://b.test/" {
		t.Fatalf("after a crash the tabs are %v", tabs)
	}
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
	})
	again.stop()

	// A clean quit: one empty tab, as before.
	clean := start(t, dir)
	defer clean.stop()
	clean.onMain(func() {
		if n := len(clean.a.Tabs.All()); n != 1 {
			t.Errorf("after a clean quit the app came back with %d tabs", n)
		}
	})
}
