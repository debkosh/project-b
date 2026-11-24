package app_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"projectb/engine"
	"projectb/mockengine"
)

// startOn starts the app on a profile whose shell is the one named.
func startOn(t *testing.T, shell string) *rig {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"shell":"`+shell+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return start(t, dir)
}

// captures are the pictures the app has asked the engine for, once what was sent before
// has run.
func (r *rig) captures() []mockengine.Capture {
	got := make(chan []mockengine.Capture)
	r.e.Post(func() { got <- r.e.Captures() })
	return <-got
}

// activeTab is the window's active tab.
func (r *rig) activeTab() engine.TabID {
	got := make(chan engine.TabID)
	r.e.Post(func() { got <- r.a.Tabs.ActiveIn(1) })
	return <-got
}

// On a phone shell, the tab a window leaves is pictured as the next becomes active, while
// its page still shows, and the tab's thumb then points at the picture.
func TestAPhoneShellPicturesTheTabItLeaves(t *testing.T) {
	r := startOn(t, "mwshell")
	r.visit("https://a.test/")
	a := r.activeTab()
	r.shell(`{"t":"newTab","text":"https://b.test/"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["id"] == float64(a) && tab["thumb"] == fmt.Sprintf("/mock/thumb/%d?v=1", a)
	})
	if got, want := r.captures(), []mockengine.Capture{{Tab: a, MaxWidth: 720}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("captures %v, want %v", got, want)
	}
}

// The shell's captureTab pictures the active tab, as the Mobile shell's switcher opens.
func TestCaptureTabPicturesTheActiveTab(t *testing.T) {
	r := startOn(t, "mwshell")
	r.visit("https://a.test/")
	a := r.activeTab()
	r.shell(`{"t":"captureTab"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["id"] == float64(a) && tab["thumb"] != nil })
	if got := r.captures(); len(got) != 1 || got[0].Tab != a {
		t.Fatalf("captures %v", got)
	}
}

// A tab with no page yet shows the shell's new tab page, which is no picture of a page.
func TestATabWithNoPageIsNotPictured(t *testing.T) {
	r := startOn(t, "mwshell")
	r.shell(`{"t":"captureTab"}`)
	r.shell(`{"t":"newTab"}`)
	if got := r.captures(); len(got) != 0 {
		t.Fatalf("captures %v", got)
	}
}

// A desktop shell's windows never ask for a picture, whatever happens.
func TestADesktopShellAsksForNoPictures(t *testing.T) {
	r := start(t, t.TempDir())
	r.visit("https://a.test/")
	r.shell(`{"t":"captureTab"}`)
	r.shell(`{"t":"newTab","text":"https://b.test/"}`)
	r.shell(`{"t":"activateTab","id":1}`)
	if got := r.captures(); len(got) != 0 {
		t.Fatalf("captures %v", got)
	}
}

// A window asks for one picture each half second at most: a burst of switches, or the
// switcher opening as a switch happens, costs one copy, and the next after the gap is asked.
func TestPicturesAreAskedForAtMostTwiceASecond(t *testing.T) {
	r := startOn(t, "mwshell")
	r.visit("https://a.test/")
	a := r.activeTab()
	r.shell(`{"t":"newTab","text":"https://b.test/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://b.test/" && tab["loading"] == false })
	b := r.activeTab()
	r.shell(fmt.Sprintf(`{"t":"activateTab","id":%d}`, a))
	r.shell(fmt.Sprintf(`{"t":"activateTab","id":%d}`, b))
	r.shell(`{"t":"captureTab"}`)
	if got := r.captures(); len(got) != 1 || got[0].Tab != a {
		t.Fatalf("a burst asked for %v", got)
	}
	time.Sleep(550 * time.Millisecond)
	r.shell(`{"t":"captureTab"}`)
	if got := r.captures(); len(got) != 2 || got[1].Tab != b {
		t.Fatalf("after the gap: %v", got)
	}
}
