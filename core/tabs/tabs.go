// Package tabs owns the windows and the tabs: which windows exist, which is in front,
// each one's tabs in order and which of them is active, and each tab's state, the
// workspace it is in among it. Tab state changes only through this manager. A window's active tab is the one the user acts on
// there: its shell shows it and gives it the keyboard, and says where it put it in its
// layout, which shows it. A window always has a tab and an active one, and closes when its
// last tab goes (OnEmpty). Commands act on the active tab of the window in front. A tab
// keeps its id for its life.
//
// A tab is a web tab or one of ours, an internal tab: one of our own pages, such as
// settings, in our pages' own context. Its kind is fixed for life, and every manager and
// shell hears it in the tab's page (docs/windows.md, "Our own pages are tabs").
package tabs

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strings"

	"projectb/engine"
	"projectb/search"
)

// Tab is a copy of one tab's state. Subscribers can keep it.
type Tab struct {
	ID engine.TabID `json:"id"`
	// Page is the internal page one of our tabs shows, "settings", or "" for a web tab. It
	// is given when the tab is made and never changes.
	Page string `json:"page,omitempty"`
	// Workspace is the id of the workspace the tab is in: "" is the profile's own storage,
	// Main's. It is given when the tab is made and never changes, whichever window the tab
	// is in. One of our tabs is in no workspace: this is the one of the tab it was opened
	// from, where New Tab from it goes (HomeOf).
	Workspace string `json:"workspace"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Loading   bool   `json:"loading"`
	// Progress is how far the load has got, 0 to 1.
	Progress float64 `json:"progress"`
	Blocked  bool    `json:"blocked"`
	Crashed  bool    `json:"crashed"`
	// Error is why its page failed to load, one of engine.Failure's words, and ErrorCode
	// the engine's own name for it, only to show.
	Error     engine.Failure `json:"error,omitempty"`
	ErrorCode string         `json:"errorCode,omitempty"`
	// Favicon is a data: URL, or "" when the page has none.
	Favicon string `json:"favicon,omitempty"`
	// Thumb is the address of a picture of its page as it last looked, a small JPEG our
	// pages can load, or "" for none yet. A commit at another address drops it, as it is of
	// the page before; one that only changes the #fragment keeps it.
	Thumb string `json:"thumb,omitempty"`

	CanGoBack    bool `json:"canGoBack"`
	CanGoForward bool `json:"canGoForward"`
	// Connection is how its page reached it: "secure", "insecure", or "" for one that
	// isn't a site, as our own pages and the blank page aren't. ConnectionNote says why in
	// one line, when there is something to say (docs/daily-driver.md).
	Connection     string `json:"connection,omitempty"`
	ConnectionNote string `json:"connectionNote,omitempty"`
	// Notice is what a shell draws over its page while it has nothing to show (notice.go).
	Notice *Notice `json:"notice,omitempty"`
	// AdsBlocked is how many requests of its page the ad blocker kept from their hosts.
	AdsBlocked int `json:"adsBlocked,omitempty"`
	// PopupsBlocked is how many tabs its page opened that Links to other sites closed.
	PopupsBlocked int `json:"popupsBlocked,omitempty"`
	// Group is the id of the tab group it is in, "" for none (groups.go).
	Group string `json:"group,omitempty"`
	// Pin is the id of the pin the tab is open for, "" for none (package pins). A window
	// has one tab for a pin at most, and its pinned tabs come first in its order, in the
	// pins' order. A pinned tab is in no group.
	Pin string `json:"pin,omitempty"`

	// committed is the address of the page the tab last committed, which Thumb is of.
	committed string
	// started is the engine's id for the latest navigation that started, whose address
	// the tab shows until it commits or fails.
	started int64
	// The title and icon of the page shown as a navigation started, which it gets back when
	// the navigation ends with nothing shown instead (refused on its way, stopped, a
	// download).
	shownTitle, shownFavicon string
	// byPage says the page started the latest navigation (pagenav.go).
	byPage bool
	// The tab whose page opened this one, and its page then; ownPage says this tab has
	// had a page of its own since.
	opener    engine.TabID
	openerURL string
	ownPage   bool
	// refused: Links to other sites closes the tab, which nothing may load meanwhile.
	refused bool
}

// Gate runs before a navigation. Returning false blocks it, and later gates don't run.
type Gate func(url string) bool

// Pages are our pages a tab of ours can show, each at the engine's address for it.
var Pages = []string{"settings", "history", "bookmarks", "downloads", "welcome"}

// PageOf is which of Pages an address is, whichever of its sections it names, or "" for
// any other address.
func (m *Manager) PageOf(u string) string {
	p, ours := m.eng.OurPage(u)
	page, _, _ := strings.Cut(string(p), "/")
	if !ours || !slices.Contains(Pages, page) {
		return ""
	}
	return page
}

// PageURL is the address of one of Pages.
func (m *Manager) PageURL(page string) string { return m.eng.PageURL(engine.Page(page)) }

// Workspaces is what the manager reads of the profile's list of workspaces.
type Workspaces interface {
	// First is the first workspace in the list, where a tab opens when nothing picks one.
	First() string
	Has(id string) bool
}

// Pins is what the manager reads of the profile's pins.
type Pins interface {
	// Index is where a pin is in the profile's order of pins.
	Index(id string) int
}

// BlankURL is the empty page every engine has. A tab showing it reads as empty, as a new
// tab does: no address, titled New Tab, nothing in history.
const BlankURL = engine.BlankURL

// Manager owns the tabs.
type Manager struct {
	eng        engine.Engine
	own        own
	guard      func(PageNavigation) bool
	workspaces Workspaces
	pins       Pins
	// search turns words into a search's address.
	search   func(words string) string
	tabs     map[engine.TabID]*Tab
	windowOf map[engine.TabID]engine.WindowID
	windows  map[engine.WindowID]*window
	// used is every window, the one in front first, then in the order they were last in
	// front.
	used []engine.WindowID

	gates      []Gate
	onChanged  []func(Tab)
	onFinished []func(Tab)
	onTitle    []func(Tab)
	onList     []func()
	onClosed   []func(Tab)
	onEmpty    []func(w engine.WindowID)
	onFront    []func(engine.WindowID)
	onMoved    []func(tab engine.TabID, from engine.WindowID, fromIndex int, to engine.WindowID)
	onLeaving  []func(engine.TabID)
	// groupSeq numbers the groups, for their ids.
	groupSeq int
	// certificates is each tab's certificate error, until it navigates again; passed is
	// the hosts whose certificate errors the person chose to pass, until the app quits.
	certificates map[engine.TabID]*certificate
	passed       map[string]bool
	// waiting is each restored tab not shown yet, with the address it loads when it is
	// first made active. Until then it has its address and title, and the blank page.
	waiting map[engine.TabID]string
	// onClosedAt hear of a tab the person closed: as it was, its window and its place.
	onClosedAt []func(t Tab, w engine.WindowID, index int)
}

// window is one window's tabs: their order, the active one, and recent use.
type window struct {
	order  []engine.TabID
	active engine.TabID
	// recent is every tab, the most recently shown first. While a cycle through it runs,
	// cycle is the order it started with and at is where it has got to.
	recent []engine.TabID
	cycle  []engine.TabID
	at     int
	// groups are its tab groups, in the order they were made.
	groups []*Group
}

// New returns a manager with no windows, on the profile's workspaces and pins.
func New(eng engine.Engine, w Workspaces, p Pins) *Manager {
	o := own{Engine: eng, issued: map[engine.TabID]string{}}
	return &Manager{eng: o, own: o, workspaces: w, pins: p, search: defaultEngine().For, tabs: map[engine.TabID]*Tab{}, windowOf: map[engine.TabID]engine.WindowID{}, windows: map[engine.WindowID]*window{}, certificates: map[engine.TabID]*certificate{}, passed: map[string]bool{}, waiting: map[engine.TabID]string{}}
}

// defaultEngine is the engine a manager searches with until SetSearch says another.
func defaultEngine() search.Engine {
	e, _ := search.Get(search.Default)
	return e
}

// AddGate adds a gate. Gates run in the order they were added.
func (m *Manager) AddGate(g Gate) { m.gates = append(m.gates, g) }

// OnChanged subscribes to any change in a tab's state.
func (m *Manager) OnChanged(f func(Tab)) { m.onChanged = append(m.onChanged, f) }

// OnNavigationFinished subscribes to committed navigations.
func (m *Manager) OnNavigationFinished(f func(Tab)) { m.onFinished = append(m.onFinished, f) }

// OnTitleChanged subscribes to title changes.
func (m *Manager) OnTitleChanged(f func(Tab)) { m.onTitle = append(m.onTitle, f) }

// OnListChanged subscribes to a tab opening or closing, or a window's active tab changing.
func (m *Manager) OnListChanged(f func()) { m.onList = append(m.onList, f) }

// OnClosed subscribes to a tab closing, on its own or with its window.
func (m *Manager) OnClosed(f func(Tab)) { m.onClosed = append(m.onClosed, f) }

// OnEmpty subscribes to a window's last tab leaving it, closed or moved to another window,
// for the window to close: a window always has a tab.
func (m *Manager) OnEmpty(f func(w engine.WindowID)) { m.onEmpty = append(m.onEmpty, f) }

// OnLeaving subscribes to a window's active tab just before another becomes active there,
// while its page still shows. Not for a tab that closed or moved away.
func (m *Manager) OnLeaving(f func(engine.TabID)) { m.onLeaving = append(m.onLeaving, f) }

// OnFrontChanged subscribes to another window coming to the front.
func (m *Manager) OnFrontChanged(f func(engine.WindowID)) { m.onFront = append(m.onFront, f) }

// OnMoved subscribes to a tab moving from its place in one window to another window.
func (m *Manager) OnMoved(f func(tab engine.TabID, from engine.WindowID, fromIndex int, to engine.WindowID)) {
	m.onMoved = append(m.onMoved, f)
}

// AddWindow adds a window with no tabs. It is the window in front.
func (m *Manager) AddWindow(w engine.WindowID) {
	m.windows[w] = &window{}
	m.toFront(w)
}

// toFront puts a window first in the order of use, and says so when it wasn't.
func (m *Manager) toFront(w engine.WindowID) {
	was := m.Front()
	m.used = slices.Insert(slices.DeleteFunc(m.used, func(u engine.WindowID) bool { return u == w }), 0, w)
	m.frontMaybeChanged(was)
}

func (m *Manager) frontMaybeChanged(was engine.WindowID) {
	if now := m.Front(); now != was && now != 0 {
		for _, f := range m.onFront {
			f(now)
		}
	}
}

// RemoveWindow closes a window's tabs and forgets it. The window used before it is in
// front now.
func (m *Manager) RemoveWindow(w engine.WindowID) {
	for _, id := range m.windows[w].order {
		t := *m.tabs[id]
		delete(m.tabs, id)
		delete(m.certificates, id)
		delete(m.waiting, id)
		delete(m.windowOf, id)
		m.eng.CloseTab(id)
		m.closed(t)
	}
	delete(m.windows, w)
	was := m.Front()
	m.used = slices.DeleteFunc(m.used, func(u engine.WindowID) bool { return u == w })
	m.listChanged()
	m.frontMaybeChanged(was)
}

// Focus says a window came to the front.
func (m *Manager) Focus(w engine.WindowID) {
	if m.windows[w] != nil {
		m.toFront(w)
	}
}

// Front is the window in front, the one used last, or 0 when there is none.
func (m *Manager) Front() engine.WindowID {
	if len(m.used) == 0 {
		return 0
	}
	return m.used[0]
}

// Windows returns the windows, the one in front first.
func (m *Manager) Windows() []engine.WindowID { return slices.Clone(m.used) }

// WindowOf is the window a tab is in, or 0 for no tab.
func (m *Manager) WindowOf(id engine.TabID) engine.WindowID { return m.windowOf[id] }

// front is the window in front's tabs.
func (m *Manager) front() *window { return m.windows[m.Front()] }

// Create is New Tab: it opens a new tab at the end of window w, loading nothing, in the
// workspace New Tab takes there (NewTabWorkspace), and makes it active.
func (m *Manager) Create(w engine.WindowID) engine.TabID {
	return m.CreateIn(w, m.NewTabWorkspace(w))
}

// NewTabWorkspace is the workspace a new tab in window w opens in: that of w's active tab,
// or the first when w has none.
func (m *Manager) NewTabWorkspace(w engine.WindowID) string {
	if t := m.tabs[m.ActiveIn(w)]; t != nil {
		return m.HomeOf(t.ID)
	}
	return m.workspaces.First()
}

// HomeOf is the workspace a tab's new tabs go in: the one it is in, or for one of ours the
// one it was opened from, or the first when that one is gone.
func (m *Manager) HomeOf(id engine.TabID) string {
	t := m.tabs[id]
	if t.Page != "" && !m.workspaces.Has(t.Workspace) {
		return m.workspaces.First()
	}
	return t.Workspace
}

// FirstWorkspace is the first workspace in the profile's list, where a tab opens when
// nothing picks one.
func (m *Manager) FirstWorkspace() string { return m.workspaces.First() }

// PageIn is window w's tab of one of our pages, or 0 when it has none.
func (m *Manager) PageIn(w engine.WindowID, page string) engine.TabID {
	for _, id := range m.windows[w].order {
		if m.tabs[id].Page == page {
			return id
		}
	}
	return 0
}

// OpenPage shows one of our pages in window w, which has one tab for each: its tab, or a
// new one beside the active tab, opened from it. Either is made active.
func (m *Manager) OpenPage(w engine.WindowID, page string) engine.TabID {
	if id := m.PageIn(w, page); id != 0 {
		m.Activate(id)
		return id
	}
	workspace, active := m.NewTabWorkspace(w), m.ActiveIn(w)
	id := m.eng.CreateInternalTab()
	t := m.add(id, w)
	t.Page, t.Workspace, t.URL = page, workspace, m.PageURL(page)
	if active != 0 {
		m.beside(id, active)
	}
	m.eng.LoadURL(id, t.URL)
	m.Activate(id)
	return id
}

// CreateIn opens a new tab at the end of window w, loading nothing, in a workspace, by
// its id, and makes it active.
func (m *Manager) CreateIn(w engine.WindowID, workspace string) engine.TabID {
	return m.Add(w, workspace, "", true)
}

// Add opens a new tab at the end of window w, in a workspace, at u, an address or a search,
// or loading nothing for "", and makes it active when foreground, or leaves it behind the
// active one.
func (m *Manager) Add(w engine.WindowID, workspace, u string, foreground bool) engine.TabID {
	id := m.create(w, workspace, u == "")
	m.shown(id, foreground)
	if u != "" {
		m.eng.LoadURL(id, u)
	}
	return id
}

// OnClosedAt subscribes to a tab closing on its own, not with its window: the tab as it
// was, group and all, its window and its place there.
func (m *Manager) OnClosedAt(f func(t Tab, w engine.WindowID, index int)) {
	m.onClosedAt = append(m.onClosedAt, f)
}

// Restore opens a tab the profile had before, at the end of window w, behind the active
// one, with its address and title. It loads now when load, else once first made active.
func (m *Manager) Restore(w engine.WindowID, workspace, u, title string, load bool) engine.TabID {
	id := m.create(w, workspace, !load)
	t := m.tabs[id]
	t.URL, t.Title = u, title
	if load {
		m.eng.LoadURL(id, u)
	} else {
		m.waiting[id] = u
	}
	m.shown(id, false)
	return id
}

// shown makes a new tab active, or tells of it behind the active one. In a window with
// no tabs before it, it is the active one either way.
func (m *Manager) shown(id engine.TabID, foreground bool) {
	if foreground || m.windows[m.windowOf[id]].active == 0 {
		m.Activate(id)
	} else {
		m.listChanged()
	}
}

// create opens a new tab at the end of window w, in a workspace. It loads nothing; an empty
// one, which nothing loads into yet, starts no renderer until it does, and the shell draws a
// new tab where its page goes.
func (m *Manager) create(w engine.WindowID, workspace string, empty bool) engine.TabID {
	id := m.eng.CreateTab(workspace, empty)
	m.add(id, w).Workspace = workspace
	return id
}

// address is what a tab shows for a URL: nothing for the blank page.
func address(u string) string {
	if u == BlankURL {
		return ""
	}
	return u
}

func (m *Manager) add(id engine.TabID, w engine.WindowID) *Tab {
	t := &Tab{ID: id}
	m.tabs[id] = t
	m.windowOf[id] = w
	win := m.windows[w]
	win.order = append(win.order, id)
	win.recent = append(win.recent, id)
	return t
}

// Activate makes a tab the active one in its window.
func (m *Manager) Activate(id engine.TabID) {
	if m.tabs[id] == nil {
		return
	}
	win := m.windows[m.windowOf[id]]
	if id == win.active {
		return
	}
	if win.active != 0 {
		for _, f := range m.onLeaving {
			f(win.active)
		}
	}
	win.active = id
	if win.cycle == nil {
		win.touch(id)
	}
	if u, ok := m.waiting[id]; ok {
		delete(m.waiting, id)
		m.eng.LoadURL(id, u)
	}
	m.listChanged()
}

// touch makes a tab the most recent.
func (win *window) touch(id engine.TabID) {
	if i := slices.Index(win.recent, id); i >= 0 {
		win.recent = slices.Delete(win.recent, i, i+1)
	}
	win.recent = slices.Insert(win.recent, 0, id)
}

// CycleRecent shows the next tab back in recent order in the window in front, as the
// OS's app switcher does while its key is held: each press goes one further, past the
// oldest it comes round to the newest, and the order stays as it was until EndCycle.
// Reverse goes the other way.
func (m *Manager) CycleRecent(reverse bool) {
	win := m.front()
	if len(win.recent) < 2 {
		return
	}
	if win.cycle == nil {
		win.cycle = slices.Clone(win.recent)
		win.at = 0
	}
	n := len(win.cycle)
	if reverse {
		win.at = (win.at + n - 1) % n
	} else {
		win.at = (win.at + 1) % n
	}
	m.Activate(win.cycle[win.at])
}

// EndCycle ends a cycle through recent tabs: the tab it stopped on becomes the most recent.
func (m *Manager) EndCycle() {
	for _, win := range m.windows {
		win.endCycle()
	}
}

func (win *window) endCycle() {
	if win.cycle == nil {
		return
	}
	win.cycle = nil
	win.touch(win.active)
}

// ActivateAt shows the tab at a position in the window in front, counting from zero. Out
// of range does nothing.
func (m *Manager) ActivateAt(i int) {
	if win := m.front(); i >= 0 && i < len(win.order) {
		m.Activate(win.order[i])
	}
}

// ActivateLast shows the last tab of the window in front.
func (m *Manager) ActivateLast() { m.ActivateAt(len(m.front().order) - 1) }

// ActivateNext shows the tab after the active one in the window in front, or before it,
// wrapping around.
func (m *Manager) ActivateNext(reverse bool) {
	win := m.front()
	n := len(win.order)
	if n < 2 {
		return
	}
	i := slices.Index(win.order, win.active)
	if reverse {
		m.ActivateAt((i + n - 1) % n)
	} else {
		m.ActivateAt((i + 1) % n)
	}
}

// Close closes a tab. Closing the active one activates its right neighbour, or the
// left one at the end. Closing a window's last tab fires OnEmpty, for the window to close.
func (m *Manager) Close(id engine.TabID) { m.close(id, false) }

// CloseToRecent closes a tab as Close does, but when it was the active one, the tab used
// just before it becomes active rather than its neighbour, as a phone's back key does on
// a tab with no history left.
func (m *Manager) CloseToRecent(id engine.TabID) { m.close(id, true) }

func (m *Manager) close(id engine.TabID, recent bool) {
	if m.tabs[id] == nil {
		return
	}
	w := m.windowOf[id]
	// As it was, in its group, for whoever keeps closed tabs to bring back.
	was := *m.tabs[id]
	if u, ok := m.waiting[id]; ok {
		was.URL = u
	}
	m.leaveGroup(id)
	i := m.detach(id)
	t := *m.tabs[id]
	delete(m.tabs, id)
	delete(m.certificates, id)
	delete(m.waiting, id)
	m.eng.CloseTab(id)
	m.closed(t)
	for _, f := range m.onClosedAt {
		f(was, w, i)
	}
	m.left(w, id, i, recent)
}

func (m *Manager) closed(t Tab) {
	for _, f := range m.onClosed {
		f(t)
	}
}

// Move moves a tab to window w, at index in its list, and makes it the active tab there:
// a pinned tab goes to its place among the pinned tabs, and another tab after them. The
// tab keeps its id, its pin and its page, which goes on running. The window it left goes on
// as if the tab had closed there.
func (m *Manager) Move(id engine.TabID, w engine.WindowID, index int) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	from := m.windowOf[id]
	// A window has one tab of each of our pages, and one for each pin: a tab moved to a
	// window that has a tab of its page or its pin shows that window's tab, and stays where
	// it was.
	if from != w {
		if there := m.twinIn(w, t); there != 0 {
			m.Activate(there)
			return
		}
	}
	if from != w {
		m.leaveGroup(id)
	}
	i := m.detach(id)
	m.windowOf[id] = w
	win := m.windows[w]
	win.order = slices.Insert(win.order, index, id)
	m.arrange(w)
	win.recent = append(win.recent, id)
	win.active = id
	win.touch(id)
	for _, f := range m.onMoved {
		f(id, from, i, w)
	}
	m.left(from, id, i, false)
}

// twinIn is window w's tab of the same one of our pages as tab t, or open for the same pin,
// or 0 when it has none.
func (m *Manager) twinIn(w engine.WindowID, t *Tab) engine.TabID {
	switch {
	case t.Page != "":
		return m.PageIn(w, t.Page)
	case t.Pin != "":
		return m.PinIn(w, t.Pin)
	}
	return 0
}

// detach takes a tab out of its window's lists, and says where it was in its order.
func (m *Manager) detach(id engine.TabID) int {
	win := m.windows[m.windowOf[id]]
	win.endCycle()
	i := slices.Index(win.order, id)
	delete(m.windowOf, id)
	win.order = slices.Delete(win.order, i, i+1)
	if j := slices.Index(win.recent, id); j >= 0 {
		win.recent = slices.Delete(win.recent, j, j+1)
	}
	return i
}

// left is what window w does once a tab at position i has left it, closed or moved: with
// no tabs it fires OnEmpty, for the window to close; if the tab was active, its right
// neighbour, or the left one at the end, is, or with recent the tab used before it.
func (m *Manager) left(w engine.WindowID, id engine.TabID, i int, recent bool) {
	win := m.windows[w]
	if len(win.order) == 0 {
		win.active = 0
		m.listChanged()
		for _, f := range m.onEmpty {
			f(w)
		}
		return
	}
	if id == win.active {
		win.active = 0
		next := win.order[min(i, len(win.order)-1)]
		if recent && len(win.recent) > 0 {
			next = win.recent[0]
		}
		m.Activate(next)
		return
	}
	m.listChanged()
}

// All returns copies of every tab, window by window in the order they were opened, each
// window's in order.
func (m *Manager) All() []Tab {
	ids := slices.Sorted(maps.Keys(m.windows))
	var out []Tab
	for _, w := range ids {
		out = append(out, m.AllIn(w)...)
	}
	return out
}

// CountByWorkspace returns how many web tabs each workspace has open, in every window, by
// the workspace's id, copying none. Our tabs are in none.
func (m *Manager) CountByWorkspace() map[string]int {
	counts := map[string]int{}
	for _, t := range m.tabs {
		if t.Page == "" {
			counts[t.Workspace]++
		}
	}
	return counts
}

// AllIn returns copies of window w's tabs, in order.
func (m *Manager) AllIn(w engine.WindowID) []Tab {
	order := m.windows[w].order
	out := make([]Tab, len(order))
	for i, id := range order {
		out[i] = *m.tabs[id]
	}
	return out
}

// RecentIn returns copies of window w's tabs, the most recently shown first, its active
// tab left out.
func (m *Manager) RecentIn(w engine.WindowID) []Tab {
	win := m.windows[w]
	if win == nil {
		return nil
	}
	var out []Tab
	for _, id := range win.recent {
		if t, ok := m.tabs[id]; ok && id != win.active {
			out = append(out, *t)
		}
	}
	return out
}

// ActiveID returns the active tab of the window in front, or zero when there is none.
func (m *Manager) ActiveID() engine.TabID {
	if win := m.front(); win != nil {
		return win.active
	}
	return 0
}

// ActiveIn returns window w's active tab, or zero when it has none.
func (m *Manager) ActiveIn(w engine.WindowID) engine.TabID { return m.windows[w].active }

// Active returns a copy of the active tab of the window in front.
func (m *Manager) Active() Tab {
	if t, ok := m.tabs[m.ActiveID()]; ok {
		return *t
	}
	return Tab{}
}

// Navigate turns what the user typed into a URL and loads it in window w's active tab.
func (m *Manager) Navigate(w engine.WindowID, text string) {
	u := m.Normalize(text)
	if id := m.ActiveIn(w); u != "" && id != 0 {
		m.eng.LoadURL(id, u)
	}
}

// Back moves the active tab back in its history.
func (m *Manager) Back() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.CanGoBack {
		m.eng.GoBack(m.ActiveID())
	}
}

// Forward moves the active tab forward in its history.
func (m *Manager) Forward() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.CanGoForward {
		m.eng.GoForward(m.ActiveID())
	}
}

// Reload reloads the active tab.
func (m *Manager) Reload() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.URL != "" {
		m.eng.Reload(m.ActiveID())
	}
}

// ReloadBypassingCache reloads the active tab without the cache.
func (m *Manager) ReloadBypassingCache() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.URL != "" {
		m.eng.ReloadBypassingCache(m.ActiveID())
	}
}

// Stop stops the active tab loading.
func (m *Manager) Stop() {
	if t := m.tabs[m.ActiveID()]; t != nil && t.Loading {
		m.eng.Stop(m.ActiveID())
	}
}

// Duplicate opens a copy of the active web tab, history and all, as the last tab, in its
// workspace, and shows it. A tab of ours has no copy: a window has one of each.
func (m *Manager) Duplicate() {
	t := m.tabs[m.ActiveID()]
	if t == nil || t.Page != "" {
		return
	}
	id := m.eng.DuplicateTab(t.ID)
	c := m.add(id, m.windowOf[t.ID])
	c.URL, c.Workspace = t.URL, t.Workspace
	if t.Group != "" {
		m.join(id, t.Group, t.ID)
	}
	m.Activate(id)
}

// Open opens a URL in a new last tab of the window in front and shows it.
func (m *Manager) Open(u string) { m.OpenIn(u, true) }

// OpenIn opens a URL in a new last tab of the window in front, in the first workspace,
// shown or behind the active one.
func (m *Manager) OpenIn(u string, foreground bool) engine.TabID {
	return m.open(m.ActiveID(), u, m.workspaces.First(), foreground)
}

// OpenFor opens a URL about a tab's page, such as its source, in a new last tab of that
// tab's window, in its workspace, and shows it. The URL loads the page again, so in another
// workspace it would load it without the tab's cookies.
func (m *Manager) OpenFor(tab engine.TabID, u string) {
	if m.tabs[tab] != nil {
		m.open(tab, u, m.HomeOf(tab), true)
	}
}

// OpenFrom opens a URL in a new tab that tab opener's page asked for, as a link opened in
// a new tab is: in opener's window and group, or in the window in front for 0, in the
// workspace given, shown or behind the active one.
func (m *Manager) OpenFrom(opener engine.TabID, u, workspace string, foreground bool) engine.TabID {
	return m.open(opener, u, workspace, foreground)
}

// OpenBeside opens a URL one of our tabs asked for, which isn't one of our pages, as a web tab
// beside it, in the workspace it was opened from, and shows it.
func (m *Manager) OpenBeside(ours engine.TabID, u string) engine.TabID {
	workspace := m.HomeOf(ours)
	id := m.eng.CreateTab(workspace, false)
	t := m.add(id, m.windowOf[ours])
	t.URL, t.Workspace = address(u), workspace
	m.beside(id, ours)
	m.Activate(id)
	m.eng.LoadURL(id, u)
	return id
}

// beside moves tab id, which add just put last in its window, to after tab after, or after
// the pinned tabs when after is one.
func (m *Manager) beside(id, after engine.TabID) {
	w := m.windowOf[id]
	win := m.windows[w]
	win.order = slices.Insert(win.order[:len(win.order)-1], slices.Index(win.order, after)+1, id)
	m.arrange(w)
}

// PinIn is window w's tab open for a pin, or 0 when it has none.
func (m *Manager) PinIn(w engine.WindowID, pin string) engine.TabID {
	for _, id := range m.windows[w].order {
		if m.tabs[id].Pin == pin {
			return id
		}
	}
	return 0
}

// Bind makes a web tab the one open for a pin in its window: it leaves its group, as a
// pinned tab is in none, and takes its place among the window's pinned tabs. Making the pin,
// and seeing the tab is one to pin, are the caller's.
func (m *Manager) Bind(id engine.TabID, pin string) {
	m.leaveGroup(id)
	m.tabs[id].Pin = pin
	m.arrange(m.windowOf[id])
	m.listChanged()
}

// Unbind makes each tab open for a pin an ordinary tab again, first after its window's
// pinned tabs, as the pin goes.
func (m *Manager) Unbind(pin string) {
	for id, t := range m.tabs {
		if t.Pin == pin {
			t.Pin = ""
			m.arrange(m.windowOf[id])
		}
	}
	m.listChanged()
}

// OpenPin opens a tab for a pin in window w, in its place among the pinned tabs, in the
// pin's workspace and at its address, and makes it active. It has the address from the
// start, as a tab a page opens does.
func (m *Manager) OpenPin(w engine.WindowID, pin, workspace, u string) engine.TabID {
	id := m.eng.CreateTab(workspace, false)
	t := m.add(id, w)
	t.URL, t.Workspace, t.Pin = address(u), workspace, pin
	m.arrange(w)
	m.Activate(id)
	m.eng.LoadURL(id, u)
	return id
}

// arrange keeps window w's pinned tabs first, in their pins' order, and the rest in their
// own after them.
func (m *Manager) arrange(w engine.WindowID) {
	rank := func(id engine.TabID) int {
		if pin := m.tabs[id].Pin; pin != "" {
			return m.pins.Index(pin)
		}
		return math.MaxInt
	}
	slices.SortStableFunc(m.windows[w].order, func(a, b engine.TabID) int { return cmp.Compare(rank(a), rank(b)) })
}

// pinnedIn is how many of window w's tabs are pinned: the first that many in its order.
func (m *Manager) pinnedIn(w engine.WindowID) int {
	n := 0
	for _, id := range m.windows[w].order {
		if m.tabs[id].Pin != "" {
			n++
		}
	}
	return n
}

// open opens a URL in a new web tab, or one of our pages in the window's tab of it.
func (m *Manager) open(opener engine.TabID, u, workspace string, foreground bool) engine.TabID {
	if page := m.PageOf(u); page != "" {
		w := m.windowOf[opener]
		if w == 0 {
			w = m.Front()
		}
		return m.OpenPage(w, page)
	}
	id := m.eng.CreateTab(workspace, false)
	m.HandleTabOpened(opener, id, u, workspace, foreground)
	m.eng.LoadURL(id, u)
	return id
}

// Get returns a copy of one tab.
func (m *Manager) Get(id engine.TabID) (Tab, bool) {
	if t, ok := m.tabs[id]; ok {
		return *t, true
	}
	return Tab{}, false
}

// ReloadTab reloads one tab, with or without the cache.
func (m *Manager) ReloadTab(id engine.TabID, bypassCache bool) {
	if m.tabs[id] == nil {
		return
	}
	if bypassCache {
		m.eng.ReloadBypassingCache(id)
	} else {
		m.eng.Reload(id)
	}
}

// Load loads a URL in one tab.
func (m *Manager) Load(id engine.TabID, u string) {
	if m.tabs[id] != nil {
		m.eng.LoadURL(id, u)
	}
}

// zoomFactors are the steps zooming in and out moves through, as browsers have them.
var zoomFactors = []float64{0.25, 1.0 / 3, 0.5, 2.0 / 3, 0.75, 0.8, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2, 2.5, 3, 4, 5}

// ZoomIn and ZoomOut move the active tab's page zoom one step; ZoomReset puts it back to
// actual size.
func (m *Manager) ZoomIn()    { m.zoomStep(1) }
func (m *Manager) ZoomOut()   { m.zoomStep(-1) }
func (m *Manager) ZoomReset() { m.zoomTo(1) }

func (m *Manager) zoomStep(dir int) {
	if m.tabs[m.ActiveID()] == nil {
		return
	}
	now := m.eng.ZoomOf(m.ActiveID())
	const near = 0.001
	if dir > 0 {
		for _, f := range zoomFactors {
			if f > now+near {
				m.zoomTo(f)
				return
			}
		}
		return
	}
	for i := len(zoomFactors) - 1; i >= 0; i-- {
		if zoomFactors[i] < now-near {
			m.zoomTo(zoomFactors[i])
			return
		}
	}
}

func (m *Manager) zoomTo(f float64) {
	if m.tabs[m.ActiveID()] != nil {
		m.eng.Zoom(m.ActiveID(), f)
	}
}

// HandleTabOpened records a tab a page opened, in the opener's window, or in the window
// in front when no tab opened it, in the workspace the engine made it in, and in its
// opener's group, and shows it if the page asked for the foreground, or if it is its
// window's first.
func (m *Manager) HandleTabOpened(opener, id engine.TabID, u, workspace string, foreground bool) {
	w := m.windowOf[opener]
	if w == 0 {
		w = m.Front()
	}
	t := m.add(id, w)
	t.URL, t.Workspace = address(u), workspace
	// It goes by its opener's page until it has a page of its own: one that opens on the
	// blank page, or another site, its opener's Links to other sites may refuse at once.
	if o := m.tabs[opener]; o != nil {
		t.opener, t.openerURL = opener, o.URL
		t.byPage = true
		if m.guard != nil && !m.pageMayOpen(t, u) {
			return
		}
	}
	// A tab a page opens joins its opener's group, after it.
	if o := m.tabs[opener]; o != nil && o.Group != "" {
		m.join(id, o.Group, opener)
	}
	m.shown(id, foreground)
}

// HandleTabFocused makes the tab whose page took the keyboard the active one.
func (m *Manager) HandleTabFocused(id engine.TabID) { m.Activate(id) }

// HandleNavigationStarting is the one check every navigation in a tab passes, however it
// started. A tab of ours goes on only to one of our pages, and takes any other address to
// a new web tab beside it. A web tab takes one of our pages to the window's tab of it.
// Otherwise a web tab runs the gates, and the tab records the result. nav is the engine's
// id for the navigation; one taken elsewhere is none of the tab's.
func (m *Manager) HandleNavigationStarting(id engine.TabID, nav int64, u string) engine.Navigation {
	t := m.tabs[id]
	if t == nil {
		return engine.NavigationBlock
	}
	switch page := m.PageOf(u); {
	case t.Page != "" && page == "":
		m.OpenBeside(id, u)
		return engine.NavigationElsewhere
	case t.Page == "" && page != "":
		m.OpenPage(m.windowOf[id], page)
		return engine.NavigationElsewhere
	}
	// A restored tab keeps its address and title until anything loads.
	delete(m.waiting, id)
	// A page's navigation its site's Links to other sites refuses goes nowhere, and the tab
	// stays as it was.
	if !m.pageMay(t, nav, u) {
		return engine.NavigationElsewhere
	}
	t.started = nav
	t.URL, t.Error, t.ErrorCode, t.Crashed = address(u), "", "", false
	delete(m.certificates, id)
	if t.Page == "" && !m.allowed(u) {
		t.Blocked, t.Loading = true, false
		m.changed(t)
		return engine.NavigationBlock
	}
	t.shownTitle, t.shownFavicon = t.Title, t.Favicon
	t.Blocked, t.Title, t.Favicon, t.Progress = false, "", "", 0
	m.changed(t)
	return engine.NavigationProceed
}

// allowed runs the gates, in order, until one says no.
func (m *Manager) allowed(u string) bool {
	for _, g := range m.gates {
		if !g(u) {
			return false
		}
	}
	return true
}

// HandleNavigationFinished records a committed navigation. One older than the latest
// started leaves the tab as it is: a new tab's blank page can commit after the user's
// navigation began, and the address that navigation is going to shows until it commits or
// fails, as Chrome shows its pending entry.
func (m *Manager) HandleNavigationFinished(id engine.TabID, nav int64, u string) {
	t := m.tabs[id]
	if _, ok := m.waiting[id]; t == nil || ok || nav < t.started {
		return
	}
	// A page restored from the back-forward cache never reports a start, so a commit
	// has to clear what an earlier navigation left behind.
	t.URL, t.Error, t.ErrorCode, t.Blocked, t.Crashed = address(u), "", "", false, false
	if withoutFragment(u) != withoutFragment(t.committed) {
		t.Thumb = ""
	}
	t.committed = u
	t.AdsBlocked, t.PopupsBlocked = 0, 0
	// The blank page a popup opens on isn't a page of its own: it still goes by its
	// opener's.
	t.ownPage = t.ownPage || t.URL != ""
	for _, f := range m.onFinished {
		f(*t)
	}
	m.changed(t)
}

// HandleAdBlocked counts one request of the tab's page the ad blocker kept from its host.
func (m *Manager) HandleAdBlocked(id engine.TabID) {
	if t := m.tabs[id]; t != nil {
		t.AdsBlocked++
		m.changed(t)
	}
}

// HandleNavigationFailed records a failed navigation, unless it is older than the latest
// started, as HandleNavigationFinished does.
func (m *Manager) HandleNavigationFailed(id engine.TabID, nav int64, u string, why engine.Failure, code string) {
	t := m.tabs[id]
	if t == nil || nav < t.started {
		return
	}
	t.URL, t.Error, t.ErrorCode, t.Loading = address(u), why, code, false
	if why == "" && code == "" && t.Title == "" {
		t.Title, t.Favicon = t.shownTitle, t.shownFavicon
	}
	m.changed(t)
}

// HandleHistoryChanged records whether the tab can go back and forward.
func (m *Manager) HandleHistoryChanged(id engine.TabID, canGoBack, canGoForward bool) {
	t := m.tabs[id]
	if t == nil || (t.CanGoBack == canGoBack && t.CanGoForward == canGoForward) {
		return
	}
	t.CanGoBack, t.CanGoForward = canGoBack, canGoForward
	m.changed(t)
}

// HandleTitleChanged records a new title. Content titles an untitled page with its
// address, so the blank page arrives titled about:blank; the tab shows no title.
func (m *Manager) HandleTitleChanged(id engine.TabID, title string) {
	if title == BlankURL {
		title = ""
	}
	t := m.tabs[id]
	if _, ok := m.waiting[id]; t == nil || ok || t.Title == title {
		return
	}
	t.Title = title
	for _, f := range m.onTitle {
		f(*t)
	}
	m.changed(t)
}

// connectionNotes are what the site view says of an insecure connection, by reason.
var connectionNotes = map[string]string{
	"not-encrypted": "Your connection to this site isn't encrypted, so others on the network can see what you send",
	"certificate":   "This site's certificate isn't valid, so it may not be the site it says it is",
	"mixed-content": "Parts of this page load without encryption",
}

// HandleConnectionChanged records how the tab's page reached it.
func (m *Manager) HandleConnectionChanged(id engine.TabID, c engine.Connection) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	state, note := "", ""
	switch c.State {
	case engine.ConnectionSecure:
		state = "secure"
	case engine.ConnectionInsecure:
		state, note = "insecure", connectionNotes[c.Reason]
	}
	if t.Connection == state && t.ConnectionNote == note {
		return
	}
	t.Connection, t.ConnectionNote = state, note
	m.changed(t)
}

// HandleFaviconChanged records the page's icon.
func (m *Manager) HandleFaviconChanged(id engine.TabID, dataURL string) {
	t := m.tabs[id]
	if t == nil || t.Favicon == dataURL {
		return
	}
	t.Favicon = dataURL
	m.changed(t)
}

// HandleThumbnail records that the engine has a new picture of the tab's page.
func (m *Manager) HandleThumbnail(id engine.TabID, revision int64) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	t.Thumb = m.eng.ThumbnailURL(id, revision)
	m.changed(t)
}

// withoutFragment is an address without its #fragment.
func withoutFragment(u string) string {
	u, _, _ = strings.Cut(u, "#")
	return u
}

// HandleLoadingChanged records the loading state.
func (m *Manager) HandleLoadingChanged(id engine.TabID, loading bool) {
	t := m.tabs[id]
	if t == nil || t.Loading == loading {
		return
	}
	t.Loading = loading
	m.changed(t)
}

// HandleLoadProgressChanged records how far the load has got.
func (m *Manager) HandleLoadProgressChanged(id engine.TabID, progress float64) {
	t := m.tabs[id]
	if t == nil || t.Progress == progress {
		return
	}
	t.Progress = progress
	m.changed(t)
}

// HandlePageCrashed records that the tab's renderer died.
func (m *Manager) HandlePageCrashed(id engine.TabID) {
	t := m.tabs[id]
	if t == nil {
		return
	}
	t.Crashed, t.Loading = true, false
	m.changed(t)
}

func (m *Manager) changed(t *Tab) {
	t.Notice = m.noticeOf(t)
	for _, f := range m.onChanged {
		f(*t)
	}
}

func (m *Manager) listChanged() {
	for _, f := range m.onList {
		f()
	}
}

// SetSearch gives the manager what turns words into a search's address: the profile's
// search engine.
func (m *Manager) SetSearch(f func(words string) string) { m.search = f }

// SearchURL is a search for the words, however they read.
func (m *Manager) SearchURL(words string) string { return m.search(words) }

// Normalize turns address bar text into a URL, or a search for anything else.
func (m *Manager) Normalize(text string) string { return NormalizeInput(text, m.search) }

// LooksLikeAddress says text reads as an address, not as words to search for.
func LooksLikeAddress(text string) bool {
	never := func(string) string { return "" }
	return NormalizeInput(text, never) != ""
}

// NormalizeInput turns address bar text into a URL, or into what search makes of anything
// else.
func NormalizeInput(text string, search func(words string) string) string {
	s := strings.TrimSpace(text)
	switch {
	case s == "":
		return ""
	case strings.Contains(s, "://"),
		strings.HasPrefix(s, "about:"),
		strings.HasPrefix(s, "data:"):
		return s
	case strings.ContainsAny(s, " \t"):
		return search(s)
	case s == "localhost", strings.HasPrefix(s, "localhost:"), strings.HasPrefix(s, "127.0.0.1"):
		return "http://" + s
	case strings.Contains(s, "."):
		return "https://" + s
	}
	return search(s)
}
