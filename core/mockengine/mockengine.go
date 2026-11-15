// Package mockengine is a browser engine with no browser in it. Its page is a set of
// buttons that fire the events real content would, for the tab it shows: navigation,
// title, failure, crash, a page opening a tab, a key press, full screen. A window shows a
// mock page for each tab its layout lists.
package mockengine

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"projectb/engine"
)

// loadDelay is how long a mock page takes to load.
const loadDelay = 100 * time.Millisecond

// Controls is where the mock's window controls sit: what macOS 26 gives a window with
// a unified toolbar, whose buttons centre 26px down. Full screen hides them.
var Controls = engine.Rect{X: 19, Y: 19, W: 60, H: 14}

// CornerRadius is what macOS 26 rounds that window's corners with. Full screen has none.
const CornerRadius = 26.0

// Width and Height are a mock window's content size until its dev window reports its
// own, as the real window's first size.
const (
	Width  = 1100.0
	Height = 700.0
)

// Keys are the names the mock page uses for key presses, as a Mac keyboard has them.
var Keys = map[string]engine.Key{
	"focusAddressBar": {Code: 'L', Mods: engine.ModCmd},
	"escape":          {Code: 0x1b},
	"newTab":          {Code: 'T', Mods: engine.ModCmd},
	"newWindow":       {Code: 'N', Mods: engine.ModCmd},
	"closeTab":        {Code: 'W', Mods: engine.ModCmd},
	"nextTab":         {Code: 0xDD, Mods: engine.ModCmd | engine.ModShift},
	"previousTab":     {Code: 0xDB, Mods: engine.ModCmd | engine.ModShift},
	"recentTab":       {Code: 0x09, Mods: engine.ModCtrl},
	"recentTabBack":   {Code: 0x09, Mods: engine.ModCtrl | engine.ModShift},
	"tab1":            {Code: '1', Mods: engine.ModCmd},
	"tab2":            {Code: '2', Mods: engine.ModCmd},
	"tab3":            {Code: '3', Mods: engine.ModCmd},
	"tab9":            {Code: '9', Mods: engine.ModCmd},
	"settings":        {Code: 0xBC, Mods: engine.ModCmd},
	"splitRight":      {Code: 'D', Mods: engine.ModCmd | engine.ModAlt},
	"splitDown":       {Code: 'D', Mods: engine.ModCmd | engine.ModShift},
	"paneLeft":        {Code: 0x25, Mods: engine.ModCtrl | engine.ModCmd},
	"paneRight":       {Code: 0x27, Mods: engine.ModCtrl | engine.ModCmd},
	"paneUp":          {Code: 0x26, Mods: engine.ModCtrl | engine.ModCmd},
	"paneDown":        {Code: 0x28, Mods: engine.ModCtrl | engine.ModCmd},
	"dividerLeft":     {Code: 0x25, Mods: engine.ModCtrl | engine.ModShift | engine.ModCmd},
	"dividerRight":    {Code: 0x27, Mods: engine.ModCtrl | engine.ModShift | engine.ModCmd},
	"dividerUp":       {Code: 0x26, Mods: engine.ModCtrl | engine.ModShift | engine.ModCmd},
	"dividerDown":     {Code: 0x28, Mods: engine.ModCtrl | engine.ModShift | engine.ModCmd},
	"zoomPane":        {Code: 0x0D, Mods: engine.ModShift | engine.ModCmd},
}

// tab is one mock tab's session history and what its page shows.
type tab struct {
	history []string // The committed URLs.
	pos     int      // The current one in history, or -1 before the first.
	navSeq  int
	// nav is the id of the latest navigation, from the engine's one counter, as content's.
	nav     int64
	state   string // What the mock page shows: empty, loading, loaded, blocked, crashed.
	current string
	zoom    float64
	// workspace is the app's id for the workspace the tab is in, "" for the profile's own storage.
	workspace string
	// internal says it is one of ours, in our pages' context, and page is which of our
	// pages it shows, as its address names it.
	internal bool
	page     string
}

