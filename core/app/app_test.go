package app_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"projectb/app"
	"projectb/commands"
	"projectb/engine"
	"projectb/mockengine"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/shells"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/tones"
	"projectb/update"
	"projectb/window"
	"projectb/workspaces"
)

// rig runs the real managers on the mock engine, as the dev server does.
type rig struct {
	t     *testing.T
	e     *mockengine.Engine
	a     *app.App
	got   <-chan []byte
	pages <-chan []byte // What the mock page is told to show.
	// settingsTab is the tab openSettings opened, whose page settings speaks for.
	settingsTab engine.TabID
}

func start(t *testing.T, dir string, blocked ...string) *rig {
	t.Helper()
	return startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), BlockedHosts: blocked, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
}

// plainShell is the first shell the registry lists that takes none of Go's actions.
func plainShell() string {
	for _, s := range shells.Listed() {
		if len(s.Takes) == 0 {
			return s.ID
		}
	}
	panic("every listed shell takes an action of Go's")
}

// startWith starts the app as start does, on cfg. A profile folder with no prefs yet gets
// a shell that takes none of Go's actions, so these tests see Go's own New Tab and a first
// window with its tab, whichever shell is the default; a test of a shell that takes one
// writes its own prefs first.
func startWith(t *testing.T, cfg app.Config) *rig {
	t.Helper()
	if prefsFile := filepath.Join(cfg.ProfileDir, "prefs.json"); cfg.ProfileDir != "" {
		if _, err := os.Stat(prefsFile); os.IsNotExist(err) {
			if err := os.WriteFile(prefsFile, []byte(`{"shell":"`+plainShell()+`"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	e := mockengine.New()
	got, stop := e.Feeds(1).Shell.Subscribe()
	t.Cleanup(stop)
	pages, stopPages := e.Feeds(1).Page.Subscribe()
	t.Cleanup(stopPages)
	r := &rig{t: t, e: e, got: got, pages: pages}
	ready := make(chan struct{})
	e.Post(func() {
		r.a = app.Start(e, cfg)
		e.SetListener(r.a)
		close(ready)
	})
	go e.Run()
	<-ready
	// The shell says it's ready as it starts, and only then hears from Go.
	r.shell(`{"t":"ready"}`)
	return r
}

// The app opens its profile in the engine as it starts, once, with the profile's folder.
func TestStartOpensTheProfileInTheEngine(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	opened := make(chan []string)
	r.e.Post(func() { opened <- r.e.Profiles() })
	if got := <-opened; !reflect.DeepEqual(got, []string{dir}) {
		t.Fatalf("opened %q, want %q", got, dir)
	}
}

// The app hands the engine its blocked hosts as it starts, once, so the engine refuses
// a page's own requests to them.
func TestStartHandsTheEngineTheBlockedHosts(t *testing.T) {
	r := start(t, t.TempDir(), "blocked.test", " Other.Test ")
	got := make(chan []engine.BlockRules)
	r.e.Post(func() { got <- r.e.BlockRules() })
	want := []engine.BlockRules{{Hosts: []string{"blocked.test", "other.test"}}}
	if rules := <-got; !reflect.DeepEqual(rules, want) {
		t.Fatalf("rules %v, want %v", rules, want)
	}
}

func (r *rig) shell(cmd string) { r.e.Post(func() { r.a.ShellMessage(1, []byte(cmd)) }) }

// show reports a layout from the shell that shows one tab, as the shell does for the
// active tab.
func (r *rig) show(tab float64) {
	r.shell(fmt.Sprintf(`{"t":"layout","tabs":[{"tab":%v,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`, tab))
}

// waitFor reads shell messages until one matches.
func (r *rig) waitFor(match func(map[string]any) bool) map[string]any {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-r.got:
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				r.t.Fatalf("bad message %s: %v", b, err)
			}
			if match(m) {
				return m
			}
		case <-deadline:
			r.t.Fatal("timed out waiting for shell message")
		}
	}
}

func (r *rig) waitForTab(match func(tab map[string]any) bool) map[string]any {
	r.t.Helper()
	return r.waitFor(func(m map[string]any) bool {
		tab, ok := m["tab"].(map[string]any)
		return ok && m["t"] == "tab" && match(tab)
	})["tab"].(map[string]any)
}

func (r *rig) visit(url string) {
	r.t.Helper()
	r.shell(`{"t":"navigate","text":"` + url + `"}`)
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == url && t["loading"] == false && strings.HasPrefix(t["title"].(string), "Mock")
	})
}

func (r *rig) suggestions(text string) []any {
	r.t.Helper()
	r.shell(`{"t":"suggest","id":7,"text":"` + text + `"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "suggestions" })
	if m["id"] != float64(7) {
		r.t.Fatalf("id = %v", m["id"])
	}
	items, _ := m["items"].([]any)
	return items
}

func (r *rig) stop() {
	done := make(chan struct{})
	r.e.Post(func() { r.a.Stop(); close(done) })
	<-done
}

func TestVisitBecomesSuggestion(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://example.com/docs")

	items := r.suggestions("exam")
	if len(items) != 1 {
		t.Fatalf("items = %v", items)
	}
	item := items[0].(map[string]any)
	if item["url"] != "https://example.com/docs" || item["title"] != "Mock example.com" {
		t.Fatalf("item = %v", item)
	}
}

func TestLoadProgressIsReportedOnTheTab(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"navigate","text":"https://example.com/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["progress"] == 0.5 && tab["loading"] == true })
	r.waitForTab(func(tab map[string]any) bool { return tab["progress"] == 1.0 })
}

func TestHistorySurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.visit("https://example.com/")
	r.stop()

	if _, err := os.Stat(filepath.Join(dir, "history.jsonl")); err != nil {
		t.Fatal(err)
	}

	r2 := start(t, dir)
	defer r2.stop()
	deadline := time.Now().Add(5 * time.Second)
	for len(r2.suggestions("example")) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("history did not load")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestBlockedURLNeverLoadsOrEntersHistory(t *testing.T) {
	r := start(t, t.TempDir(), "blocked.test")
	defer r.stop()
	r.shell(`{"t":"navigate","text":"https://www.blocked.test/x"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["blocked"] == true })
	if items := r.suggestions("blocked"); len(items) != 0 {
		t.Fatalf("items = %v", items)
	}
}

func TestTheFirstTabHoldsTheBlankPage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.show(1)
	deadline := time.After(5 * time.Second)
	for loaded := false; !loaded; {
		select {
		case b := <-r.pages:
			loaded = string(b) == `{"slot":0,"state":"loaded","tab":1,"url":"about:blank"}`
		case <-deadline:
			t.Fatal("the first tab never loaded the blank page")
		}
	}
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := m["tabs"].([]any)[0].(map[string]any)
	if tab["url"] != "" || tab["title"] != "" {
		t.Fatalf("the blank page shows as url %q title %q", tab["url"], tab["title"])
	}
	if items := r.suggestions("blank"); len(items) != 0 {
		t.Fatalf("items = %v", items)
	}
}

func TestTheShellNeverLoadsInATab(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"navigate","text":"chrome://shell/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "chrome://shell/" && tab["blocked"] == true })
	r.shell(`{"t":"navigate","text":"chrome://gpu/"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "chrome://gpu/" && tab["blocked"] == false && tab["title"] == "Mock gpu"
	})
}

func TestStaleNavigationDoesNotCommit(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"navigate","text":"https://first.test/"}`)
	r.shell(`{"t":"navigate","text":"https://second.test/"}`)
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://second.test/" && tab["loading"] == false })
	if items := r.suggestions("first"); len(items) != 0 {
		t.Fatalf("items = %v", items)
	}
}

// The window has no band until its shell asks for one, and then the one it asked for.
func TestTheShellsLayoutSetsTheBand(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if _, ok := m["frameWidth"]; ok || r.e.Layout(1).FrameWidth != 0 {
		t.Fatalf("before the shell asked: snapshot %v, engine %v", m["frameWidth"], r.e.Layout(1).FrameWidth)
	}
	r.shell(`{"t":"layout","shell":[],"drag":[],"tabs":[{"tab":1,"rect":{"x":4,"y":48,"w":792,"h":548}}],"viewW":800,"viewH":600,"frame":4}`)
	r.e.Settle()
	if r.e.Layout(1).FrameWidth != 4 {
		t.Fatalf("asked for 4: %+v", r.e.Layout(1))
	}
}

func TestShellRegionsDecideTheHitTest(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48},{"x":100,"y":48,"w":400,"h":200}],"drag":[{"x":300,"y":0,"w":500,"h":32}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	r.e.Settle()

	l := r.e.Layout(1)
	if !slices.Equal(l.Tabs, []engine.VisibleTab{{Tab: 1, Rect: engine.Rect{X: 0, Y: 48, W: 800, H: 552}}}) || l.FrameColor == "" {
		t.Fatalf("layout %+v", l)
	}
	for _, c := range []struct {
		x, y float64
		want engine.Area
	}{{10, 10, engine.AreaShell}, {120, 100, engine.AreaShell}, {10, 100, engine.AreaPage}, {600, 300, engine.AreaPage}, {400, 10, engine.AreaDrag}, {400, 40, engine.AreaShell}} {
		if got := l.At(c.x, c.y); got != c.want {
			t.Errorf("At(%v,%v) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestShellWindowRequestsReachTheEngine(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48}],"drag":[{"x":300,"y":0,"w":500,"h":32}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	r.shell(`{"t":"window","text":"fullScreen"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "controls" && m["cornerRadius"] == float64(0) })
	if got := r.e.Layout(1).At(400, 10); got != engine.AreaShell {
		t.Fatalf("in full screen the empty top row is %v", got)
	}
	r.shell(`{"t":"window","text":"fullScreen"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "controls" && m["cornerRadius"] == mockengine.CornerRadius
	})
	if got := r.e.Layout(1).At(400, 10); got != engine.AreaDrag {
		t.Fatalf("back from full screen the empty top row is %v", got)
	}
	r.shell(`{"t":"window","text":"zoom"}`)
	r.shell(`{"t":"window","text":"minimise"}`)
	r.shell(`{"t":"window","text":"close"}`)
	deadline := time.Now().Add(time.Second)
	for !r.e.Closed(1) {
		if time.Now().After(deadline) {
			t.Fatal("window never closed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	want := []engine.WindowRequest{engine.WindowFullScreen, engine.WindowFullScreen, engine.WindowZoom, engine.WindowMinimise, engine.WindowClose}
	if got := r.e.Requests(); !slices.Equal(got, want) {
		t.Fatalf("requests %v", got)
	}
}

func TestTabsThroughTheShellAndKeys(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")

	r.shell(`{"t":"newTab"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	second := m["active"].(float64)
	if second == 1 {
		t.Fatalf("new tab should be active: %v", m)
	}
	// The shell shows the active tab, and its layout hides the one it showed before.
	r.show(1)
	r.show(second)
	r.e.Settle()
	if hidden := r.e.Hidden(1); !slices.Equal(hidden, []engine.TabID{1}) {
		t.Fatalf("the first tab should have been told it is hidden: %v", hidden)
	}
	r.visit("https://b.test/")

	// The page opens a tab: the engine made it, Go put it in the list and showed it.
	r.e.Fire(1, 0, "open", "https://c.test/")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	if m["active"].(float64) == second {
		t.Fatalf("opened tab should be active: %v", m)
	}
	r.waitForTab(func(tab map[string]any) bool { return tab["url"] == "https://c.test/" && tab["loading"] == false })

	r.e.Fire(1, 0, "key", "tab1")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == float64(1) })
	r.e.Fire(1, 0, "key", "nextTab")
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == second })
	r.e.Fire(1, 0, "key", "closeTab")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	if m["active"] == second {
		t.Fatalf("closed tab still active: %v", m)
	}

	// Closing the rest leaves the window open, with no tabs and no active tab.
	r.e.Fire(1, 0, "key", "closeTab")
	r.e.Fire(1, 0, "key", "closeTab")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 0 })
	r.e.Settle()
	if m["active"] != float64(0) || r.e.Closed(1) || r.e.Quit() {
		t.Fatalf("after the last tab: active %v, closed %v, quit %v", m["active"], r.e.Closed(1), r.e.Quit())
	}
	// Close Tab in a window with no tabs closes it, the last, which quits.
	r.e.Fire(1, 0, "key", "closeTab")
	r.e.Settle()
	if !r.e.Closed(1) || !r.e.Quit() {
		t.Fatalf("Close Tab in the empty window: closed %v, quit %v", r.e.Closed(1), r.e.Quit())
	}
}

