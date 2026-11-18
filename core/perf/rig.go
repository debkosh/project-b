// Package perf measures the Go core on the mock engine, with the engine's own work taken
// out: Eng answers every engine call the way cmd/lib's native engine does on the Go side
// (nothing but the crossing, and SetMenu's JSON), and counts the calls, which are the
// Go-to-C crossings the real app would make. Events arrive as the adapter's hooks would,
// as direct calls on the app, on one goroutine that plays the main thread.
//
// Measurement only; nothing here is part of the app.
package perf

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"projectb/app"
	"projectb/engine"
	"projectb/mockengine"
	"projectb/profiles"
	"projectb/timeline"
	"projectb/update"
)

// Call kinds that cross from Go to C in the real app.
const (
	CCreateTab = iota
	CCloseTab
	CLoadURL
	CNav
	CWindow
	CLayout
	CFocus
	CSendToShell
	CSendToPage
	CSetMenu
	COther
	NumCalls
)

var CallNames = [NumCalls]string{"CreateTab", "CloseTab", "LoadURL", "Back/Fwd/Reload", "Window", "SetWindowLayout", "Focus", "SendToShell", "SendToPage", "SetMenu", "Other"}

// Eng is the engine as Go sees it in the real app: every call is one crossing to C and
// no Go work, except SetMenu, which marshals the menu in cmd/lib.
type Eng struct {
	*mockengine.Engine
	mu     sync.Mutex
	posted []func()
	next   engine.TabID

	Calls      [NumCalls]int
	ShellBytes int
	ShellMsgs  int
	MenuBytes  int
	// Keep, when set, is handed every shell message, as the shell's page would get it.
	Keep func(w engine.WindowID, msg []byte)
}

func (e *Eng) EraseWorkspace(string, engine.Erasure) { e.Calls[COther]++ }

func (e *Eng) CreateTab(string) engine.TabID     { e.Calls[CCreateTab]++; e.next++; return e.next }
func (e *Eng) CreateInternalTab() engine.TabID   { e.Calls[CCreateTab]++; e.next++; return e.next }
func (e *Eng) SetFirstWorkspace(string)          { e.Calls[COther]++ }
func (e *Eng) CloseTab(engine.TabID)             { e.Calls[CCloseTab]++ }
func (e *Eng) LoadURL(engine.TabID, string)      { e.Calls[CLoadURL]++ }
func (e *Eng) GoBack(engine.TabID)               { e.Calls[CNav]++ }
func (e *Eng) GoForward(engine.TabID)            { e.Calls[CNav]++ }
func (e *Eng) Reload(engine.TabID)               { e.Calls[CNav]++ }
func (e *Eng) ReloadBypassingCache(engine.TabID) { e.Calls[CNav]++ }
func (e *Eng) Stop(engine.TabID)                 { e.Calls[CNav]++ }
func (e *Eng) Zoom(engine.TabID, float64)        { e.Calls[COther]++ }
func (e *Eng) ZoomOf(engine.TabID) float64       { e.Calls[COther]++; return 1 }
func (e *Eng) DuplicateTab(engine.TabID) engine.TabID {
	e.Calls[CCreateTab]++
	e.next++
	return e.next
}
func (e *Eng) Edit(engine.EditCommand) { e.Calls[COther]++ }
func (e *Eng) CreateWindow(w, near engine.WindowID, p engine.Page, l engine.WindowLayout) {
	e.Calls[CWindow]++
}
func (e *Eng) SetWindowPage(engine.WindowID, engine.Page)           { e.Calls[CWindow]++ }
func (e *Eng) RequestWindow(engine.WindowID, engine.WindowRequest)  { e.Calls[CWindow]++ }
func (e *Eng) SetWindowBounds(engine.WindowID, engine.Rect)         { e.Calls[CWindow]++ }
func (e *Eng) SetWindowMode(engine.WindowID, engine.WindowMode)     { e.Calls[CWindow]++ }
func (e *Eng) SetWindowLayout(engine.WindowID, engine.WindowLayout) { e.Calls[CLayout]++ }
func (e *Eng) FocusTab(engine.TabID)                                { e.Calls[CFocus]++ }
func (e *Eng) FocusShell(engine.WindowID)                           { e.Calls[CFocus]++ }
func (e *Eng) SetShellBack(engine.WindowID, bool)                   { e.Calls[COther]++ }
func (e *Eng) SendToPage(engine.TabID, []byte)                      { e.Calls[CSendToPage]++ }
func (e *Eng) AppCommand(engine.AppCommand)                         { e.Calls[COther]++ }
func (e *Eng) SetDockColor(string)                                  { e.Calls[COther]++ }
func (e *Eng) AskDefaultBrowser()                                   { e.Calls[COther]++ }
func (e *Eng) OpenFullDiskAccess()                                  { e.Calls[COther]++ }
func (e *Eng) IsDefaultBrowser() bool                               { return false }
func (e *Eng) AskNotifications()                                    { e.Calls[COther]++ }
func (e *Eng) NotificationsAllowed() bool                           { return true }
func (e *Eng) SendToShell(w engine.WindowID, msg []byte) {
	e.Calls[CSendToShell]++
	e.ShellMsgs++
	e.ShellBytes += len(msg)
	if e.Keep != nil {
		e.Keep(w, msg)
	}
}