// Engine implements engine.Engine. Run is the main thread: everything happens there,
// as it does with real content. Each window the app makes is a dev window of its own, the
// shell at /?window=<id> on the dev server.
type Engine struct {
	queue    chan func()
	listener engine.Listener
	nextTab  engine.TabID
	nextNav  int64
	tabs     map[engine.TabID]*tab
	// first is the workspace the app last said is first; erased are the erases it asked
	// for, in order. The mock makes no tab on its own, so only tests read them, as they do
	// the profile folders the app opened.
	first    string
	erased   []Erase
	profiles []string
	rules    []engine.BlockRules
	adRules  engine.AdRules
	// badCerts is the hosts a test gave a bad certificate. Main thread.
	badCerts map[string]bool
	// perms is the page's permission requests and the app's answers.
	perms permissionLog

	// Written on the main thread, read by the dev server's goroutines.
	mu         sync.Mutex
	windows    map[engine.WindowID]*window
	feeds      map[engine.WindowID]*Feeds
	requests   []engine.WindowRequest
	modes      []engine.WindowMode
	closed     []engine.WindowID
	quit       bool
	shownColor string
	menu       engine.Menu
	dockColor  string
	edits      []engine.EditCommand
	clipboard  string
	downloads  downloads
	chooser    []engine.FileChooserAnswer
	dialogs    dialogs
	finds      finds
	// focused is the tab last given the keyboard, by FocusTab or a press in its page.
	focused engine.TabID
	// unshownFocus are the tabs FocusTab was asked for while no window showed them.
	unshownFocus []engine.TabID
	// shared is the file ShareFile last offered, as a share sheet would.
	shared string
	// left is whether the back key left the app, as Android does with no tab to go back
	// in or close.
	left bool
	// showRegions is the Show window regions flag, as the app launched with it.
	showRegions bool
	// settings are what the settings page in each tab of ours hears.
	settings map[engine.TabID]*Feed

	// The fake extension runtime, in a build with extensions.
	extensions extensionRuntime
}

// window is one mock window.
type window struct {
	page   engine.Page
	layout engine.WindowLayout
	// visible are the tabs the layout shows, in its order. The window has a mock page for
	// each, by its place in the list, its slot, and pages is what each slot's shows.
	visible           []engine.TabID
	pages             [][]byte
	hidden            []engine.TabID // Tabs told they are hidden by the last layout.
	full              bool           // Full screen, so there are no window controls.
	w, h              float64        // The window's content size.
	bounds            engine.Rect    // Its frame on screen.
	minimised, zoomed bool
	// ours are our tabs whose page this window holds, as the real window holds their
	// views: the tabs of ours its layout was the last to show, until they close. The dev
	// window keeps a frame of our page for each.
	ours      []our
	shellBack bool               // The phone's back key goes to the shell.
	reported  engine.WindowState // The last one reported.
}

// our is one of our tabs, and which of our pages it shows.
type our struct {
	Tab  engine.TabID `json:"tab"`
	Page string       `json:"page"`
}

// Feeds are what one window's dev window hears.
type Feeds struct {
	Shell Feed // Messages to the shell page.
	Page  Feed // State for the mock pages, each message for one slot.
	// What the dev window draws with the shell, where the real window has views of its
	// own: the pages, ours among them, and an extension's popup. Each message is
	// WindowState's.
	Window Feed
}

// New returns an engine. Call Run to start its main thread.
func New() *Engine {
	return &Engine{queue: make(chan func(), 4096), tabs: map[engine.TabID]*tab{},
		windows: map[engine.WindowID]*window{}, feeds: map[engine.WindowID]*Feeds{}, settings: map[engine.TabID]*Feed{}}
}

// Run is the main loop. It never returns.
func (e *Engine) Run() {
	for fn := range e.queue {
		fn()
	}
}

// PageURL, OurPage and ViewSourceURL name addresses as the real engine does, though
// the mock loads none of them.
func (e *Engine) PageURL(p engine.Page) string { return "chrome://" + string(p) + "/" }

func (e *Engine) OurPage(raw string) (engine.Page, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "chrome" || u.Host == "" {
		return "", false
	}
	return engine.Page(strings.ToLower(u.Host) + strings.TrimSuffix(u.Path, "/")), true
}

func (e *Engine) ViewSourceURL(u string) string { return "view-source:" + u }

// OpenProfile records the profile's folder. The mock keeps no site data, so the folder
// is only for tests.
func (e *Engine) OpenProfile(dir string) { e.profiles = append(e.profiles, dir) }

// Profiles are the folders the app opened, in order. On the main thread.
func (e *Engine) Profiles() []string { return e.profiles }

// SetBlockRules records the rules. The mock's pages make no requests to refuse.
func (e *Engine) SetBlockRules(r engine.BlockRules) { e.rules = append(e.rules, r) }

// SetAdRules keeps the rules, for a test to read. The mock's pages make no requests.
func (e *Engine) SetAdRules(r engine.AdRules) { e.adRules = r }

// AdRules is what the app last set.
func (e *Engine) AdRules() engine.AdRules { return e.adRules }

// BlockRules are the rules the app set, in order. On the main thread.
func (e *Engine) BlockRules() []engine.BlockRules { return e.rules }

// SetSwitches takes the switches the app launches with, from the flags saved in the
// profile, as the real engine finds them on its command line before content starts.
// Call it before the app starts.
func (e *Engine) SetSwitches(switches []string) {
	e.showRegions = slices.Contains(switches, "--pb-show-regions")
}

// Feeds are window w's feeds, whether or not the window exists yet. Any goroutine.
func (e *Engine) Feeds(w engine.WindowID) *Feeds {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.feeds[w] == nil {
		e.feeds[w] = &Feeds{}
	}
	return e.feeds[w]
}

// PageFeed is what the page of ours in tab hears, whether or not the tab exists yet. Any
// goroutine.
func (e *Engine) PageFeed(tab engine.TabID) *Feed {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.settings[tab] == nil {
		e.settings[tab] = &Feed{}
	}
	return e.settings[tab]
}