// The shell's Close Tab names the active tab, which in a window with no tabs is none: it
// closes the window. A tab the shell names that is gone closes nothing.
func TestTheShellsCloseTabInAWindowWithNoTabs(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	only := snap["active"].(float64)
	closeTab := func(id float64) {
		r.e.Post(func() { r.a.ShellMessage(2, fmt.Appendf(nil, `{"t":"closeTab","id":%v}`, id)) })
		r.e.Settle()
	}
	closeTab(only)
	r.waitOn(second, isTabs(0))
	closeTab(only)
	if r.e.Closed(2) {
		t.Fatal("closing a tab that is gone closed its window")
	}
	closeTab(0)
	if !r.e.Closed(2) || !slices.Equal(r.e.Windows(), []engine.WindowID{1}) {
		t.Fatalf("Close Tab in the empty window: closed %v, windows %v", r.e.Closed(2), r.e.Windows())
	}
}

// The address in a window with no tabs opens a tab there and takes it to what was typed:
// in the first workspace, or in the one the shell names. Nothing typed opens nothing.
func TestTheAddressInAWindowWithNoTabsOpensOne(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"version":2,"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"},{"id":"","name":"Main","color":"#3470d0"}]}`), 0o600)
	r := start(t, dir)
	defer r.stop()
	empty := func() {
		r.t.Helper()
		r.e.Fire(1, 0, "key", "closeTab")
		r.waitFor(isTabs(0))
	}
	// opened says what one tab the window has: its id, its workspace in Go and in the
	// engine, and the active tab.
	opened := func(m map[string]any) (any, string, any) {
		tab := m["tabs"].([]any)[0].(map[string]any)
		var in string
		r.e.Post(func() { in = r.e.Workspace(engine.TabID(tab["id"].(float64))) })
		r.e.Settle()
		return tab["id"], tab["workspace"].(string) + "/" + in, m["active"]
	}
	empty()
	r.shell(`{"t":"navigate","text":"  "}`)
	r.e.Settle()
	var n int
	r.e.Post(func() { n = len(r.a.Tabs.AllIn(1)) })
	r.e.Settle()
	if n != 0 {
		t.Fatalf("nothing typed opened %d tabs", n)
	}

	// The tab is the window's, in the first workspace, and has the address from the first
	// the shell hears of it.
	r.shell(`{"t":"navigate","text":"n.test/"}`)
	m := r.waitFor(isTabs(1))
	if id, in, active := opened(m); active != id || in != "0a1b2c3d/0a1b2c3d" || m["tabs"].([]any)[0].(map[string]any)["url"] != "https://n.test/" {
		t.Fatalf("the tab the address opened: %v in %q, active %v; %v", id, in, active, m["tabs"])
	}
	r.waitForTab(func(t map[string]any) bool { return t["url"] == "https://n.test/" && t["title"] == "Mock n.test" })

	empty()
	r.shell(`{"t":"navigate","text":"m.test/","workspace":""}`)
	if id, in, active := opened(r.waitFor(isTabs(1))); active != id || in != "/" {
		t.Fatalf("the tab the address opened in Main: %v in %q, active %v", id, in, active)
	}
	r.waitForTab(func(t map[string]any) bool { return t["url"] == "https://m.test/" && t["title"] == "Mock m.test" })
}

// In a window with no tabs every action that needs one is off, in the menu and for a
// shell that lists them; New Tab, the address, the windows' own and Settings stay on, and
// so does Close Tab, which closes the window.
func TestActionsThatNeedATabAreOffWithNone(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.visit("https://b.test/")
	need := []string{"nav.back", "nav.reload", "nav.reloadHard", "view.zoomIn", "view.zoomOut", "view.zoomReset", "dev.viewSource",
		"tab.duplicate", "tab.moveToNewWindow", "tab.next", "tab.previous", "tab.recentNext", "tab.recentPrevious",
		"tab.select1", "tab.select8", "tab.selectLast", "focus.nextPane", "focus.previousPane"}
	stay := []string{"tab.new", "tab.close", "address.focus", "window.new", "window.close", "app.settings", "view.fullScreen", "view.tabsTop"}
	// What can run now, as a shell that lists the actions hears it, and what the menu bar
	// shows enabled.
	state := func() (runs, menu map[string]bool) {
		runs, menu = map[string]bool{}, map[string]bool{}
		var walk func([]engine.MenuItem)
		walk = func(items []engine.MenuItem) {
			for _, it := range items {
				if it.Action != "" {
					menu[it.Action] = it.Enabled
				}
				walk(it.Items)
			}
		}
		done := make(chan struct{})
		r.e.Post(func() {
			for _, a := range r.a.Commands.Titles() {
				runs[a.ID] = true
			}
			walk(r.e.Menu())
			close(done)
		})
		<-done
		return runs, menu
	}
	runs, menu := state()
	for _, id := range append(slices.Clone(need), stay...) {
		if !runs[id] {
			t.Errorf("with a tab, %s can't run", id)
		}
	}
	r.e.Fire(1, 0, "key", "closeTab")
	r.waitFor(isTabs(0))
	r.e.Settle()
	runs, menu = state()
	for _, id := range need {
		if runs[id] || menu[id] {
			t.Errorf("with no tab, %s can run %v, or is on in the menu %v", id, runs[id], menu[id])
		}
	}
	for _, id := range stay {
		if !runs[id] {
			t.Errorf("with no tab, %s can't run", id)
		}
	}
	if menu["nav.reload"] || !menu["tab.close"] || !menu["tab.new"] {
		t.Errorf("the menu: reload %v, close tab %v, new tab %v", menu["nav.reload"], menu["tab.close"], menu["tab.new"])
	}
}

func TestTabLayoutIsAnnouncedAndSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.shell(`{"t":"setTabLayout","text":"horizontal"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabLayout" && m["tabLayout"] == "horizontal" })
	r.stop()

	r2 := start(t, dir)
	defer r2.stop()
	r2.shell(`{"t":"ready"}`)
	m := r2.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if m["tabLayout"] != "horizontal" {
		t.Fatalf("snapshot after restart: %v", m["tabLayout"])
	}
}

func TestSnapshotCarriesTheWindowControls(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	controls := m["controls"].(map[string]any)
	if controls["x"] != float64(mockengine.Controls.X) || controls["w"] != float64(mockengine.Controls.W) {
		t.Fatalf("controls = %v", controls)
	}
	if m["cornerRadius"] != mockengine.CornerRadius {
		t.Fatalf("cornerRadius = %v", m["cornerRadius"])
	}
	if m["windowColor"] != "#4f9e9e" || r.e.Layout(1).FrameColor != "#4f9e9e" {
		t.Fatalf("frame colour: shell %v, engine %v", m["windowColor"], r.e.Layout(1).FrameColor)
	}

	r.e.Fire(1, 0, "fullscreen", "")
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "controls" })
	if m["controls"].(map[string]any)["w"] != float64(0) || m["cornerRadius"] != float64(0) {
		t.Fatalf("full screen should report no controls and square corners: %v", m)
	}
}

var _ engine.Listener = (*app.App)(nil)

// openSettings opens window 1's settings tab from the shell's gear and reads its page's
// snapshot.
func (r *rig) openSettings() (<-chan []byte, map[string]any) {
	r.t.Helper()
	r.shell(`{"t":"run","text":"app.settings"}`)
	deadline := time.Now().Add(5 * time.Second)
	for r.settingsTab = r.settingsIn(1); r.settingsTab == 0; r.settingsTab = r.settingsIn(1) {
		if time.Now().After(deadline) {
			r.t.Fatal("settings never opened")
		}
		time.Sleep(5 * time.Millisecond)
	}
	got, stop := r.e.SettingsFeed(r.settingsTab).Subscribe()
	r.t.Cleanup(stop)
	r.settings(`{"t":"ready"}`)
	return got, r.waitOn(got, func(m map[string]any) bool { return m["t"] == "snapshot" })
}

// settingsIn is window w's settings tab, or 0 when it has none.
func (r *rig) settingsIn(w engine.WindowID) engine.TabID {
	var tab engine.TabID
	r.e.Post(func() { tab = r.a.Tabs.PageIn(w, "settings") })
	r.e.Settle()
	return tab
}

// settings sends a message from the page of the tab openSettings opened.
func (r *rig) settings(cmd string) {
	r.e.Post(func() { r.a.SettingsMessage(r.settingsTab, []byte(cmd)) })
}

func (r *rig) waitOn(ch <-chan []byte, match func(map[string]any) bool) map[string]any {
	r.t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-ch:
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				r.t.Fatalf("bad message %s: %v", b, err)
			}
			if match(m) {
				return m
			}
		case <-deadline:
			r.t.Fatal("timed out waiting for a settings message")
		}
	}
}

// Settings is a page of its own: the snapshot carries what it shows, it sets the prefs,
// and Go says when they change.
func TestTheSettingsPageReadsAndSetsTheSettings(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, snap := r.openSettings()
	if snap["autoUpdate"] != true || snap["running"] != "" || snap["keyboard"] == nil {
		t.Fatalf("snapshot = %v", snap)
	}
	r.settings(`{"t":"setAutoUpdate","on":false}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "autoUpdate" && m["autoUpdate"] == false })
	b, _ := os.ReadFile(filepath.Join(dir, "updates", "settings.json"))
	if !strings.Contains(string(b), `"autoUpdate":false`) {
		t.Fatalf("the update folder's settings.json = %s", b)
	}
	// The theme: saved, the shell told, the window's own parts dark.
	if snap["theme"] != "system" || r.e.Layout(1).Theme != engine.ThemeSystem {
		t.Fatalf("theme starts as %v, window %v", snap["theme"], r.e.Layout(1).Theme)
	}
	r.settings(`{"t":"setTheme","text":"dark"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "theme" && m["theme"] == "dark" })
	if r.e.Layout(1).Theme != engine.ThemeDark {
		t.Fatalf("window theme = %v", r.e.Layout(1).Theme)
	}
	r.flushPrefs()
	b, _ = os.ReadFile(filepath.Join(dir, "prefs.json"))
	if !strings.Contains(string(b), `"theme":"dark"`) {
		t.Fatalf("prefs.json = %s", b)
	}
}