// SetMenu does what cmd/lib's does in Go: marshal the menu for the crossing.
func (e *Eng) SetMenu(m engine.Menu) {
	e.Calls[CSetMenu]++
	b, _ := json.Marshal(m)
	e.MenuBytes += len(b)
}

func (e *Eng) Post(fn func()) {
	e.mu.Lock()
	e.posted = append(e.posted, fn)
	e.mu.Unlock()
}

// Crossings is the total of Go-to-C calls so far.
func (e *Eng) Crossings() int {
	n := 0
	for _, c := range e.Calls {
		n += c
	}
	return n
}

// Rig is the app on Eng.
type Rig struct {
	E   *Eng
	A   *app.App
	Dir string
	// Favicon is what each navigation's page reports as its icon: a data URL.
	Favicon string
	// Progress is how many load progress events a navigation has.
	Progress int
	seq      int
	nav      int64 // the last navigation id content gave, as content numbers them
}

// Options shape a rig.
type Options struct {
	Blocked []string
	// History is how many entries the profile's history starts with.
	History int
	// FaviconBytes is the size of each page's favicon data URL; 0 for none.
	FaviconBytes int
	// Tabs is how many tabs the first window has, each navigated once, made before its
	// shell is ready, as a restored session would be.
	Tabs int
}

var startTimeline sync.Once

// Quiet starts the timeline as the app does, writing its file, but printing nothing.
func Quiet() {
	startTimeline.Do(func() {
		d, _ := os.MkdirTemp("", "perf-timeline-")
		timeline.Start(d, 0, io.Discard)
	})
}