// SetListener sets who hears the engine's events, and reports the windows the app has
// made so far to it, as the real engine does once the call that made each has returned.
func (e *Engine) SetListener(l engine.Listener) {
	e.listener = l
	for _, w := range slices.Sorted(maps.Keys(e.windows)) {
		e.reportWindow(w)
	}
}

func (e *Engine) reportWindow(w engine.WindowID) {
	win := e.windows[w]
	if win == nil {
		return
	}
	s := engine.WindowState{W: win.w, H: win.h, FullScreen: win.full, Bounds: win.bounds, Minimised: win.minimised, Zoomed: win.zoomed}
	if !win.full {
		s.Controls = Controls
		s.CornerRadius = CornerRadius
	}
	e.mu.Lock()
	win.reported = s
	e.mu.Unlock()
	e.listener.WindowChanged(w, s)
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

// Resize is window w's dev window reporting its content size, as the real window reports
// a resize. One whose window closed reports to no one. Any goroutine.
func (e *Engine) Resize(w engine.WindowID, width, height float64) {
	e.Post(func() {
		if win := e.windows[w]; win != nil {
			win.w, win.h = width, height
			e.reportWindow(w)
		}
	})
}

// Reported is the state window w last reported.
func (e *Engine) Reported(w engine.WindowID) engine.WindowState {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return win.reported
	}
	return engine.WindowState{}
}

// CreateWindow makes a window. The first one's colour is what the real window paints
// when it first goes on screen.
func (e *Engine) CreateWindow(w, near engine.WindowID, page engine.Page, l engine.WindowLayout) {
	e.mu.Lock()
	if len(e.windows) == 0 && e.shownColor == "" {
		e.shownColor = l.FrameColor
	}
	// Every mock window starts where the real first one does.
	e.windows[w] = &window{page: page, layout: l, w: Width, h: Height, bounds: engine.Rect{X: 80, Y: 60, W: Width, H: Height}}
	e.mu.Unlock()
	if e.listener != nil {
		e.Post(func() { e.reportWindow(w) })
	}
}

// SetWindowPage loads another page in the window. Its dev window loads the shell its state
// names.
func (e *Engine) SetWindowPage(w engine.WindowID, page engine.Page) {
	e.mu.Lock()
	e.windows[w].page = page
	e.mu.Unlock()
	e.Feeds(w).Window.Publish(e.WindowState(w))
}

func (e *Engine) CreateTab(workspace string) engine.TabID {
	e.nextTab++
	e.tabs[e.nextTab] = &tab{pos: -1, state: "empty", workspace: workspace}
	return e.nextTab
}

func (e *Engine) CreateInternalTab() engine.TabID {
	id := e.CreateTab("")
	e.tabs[id].internal = true
	return id
}

// Internal says whether a tab is one of ours. Main thread.
func (e *Engine) Internal(id engine.TabID) bool {
	t := e.tabs[id]
	return t != nil && t.internal
}

// Workspace is the workspace a tab is in, "" for the profile's own storage. Main thread.
func (e *Engine) Workspace(id engine.TabID) string {
	if t := e.tabs[id]; t != nil {
		return t.workspace
	}
	return ""
}

func (e *Engine) SetFirstWorkspace(workspace string) { e.first = workspace }

// FirstWorkspace is the workspace the app last said is first. Main thread.
func (e *Engine) FirstWorkspace() string { return e.first }

// Erase is one erase the app asked for: the workspace, and how many of its tabs were still
// open then, which the app closes first.
type Erase struct {
	Workspace string
	Open      int
}

func (e *Engine) EraseWorkspace(workspace string) {
	open := 0
	for _, t := range e.tabs {
		if t.workspace == workspace && !t.internal {
			open++
		}
	}
	e.erased = append(e.erased, Erase{workspace, open})
}

// Erased are the erases the app asked for, in order. Main thread.
func (e *Engine) Erased() []Erase { return slices.Clone(e.erased) }

func (e *Engine) CloseTab(id engine.TabID) {
	e.dropDialogs(id)
	delete(e.tabs, id)
	for w, win := range e.windows {
		for slot, v := range win.visible {
			if v == id {
				e.publishPage(w, slot, 0, "none", "")
			}
		}
		if e.letGo(win, id) {
			e.Feeds(w).Window.Publish(e.WindowState(w))
		}
	}
}

// letGo takes one of our tabs out of the window's, and says whether it was there.
func (e *Engine) letGo(win *window, id engine.TabID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := len(win.ours)
	win.ours = slices.DeleteFunc(win.ours, func(o our) bool { return o.Tab == id })
	return len(win.ours) != n
}

func (e *Engine) LoadURL(id engine.TabID, u string) { e.navigate(id, u) }

// Loaded is the address a tab's page last committed, "" for none. Main thread.
func (e *Engine) Loaded(id engine.TabID) string {
	if t := e.tabs[id]; t != nil {
		return t.current
	}
	return ""
}