// The frame colour is a pref: setting it from the settings page repaints the window and
// tells the shell, together, and it survives a restart.
func TestTheFrameColorChangesTheWindowAndTheShellLive(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.openSettings()
	r.settings(`{"t":"setFrameColor","text":"#3470d0"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "windowColor" && m["windowColor"] == "#3470d0" })
	if r.e.Layout(1).FrameColor != "#3470d0" {
		t.Fatalf("window = %v", r.e.Layout(1).FrameColor)
	}
	r.stop()

	r = start(t, dir)
	defer r.stop()
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if m["windowColor"] != "#3470d0" || r.e.Layout(1).FrameColor != "#3470d0" || r.e.ShownColor() != "#3470d0" {
		t.Fatalf("after restart: shell %v, window %v, shown %v", m["windowColor"], r.e.Layout(1).FrameColor, r.e.ShownColor())
	}
}

// A key added in the settings page runs its action, and the keymap now belongs to the
// user. Cmd+W closes the settings tab, as it closes any tab.
func TestAKeyAddedInSettingsRunsItsAction(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, _ := r.openSettings()
	r.settings(`{"t":"addKey","id":"tab.new","text":"Cmd+Shift+Y","reserve":"always"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "keyboard" })
	b, _ := os.ReadFile(filepath.Join(dir, "keymap.json"))
	if !strings.Contains(string(b), `"Shift+Cmd+Y"`) || strings.Contains(string(b), `"follows"`) {
		t.Fatalf("keymap.json = %s", b)
	}

	r.e.Fire(1, 0, "key", "closeTab")
	deadline := time.Now().Add(5 * time.Second)
	for r.settingsIn(1) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("Cmd+W didn't close the settings tab")
		}
		time.Sleep(5 * time.Millisecond)
	}

	r.e.Post(func() {
		r.a.Key(engine.Key{Code: 'Y', Mods: engine.ModCmd | engine.ModShift}, engine.KeyBeforePage, engine.SurfacePage)
	})
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
}

// Settings is a tab of the window: opening it again shows that tab at the section asked
// for, and a key its page was recording stops when the tab closes, by Close Tab or with
// its window.
func TestTheSettingsTabOpensAtASectionAndEndsItsRecording(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, snap := r.openSettings()
	if snap["section"] != "" {
		t.Fatalf("the first section is %v", snap["section"])
	}
	r.e.Post(func() { r.a.Tabs.Create(1) })
	r.e.Post(func() { r.a.Settings.OpenSection(1, "keyboard") })
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "section" })
	var active engine.TabID
	var count int
	r.e.Post(func() { active, count = r.a.Tabs.ActiveID(), len(r.a.Tabs.AllIn(1)) })
	r.e.Settle()
	if m["section"] != "keyboard" || active != r.settingsTab || count != 3 {
		t.Fatalf("heard %v; active %v of %d tabs, settings %v", m, active, count, r.settingsTab)
	}

	ran := func() bool {
		var ok bool
		r.e.Post(func() {
			ok = r.a.Key(engine.Key{Code: 'T', Mods: engine.ModCmd}, engine.KeyBeforePage, engine.SurfacePage)
		})
		r.e.Settle()
		return ok
	}
	r.settings(`{"t":"recording","on":true}`)
	if ran() {
		t.Fatal("a key ran while the page recorded one")
	}
	r.shell(fmt.Sprintf(`{"t":"closeTab","id":%d}`, r.settingsTab))
	if r.e.Settle(); r.settingsIn(1) != 0 || !ran() {
		t.Fatal("closing the settings tab left the keys recording")
	}
}

// A window has one settings tab: with it active, Duplicate Tab is off in the menu and runs
// nothing from a shell, and a move of it into a window that has its own shows that one.
func TestTheSettingsTabIsOneAWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.openSettings()
	enabled := func(action string) bool {
		on := false
		var walk func([]engine.MenuItem)
		walk = func(items []engine.MenuItem) {
			for _, it := range items {
				if it.Action == action {
					on = it.Enabled
				}
				walk(it.Items)
			}
		}
		r.e.Post(func() { walk(r.e.Menu()) })
		r.e.Settle()
		return on
	}
	if enabled("tab.duplicate") {
		t.Fatal("Duplicate Tab is on for the settings tab")
	}
	r.shell(`{"t":"run","text":"tab.duplicate"}`)
	var count int
	var w2, there engine.TabID
	var second engine.WindowID
	r.e.Post(func() {
		count = len(r.a.Tabs.AllIn(1))
		second, _ = r.a.NewWindow("")
		there = r.a.Settings.Open(second)
		r.a.Tabs.Move(r.settingsTab, second, 0)
		w2 = r.a.Tabs.ActiveIn(second)
	})
	r.e.Settle()
	if count != 2 || w2 != there || r.settingsIn(1) != r.settingsTab {
		t.Fatalf("tabs in window 1 %d; window %v shows %v, its settings tab %v; window 1's %v", count, second, w2, there, r.settingsIn(1))
	}
}

// Our pages' one navigation rule, through the engine: a page the settings tab is asked to
// go to opens in a web tab beside it, and the settings tab stays; an address of ours typed
// or asked for in a new tab shows the settings tab, and the web tab stays as it was.
func TestTheNavigationRuleThroughTheEngine(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://example.com/")
	r.openSettings()
	var first engine.TabID
	var all []tabs.Tab
	read := func() {
		r.e.Settle()
		r.e.Post(func() { all, first = r.a.Tabs.AllIn(1), r.a.Tabs.ActiveID() })
		r.e.Settle()
	}

	r.e.Fire(1, r.settingsTab, "navigate", "https://p.test/")
	r.waitForTab(func(t map[string]any) bool { return t["url"] == "https://p.test/" && t["title"] == "Mock p.test" })
	read()
	if len(all) != 3 || all[1].ID != r.settingsTab || all[1].URL != "chrome://settings/" || all[2].URL != "https://p.test/" || first != all[2].ID {
		t.Fatalf("after the settings tab was asked for a web page: %+v, active %v", all, first)
	}

	r.e.Post(func() { r.a.Tabs.Activate(all[0].ID) })
	r.shell(`{"t":"navigate","text":"chrome://settings"}`)
	// The shell asks for the page of the tab it typed in, still its active one when it asked.
	r.shell(fmt.Sprintf(`{"t":"focusTab","id":%d}`, all[0].ID))
	read()
	if len(all) != 3 || first != r.settingsTab || all[0].URL != "https://example.com/" {
		t.Fatalf("after chrome://settings was typed in a web tab: %+v, active %v", all, first)
	}
	if r.e.Focused() == all[0].ID || slices.Contains(r.e.UnshownFocus(), all[0].ID) {
		t.Fatalf("the web tab the shell asked to focus, no longer active, was given the keyboard")
	}
	r.shell(`{"t":"newTab","text":"chrome://settings/"}`)
	read()
	if len(all) != 3 || first != r.settingsTab {
		t.Fatalf("after a new tab for chrome://settings/: %+v, active %v", all, first)
	}
}

// Ctrl+Tab goes back through recent tabs while Ctrl is held, as Cmd+Tab does through apps,
// and letting go settles on the tab it reached.
func TestCtrlTabCyclesRecentTabs(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	active := func(id float64) {
		t.Helper()
		r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && m["active"] == id })
	}
	// Recent: 3, 2, 1.
	r.e.Fire(1, 0, "key", "recentTab")
	active(2)
	r.e.Fire(1, 0, "key", "recentTab")
	active(1)
	r.e.Fire(1, 0, "release", "")
	// Recent: 1, 3, 2.
	r.e.Fire(1, 0, "key", "recentTab")
	active(3)
	r.e.Fire(1, 0, "release", "")
	// Recent: 3, 1, 2. Backward from the newest comes round to the oldest.
	r.e.Fire(1, 0, "key", "recentTabBack")
	active(2)
	r.e.Fire(1, 0, "release", "")
	r.e.Fire(1, 0, "key", "recentTab")
	active(3)
}

// The menu comes from the tree: Edit has Redo, View has Full Screen with its macOS key,
// and the hidden Select Tab submenu isn't there.
func TestTheMenuComesFromTheTree(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var menu engine.Menu
	done := make(chan struct{})
	r.e.Post(func() { menu = r.e.Menu(); close(done) })
	<-done
	find := func(items []engine.MenuItem, title string) *engine.MenuItem {
		for i := range items {
			if items[i].Title == title {
				return &items[i]
			}
		}
		return nil
	}
	edit, view, tab := find(menu, "Edit"), find(menu, "View"), find(menu, "Tab")
	if edit == nil || view == nil || tab == nil {
		t.Fatalf("menu = %+v", menu)
	}
	if find(edit.Items, "Redo") == nil {
		t.Fatal("no Redo")
	}
	full := find(view.Items, "Full Screen")
	if full == nil || full.Key == nil || *full.Key != (engine.Key{Code: 'F', Mods: engine.ModCtrl | engine.ModCmd}) {
		t.Fatalf("Full Screen = %+v", full)
	}
	if find(tab.Items, "Select Tab") != nil {
		t.Fatal("the hidden Select Tab submenu is in the menu")
	}
}

// isTabs matches a tabs message with n tabs.
func isTabs(n int) func(map[string]any) bool {
	return func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == n }
}

// openSecondWindow presses Cmd+N in window 1 and waits for window 2's shell to start.
func (r *rig) openSecondWindow() <-chan []byte {
	r.t.Helper()
	second, stop := r.e.Feeds(2).Shell.Subscribe()
	r.t.Cleanup(stop)
	r.e.Fire(1, 0, "key", "newWindow")
	r.e.Settle()
	if w := r.e.Windows(); !slices.Equal(w, []engine.WindowID{1, 2}) {
		r.t.Fatalf("windows after Cmd+N: %v", w)
	}
	// A new window's shell hears nothing until it says it's ready.
	select {
	case b := <-second:
		r.t.Fatalf("window 2's shell heard %s before it was ready", b)
	default:
	}
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"ready"}`)) })
	return second
}

// Cmd+N opens a second window with one new tab of its own and its own shell, in front.
// Commands act on the window in front, settings opens over the window it was asked in,
// closing a window's last tab leaves it open with none, Close Tab then closes it, and
// closing the last window quits.
func TestASecondWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	tabs := snap["tabs"].([]any)
	if len(tabs) != 1 || tabs[0].(map[string]any)["id"] != snap["active"] || snap["active"] == float64(1) {
		t.Fatalf("window 2 starts with one new tab of its own: %v active %v", tabs, snap["active"])
	}

	// Cmd+T acts on the window in front, window 2.
	r.e.Fire(2, 0, "key", "newTab")
	r.waitOn(second, isTabs(2))
	// Window 1 comes to the front, and Cmd+T acts on it.
	r.e.Fire(1, 0, "focus", "")
	r.e.Fire(1, 0, "key", "newTab")
	r.waitFor(isTabs(2))

	// Settings opens as a tab of the window it was asked in.
	r.e.Fire(2, 0, "focus", "")
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"run","text":"app.settings"}`)) })
	r.e.Settle()
	if r.settingsIn(2) == 0 || r.settingsIn(1) != 0 {
		t.Fatalf("settings tabs: window 1 %v, window 2 %v", r.settingsIn(1), r.settingsIn(2))
	}

	// Closing window 2's tabs leaves it open with none, a tab at a time, the settings tab
	// among them. Close Tab again closes it, and the app goes on with window 1.
	for range 3 {
		r.e.Fire(2, 0, "key", "closeTab")
	}
	m := r.waitOn(second, isTabs(0))
	r.e.Settle()
	if m["active"] != float64(0) || r.e.Closed(2) || !slices.Equal(r.e.Windows(), []engine.WindowID{1, 2}) {
		t.Fatalf("after window 2's last tab: active %v, closed %v, windows %v", m["active"], r.e.Closed(2), r.e.Windows())
	}
	r.e.Fire(2, 0, "key", "closeTab")
	r.e.Settle()
	if !r.e.Closed(2) || !slices.Equal(r.e.Windows(), []engine.WindowID{1}) || r.e.Quit() {
		t.Fatalf("after Close Tab in empty window 2: closed %v, windows %v, quit %v", r.e.Closed(2), r.e.Windows(), r.e.Quit())
	}

	// Window 1's own close button closes it, the last window, which quits.
	r.e.Fire(1, 0, "close", "")
	r.e.Settle()
	if !r.e.Quit() {
		t.Fatal("closing the last window didn't quit")
	}
}