// New starts the app in a fresh profile under dir, with one window whose shell is ready.
// The timeline writes its file, as in the app, but prints nothing.
func New(dir string, o Options) *Rig {
	Quiet()
	if o.History > 0 {
		writeHistory(dir, o.History)
	}
	e := &Eng{Engine: mockengine.New()}
	r := &Rig{E: e, Dir: dir, Progress: 5}
	if o.FaviconBytes > 0 {
		r.Favicon = "data:image/png;base64," + strings.Repeat("iVBORw0KGgo", o.FaviconBytes/11+1)[:o.FaviconBytes]
	}
	r.A = app.Start(e, app.Config{ProfileDir: dir, Profiles: profiles.New(filepath.Join(dir, "root")),
		BlockedHosts: o.Blocked, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	// History loads on a worker and posts back.
	deadline := time.Now().Add(10 * time.Second)
	for o.History > 0 && len(r.A.Suggest.Query(0, "site", nil)) == 0 && time.Now().Before(deadline) {
		r.Drain()
		time.Sleep(time.Millisecond)
	}
	r.Drain()
	if o.Tabs > 0 {
		r.Events(r.A.Tabs.ActiveIn(1), "https://www.start.example.com/")
		for i := 1; i < o.Tabs; i++ {
			r.Events(r.A.Tabs.Add(1, "", true), fmt.Sprintf("https://www.site%d.example.com/page", i))
		}
	}
	r.WindowUp(1)
	return r
}

// writeHistory writes n visits to the profile's history file, as earlier runs would have.
func writeHistory(dir string, n int) {
	os.MkdirAll(dir, 0o700)
	f, err := os.Create(filepath.Join(dir, "history.jsonl"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for i := 0; i < n; i++ {
		enc.Encode(map[string]any{"url": fmt.Sprintf("https://www.site%d.example.com/path/page-%d?q=%d", i%5000, i, i*7),
			"title": fmt.Sprintf("Page %d of a site about topic %d", i, i%97), "visits": 1 + i%13, "lastVisit": 1700000000 + i})
	}
}

// Drain runs what was posted to the main thread, as PBRunPosted does.
func (r *Rig) Drain() {
	for {
		r.E.mu.Lock()
		fns := r.E.posted
		r.E.posted = nil
		r.E.mu.Unlock()
		if len(fns) == 0 {
			return
		}
		for _, fn := range fns {
			fn()
		}
	}
}

// WindowUp is what a new window's engine and shell report: its state, ready, its first
// layout, shown.
func (r *Rig) WindowUp(w engine.WindowID) {
	r.A.WindowChanged(w, engine.WindowState{W: mockengine.Width, H: mockengine.Height, Controls: mockengine.Controls,
		CornerRadius: mockengine.CornerRadius, Bounds: engine.Rect{X: 80, Y: 60, W: mockengine.Width, H: mockengine.Height}})
	r.A.ShellMessage(w, []byte(`{"t":"ready"}`))
	r.Layout(w, r.A.Tabs.ActiveIn(w))
	r.A.ShellMessage(w, []byte(`{"t":"shown"}`))
}

// NewWindow is File, New Window: a window with one new tab, its shell up.
func (r *Rig) NewWindow() engine.WindowID {
	w, _ := r.A.NewWindow("")
	r.WindowUp(w)
	return w
}

// NewWindowQuiet opens a window whose shell has not loaded yet: it hears nothing until
// WindowUp.
func (r *Rig) NewWindowQuiet() engine.WindowID {
	w, _ := r.A.NewWindow("")
	return w
}

// Layout is the shell's report after it drew: its regions and the tab it shows.
func (r *Rig) Layout(w engine.WindowID, tab engine.TabID) {
	r.seq++
	// The size changes a little each time so the window manager never skips it as unchanged.
	h := 652 - float64(r.seq%2)
	msg := fmt.Appendf(nil, `{"t":"layout","shell":[{"x":0,"y":0,"w":1100,"h":48},{"x":0,"y":0,"w":240,"h":700}],`+
		`"controls":[{"x":90,"y":8,"w":32,"h":32},{"x":130,"y":8,"w":32,"h":32},{"x":300,"y":8,"w":600,"h":32}],`+
		`"drag":[{"x":0,"y":0,"w":1100,"h":44}],"tabs":[{"tab":%d,"rect":{"x":240,"y":48,"w":860,"h":%v}}],"viewW":1100,"viewH":700,"frame":0}`, tab, h)
	r.A.ShellMessage(w, msg)
}

// painted ends the span the shell ends once it has drawn.
func painted(span string) { timeline.End(span) }

// NewTab is the shell's New Tab button, the layout the shell reports after drawing it,
// and the shell's end of the span.
func (r *Rig) NewTab(w engine.WindowID) engine.TabID {
	r.A.ShellMessage(w, []byte(`{"t":"newTab"}`))
	id := r.A.Tabs.ActiveIn(w)
	r.Layout(w, id)
	r.A.ShellMessage(w, []byte(`{"t":"focusTab","id":`+fmt.Sprint(id)+`}`))
	painted("tab.new")
	return id
}

// Activate is a click on a tab, and the shell's layout after.
func (r *Rig) Activate(w engine.WindowID, id engine.TabID) {
	r.A.ShellMessage(w, fmt.Appendf(nil, `{"t":"activateTab","id":%d}`, id))
	r.Layout(w, id)
	r.A.ShellMessage(w, fmt.Appendf(nil, `{"t":"focusTab","id":%d}`, id))
	painted("tab.activate")
}

// Close is a click on a tab's close button, and the shell's layout after.
func (r *Rig) Close(w engine.WindowID, id engine.TabID) {
	r.A.ShellMessage(w, fmt.Appendf(nil, `{"t":"closeTab","id":%d}`, id))
	if a := r.A.Tabs.ActiveIn(w); a != 0 {
		r.Layout(w, a)
	}
	painted("tab.close")
}

// Navigate is the address bar's Enter, then every event content raises for a navigation
// that commits, as cmd/lib's hooks hand them to the app.
func (r *Rig) Navigate(w engine.WindowID, text string) {
	r.A.ShellMessage(w, append(append([]byte(`{"t":"navigate","text":"`), text...), `"}`...))
	r.Events(r.A.Tabs.ActiveIn(w), "https://"+text)
}

// Events is one navigation of tab id to u, as content reports it.
func (r *Rig) Events(id engine.TabID, u string) {
	a := r.A
	r.nav++
	if a.NavigationStarting(id, r.nav, u) != engine.NavigationProceed {
		return
	}
	a.LoadingChanged(id, true)
	for i := 1; i <= r.Progress; i++ {
		a.LoadProgressChanged(id, float64(i)/float64(r.Progress+1))
	}
	a.HistoryChanged(id, true, false)
	a.NavigationFinished(id, r.nav, u)
	a.TitleChanged(id, "A page at "+u)
	if r.Favicon != "" {
		a.FaviconChanged(id, r.Favicon+u[len(u)-4:])
	}
	a.LoadProgressChanged(id, 1)
	a.LoadingChanged(id, false)
}

// Type is typing text in the address bar: a suggest query per key.
func (r *Rig) Type(w engine.WindowID, text string) {
	for i := 1; i <= len(text); i++ {
		r.A.ShellMessage(w, fmt.Appendf(nil, `{"t":"suggest","id":%d,"text":%q}`, i, text[:i]))
	}
}