func (e *Engine) GoBack(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.pos > 0 {
		e.moveTo(id, t.pos-1)
	}
}

func (e *Engine) GoForward(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.pos < len(t.history)-1 {
		e.moveTo(id, t.pos+1)
	}
}

func (e *Engine) Reload(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.pos >= 0 {
		e.moveTo(id, t.pos)
	}
}

// RequestWindow does what the real window does with each request, as far as what it
// reports goes, and records every request. Closing says the window closed.
func (e *Engine) RequestWindow(w engine.WindowID, r engine.WindowRequest) {
	e.mu.Lock()
	e.requests = append(e.requests, r)
	e.mu.Unlock()
	switch r {
	case engine.WindowClose:
		// Like the real one, it closes on a later turn, after the call that asked.
		e.Post(func() {
			for slot := range max(1, len(e.windows[w].visible)) {
				e.publishPage(w, slot, 0, "closed", "")
			}
			e.mu.Lock()
			delete(e.windows, w)
			e.closed = append(e.closed, w)
			e.mu.Unlock()
			e.listener.WindowClosed(w)
		})
	case engine.WindowFullScreen:
		e.windows[w].full = !e.windows[w].full
		e.reportWindow(w)
	case engine.WindowMinimise:
		e.windows[w].minimised = true
		e.reportWindow(w)
	case engine.WindowZoom:
		e.windows[w].zoomed = !e.windows[w].zoomed
		e.reportWindow(w)
	case engine.WindowFront:
		e.windows[w].minimised = false
		e.reportWindow(w)
	}
}

// SetWindowMode puts the window into a mode, as the real one does, and reports it.
func (e *Engine) SetWindowMode(w engine.WindowID, m engine.WindowMode) {
	e.mu.Lock()
	e.modes = append(e.modes, m)
	e.mu.Unlock()
	win := e.windows[w]
	win.full, win.minimised, win.zoomed = m == engine.ModeFullScreen, m == engine.ModeMinimised, m == engine.ModeZoomed
	e.reportWindow(w)
}

// SetWindowBounds moves and sizes the window, as the real one does, and reports it.
func (e *Engine) SetWindowBounds(w engine.WindowID, r engine.Rect) {
	e.windows[w].bounds = r
	e.reportWindow(w)
}

// The real window puts each visible tab's view where the layout says, and tells a tab
// that leaves the list it is hidden. The mock says where each goes on the window's Window
// feed, and its dev window puts a mock page there for each, by slot: a slot that shows
// another tab now hears what that tab shows.
func (e *Engine) SetWindowLayout(w engine.WindowID, l engine.WindowLayout) {
	win := e.windows[w]
	var visible []engine.TabID
	for _, v := range l.Tabs {
		if e.tabs[v.Tab] != nil {
			visible = append(visible, v.Tab)
		}
	}
	// Our tabs it shows are its now, and no other window's.
	for _, id := range visible {
		if t := e.tabs[id]; t.internal && !slices.ContainsFunc(win.ours, func(o our) bool { return o.Tab == id }) {
			for other, o := range e.windows {
				if other != w && e.letGo(o, id) {
					e.Feeds(other).Window.Publish(e.WindowState(other))
				}
			}
			e.mu.Lock()
			win.ours = append(win.ours, our{id, t.page})
			e.mu.Unlock()
		}
	}
	e.mu.Lock()
	win.layout = l
	win.hidden = win.hidden[:0]
	for _, id := range win.visible {
		if !slices.Contains(visible, id) && e.tabs[id] != nil && !e.shownElsewhere(w, id) {
			win.hidden = append(win.hidden, id)
		}
	}
	old := win.visible
	win.visible = visible
	e.mu.Unlock()
	e.Feeds(w).Window.Publish(e.WindowState(w))
	for slot, id := range visible {
		if slot >= len(old) || old[slot] != id {
			t := e.tabs[id]
			e.publishPage(w, slot, id, t.state, t.current)
		}
	}
}

// shown is the tab in window w's slot, or 0.
func (e *Engine) shown(w engine.WindowID, slot int) engine.TabID {
	if win := e.windows[w]; win != nil && slot < len(win.visible) {
		return win.visible[slot]
	}
	return 0
}

// shownElsewhere says whether a window other than w shows a tab: one that moved there,
// which the real engine doesn't tell it is hidden as it leaves w.
func (e *Engine) shownElsewhere(w engine.WindowID, id engine.TabID) bool {
	for other, win := range e.windows {
		if other != w && slices.Contains(win.visible, id) {
			return true
		}
	}
	return false
}

// Page is where a layout puts the first visible tab's page, or nothing.
func Page(l engine.WindowLayout) engine.Rect {
	if len(l.Tabs) == 0 {
		return engine.Rect{}
	}
	return l.Tabs[0].Rect
}

// Layout is the last layout the app gave window w, or none once it closed.
func (e *Engine) Layout(w engine.WindowID) engine.WindowLayout {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return win.layout
	}
	return engine.WindowLayout{}
}