// A second start's URLs open as new tabs in the window used last, which comes to the
// front.
func TestAHandoverOpensInTheWindowUsedLast(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	// Tab 1 done loading, so nothing else of it is sent once it moves.
	r.e.Post(func() { r.a.LoadingChanged(1, false) })
	r.e.Settle()
	second := r.openSecondWindow()
	r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.e.Fire(1, 0, "focus", "")
	r.e.Post(func() { r.a.Handover([]string{"https://h.test/"}) })
	m := r.waitFor(isTabs(2))
	if url := m["tabs"].([]any)[1].(map[string]any)["url"]; url != "https://h.test/" {
		t.Fatalf("window 1's new tab is at %v", url)
	}
	r.waitForTab(func(tab map[string]any) bool { return tab["title"] == "Mock h.test" && tab["loading"] == false })
	var inSecond int
	done := make(chan struct{})
	r.e.Post(func() { inSecond = len(r.a.Tabs.AllIn(2)); close(done) })
	<-done
	if req := r.e.Requests(); inSecond != 1 || req[len(req)-1] != engine.WindowFront {
		t.Fatalf("window 2 has %d tabs; requests %v", inSecond, req)
	}
}

// A crashed shell waits to say it's ready again, and only its own window's does: the other
// window's shell goes on hearing.
func TestAShellCrashResetsOnlyItsWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	// Tab 1 done loading, so nothing else of it is sent once it moves.
	r.e.Post(func() { r.a.LoadingChanged(1, false) })
	r.e.Settle()
	second := r.openSecondWindow()
	r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.e.Post(func() { r.a.ShellCrashed(2) })
	r.e.Fire(2, 0, "key", "newTab")
	r.e.Settle()
	select {
	case b := <-second:
		t.Fatalf("window 2's crashed shell heard %s", b)
	default:
	}
	// Window 1's shell still hears what every shell hears.
	r.e.Post(func() { r.a.Prefs.SetTheme("dark") })
	r.waitFor(func(m map[string]any) bool { return m["t"] == "theme" })
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"ready"}`)) })
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	if len(snap["tabs"].([]any)) != 2 {
		t.Fatalf("the reloaded shell's snapshot: %v", snap["tabs"])
	}
}

// moveTab asks Go to move a tab, as ProjectB.moveTab does over DevTools.
func (r *rig) moveTab(tab engine.TabID, window engine.WindowID, index int) {
	r.t.Helper()
	r.e.Post(func() {
		if _, err := r.a.DevToolsCommand("ProjectB.moveTab", []byte(fmt.Sprintf(`{"tab":%d,"window":%d,"index":%d}`, tab, window, index))); err != nil {
			r.t.Errorf("moveTab: %v", err)
		}
	})
	r.e.Settle()
}

// A tab moves to another window, where it is the active tab, and both shells hear their
// new lists; a window whose last tab moves away closes. A tab moved into a new window of
// its own is that window's only tab, and a window's only tab doesn't move.
func TestMovingATabToAnotherWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	second := r.openSecondWindow()
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	inSecond := snap["active"].(float64)
	r.e.Fire(1, 0, "focus", "")
	r.e.Fire(1, 0, "key", "newTab")
	r.waitFor(isTabs(2))

	r.moveTab(1, 2, 0)
	m := r.waitOn(second, isTabs(2))
	if m["active"] != float64(1) || m["tabs"].([]any)[0].(map[string]any)["id"] != float64(1) {
		t.Fatalf("window 2 after the move: %v", m)
	}
	r.waitFor(isTabs(1))

	// Into a new window of its own: window 3 starts with the tab, and no tab of its own.
	third, stop := r.e.Feeds(3).Shell.Subscribe()
	t.Cleanup(stop)
	var made engine.WindowID
	r.e.Post(func() { made = r.a.NewWindowWith(engine.TabID(inSecond)) })
	r.e.Settle()
	if made != 3 {
		t.Fatalf("the new window holding the tab is %v", made)
	}
	r.e.Post(func() { r.a.ShellMessage(3, []byte(`{"t":"ready"}`)) })
	snap = r.waitOn(third, func(m map[string]any) bool { return m["t"] == "snapshot" })
	if tabs := snap["tabs"].([]any); len(tabs) != 1 || snap["active"] != inSecond {
		t.Fatalf("window 3: %v active %v", tabs, snap["active"])
	}
	// Its only tab has a window already, which is the one Go answers with.
	var stays engine.WindowID
	r.e.Post(func() { stays = r.a.NewWindowWith(engine.TabID(inSecond)) })
	r.e.Settle()
	if w := r.e.Windows(); stays != 3 || !slices.Equal(w, []engine.WindowID{1, 2, 3}) {
		t.Fatalf("moving a window's only tab out answered %v; windows %v", stays, w)
	}

	// Window 3's last tab moves to window 1, and window 3 closes.
	r.moveTab(engine.TabID(inSecond), 1, 0)
	if !r.e.Closed(3) || !slices.Equal(r.e.Windows(), []engine.WindowID{1, 2}) {
		t.Fatalf("closed %v, windows %v", r.e.Closed(3), r.e.Windows())
	}

	// A new window answers with its id and its first tab's.
	var w engine.WindowID
	var tab engine.TabID
	var in []tabs.Tab
	r.e.Post(func() { w, tab = r.a.NewWindow(""); in = r.a.Tabs.AllIn(w) })
	r.e.Settle()
	if w != 4 || len(in) != 1 || in[0].ID != tab {
		t.Fatalf("NewWindow answered %v %v; its tabs %+v", w, tab, in)
	}
}

// Tab, Move Tab to New Window moves the active tab of the window in front into a window
// of its own.
func TestMoveTabToNewWindowFromTheMenu(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.waitFor(isTabs(2))
	second, stop := r.e.Feeds(2).Shell.Subscribe()
	t.Cleanup(stop)
	r.e.Post(func() { r.a.MenuChosen("tab.moveToNewWindow") })
	r.e.Settle()
	r.e.Post(func() { r.a.ShellMessage(2, []byte(`{"t":"ready"}`)) })
	snap := r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	if tabs := snap["tabs"].([]any); len(tabs) != 1 || snap["active"] != float64(2) {
		t.Fatalf("window 2: %v active %v", tabs, snap["active"])
	}
	r.waitFor(isTabs(1))
}

// The window manager is the one place another manager hears of windows opening, closing,
// coming to the front and tabs moving between them, and reads each window's bounds and
// state; bounds and state change as asked.
func TestWindowsAsAnotherManagerSeesThem(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var events []window.Event
	r.e.Post(func() { r.a.Window.OnEvent(func(e window.Event) { events = append(events, e) }) })
	r.e.Settle()
	r.openSecondWindow()
	r.e.Fire(1, 0, "focus", "")
	r.e.Settle()
	r.moveTab(1, 2, 0)

	var list []window.Window
	r.e.Post(func() { r.a.Window.SetBounds(2, engine.Rect{X: 200, Y: 100, W: 900, H: 600}) })
	r.e.Settle()
	// A state is set outright, and setting the one it is in asks the engine nothing.
	r.e.Post(func() {
		r.a.Window.SetState(2, window.StateZoomed)
	})
	r.e.Settle()
	r.e.Post(func() {
		r.a.Window.SetState(2, window.StateZoomed)
		r.a.Window.SetState(2, window.StateMinimised)
	})
	r.e.Settle()
	r.e.Post(func() { list = r.a.Window.List() })
	r.e.Settle()
	want := []window.Window{
		{ID: 2, Bounds: engine.Rect{X: 200, Y: 100, W: 900, H: 600}, State: window.StateMinimised, Front: true},
	}
	// Window 1 closed when its only tab moved away, so window 2 is in front.
	if !reflect.DeepEqual(list, want) || !slices.Equal(r.e.Modes(), []engine.WindowMode{engine.ModeZoomed, engine.ModeMinimised}) {
		t.Fatalf("windows %+v, modes asked %v", list, r.e.Modes())
	}
	wantEvents := []window.Event{
		{Kind: window.Opened, Window: 2}, {Kind: window.FrontChanged, Window: 2},
		{Kind: window.BoundsChanged, Window: 2},
		{Kind: window.FrontChanged, Window: 1}, {Kind: window.TabMoved, Window: 2, Tab: 1, From: 1, FromIndex: 0},
		{Kind: window.Closed, Window: 1}, {Kind: window.FrontChanged, Window: 2},
		{Kind: window.BoundsChanged, Window: 2},
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events %+v", events)
	}
}

// A DevTools client's new target with newWindow opens in a window of its own, and asking
// to show a tab brings its window to the front.
func TestADevToolsTargetInANewWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var tab engine.TabID
	r.e.Post(func() {
		tab = r.e.CreateTab("")
		r.a.TabOpened(0, tab, "https://n.test/", "", true, true)
	})
	r.e.Settle()
	var inSecond []tabs.Tab
	var front engine.WindowID
	r.e.Post(func() { inSecond, front = r.a.Tabs.AllIn(2), r.a.Tabs.Front() })
	r.e.Settle()
	if len(inSecond) != 1 || inSecond[0].ID != tab || front != 2 {
		t.Fatalf("window 2 %+v, in front %v", inSecond, front)
	}
	before := len(r.e.Requests())
	r.e.Post(func() { r.a.ActivateRequested(1) })
	r.e.Settle()
	if req := r.e.Requests()[before:]; !slices.Equal(req, []engine.WindowRequest{engine.WindowFront}) {
		t.Fatalf("requests %v", req)
	}
}

// A window made for a tab opened in a window of its own, as a DevTools client's new target
// with newWindow is, closes with its last tab: here its page closes itself, as
// window.close() does. A window the user opened stays.
func TestAWindowMadeForATabClosesWithIt(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var tab engine.TabID
	r.e.Post(func() {
		tab = r.e.CreateTab("")
		r.a.TabOpened(0, tab, "https://n.test/", "", true, true)
	})
	r.e.Settle()
	r.e.Post(func() { r.a.CloseRequested(tab) })
	r.e.Settle()
	if !r.e.Closed(2) || !slices.Equal(r.e.Windows(), []engine.WindowID{1}) || r.e.Quit() {
		t.Fatalf("after its tab closed: closed %v, windows %v, quit %v", r.e.Closed(2), r.e.Windows(), r.e.Quit())
	}
	r.e.Post(func() { r.a.CloseRequested(1) })
	r.e.Settle()
	if r.e.Closed(1) || r.e.Quit() {
		t.Fatalf("window 1 after its tab closed: closed %v, quit %v", r.e.Closed(1), r.e.Quit())
	}
}

// The app is ready once the shell shows its first snapshot: app.ready, the line tests wait
// for, is marked then and only once, not again for a shell that comes back from a crash.
func TestAppReadyOnceTheShellShowsItsState(t *testing.T) {
	_, from := timeline.Since(^uint64(0) >> 1)
	readies := func() int {
		entries, _ := timeline.Since(from)
		n := 0
		for _, e := range entries {
			if e.Kind == timeline.KindMark && e.Name == "app.ready" {
				n++
			}
		}
		return n
	}
	r := start(t, t.TempDir())
	defer r.stop()
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	sync := func() {
		done := make(chan struct{})
		r.e.Post(func() { close(done) })
		<-done
	}
	sync()
	if n := readies(); n != 0 {
		t.Fatalf("app.ready marked %d times before the shell showed anything", n)
	}
	r.shell(`{"t":"shown"}`)
	sync()
	if n := readies(); n != 1 {
		t.Fatalf("app.ready marked %d times once the shell showed its state", n)
	}
	r.e.Post(func() { r.a.ShellCrashed(1) })
	r.shell(`{"t":"ready"}`)
	r.shell(`{"t":"shown"}`)
	sync()
	if n := readies(); n != 1 {
		t.Fatalf("app.ready marked %d times after the shell came back", n)
	}
}

// The Profiles menu lists every profile as the list has it when the menu opens, with a
// tick on this one. The profile's colour is its frame colour: on the Dock icon and in
// the list.
func TestTheProfilesMenuAndTheProfilesColour(t *testing.T) {
	root := t.TempDir()
	list := profiles.New(root)
	c, err := list.Choose("", false)
	if err != nil {
		t.Fatal(err)
	}
	e := mockengine.New()
	var a *app.App
	ready := make(chan struct{})
	e.Post(func() {
		a = app.Start(e, app.Config{ProfileDir: c.Dir, Profiles: list, Profile: c.ID})
		e.SetListener(a)
		close(ready)
	})
	go e.Run()
	<-ready
	e.Settle()
	if got := e.DockColor(); got != "#4f9e9e" {
		t.Fatalf("the Dock shows %q", got)
	}
	var items engine.Menu
	e.Post(func() { items = a.MenuList("profiles") })
	e.Settle()
	if len(items) != 1 || items[0].Title != "Personal" || !items[0].Checked {
		t.Fatalf("the Profiles menu holds %+v", items)
	}
	e.Post(func() { a.Prefs.SetFrameColor("#3470d0") })
	e.Settle()
	if p, _, _ := list.Get(c.ID); e.DockColor() != "#3470d0" || p.Color != "#3470d0" {
		t.Fatalf("the Dock shows %q and the list %q", e.DockColor(), p.Color)
	}
}

// Tab, New Tab in Workspace lists the profile's workspaces, read as the menu opens, and
// choosing one opens a new tab in it in the window in front: Go and the engine have the tab
// there, a tab its page opens is there too, and its visits go into the profile's one
// history, as every tab's do.
func TestNewTabInWorkspaceFromTheMenu(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("workspaces") })
	r.e.Settle()
	if len(items) != 1 || items[0].Title != "Main" || items[0].Action != "workspaces:" || !items[0].Enabled {
		t.Fatalf("a new profile's list holds %+v", items)
	}
	var work workspaces.Workspace
	r.e.Post(func() {
		work, _ = r.a.Workspaces.Create("Work", "#5b7fd6")
		items = r.a.MenuList("workspaces")
	})
	r.e.Settle()
	if len(items) != 2 || items[1].Title != "Work" || items[1].Action != "workspaces:"+work.ID {
		t.Fatalf("the list holds %+v", items)
	}
	r.e.Post(func() { r.a.MenuChosen(items[1].Action) })
	m := r.waitFor(isTabs(2))
	tab := m["tabs"].([]any)[1].(map[string]any)
	id := engine.TabID(tab["id"].(float64))
	var in string
	r.e.Post(func() { in = r.e.Workspace(id) })
	r.e.Settle()
	if m["active"] != tab["id"] || tab["workspace"] != work.ID || in != work.ID {
		t.Fatalf("the new tab %v, active %v, is in %q in the engine", tab, m["active"], in)
	}
	r.show(tab["id"].(float64))
	r.visit("https://work.test/")
	r.e.Fire(1, 0, "open", "https://work.test/more")
	m = r.waitFor(isTabs(3))
	if opened := m["tabs"].([]any)[2].(map[string]any); opened["workspace"] != work.ID {
		t.Fatalf("the tab its page opened is %v", opened)
	}
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == "https://work.test/more" && strings.HasPrefix(t["title"].(string), "Mock")
	})
	if items := r.suggestions("work"); len(items) != 2 {
		t.Fatal("a visit in a workspace isn't in the history")
	}
}

// New Tab, by Cmd+T or the shell's newTab, opens in the workspace of the window's active
// tab. The shell's newTab can name a workspace instead, Main by "".
func TestANewTabOpensInTheActiveTabsWorkspace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work workspaces.Workspace
	r.e.Post(func() { work, _ = r.a.Workspaces.Create("Work", "#5b7fd6") })
	r.e.Settle()
	// opens runs a step and checks it opened one tab, active, in want, in Go and in the
	// engine.
	n := 1
	opens := func(step string, do func(), want string) {
		t.Helper()
		do()
		r.e.Settle()
		var tab tabs.Tab
		var count int
		var in string
		r.e.Post(func() {
			tab, count = r.a.Tabs.Active(), len(r.a.Tabs.All())
			in = r.e.Workspace(tab.ID)
		})
		r.e.Settle()
		n++
		if count != n || tab.Workspace != want || in != want {
			t.Fatalf("%s: %d tabs, the active one in %q, in %q in the engine; want %q", step, count, tab.Workspace, in, want)
		}
	}
	cmdT := func() { r.e.Fire(1, 0, "key", "newTab") }
	shellNewTab := func() { r.shell(`{"t":"newTab"}`) }
	opens("the shell names a workspace", func() { r.shell(`{"t":"newTab","workspace":"` + work.ID + `"}`) }, work.ID)
	opens("Cmd+T from a tab in it", cmdT, work.ID)
	opens("the shell's newTab from a tab in it", shellNewTab, work.ID)
	opens("the shell names Main", func() { r.shell(`{"t":"newTab","workspace":""}`) }, "")
	opens("Cmd+T from a tab in Main", cmdT, "")
	opens("the shell's newTab from a tab in Main", shellNewTab, "")
}

// A tab nothing places opens in the first workspace in the user's order: the first window's
// at start, a new window's and a link's from another app. The engine hears which is first at
// start and whenever another comes to the top, for the tabs it makes on its own.
func TestATabNothingPlacesOpensInTheFirstWorkspace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"version":2,"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"},{"id":"","name":"Main","color":"#3470d0"}]}`), 0o600)
	r := start(t, dir)
	defer r.stop()
	// in says where a tab is, in Go and in the engine.
	in := func(id engine.TabID) string {
		tab, _ := r.a.Tabs.Get(id)
		return tab.Workspace + "/" + r.e.Workspace(id)
	}
	var atStart, inWindow, handed, firstThen, firstNow, afterMove string
	r.e.Post(func() {
		atStart, firstThen = in(r.a.Tabs.AllIn(1)[0].ID), r.e.FirstWorkspace()
		r.a.Handover([]string{"https://elsewhere.test/"})
		handed = in(r.a.Tabs.ActiveID())
	})
	// The page loads, and its visit is saved, before anything else.
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == "https://elsewhere.test/" && t["loading"] == false && strings.HasPrefix(t["title"].(string), "Mock")
	})
	r.e.Post(func() {
		_, tab := r.a.NewWindow("")
		inWindow = in(tab)
		r.a.Workspaces.Move("", -1)
		firstNow = r.e.FirstWorkspace()
		_, tab = r.a.NewWindow("")
		afterMove = in(tab)
	})
	r.e.Settle()
	work := "0a1b2c3d/0a1b2c3d"
	if atStart != work || inWindow != work || handed != work || firstThen != "0a1b2c3d" {
		t.Fatalf("with Work first: the first tab is in %q, a new window's in %q, a handed link's in %q; the engine has %q first", atStart, inWindow, handed, firstThen)
	}
	if firstNow != "" || afterMove != "/" {
		t.Fatalf("with Main moved to the top the engine has %q first, and a new window's tab is in %q", firstNow, afterMove)
	}
}