// Open says whether window w is open. Main thread.
func (e *Engine) Open(w engine.WindowID) bool { return e.windows[w] != nil }

// Windows are the windows open now.
func (e *Engine) Windows() []engine.WindowID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Sorted(maps.Keys(e.windows))
}

// Requests are the window requests the app made, in order.
func (e *Engine) Requests() []engine.WindowRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.WindowRequest(nil), e.requests...)
}

// Modes are the modes the app asked windows into, in order.
func (e *Engine) Modes() []engine.WindowMode {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.WindowMode(nil), e.modes...)
}

// ShownColor is the colour the first window painted when it went on screen.
func (e *Engine) ShownColor() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.shownColor
}

func (e *Engine) ReloadBypassingCache(id engine.TabID) { e.Reload(id) }

func (e *Engine) Stop(id engine.TabID) {
	if t := e.tabs[id]; t != nil && t.state == "loading" {
		t.navSeq++
		t.state = "loaded"
		e.listener.LoadingChanged(id, false)
	}
}

func (e *Engine) Zoom(id engine.TabID, factor float64) {
	if t := e.tabs[id]; t != nil {
		t.zoom = factor
	}
}

func (e *Engine) ZoomOf(id engine.TabID) float64 {
	if t := e.tabs[id]; t != nil && t.zoom != 0 {
		return t.zoom
	}
	return 1
}

// DuplicateTab copies the tab's history into a new hidden tab, of its kind and in its
// workspace.
func (e *Engine) DuplicateTab(id engine.TabID) engine.TabID {
	n := e.CreateTab(e.Workspace(id))
	if t := e.tabs[id]; t != nil {
		c := *t
		c.history = append([]string(nil), t.history...)
		e.tabs[n] = &c
	}
	return n
}

func (e *Engine) Edit(c engine.EditCommand) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.edits = append(e.edits, c)
}

func (e *Engine) WriteClipboard(text string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clipboard = text
}

// Clipboard is the text the app last put on the clipboard.
func (e *Engine) Clipboard() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.clipboard
}

// FileChoosers are the app's answers to the pages' file panels, in order.
func (e *Engine) FileChoosers() []engine.FileChooserAnswer {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.FileChooserAnswer(nil), e.chooser...)
}

// Edits are the editing commands the app ran, in order.
func (e *Engine) Edits() []engine.EditCommand {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.EditCommand(nil), e.edits...)
}

// WindowState is what window w holds besides the shell, and how it draws and hit-tests,
// as its Window feed sends it:
//
//	{"t":"window", "shell":the id of the shell the window loads,
//	 "pages":[{"tab":id,"rect":{x,y,w,h},"page":one of ours}] where each tab's page goes,
//	  by slot, page only for a tab of ours, whose page shows there in place of a mock one,
//	 "ours":[{"tab":id,"page":"settings"}] our tabs whose page the window holds,
//	 "regions":[{x,y,w,h,"area":"drag"|"shell"}],
//	 "frameColor":"#rrggbb", "frameWidth":px, "controls":the window buttons or null,
//	 "cornerRadius":px, "popup":the extension popup or null,
//	 "showRegions":whether to outline every rectangle the window manager tracks}
//
// The rectangles are the layout's, in the shell's coordinates, which on
// the mock are the dev window's. The regions are in order: the first that holds a point
// decides. A window that closed holds nothing. Any goroutine.
func (e *Engine) WindowState(w engine.WindowID) []byte {
	type region struct {
		engine.Rect
		Area string `json:"area"`
	}
	type shown struct {
		engine.VisibleTab
		Page string `json:"page,omitempty"`
	}
	e.mu.Lock()
	var l engine.WindowLayout
	var s engine.WindowState
	var page engine.Page
	pages := []shown{}
	ours := []our{}
	if win := e.windows[w]; win != nil {
		l, s, page, ours = win.layout, win.reported, win.page, append(ours, win.ours...)
		for _, v := range l.Tabs {
			if slices.Contains(win.visible, v.Tab) {
				p := shown{VisibleTab: v}
				if i := slices.IndexFunc(win.ours, func(o our) bool { return o.Tab == v.Tab }); i >= 0 {
					p.Page = win.ours[i].Page
				}
				pages = append(pages, p)
			}
		}
	}
	e.mu.Unlock()
	regions := []region{}
	for _, r := range l.Regions {
		regions = append(regions, region{r.Rect, AreaNames[r.Area]})
	}
	var controls *engine.Rect
	if s.Controls != (engine.Rect{}) {
		controls = &s.Controls
	}
	b, _ := json.Marshal(map[string]any{"t": "window", "shell": page.Shell(), "pages": pages, "ours": ours, "regions": regions,
		"frameColor": l.FrameColor, "frameWidth": l.FrameWidth, "controls": controls, "cornerRadius": s.CornerRadius,
		"popup": e.openPopup(w), "showRegions": e.showRegions})
	return b
}

// AreaNames are what the dev window and its tests call each area.
var AreaNames = [...]string{engine.AreaPage: "page", engine.AreaShell: "shell", engine.AreaDrag: "drag"}

func (e *Engine) SendToPage(tab engine.TabID, msg []byte) { e.PageFeed(tab).Publish(msg) }

func (e *Engine) SetMenu(m engine.Menu) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.menu = m
}

// PageOf is which page covers window w, or "" for one that isn't open.
func (e *Engine) PageOf(w engine.WindowID) engine.Page {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return win.page
	}
	return ""
}

// Menu is the last menu the app gave the window.
func (e *Engine) Menu() engine.Menu {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.menu
}

// AppCommand records a quit; the rest are the OS's.
func (e *Engine) SetDockColor(color string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dockColor = color
}

// DockColor is the colour the app last put on its Dock icon, or "".
func (e *Engine) DockColor() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.dockColor
}

func (e *Engine) AppCommand(c engine.AppCommand) {
	if c == engine.AppQuit {
		e.mu.Lock()
		e.quit = true
		e.mu.Unlock()
	}
}

// FocusTab gives the tab the keyboard, which the mock only remembers. The real window
// gives it only to a tab it shows: a page view it hides can't take it, and the keyboard
// is left where no key reaches Go. So a tab no window shows doesn't get it here either,
// and the mock remembers that it was asked.
func (e *Engine) FocusTab(tab engine.TabID) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.tabs[tab] != nil {
		shown := false
		for _, win := range e.windows {
			shown = shown || slices.Contains(win.visible, tab)
		}
		if !shown {
			e.unshownFocus = append(e.unshownFocus, tab)
			return
		}
	}
	e.focused = tab
}

// UnshownFocus are the tabs FocusTab was asked for while no window showed them.
func (e *Engine) UnshownFocus() []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]engine.TabID{}, e.unshownFocus...)
}

// Focused is the tab last given the keyboard, by FocusTab or a press in its page.
func (e *Engine) Focused() engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.focused
}

// FocusShell gives window w's shell the keyboard: the dev window focuses the shell's
// frame, as the real window gives the shell's view focus.
func (e *Engine) FocusShell(w engine.WindowID) {
	e.Feeds(w).Window.Publish([]byte(`{"t":"focusShell"}`))
}

func (e *Engine) SendToShell(w engine.WindowID, msg []byte) { e.Feeds(w).Shell.Publish(msg) }

func (e *Engine) ShareFile(w engine.WindowID, path string, anchor engine.Rect) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.shared = path
}

// Shared is the file ShareFile last offered, or "".
func (e *Engine) Shared() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.shared
}

// Left is whether the back key left the app, with no tab to go back in or close.
func (e *Engine) Left() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.left
}

func (e *Engine) SetShellBack(w engine.WindowID, on bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		win.shellBack = on
	}
}

func (e *Engine) Post(fn func()) { e.queue <- fn }

// Settle returns once the engine has run everything posted to it so far, and everything
// that posted in turn: the queue has drained. Any goroutine but the main thread.
func (e *Engine) Settle() {
	done := make(chan struct{})
	var check func()
	check = func() {
		if len(e.queue) > 0 {
			e.Post(check)
			return
		}
		close(done)
	}
	e.Post(check)
	<-done
}

// Closed reports whether window w closed.
func (e *Engine) Closed(w engine.WindowID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Contains(e.closed, w)
}

// Quit reports whether the app quit.
func (e *Engine) Quit() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.quit
}

// Hidden returns the tabs in window w told they are hidden by its last layout.
func (e *Engine) Hidden(w engine.WindowID) []engine.TabID {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil {
		return append([]engine.TabID(nil), win.hidden...)
	}
	return nil
}