// View Source opens in the workspace of the tab whose source it shows, in Go and in the
// engine: the source is the page fetched again, with that workspace's cookies.
func TestViewSourceOpensInItsTabsWorkspace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work workspaces.Workspace
	r.e.Post(func() {
		work, _ = r.a.Workspaces.Create("Work", "#5b7fd6")
		r.a.Tabs.CreateIn(1, work.ID)
	})
	m := r.waitFor(isTabs(2))
	r.show(m["tabs"].([]any)[1].(map[string]any)["id"].(float64))
	r.visit("https://work.test/")
	r.e.Post(func() { r.a.MenuChosen("dev.viewSource") })
	m = r.waitFor(isTabs(3))
	source := m["tabs"].([]any)[2].(map[string]any)
	r.waitForTab(func(t map[string]any) bool {
		return t["url"] == "view-source:https://work.test/" && strings.HasPrefix(t["title"].(string), "Mock")
	})
	var in string
	r.e.Post(func() { in = r.e.Workspace(engine.TabID(source["id"].(float64))) })
	r.e.Settle()
	if source["workspace"] != work.ID || in != work.ID || m["active"] != source["id"] {
		t.Fatalf("the source's tab %v, active %v, is in %q in the engine", source, m["active"], in)
	}
}

// Every shell has the workspaces from its snapshot on, and hears of one made or renamed at
// once, so the dot on a tab in one is in its colour.
func TestTheShellHearsTheWorkspaces(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if l, ok := snap["workspaces"].([]any); !ok || len(l) != 1 || l[0].(map[string]any)["name"] != "Main" || l[0].(map[string]any)["id"] != "" {
		t.Fatalf("the snapshot's workspaces: %v", snap["workspaces"])
	}
	if tab := snap["tabs"].([]any)[0].(map[string]any); tab["workspace"] != "" {
		t.Fatalf("the first tab is %v", tab)
	}
	r.e.Post(func() { r.a.Workspaces.Create("Work", "#5b7fd6") })
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" })
	w := m["workspaces"].([]any)[1].(map[string]any)
	if w["name"] != "Work" || w["color"] != "#5b7fd6" || w["id"] == "" {
		t.Fatalf("the shell heard %v", m)
	}
	r.e.Post(func() { r.a.Workspaces.SetName(w["id"].(string), "Office") })
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" })
	if m["workspaces"].([]any)[1].(map[string]any)["name"] != "Office" {
		t.Fatalf("after the rename the shell heard %v", m)
	}
}

// The settings page's Workspaces section lists the workspaces, makes one with a name and a
// colour and renames one: Go keeps them in the profile's list, and the page, the shells and
// the Tab menu's list have them at once.
func TestTheSettingsPageMakesAndRenamesWorkspaces(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, snap := r.openSettings()
	if l, ok := snap["workspaces"].([]any); !ok || len(l) != 1 || l[0].(map[string]any)["name"] != "Main" {
		t.Fatalf("the snapshot's workspaces: %v", snap["workspaces"])
	}
	r.settings(`{"t":"createWorkspace","text":"Work","color":"#c2410c"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaces" })
	w := m["workspaces"].([]any)[1].(map[string]any)
	if w["name"] != "Work" || w["color"] != "#c2410c" {
		t.Fatalf("the page heard %v", m)
	}
	r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" })
	r.settings(`{"t":"renameWorkspace","id":"` + w["id"].(string) + `","text":"Office"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaces" })
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("workspaces") })
	r.e.Settle()
	if len(items) != 2 || items[1].Title != "Office" {
		t.Fatalf("the Tab menu's list holds %+v", items)
	}
	if l := workspaces.New(dir).List(); len(l) != 2 || l[1].ID != w["id"] || l[1].Name != "Office" {
		t.Fatalf("the profile's list holds %+v", l)
	}
}

// The section's dialog edits a workspace's name and colour in one message, a name another
// has is refused whole, and Move up and Move down reorder the list for the Tab menu too.
// The page has the number of tabs open in each workspace, and hears it again when a tab
// opens or closes in one.
func TestTheSettingsPageEditsMovesAndCountsWorkspaces(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	var work, shop string
	r.e.Post(func() {
		w, _ := r.a.Workspaces.Create("Work", "#5b7fd6")
		s, _ := r.a.Workspaces.Create("Shopping", "#c2410c")
		work, shop = w.ID, s.ID
	})
	r.e.Settle()
	got, snap := r.openSettings()
	if c, ok := snap["workspaceTabs"].(map[string]any); !ok || len(c) != 1 || c[""] != float64(1) {
		t.Fatalf("the snapshot's counts: %v", snap["workspaceTabs"])
	}
	r.settings(`{"t":"editWorkspace","id":"` + work + `","text":"shopping","color":"#2f9e44"}`)
	r.settings(`{"t":"editWorkspace","id":"` + work + `","text":"Office","color":"#2f9e44"}`)
	r.settings(`{"t":"moveWorkspace","id":"` + shop + `","by":-1}`)
	r.waitOn(got, func(m map[string]any) bool {
		return m["t"] == "workspaces" && m["workspaces"].([]any)[1].(map[string]any)["id"] == shop
	})
	l := workspaces.New(dir).List()
	if len(l) != 3 || l[0].ID != "" || l[1].ID != shop || l[2].ID != work || l[2].Name != "Office" || l[2].Color != "#2f9e44" {
		t.Fatalf("the profile's list holds %+v", l)
	}
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("workspaces") })
	r.e.Settle()
	if len(items) != 3 || items[0].Title != "Main" || items[1].Title != "Shopping" || items[2].Title != "Office" {
		t.Fatalf("the Tab menu's list holds %+v", items)
	}
	var tab engine.TabID
	r.e.Post(func() { tab = r.a.Tabs.CreateIn(r.a.Tabs.Front(), work) })
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaceTabs" })
	if c := m["workspaceTabs"].(map[string]any); len(c) != 2 || c[work] != float64(1) || c[""] != float64(1) {
		t.Fatalf("after a tab opened in Office the page heard %v", m)
	}
	r.e.Post(func() { r.a.Tabs.Close(tab) })
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaceTabs" })
	if c := m["workspaceTabs"].(map[string]any); len(c) != 1 || c[work] != nil {
		t.Fatalf("after it closed the page heard %v", m)
	}
}

// New tab in workspace, from a workspace's row menu, opens a tab in it at the end of the
// page's own window and shows it; the settings tab stays. An id the list doesn't hold opens
// nothing.
func TestTheSettingsPageOpensATabInAWorkspace(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	var work string
	r.e.Post(func() {
		w, _ := r.a.Workspaces.Create("Work", "#5b7fd6")
		work = w.ID
	})
	r.e.Settle()
	r.openSettings()
	var before int
	r.e.Post(func() { before = len(r.a.Tabs.All()) })
	r.e.Settle()
	r.settings(`{"t":"openInWorkspace","id":"nothere"}`)
	r.settings(`{"t":"openInWorkspace","id":"` + work + `"}`)
	r.e.Settle()
	var in []string
	var active engine.TabID
	r.e.Post(func() {
		for _, tab := range r.a.Tabs.All()[before:] {
			in = append(in, tab.Workspace)
		}
		active = r.a.Tabs.ActiveID()
	})
	r.e.Settle()
	if len(in) != 1 || in[0] != work {
		t.Fatalf("new tabs are in %q, want one in %s", in, work)
	}
	if r.settingsIn(1) != r.settingsTab || active == r.settingsTab {
		t.Fatalf("the settings tab %v, now %v; active %v", r.settingsTab, r.settingsIn(1), active)
	}
}

// Deleting a workspace from the settings page takes it out of the list everyone has, the Tab
// menu's too, and tells the engine the new first; closes its tabs in every window, a window
// that held only those staying open with none; and only then has the engine erase its
// storage. The last one can't be deleted, nor can one the list doesn't hold.
func TestTheSettingsPageDeletesAWorkspace(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	var work string
	var w2 engine.WindowID
	r.e.Post(func() {
		w, _ := r.a.Workspaces.Create("Work", "#5b7fd6")
		work = w.ID
		r.a.Workspaces.Move(work, -1)
		r.a.Tabs.CreateIn(1, work)
		w2, _ = r.a.NewWindow("")
		r.a.WindowFocused(1)
	})
	r.e.Settle()
	got, _ := r.openSettings()
	var first string
	r.e.Post(func() { first = r.e.FirstWorkspace() })
	r.e.Settle()
	if first != work {
		t.Fatalf("with Work first the engine has %q", first)
	}
	r.settings(`{"t":"deleteWorkspace","id":"nothere"}`)
	r.settings(`{"t":"deleteWorkspace","id":"` + work + `"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "workspaces" })
	if l := m["workspaces"].([]any); len(l) != 1 || l[0].(map[string]any)["id"] != "" {
		t.Fatalf("the page heard %v", m)
	}
	r.waitOn(got, func(m map[string]any) bool {
		c, ok := m["workspaceTabs"].(map[string]any)
		return m["t"] == "workspaceTabs" && ok && len(c) == 1 && c[""] == float64(1)
	})
	r.e.Settle()
	var left, inW2 []tabs.Tab
	var erased []mockengine.Erase
	var items engine.Menu
	r.e.Post(func() {
		left, inW2, erased, first, items = r.a.Tabs.AllIn(1), r.a.Tabs.AllIn(w2), r.e.Erased(), r.e.FirstWorkspace(), r.a.MenuList("workspaces")
	})
	r.e.Settle()
	// Window 1 keeps its tab in Main, and the settings tab, which is in no workspace.
	if len(left) != 2 || left[0].Workspace != "" || left[1].ID != r.settingsTab || len(inW2) != 0 || r.e.Closed(w2) || r.e.Closed(1) {
		t.Fatalf("window 1 has %+v and window 2 %+v; window 2 closed %v, window 1 %v", left, inW2, r.e.Closed(w2), r.e.Closed(1))
	}
	if !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: work}}) || first != "" || len(items) != 1 || items[0].Title != "Main" {
		t.Fatalf("the engine erased %+v and has %q first; the Tab menu lists %+v", erased, first, items)
	}
	if l := workspaces.New(dir).List(); len(l) != 1 || l[0].ID != "" {
		t.Fatalf("the profile's list holds %+v", l)
	}
	r.settings(`{"t":"deleteWorkspace","id":""}`)
	r.e.Settle()
	r.e.Post(func() { left, erased = r.a.Tabs.All(), r.e.Erased() })
	r.e.Settle()
	if len(left) != 2 || len(erased) != 1 || len(workspaces.New(dir).List()) != 1 {
		t.Fatalf("deleting the last left %+v and erased %+v", left, erased)
	}
}

// A window whose every tab is in the workspace deleted stays open with no tabs, and the
// engine is asked to erase the workspace.
func TestDeletingAWorkspaceLeavesAWindowOfOnlyItsTabsEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"version":2,"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"},{"id":"","name":"Main","color":"#3470d0"}]}`), 0o600)
	r := start(t, dir)
	defer r.stop()
	r.e.Post(func() { r.a.Tabs.Create(1) })
	r.shell(`{"t":"deleteWorkspace","workspace":"0a1b2c3d"}`)
	r.waitFor(isTabs(0))
	r.e.Settle()
	var left []tabs.Tab
	var erased []mockengine.Erase
	r.e.Post(func() { left, erased = r.a.Tabs.AllIn(1), r.e.Erased() })
	r.e.Settle()
	if r.e.Closed(1) || r.e.Quit() || len(left) != 0 {
		t.Fatalf("window 1 closed %v, the app quit %v; the window has %+v", r.e.Closed(1), r.e.Quit(), left)
	}
	if !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: "0a1b2c3d"}}) {
		t.Fatalf("the engine erased %+v", erased)
	}
}

// One of our tabs is in no workspace: the counts leave it out, history records nothing of
// it, and deleting the workspace it was opened from leaves it, a new tab from it then
// opening in the first.
func TestOurTabIsInNoWorkspace(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "workspaces.json"), []byte(`{"version":2,"workspaces":[{"id":"0a1b2c3d","name":"Work","color":"#5b7fd6"},{"id":"","name":"Main","color":"#3470d0"}]}`), 0o600)
	r := start(t, dir)
	defer r.stop()
	// The first tab is in Work, the first workspace; then one in Main, and ours opened from it.
	var main, ours engine.TabID
	r.e.Post(func() {
		main = r.a.Tabs.CreateIn(1, "")
		ours = r.a.Tabs.OpenPage(1, "settings")
	})
	r.waitForTab(func(t map[string]any) bool {
		return t["id"] == float64(ours) && t["page"] == "settings" && t["url"] == "chrome://settings/" && t["loading"] == false
	})
	if items := r.suggestions("settings"); len(items) != 0 {
		t.Fatalf("history has %v", items)
	}
	_, snap := r.openSettings()
	if c := snap["workspaceTabs"].(map[string]any); len(c) != 2 || c[""] != float64(1) || c["0a1b2c3d"] != float64(1) {
		t.Fatalf("the counts are %v", c)
	}
	r.settings(`{"t":"deleteWorkspace","id":""}`)
	r.e.Settle()
	var left []tabs.Tab
	var erased []mockengine.Erase
	var fromOurs tabs.Tab
	r.e.Post(func() {
		left, erased = r.a.Tabs.AllIn(1), r.e.Erased()
		r.a.Tabs.Activate(ours)
		fromOurs, _ = r.a.Tabs.Get(r.a.Tabs.Create(1))
	})
	r.e.Settle()
	if len(left) != 2 || left[1].ID != ours || slices.ContainsFunc(left, func(t tabs.Tab) bool { return t.ID == main }) {
		t.Fatalf("after deleting Main window 1 has %+v", left)
	}
	if !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: ""}}) || fromOurs.Workspace != "0a1b2c3d" {
		t.Fatalf("the engine erased %+v; a new tab from ours is in %q", erased, fromOurs.Workspace)
	}
}