// Fire makes the engine do what a button on a mock page of window w says, to the tab the
// page shows, or to the first tab the window shows for 0. Any goroutine.
func (e *Engine) Fire(w engine.WindowID, tab engine.TabID, kind, value string) {
	e.Post(func() {
		win := e.windows[w]
		if win == nil {
			return
		}
		shown := tab
		if shown == 0 {
			shown = e.shown(w, 0)
		}
		t := e.tabs[shown]
		switch kind {
		case "navigate":
			e.navigate(shown, value)
		case "title":
			e.listener.TitleChanged(shown, value)
		case "backKey":
			// The phone's back key, as Android's window takes it: the shell's own, then the
			// page's history, then the tab itself, the one used before it taking its place.
			// With no tab shown it is Android's: the app goes.
			switch {
			case win.shellBack:
				e.listener.ShellMessage(w, []byte(`{"t":"backKey"}`))
			case len(win.visible) == 0:
				e.mu.Lock()
				e.left = true
				e.mu.Unlock()
			case t != nil && t.pos > 0:
				e.listener.ShellMessage(w, []byte(`{"t":"back"}`))
			default:
				e.listener.ShellMessage(w, []byte(`{"t":"closeTab","then":"recent"}`))
			}
		case "contextMenu":
			// A right-click or a long press, as content reports it: on a link to value, or
			// on what value says as JSON, {"link", "src", "media", "selection", "editable", ...}.
			m := engine.ContextMenu{X: 40, Y: 60, Link: value, LinkText: "a link"}
			if strings.HasPrefix(value, "{") {
				m = engine.ContextMenu{X: 40, Y: 60}
				json.Unmarshal([]byte(value), &m)
			}
			e.listener.ContextMenu(shown, m)
		case "pageFullscreen":
			// The page asks to fill the screen, as a video's button does.
			e.listener.PageFullScreen(shown, true)
		case "alert", "confirm", "prompt", "beforeunload", "signin":
			// The page asks; value is its words.
			e.openDialog(shown, kind, value)
		case "chooseFile":
			// The page asks for a file; the user picks one in folder value, or cancels
			// with "".
			a := e.listener.FileChooser(engine.FileChooserRequest{Tab: shown, Mode: engine.FileChooserOpen})
			e.mu.Lock()
			e.chooser = append(e.chooser, a)
			e.mu.Unlock()
			if a.Allow && value != "" {
				e.listener.FilesChosen(shown, 1, value)
			}
		case "download":
			// The page starts a download of value.
			e.download(shown, value, false)
		case "notification":
			// The page shows a notification, its id in value.
			e.showNotification(shown, value)
		case "notificationClick":
			// The person clicks the notification value on the system.
			e.listener.NotificationClicked(value)
		case "permission", "permissionQuiet":
			// The page asks for the kinds in value, from a press, or on its own.
			e.requestPermission(shown, value, kind == "permission")
		case "scroll":
			// value is down or up, as a page that scrolls makes content say.
			e.listener.ScrollDirectionChanged(shown, value == "down")
		case "certificate":
			// The page goes to value, whose host has an expired certificate from now on.
			if e.badCerts == nil {
				e.badCerts = map[string]bool{}
			}
			e.badCerts[hostOf(value)] = true
			e.navigate(shown, value)
		case "fail":
			// value is why, as engine.Failure says it, then the error's own name.
			if t != nil {
				t.navSeq++
				why, code, _ := strings.Cut(value, " ")
				e.listener.LoadingChanged(shown, false)
				e.listener.NavigationFailed(shown, t.nav, t.current, engine.Failure(why), code)
			}
		case "crash":
			if t != nil {
				t.navSeq++
				t.state = "crashed"
				e.listener.PageCrashed(shown)
				e.showPage(shown, t.state, t.current)
			}
		case "favicon":
			e.listener.FaviconChanged(shown, value)
		case "focusTab":
			// A press in the page, which takes the keyboard, as a click does in the real
			// window's.
			if t != nil {
				e.FocusTab(shown)
				e.listener.TabFocused(shown)
			}
		case "menu":
			// A menu bar item chosen, by its action id, as the real menu bar says.
			e.listener.MenuChosen(value)
		case "open":
			// What a page's target=_blank or window.open does: the engine makes the
			// tab, in the page's workspace, tells the app, then loads it.
			id := e.CreateTab(e.Workspace(shown))
			e.listener.TabOpened(shown, id, value, e.Workspace(shown), true, false)
			e.navigate(id, value)
		case "key":
			// A key pressed in the page, which uses none of them.
			if k, ok := Keys[value]; ok && !e.listener.Key(k, engine.KeyBeforePage, engine.SurfacePage) {
				e.listener.Key(k, engine.KeyUnhandled, engine.SurfacePage)
			}
		case "release":
			// Every modifier let go.
			e.listener.ModsChanged(0)
		case "fullscreen":
			// The system's own button, which asks no one.
			win.full = !win.full
			e.reportWindow(w)
		case "focus":
			// The window comes to the front, as a click on it does.
			e.listener.WindowFocused(w)
		case "close":
			// Its own close button, which asks no one.
			e.mu.Lock()
			delete(e.windows, w)
			e.closed = append(e.closed, w)
			e.mu.Unlock()
			e.listener.WindowClosed(w)
		}
	})
}

// navigate is a new page: it replaces whatever was ahead of the current one.
func (e *Engine) navigate(id engine.TabID, u string) {
	e.load(id, u, func(t *tab) {
		t.history = append(t.history[:t.pos+1], u)
		t.pos = len(t.history) - 1
	})
}

// moveTo loads a page already in the history.
func (e *Engine) moveTo(id engine.TabID, pos int) {
	e.load(id, e.tabs[id].history[pos], func(t *tab) { t.pos = pos })
}