// A shell changes the workspaces with the settings page's messages, naming a workspace by
// "workspace", under the same rules: it makes, edits and moves one, and every shell hears
// the list; its delete closes the workspace's tabs and only then has the engine erase it;
// and the last one isn't deleted.
func TestAShellChangesTheWorkspaces(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	list := func(m map[string]any) []any { l, _ := m["workspaces"].([]any); return l }
	r.shell(`{"t":"createWorkspace","text":"Work","color":"#5b7fd6"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" && len(list(m)) == 2 })
	work := list(m)[1].(map[string]any)["id"].(string)
	r.shell(`{"t":"editWorkspace","workspace":"` + work + `","text":"Office","color":"#c2410c"}`)
	r.shell(`{"t":"moveWorkspace","workspace":"` + work + `","by":-1}`)
	r.waitFor(func(m map[string]any) bool {
		if m["t"] != "workspaces" {
			return false
		}
		w := list(m)[0].(map[string]any)
		return w["id"] == work && w["name"] == "Office" && w["color"] == "#c2410c"
	})
	var tab engine.TabID
	r.e.Post(func() { tab = r.a.Tabs.CreateIn(1, work) })
	r.shell(`{"t":"deleteWorkspace","workspace":"` + work + `"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" && len(list(m)) == 1 })
	r.shell(`{"t":"deleteWorkspace","workspace":""}`)
	r.e.Settle()
	var left []tabs.Tab
	var erased []mockengine.Erase
	var open bool
	r.e.Post(func() { left, erased = r.a.Tabs.All(), r.e.Erased(); _, open = r.a.Tabs.Get(tab) })
	r.e.Settle()
	if open || len(left) != 1 || left[0].Workspace != "" || !reflect.DeepEqual(erased, []mockengine.Erase{{Workspace: work}}) {
		t.Fatalf("left %+v, the engine erased %+v", left, erased)
	}
	if l := workspaces.New(dir).List(); len(l) != 1 || l[0].ID != "" {
		t.Fatalf("the profile's list holds %+v", l)
	}
}

// View, Shell lists the shells the user can choose, with a tick on the one in use. Choosing
// another makes it the profile's and loads it in the window, which hears nothing until it
// says it's ready; what the old shell claimed of the window goes, and the tab stays shown.
func TestViewShellSwitchesTheShellLive(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	r.shell(`{"t":"layout","shell":[{"x":0,"y":0,"w":800,"h":48}],"tabs":[{"tab":1,"rect":{"x":0,"y":48,"w":800,"h":552}}],"viewW":800,"viewH":600}`)
	var items engine.Menu
	r.e.Post(func() { items = r.a.MenuList("shells") })
	r.e.Settle()
	listed := shells.Listed()
	if len(items) != len(listed) {
		t.Fatalf("View, Shell lists %+v", items)
	}
	for i, s := range listed {
		if items[i].Title != s.Name || items[i].Action != "shells:"+s.ID || items[i].Checked != (s.ID == plainShell()) {
			t.Fatalf("View, Shell lists %+v", items)
		}
	}
	for len(r.got) > 0 {
		<-r.got
	}
	r.e.Post(func() { r.a.MenuChosen("shells:minimal") })
	r.e.Settle()
	l := r.e.Layout(1)
	r.flushPrefs()
	if r.e.PageOf(1) != "shell/minimal" || prefs.New(dir).Shell() != "minimal" || len(l.Regions) != 0 || len(l.Tabs) != 1 {
		t.Fatalf("the window loads %q, the pref is %q, the layout %+v", r.e.PageOf(1), prefs.New(dir).Shell(), l)
	}
	r.e.Post(func() { r.a.Shell.TabsChanged() })
	r.e.Settle()
	select {
	case b := <-r.got:
		t.Fatalf("the new shell heard %s before it said it's ready", b)
	default:
	}
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
}

// A shell's own data is kept per window, as it last saved it, and comes back in the
// snapshot: after its shell crashed and reloaded too. Another window's shell has none. It
// goes when the shell changes.
func TestAShellsOwnDataIsKeptPerWindow(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	snapshot := func() map[string]any {
		r.e.Settle()
		for len(r.got) > 0 {
			<-r.got
		}
		r.shell(`{"t":"ready"}`)
		return r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	}
	if m := snapshot(); m["data"] != nil {
		t.Fatalf("a new window's shell has data %v", m["data"])
	}
	r.shell(`{"t":"data","data":{"panes":[1]}}`)
	r.shell(`{"t":"data","data":{"panes":[1,2]}}`)
	r.e.Post(func() { r.a.ShellCrashed(1) })
	if m := snapshot(); !reflect.DeepEqual(m["data"], map[string]any{"panes": []any{1.0, 2.0}}) {
		t.Fatalf("after a crash the shell has %v", m["data"])
	}
	var w2 engine.WindowID
	r.e.Post(func() { w2, _ = r.a.NewWindow("") })
	r.e.Settle()
	got, stop := r.e.Feeds(w2).Shell.Subscribe()
	defer stop()
	r.e.Post(func() { r.a.ShellMessage(w2, []byte(`{"t":"ready"}`)) })
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "snapshot" }); m["data"] != nil {
		t.Fatalf("another window's shell has %v", m["data"])
	}
	r.e.Post(func() { r.a.MenuChosen("shells:minimal") })
	if m := snapshot(); m["data"] != nil {
		t.Fatalf("the next shell has %v", m["data"])
	}
}

// A shell's own action runs only while its shell is the profile's, and sends that shell
// action with its id. With another shell on, its key isn't taken, so the page keeps it, and
// the shell hears nothing.
func TestAShellsOwnActionRunsOnlyWhileItsShellIsOn(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	took := func(k engine.Key) bool {
		var ok bool
		r.e.Post(func() { ok = r.a.Key(k, engine.KeyUnhandled, engine.SurfacePage) })
		r.e.Settle()
		return ok
	}
	cmdD := engine.Key{Code: 'D', Mods: engine.ModCmd}
	shiftCmdD := engine.Key{Code: 'D', Mods: engine.ModCmd | engine.ModShift}
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if took(cmdD) || took(shiftCmdD) {
		t.Fatal("with the standard shell on, the terminal's keys were taken")
	}
	r.e.Post(func() { r.a.MenuChosen("terminal.splitRight") })
	r.e.Settle()
	for len(r.got) > 0 {
		var m map[string]any
		json.Unmarshal(<-r.got, &m)
		if m["t"] == "action" {
			t.Fatalf("the standard shell heard %v", m)
		}
	}

	r.e.Post(func() { r.a.MenuChosen("shells:terminal") })
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if !took(cmdD) {
		t.Fatal("with the terminal shell on, Cmd+D wasn't taken")
	}
	if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != "terminal.splitRight" {
		t.Fatalf("Cmd+D sent %v", m)
	}
	if !took(shiftCmdD) {
		t.Fatal("with the terminal shell on, Shift+Cmd+D wasn't taken")
	}
	if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != "terminal.splitDown" {
		t.Fatalf("Shift+Cmd+D sent %v", m)
	}
}

// The addresses on the command line open as the first window's tabs at the start, in place
// of the blank one, whether the shell takes New Tab or not: ChromeDriver starts the app so
// and looks for a page.
func TestTheCommandLinesAddressesOpenAtTheStart(t *testing.T) {
	urls := []string{"https://example.com/", "https://example.org/"}
	shellsToTry := []string{plainShell()}
	for _, s := range shells.Listed() {
		if slices.Contains(s.Takes, "tab.new") {
			shellsToTry = append(shellsToTry, s.ID)
			break
		}
	}
	for _, id := range shellsToTry {
		t.Run(id, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"shell":"`+id+`"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), URLs: urls, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
			defer r.stop()
			// Each loads, and only then is the run stopped, as a load ends in history.
			var got []string
			for deadline := time.Now().Add(5 * time.Second); ; {
				loading := false
				got = got[:0]
				r.e.Post(func() {
					for _, tab := range r.a.Tabs.AllIn(1) {
						got = append(got, tab.URL)
						loading = loading || tab.Loading
					}
				})
				r.e.Settle()
				if (!loading && len(got) == len(urls)) || time.Now().After(deadline) {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if !slices.Equal(got, urls) {
				t.Fatalf("the first window's tabs load %v, want %v", got, urls)
			}
		})
	}
}

// A shell that takes New Tab runs it itself. With it the profile's shell, the app starts
// with a window of no tabs, and New Window opens another; New Tab from the menu or its key
// sends the shell the action, and its span to end once the shell has drawn what it does,
// and makes no tab, while the shell's own newTab still makes one. With another shell on,
// New Tab makes a tab and New Window opens with one, as ever. It runs on the first shell
// the registry lists that takes tab.new, and is skipped while none does.
func TestAShellTakesNewTab(t *testing.T) {
	var taker, other string
	for _, s := range shells.Listed() {
		switch {
		case taker == "" && slices.Contains(s.Takes, "tab.new"):
			taker = s.ID
		case other == "" && !slices.Contains(s.Takes, "tab.new"):
			other = s.ID
		}
	}
	if taker == "" {
		t.Skip("no shell in the registry takes tab.new yet")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"shell":"`+taker+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r := start(t, dir)
	defer r.stop()
	snapshot := func(m map[string]any) bool { return m["t"] == "snapshot" }
	r.waitFor(snapshot)
	// How many tabs each window has, in the order the windows were opened.
	tabs := func() []int {
		var n []int
		r.e.Post(func() {
			for _, w := range r.a.Window.Windows() {
				n = append(n, len(r.a.Tabs.AllIn(w)))
			}
		})
		r.e.Settle()
		return n
	}
	want := func(when string, counts ...int) {
		t.Helper()
		if got := tabs(); !slices.Equal(got, counts) {
			t.Fatalf("%s the windows' tabs are %v, want %v", when, got, counts)
		}
	}
	want("at the start", 0)
	told := func(how string) {
		t.Helper()
		if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != "tab.new" {
			t.Fatalf("New Tab from %s sent %v", how, m)
		}
		if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "paint" }); m["span"] != "tab.new" {
			t.Fatalf("New Tab from %s asked for the end of %v", how, m)
		}
		want("after New Tab from "+how, 0)
	}
	r.e.Post(func() { r.a.MenuChosen("tab.new") })
	told("the menu")
	r.e.Post(func() { r.a.Key(engine.Key{Code: 'T', Mods: engine.ModCmd}, engine.KeyUnhandled, engine.SurfacePage) })
	told("its key")

	r.shell(`{"t":"newTab"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" })
	want("after the shell's newTab", 1)

	r.e.Post(func() { r.a.MenuChosen("window.new") })
	want("after New Window", 1, 0)

	r.e.Post(func() { r.a.MenuChosen("shells:" + other) })
	r.shell(`{"t":"ready"}`)
	r.waitFor(snapshot)
	r.e.Post(func() { r.a.MenuChosen("window.new") })
	want("with "+other+" on, after New Window", 1, 0, 1)
	r.e.Post(func() { r.a.MenuChosen("tab.new") })
	want("with "+other+" on, after New Tab in it", 1, 0, 2)
}

// The tab groups through the shell's messages: a new tab beside one starts a group of the
// two, which every tabs message carries; a tab a page opens joins its opener's group;
// editGroup renames it; closeGroup closes its tabs.
func TestGroupsThroughTheShell(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	groups := func(m map[string]any) []any { g, _ := m["groups"].([]any); return g }

	r.shell(`{"t":"newTab","groupWith":1}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(groups(m)) == 1 })
	g := groups(m)[0].(map[string]any)
	if ids := g["tabs"].([]any); len(ids) != 2 || ids[0].(float64) != 1 || g["color"] != tabs.GroupColors[0] {
		t.Fatalf("group %v", g)
	}
	r.e.Fire(1, 1, "open", "https://c.test/")
	m = r.waitFor(func(m map[string]any) bool {
		return m["t"] == "tabs" && len(groups(m)) == 1 && len(groups(m)[0].(map[string]any)["tabs"].([]any)) == 3
	})
	r.shell(`{"t":"editGroup","group":"` + g["id"].(string) + `","name":"Release"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "tabs" && len(groups(m)) == 1 && groups(m)[0].(map[string]any)["name"] == "Release"
	})
	r.shell(`{"t":"newTab"}`)
	r.shell(`{"t":"closeGroup","group":"` + g["id"].(string) + `"}`)
	r.waitFor(func(m map[string]any) bool {
		return m["t"] == "tabs" && len(m["tabs"].([]any)) == 1 && len(groups(m)) == 0
	})
}

// What a shell saves for the profile reaches every window's shell, and comes back in the
// snapshot after a restart.
func TestProfileDataOutlivesARestart(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	r.shell(`{"t":"profileData","data":{"pinned":["https://a.test/"]}}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "profileData" })
	if p := m["profileData"].(map[string]any)["pinned"].([]any); len(p) != 1 {
		t.Fatalf("profileData %v", m)
	}
	r.stop()
	r = start(t, dir)
	defer r.stop()
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if p, _ := m["profileData"].(map[string]any); p == nil || len(p["pinned"].([]any)) != 1 {
		t.Fatalf("snapshot profileData %v", m["profileData"])
	}
}

// A shell's snapshot lists the actions it can run, by title.
func TestTheShellHearsTheActionsByTitle(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	found := false
	for _, a := range m["actions"].([]any) {
		found = found || a.(map[string]any)["id"] == "app.settings"
	}
	if !found {
		t.Fatalf("no app.settings in the snapshot's actions: %v", m["actions"])
	}
}

// The glide keymap gives Cmd+T to the Command Bar, Cmd+D to pinning and Option+Cmd+Right to
// the next Space; Ctrl+1 is Space 1 in every keymap. Each reaches Glide as its action.
func TestTheGlideKeymap(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.e.Post(func() { r.a.MenuChosen("shells:glide") })
	r.shell(`{"t":"ready"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	press := func(keys, want string) {
		t.Helper()
		k, err := commands.ParseKeys(keys)
		if err != nil {
			t.Fatal(err)
		}
		var took bool
		r.e.Post(func() {
			took = r.a.Key(k, engine.KeyBeforePage, engine.SurfacePage) || r.a.Key(k, engine.KeyUnhandled, engine.SurfacePage)
		})
		r.e.Settle()
		if !took {
			t.Fatalf("%v wasn't taken", k)
		}
		if m := r.waitFor(func(m map[string]any) bool { return m["t"] == "action" }); m["id"] != want {
			t.Fatalf("%v sent %v, want %s", k, m, want)
		}
	}
	press("Ctrl+1", "glide.space1")
	r.e.Post(func() { r.a.Commands.UseTemplate("glide") })
	r.e.Settle()
	press("Cmd+T", "glide.commandBar")
	press("Cmd+D", "glide.pin")
	press("Option+Cmd+Right", "glide.spaceNext")
}

// The settings page's Install update hands a ready release to the installer, as the
// shell's button does, on a phone build: it once relaunched the app instead, which on a
// phone does nothing.
func TestTheSettingsPageInstallsAReadyUpdate(t *testing.T) {
	apk := []byte("an APK, as far as the updater cares")
	sum := sha256.Sum256(apk)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/asset" {
			w.Write(apk)
			return
		}
		fmt.Fprintf(w, `{"spec":1,"project":"project-b","published":"2026-11-14T00:00:00Z","channels":{"stable":"0.9.0"},
		  "releases":{"0.9.0":{"assets":{"%s/%s":{"url":"%s/asset","sha256":"%s","size":%d}}}}}`,
			runtime.GOOS, runtime.GOARCH, srv.URL, hex.EncodeToString(sum[:]), len(apk))
	}))
	defer srv.Close()
	dir := t.TempDir()
	installed := make(chan string, 4)
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Update: update.Config{
		Project: "project-b", Version: "0.1.0", Releases: srv.URL, StateDir: filepath.Join(dir, "updates"),
		Install: func(apk string) { installed <- apk }}})
	defer r.stop()
	r.e.Post(func() { r.a.Update.CheckNow() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var s update.State
		done := make(chan struct{})
		r.e.Post(func() { s = r.a.Update.State(); close(done) })
		<-done
		if s.Phase == update.Ready {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	r.settings(`{"t":"relaunch"}`)
	select {
	case got := <-installed:
		if b, err := os.ReadFile(got); err != nil || string(b) != string(apk) {
			t.Fatalf("installed %q: %q %v", got, b, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the settings page's Install update never reached the installer")
	}
}

// moveTab from the shell: a tab moves in its window to the index given, the active one
// staying; into another window it goes there at the index, clamped; a window it leaves
// empty closes; an unknown window changes nothing.
func TestTheShellMovesATab(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.shell(`{"t":"newTab"}`)
	ids := func(m map[string]any) []float64 {
		var out []float64
		for _, t := range m["tabs"].([]any) {
			out = append(out, t.(map[string]any)["id"].(float64))
		}
		return out
	}
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 3 })
	if got := ids(m); !slices.Equal(got, []float64{1, 2, 3}) || m["active"] != 3.0 {
		t.Fatalf("tabs %v, active %v", got, m["active"])
	}
	r.shell(`{"t":"moveTab","id":3,"index":0}`)
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && ids(m)[0] == 3 })
	if got := ids(m); !slices.Equal(got, []float64{3, 1, 2}) || m["active"] != 3.0 {
		t.Fatalf("after moving 3 first: tabs %v, active %v", got, m["active"])
	}
	r.shell(`{"t":"moveTab","id":1,"index":9,"window":77}`)
	r.e.Settle()
	if got := r.a.Tabs.WindowOf(1); got != 1 {
		t.Fatalf("a move to an unknown window took the tab to %v", got)
	}
	var w2 engine.WindowID
	r.e.Post(func() { w2, _ = r.a.NewWindow("") })
	r.e.Settle()
	r.shell(fmt.Sprintf(`{"t":"moveTab","id":1,"index":9,"window":%d}`, w2))
	r.e.Settle()
	if got := r.a.Tabs.AllIn(w2); r.a.Tabs.WindowOf(1) != w2 || got[len(got)-1].ID != 1 {
		t.Fatalf("tab 1 is in %v, %v's tabs %v", r.a.Tabs.WindowOf(1), w2, got)
	}
	// Both of the second window's tabs move back to the first, and the window, left empty,
	// closes.
	first := r.a.Tabs.AllIn(w2)[0].ID
	r.e.Post(func() {
		r.a.ShellMessage(w2, []byte(fmt.Sprintf(`{"t":"moveTab","id":%d,"index":0,"window":1}`, first)))
	})
	r.e.Post(func() { r.a.ShellMessage(w2, []byte(`{"t":"moveTab","id":1,"index":0,"window":1}`)) })
	r.e.Settle()
	if slices.Contains(r.a.Window.Windows(), w2) {
		t.Fatalf("window %v stays open with no tabs", w2)
	}
}

// A shell may set any of Go's prefs by name, with the setters' own checks: the frame colour
// changes the window and reaches every window as windowColor and in prefs; a value Go
// refuses, or a name that isn't a pref, changes nothing. The snapshot carries the prefs.
func TestAShellSetsGoPrefsByName(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if p, _ := snap["prefs"].(map[string]any); p == nil || p["frameColor"] != "#4f9e9e" || p["theme"] != "system" {
		t.Fatalf("the snapshot's prefs are %v", snap["prefs"])
	}
	r.shell(`{"t":"setPref","name":"frameColor","value":"#d03434"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "windowColor" && m["windowColor"] == "#d03434" })
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "prefs" })
	if p := m["prefs"].(map[string]any); p["frameColor"] != "#d03434" {
		t.Fatalf("prefs after setPref: %v", p)
	}
	r.flushPrefs()
	if prefs.New(dir).FrameColor() != "#d03434" || r.e.Layout(1).FrameColor != "#d03434" {
		t.Fatal("the frame colour wasn't kept, or the window isn't painted with it")
	}
	r.shell(`{"t":"setPref","name":"theme","value":"dark"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "theme" && m["theme"] == "dark" })
	for _, bad := range []string{`{"t":"setPref","name":"frameColor","value":"red"}`, `{"t":"setPref","name":"theme","value":"sepia"}`, `{"t":"setPref","name":"nope","value":"x"}`, `{"t":"setPref","value":"x"}`} {
		r.shell(bad)
	}
	r.e.Settle()
	r.flushPrefs()
	if got := prefs.New(dir).All(); got.FrameColor != "#d03434" || got.Theme != "dark" {
		t.Fatalf("a refused setPref changed the prefs to %+v", got)
	}
}

// The shells paint with Go's tones (docs/native-shell.md): both palettes of the frame
// colour come in the snapshot and again with each new colour, and each workspace has its
// colour's, for a shell that paints with it.
func TestTheTonesComeWithTheFrameColour(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	accent := func(m map[string]any, theme string) any {
		both, _ := m["tones"].(map[string]any)
		palette, _ := both[theme].(map[string]any)
		return palette["--accent"]
	}
	r.shell(`{"t":"ready"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if accent(m, "light") != tones.For("#4f9e9e", false)["--accent"] || accent(m, "dark") != tones.For("#4f9e9e", true)["--accent"] {
		t.Fatalf("snapshot tones = %v", m["tones"])
	}
	ws := m["workspaces"].([]any)[0].(map[string]any)
	if accent(ws, "dark") != tones.For(ws["color"].(string), true)["--accent"] {
		t.Fatalf("workspace tones = %v", ws)
	}

	r.openSettings()
	r.settings(`{"t":"setFrameColor","text":"#d82b1f"}`)
	m = r.waitFor(func(m map[string]any) bool { return m["t"] == "windowColor" && m["windowColor"] == "#d82b1f" })
	if accent(m, "dark") != tones.For("#d82b1f", true)["--accent"] {
		t.Fatalf("windowColor tones = %v", m["tones"])
	}
}

// On a phone, the settings page offers the phone's shells, and a choice there loads it:
// a phone has no Profile section or View menu.
func TestAPhonesSettingsChooseItsShell(t *testing.T) {
	defer func(was bool) { shells.OnPhone = was }(shells.OnPhone)
	shells.OnPhone = true
	r := start(t, t.TempDir())
	defer r.stop()
	_, snap := r.openSettings()
	ps, _ := snap["phoneShells"].(map[string]any)
	list, _ := ps["shells"].([]any)
	if ps["shell"] != "mobile" || len(list) == 0 || list[0].(map[string]any)["id"] != "mobile" {
		t.Fatalf("phone shells %v", snap["phoneShells"])
	}
	r.settings(`{"t":"setShell","text":"mobile"}`)
	// Off a phone the part isn't there: the Profile section offers the shells.
	shells.OnPhone = false
	r2 := start(t, t.TempDir())
	defer r2.stop()
	if _, snap2 := r2.openSettings(); snap2["phoneShells"] != nil {
		t.Fatalf("a desktop's settings offer phone shells: %v", snap2["phoneShells"])
	}
}

// The menu skips a tab's changes it doesn't read, and still follows the ones it does: a tab
// that began loading while in the background shows Stop when made active, and Stop goes
// off when it finishes there.
func TestTheMenuFollowsTheActiveTabsLoading(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.e.Settle()
	stop := func() bool {
		var on bool
		done := make(chan struct{})
		r.e.Post(func() {
			for _, top := range r.e.Menu() {
				for _, item := range top.Items {
					if item.Action == "nav.stop" {
						on = item.Enabled
					}
				}
			}
			close(done)
		})
		<-done
		return on
	}
	// The mock's new tabs are loading their blank page: both stop first.
	r.e.Post(func() { r.a.LoadingChanged(1, false); r.a.LoadingChanged(2, false) })
	r.e.Post(func() { r.a.Tabs.Activate(1) })
	r.e.Post(func() { r.a.LoadingChanged(2, true) })
	r.e.Settle()
	if stop() {
		t.Fatal("Stop is on for a tab in the background that loads")
	}
	r.e.Post(func() { r.a.Tabs.Activate(2) })
	r.e.Settle()
	if !stop() {
		t.Fatal("Stop is off for the active tab while it loads")
	}
	r.e.Post(func() { r.a.TitleChanged(2, "A title") })
	r.e.Post(func() { r.a.LoadingChanged(2, false) })
	r.e.Settle()
	if stop() {
		t.Fatal("Stop is still on once the active tab finished loading")
	}
}

// flushPrefs waits until the prefs are written, which happens off the main thread, as the
// app does on its way out.
func (r *rig) flushPrefs() {
	r.t.Helper()
	done := make(chan struct{})
	r.e.Post(func() { r.a.Prefs.Close(); close(done) })
	<-done
}

// The tabs message carries no favicons, a data URL each that it would send again on every
// change: a favicon comes in a tab message, when it changes and when its tab moves into
// another window, after the list that has it there.
func TestFaviconsComeOnlyInTabMessages(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"newTab"}`)
	r.waitFor(isTabs(2))
	const icon = "data:image/png;base64,AAAA"
	r.e.Post(func() { r.a.FaviconChanged(1, icon) })
	r.waitForTab(func(tab map[string]any) bool { return tab["id"] == 1.0 && tab["favicon"] == icon })
	r.shell(`{"t":"newTab"}`)
	m := r.waitFor(isTabs(3))
	for _, tab := range m["tabs"].([]any) {
		if f, ok := tab.(map[string]any)["favicon"]; ok {
			t.Fatalf("the tabs message carries a favicon: %v", f)
		}
	}

	// Tab 1 done loading, so nothing else of it is sent once it moves.
	r.e.Post(func() { r.a.LoadingChanged(1, false) })
	r.e.Settle()
	second := r.openSecondWindow()
	r.waitOn(second, func(m map[string]any) bool { return m["t"] == "snapshot" })
	r.shell(`{"t":"moveTab","id":1,"index":0,"window":2}`)
	r.waitOn(second, isTabs(2))
	r.waitOn(second, func(m map[string]any) bool {
		tab, ok := m["tab"].(map[string]any)
		return m["t"] == "tab" && ok && tab["id"] == 1.0 && tab["favicon"] == icon
	})
}