// load behaves like a real load: gates first, then later the commit, unless a newer
// navigation replaced it.
func (e *Engine) load(id engine.TabID, u string, commit func(*tab)) {
	t := e.tabs[id]
	if t == nil {
		return
	}
	if e.listener != nil {
		e.dropDialogs(id)
	}
	// A load asked for while the app is still starting begins once it listens.
	if e.listener == nil {
		e.Post(func() { e.load(id, u, commit) })
		return
	}
	e.nextNav++
	nav := e.nextNav
	switch e.listener.NavigationStarting(id, nav, u) {
	case engine.NavigationElsewhere:
		// The app took the address to another tab: this one stays as it was.
		return
	case engine.NavigationBlock:
		t.navSeq++
		t.nav = nav
		t.state = "blocked"
		e.showPage(id, t.state, u)
		return
	}
	t.navSeq++
	seq := t.navSeq
	t.nav = nav
	if t.internal {
		t.page = hostOf(u)
	}
	e.listener.LoadingChanged(id, true)
	e.listener.LoadProgressChanged(id, 0.5)
	t.state = "loading"
	e.showPage(id, t.state, u)
	time.AfterFunc(loadDelay, func() {
		e.Post(func() {
			if e.tabs[id] != t || seq != t.navSeq {
				return
			}
			// A host the test gave a bad certificate: the app says whether it loads. Refused,
			// the address is the tab's entry all the same, as content's error page is, so a
			// reload asks again.
			insecure := false
			if e.badCerts[hostOf(u)] {
				if !e.listener.CertificateError(id, engine.CertificateError{URL: u, Reason: "expired", MainFrame: true, Passable: true}) {
					commit(t)
					t.current = u
					t.state = "failed"
					e.listener.HistoryChanged(id, t.pos > 0, t.pos < len(t.history)-1)
					e.listener.LoadingChanged(id, false)
					e.listener.NavigationFailed(id, nav, u, engine.FailTLS, "ERR_CERT_DATE_INVALID")
					e.showPage(id, t.state, u)
					return
				}
				insecure = true
			}
			commit(t)
			t.current = u
			t.state = "loaded"
			e.listener.HistoryChanged(id, t.pos > 0, t.pos < len(t.history)-1)
			e.listener.NavigationFinished(id, nav, u)
			if insecure {
				e.listener.ConnectionChanged(id, engine.Connection{State: engine.ConnectionInsecure, Reason: "certificate"})
			} else {
				e.listener.ConnectionChanged(id, connectionOf(u))
			}
			// Our page is the real one, which gives its own title (core/cmd/dev).
			if !t.internal {
				e.listener.TitleChanged(id, titleOf(u))
			}
			e.listener.LoadProgressChanged(id, 1)
			e.listener.LoadingChanged(id, false)
			e.showPage(id, t.state, u)
		})
	})
}

// publishPage tells the mock page in window w's slot what to show: a tab, or 0 for none.
// The window keeps it for a page that connects later.
func (e *Engine) publishPage(w engine.WindowID, slot int, id engine.TabID, state, u string) {
	b, _ := json.Marshal(map[string]any{"slot": slot, "tab": id, "state": state, "url": u})
	if win := e.windows[w]; win != nil {
		e.mu.Lock()
		for len(win.pages) <= slot {
			win.pages = append(win.pages, nil)
		}
		win.pages[slot] = b
		e.mu.Unlock()
	}
	e.Feeds(w).Page.Publish(b)
}

// PageState is the last message for the mock page in window w's slot, or nil before any.
// Any goroutine.
func (e *Engine) PageState(w engine.WindowID, slot int) []byte {
	e.mu.Lock()
	defer e.mu.Unlock()
	if win := e.windows[w]; win != nil && slot < len(win.pages) {
		return win.pages[slot]
	}
	return nil
}

// showPage tells the mock page of each slot that shows a tab what it shows now.
func (e *Engine) showPage(id engine.TabID, state, u string) {
	for w, win := range e.windows {
		for slot, v := range win.visible {
			if v == id {
				e.publishPage(w, slot, id, state, u)
			}
		}
	}
}

// titleOf is a mock page's title. The blank page has none, so content titles it with
// its address, as it does any untitled page.
func titleOf(u string) string {
	if u == engine.BlankURL {
		return u
	}
	return "Mock " + hostOf(u)
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// Feed sends messages to whoever is connected right now.
type Feed struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

// Subscribe returns a channel of messages and a function to stop.
func (f *Feed) Subscribe() (<-chan []byte, func()) {
	c := make(chan []byte, 256)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.subs == nil {
		f.subs = map[chan []byte]struct{}{}
	}
	f.subs[c] = struct{}{}
	return c, func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.subs, c)
	}
}

// Publish sends to everyone connected and never blocks the main thread.
func (f *Feed) Publish(msg []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for c := range f.subs {
		select {
		case c <- msg:
		default:
		}
	}
}

// connectionOf is how the mock says a page at u reached it: https is secure, http isn't
// encrypted, and anything else isn't a site.
func connectionOf(u string) engine.Connection {
	switch {
	case strings.HasPrefix(u, "https://"):
		return engine.Connection{State: engine.ConnectionSecure}
	case strings.HasPrefix(u, "http://"):
		return engine.Connection{State: engine.ConnectionInsecure, Reason: "not-encrypted"}
	}
	return engine.Connection{}
}
