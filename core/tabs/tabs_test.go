package tabs

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"projectb/engine"
)

// fakeEngine records what the manager asks of it.
type fakeEngine struct {
	engine.Engine
	next   engine.TabID
	closed []engine.TabID
	// reloads counts the reloads asked for.
	reloads int
	// in is the workspace each tab was made in, by its id.
	in map[engine.TabID]string
	// ours are the tabs made in our pages' context.
	ours []engine.TabID
}

func (f *fakeEngine) CreateInternalTab() engine.TabID {
	f.next++
	f.ours = append(f.ours, f.next)
	return f.next
}

// PageURL and OurPage name our pages' addresses as the real engine does.
func (f *fakeEngine) PageURL(p engine.Page) string { return "chrome://" + string(p) + "/" }

func (f *fakeEngine) OurPage(raw string) (engine.Page, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "chrome" || u.Host == "" {
		return "", false
	}
	return engine.Page(u.Host + strings.TrimSuffix(u.Path, "/")), true
}

func (f *fakeEngine) CreateTab(workspace string) engine.TabID {
	f.next++
	if f.in == nil {
		f.in = map[engine.TabID]string{}
	}
	f.in[f.next] = workspace
	return f.next
}

func (f *fakeEngine) ThumbnailURL(tab engine.TabID, revision int64) string {
	return fmt.Sprintf("chrome://thumb/%d?v=%d", tab, revision)
}

func (f *fakeEngine) DuplicateTab(engine.TabID) engine.TabID { f.next++; return f.next }
func (f *fakeEngine) WriteClipboard(string)                  {}
func (f *fakeEngine) CloseTab(id engine.TabID)               { f.closed = append(f.closed, id) }
func (f *fakeEngine) LoadURL(engine.TabID, string)           {}
func (f *fakeEngine) Reload(engine.TabID)                    { f.reloads++ }
func (f *fakeEngine) GoBack(engine.TabID)                    {}

// work is the first workspace in the tests' list.
const work = "0a1b2c3d"

// list is a profile's list of workspaces, by their ids.
type list []string

func (l *list) First() string      { return (*l)[0] }
func (l *list) Has(id string) bool { return slices.Contains(*l, id) }
func (l *list) remove(id string)   { *l = slices.DeleteFunc(*l, func(x string) bool { return x == id }) }

// pinList is a profile's pins, by their ids, in their order.
type pinList []string

func (l *pinList) Index(id string) int { return slices.Index(*l, id) }

// oneWindow is a manager with window 1, whose list has work first, then Main, and no pins.
func oneWindow(e engine.Engine) *Manager {
	m := New(e, &list{work, ""}, &pinList{})
	m.AddWindow(1)
	return m
}

func ids(tabs []Tab) []engine.TabID {
	out := make([]engine.TabID, len(tabs))
	for i, t := range tabs {
		out[i] = t.ID
	}
	return out
}

func TestCreateCloseAndActivate(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	lists := 0
	m.OnListChanged(func() { lists++ })

	a, b, c := m.Create(1), m.Create(1), m.Create(1)
	if m.ActiveID() != c || !slices.Equal(ids(m.All()), []engine.TabID{a, b, c}) {
		t.Fatalf("after create: active %v all %v", m.ActiveID(), ids(m.All()))
	}

	m.Activate(a)
	if m.ActiveID() != a {
		t.Fatalf("activate: active %v", m.ActiveID())
	}
	m.ActivateNext(false)
	m.ActivateNext(false)
	m.ActivateNext(false)
	if m.ActiveID() != a {
		t.Fatalf("next wraps: active %v", m.ActiveID())
	}
	m.ActivateNext(true)
	if m.ActiveID() != c {
		t.Fatalf("previous wraps: active %v", m.ActiveID())
	}
	m.ActivateAt(1)
	if m.ActiveID() != b {
		t.Fatalf("at: active %v", m.ActiveID())
	}

	// Closing the active tab moves to its right neighbour; closing the last one, left.
	m.Close(b)
	if m.ActiveID() != c || !slices.Equal(e.closed, []engine.TabID{b}) {
		t.Fatalf("close active: active %v closed %v", m.ActiveID(), e.closed)
	}
	m.Close(c)
	if m.ActiveID() != a {
		t.Fatalf("close last in order: active %v", m.ActiveID())
	}
	if lists == 0 {
		t.Fatal("list changes never announced")
	}
}

// Closing a window's last tab leaves the window with no tabs and no active tab, and says
// the tab closed there rather than moved.
func TestClosingTheLastTabEmptiesTheList(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	empty, moved := false, true
	m.OnEmpty(func(w engine.WindowID, wasMoved bool) { empty, moved = w == 1, wasMoved })
	m.Close(m.Create(1))
	if !empty || moved || len(m.All()) != 0 || m.ActiveID() != 0 || !slices.Equal(m.Windows(), []engine.WindowID{1}) {
		t.Fatalf("empty %v moved %v all %v active %v windows %v", empty, moved, m.All(), m.ActiveID(), m.Windows())
	}
}

// A window with tabs always has an active one: a tab that comes into a window with none is
// its active tab, though it was opened behind.
func TestATabIntoAWindowWithNoTabsIsActive(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	m.Close(m.Create(1))
	a := m.Add(1, work, false)
	if m.ActiveIn(1) != a {
		t.Fatalf("added behind into a window with none: active %v", m.ActiveIn(1))
	}
	m.Close(a)
	m.HandleTabOpened(0, 9, "https://b.test/", work, false)
	if m.ActiveIn(1) != 9 {
		t.Fatalf("opened behind into a window with none: active %v", m.ActiveIn(1))
	}
	b := m.Add(1, work, false)
	if m.ActiveIn(1) != 9 || m.WindowOf(b) != 1 {
		t.Fatalf("added behind beside a tab: active %v", m.ActiveIn(1))
	}
}

func TestATabAPageOpensIsShownOnlyInTheForeground(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a := m.Create(1)
	m.HandleTabOpened(a, 7, "https://b.test/", "", false)
	if m.ActiveID() != a || m.All()[1].URL != "https://b.test/" {
		t.Fatalf("background open: active %v all %v", m.ActiveID(), m.All())
	}
	m.HandleTabOpened(a, 8, "https://c.test/", "", true)
	if m.ActiveID() != 8 {
		t.Fatalf("foreground open: active %v", m.ActiveID())
	}
}

func TestFaviconFollowsThePage(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://a.test/")
	m.HandleFaviconChanged(id, "data:image/png;base64,AAAA")
	if m.Active().Favicon == "" {
		t.Fatal("favicon not recorded")
	}
	m.HandleNavigationStarting(id, 2, "https://b.test/")
	if m.Active().Favicon != "" {
		t.Fatal("a new page keeps the old page's favicon")
	}
}

// A tab's picture is of the page it was taken of: a reload or a new #fragment keeps it, and
// a page at another address drops it once it commits, not while the old page still shows.
func TestThumbFollowsThePage(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://a.test/")
	m.HandleNavigationFinished(id, 1, "https://a.test/")
	if m.Active().Thumb != "" {
		t.Fatal("a picture before any was taken")
	}
	m.HandleThumbnail(id, 3)
	if got := m.Active().Thumb; got != fmt.Sprintf("chrome://thumb/%d?v=3", id) {
		t.Fatalf("thumb %q", got)
	}
	m.HandleNavigationFinished(id, 2, "https://a.test/#part")
	m.HandleNavigationStarting(id, 3, "https://a.test/")
	m.HandleNavigationFinished(id, 3, "https://a.test/")
	if m.Active().Thumb == "" {
		t.Fatal("a reload or a #fragment dropped the picture")
	}
	m.HandleNavigationStarting(id, 4, "https://b.test/")
	if m.Active().Thumb == "" {
		t.Fatal("the picture went while the old page still showed")
	}
	m.HandleNavigationFinished(id, 4, "https://b.test/")
	if m.Active().Thumb != "" {
		t.Fatal("a new page keeps the old page's picture")
	}
	m.HandleThumbnail(99, 1)
}

func TestWordsGoToTheSearchGiven(t *testing.T) {
	m := New(&fakeEngine{}, &list{""}, &pinList{})
	if got := m.Normalize("two words"); got != "https://duckduckgo.com/?q=two+words" {
		t.Fatalf("before any engine is given: %q", got)
	}
	m.SetSearch(func(w string) string { return "https://find.test/?q=" + w })
	if got := m.Normalize("word"); got != "https://find.test/?q=word" {
		t.Fatalf("Normalize = %q", got)
	}
	if got := m.Normalize("example.com"); got != "https://example.com" {
		t.Fatalf("an address stays an address: %q", got)
	}
	if !LooksLikeAddress("example.com") || LooksLikeAddress("two words") || LooksLikeAddress("word") {
		t.Fatal("LooksLikeAddress is wrong")
	}
}

func TestNormalizeInput(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"  example.com ":        "https://example.com",
		"example.com:8080/a":    "https://example.com:8080/a",
		"https://example.com/a": "https://example.com/a",
		"http://example.com":    "http://example.com",
		"localhost:3000":        "http://localhost:3000",
		"about:blank":           "about:blank",
		"how do tabs work":      "https://duckduckgo.com/?q=how+do+tabs+work",
		"tabs":                  "https://duckduckgo.com/?q=tabs",
	} {
		if got := NormalizeInput(in, defaultEngine().For); got != want {
			t.Errorf("NormalizeInput(%q) = %q, want %q", in, got, want)
		}
	}
}

// loads records what the manager asks the engine to load.
type loads struct {
	fakeEngine
	urls []string
}

func (l *loads) LoadURL(_ engine.TabID, u string) { l.urls = append(l.urls, u) }

func TestANewTabHoldsTheBlankPageAndReadsAsEmpty(t *testing.T) {
	e := &loads{}
	m := oneWindow(e)
	id := m.Create(1)
	if !slices.Equal(e.urls, []string{BlankURL}) {
		t.Fatalf("loaded %v", e.urls)
	}
	m.HandleNavigationStarting(id, 1, BlankURL)
	m.HandleNavigationFinished(id, 1, BlankURL)
	m.HandleTitleChanged(id, BlankURL) // Content's title for an untitled page.
	if tab := m.Active(); tab.URL != "" || tab.Title != "" {
		t.Fatalf("the blank page shows as url %q title %q", tab.URL, tab.Title)
	}
	m.HandleNavigationStarting(id, 2, "https://example.com/")
	m.HandleNavigationFinished(id, 2, "https://example.com/")
	m.HandleTitleChanged(id, "Example Domain")
	if tab := m.Active(); tab.URL != "https://example.com/" || tab.Title != "Example Domain" {
		t.Fatalf("a real page shows as url %q title %q", tab.URL, tab.Title)
	}
}

// A new tab's blank page can commit, or fail, after the user's navigation has begun. That
// late result leaves the address the user is going to, which shows until its own commit.
func TestALateBlankPageLeavesTheNewerAddress(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, BlankURL)
	m.HandleNavigationStarting(id, 2, "https://example.com/")
	m.HandleNavigationFinished(id, 1, BlankURL)
	if tab := m.Active(); tab.URL != "https://example.com/" {
		t.Fatalf("after the blank page's late commit the tab shows %q", tab.URL)
	}
	m.HandleNavigationFailed(id, 1, BlankURL, engine.FailOther, "ERR_FAILED")
	if tab := m.Active(); tab.URL != "https://example.com/" || tab.Error != "" {
		t.Fatalf("after the blank page's late failure the tab shows %q, error %q", tab.URL, tab.Error)
	}
	m.HandleNavigationFinished(id, 2, "https://example.com/")
	if tab := m.Active(); tab.URL != "https://example.com/" {
		t.Fatalf("the newer navigation's commit shows %q", tab.URL)
	}
}

// A navigation that shows nothing instead, as one that becomes a download or is refused on
// its way, ends as a failure with none named, at the page still shown: the address goes back
// to that page, so a restore loads it and not the download, no error shows, and the page's
// title and icon come back.
func TestANavigationThatBecomesADownloadLeavesThePageShown(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://example.com/")
	m.HandleNavigationFinished(id, 1, "https://example.com/")
	m.HandleTitleChanged(id, "Example")
	m.HandleNavigationStarting(id, 2, "https://example.com/report.pdf")
	m.HandleNavigationFailed(id, 2, "https://example.com/", "", "")
	if tab := m.Active(); tab.URL != "https://example.com/" || tab.Error != "" || tab.Loading || tab.Title != "Example" {
		t.Fatalf("after the download the tab shows %q, %q, error %q, loading %v", tab.URL, tab.Title, tab.Error, tab.Loading)
	}
}

// Each window has its own tabs, its own active tab and its own recent order; commands act
// on the window in front, and a tab a page opens goes in its opener's window.
func TestWindowsKeepTheirOwnTabs(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a, b := m.Create(1), m.Create(1)
	m.AddWindow(2)
	if m.Front() != 2 || m.ActiveID() != 0 {
		t.Fatalf("a new window is in front, with no tabs: front %v active %v", m.Front(), m.ActiveID())
	}
	c := m.Create(2)
	if m.WindowOf(c) != 2 || !slices.Equal(ids(m.AllIn(2)), []engine.TabID{c}) || !slices.Equal(ids(m.AllIn(1)), []engine.TabID{a, b}) {
		t.Fatalf("window 1 %v, window 2 %v", ids(m.AllIn(1)), ids(m.AllIn(2)))
	}
	if m.ActiveIn(1) != b || m.ActiveID() != c {
		t.Fatalf("active: window 1 %v, in front %v", m.ActiveIn(1), m.ActiveID())
	}

	// Next tab and Ctrl+Tab stay within the window in front.
	m.ActivateNext(false)
	m.CycleRecent(false)
	if m.ActiveID() != c || m.ActiveIn(1) != b {
		t.Fatalf("a window of one tab cycles to itself: %v, window 1 %v", m.ActiveID(), m.ActiveIn(1))
	}
	m.Focus(1)
	m.ActivateNext(false)
	if m.Front() != 1 || m.ActiveID() != a || m.ActiveIn(2) != c {
		t.Fatalf("after window 1 came to the front: front %v active %v, window 2 %v", m.Front(), m.ActiveID(), m.ActiveIn(2))
	}

	// A page in window 2 opens a tab: it goes in window 2, though window 1 is in front.
	m.HandleTabOpened(c, 9, "https://p.test/", "", true)
	if m.WindowOf(9) != 2 || m.ActiveIn(2) != 9 || m.ActiveID() != a {
		t.Fatalf("opened tab in window %v, window 2 active %v, in front %v", m.WindowOf(9), m.ActiveIn(2), m.ActiveID())
	}
	// One no page opened goes in the window in front.
	m.HandleTabOpened(0, 10, "https://q.test/", "", true)
	if m.WindowOf(10) != 1 {
		t.Fatalf("tab with no opener in window %v", m.WindowOf(10))
	}

	// Closing window 2's last tab empties window 2 only.
	var emptied []engine.WindowID
	m.OnEmpty(func(w engine.WindowID, _ bool) { emptied = append(emptied, w) })
	m.Close(9)
	m.Close(c)
	if !slices.Equal(emptied, []engine.WindowID{2}) || len(m.AllIn(1)) != 3 {
		t.Fatalf("emptied %v, window 1 %v", emptied, ids(m.AllIn(1)))
	}
}

// Removing a window closes its tabs, and the window used before it is in front again.
func TestRemovingAWindowClosesItsTabs(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a := m.Create(1)
	m.AddWindow(2)
	b, c := m.Create(2), m.Create(2)
	m.RemoveWindow(2)
	if m.Front() != 1 || m.ActiveID() != a || !slices.Equal(m.Windows(), []engine.WindowID{1}) {
		t.Fatalf("front %v active %v windows %v", m.Front(), m.ActiveID(), m.Windows())
	}
	if !slices.Equal(e.closed, []engine.TabID{b, c}) || len(m.All()) != 1 || m.WindowOf(b) != 0 {
		t.Fatalf("closed %v, all %v", e.closed, ids(m.All()))
	}
}

// A tab moves to a place in another window's list, keeps its id and is active there; the
// window it left goes on as if it had closed there, and one left empty is emptied, by a
// move.
func TestMovingATabBetweenWindows(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	a, b := m.Create(1), m.Create(1)
	m.AddWindow(2)
	c := m.Create(2)
	var emptied []engine.WindowID
	m.OnEmpty(func(w engine.WindowID, moved bool) {
		if moved {
			emptied = append(emptied, w)
		}
	})

	m.Move(b, 2, 0)
	if m.WindowOf(b) != 2 || !slices.Equal(ids(m.AllIn(2)), []engine.TabID{b, c}) || m.ActiveIn(2) != b {
		t.Fatalf("window 2 %v active %v", ids(m.AllIn(2)), m.ActiveIn(2))
	}
	if !slices.Equal(ids(m.AllIn(1)), []engine.TabID{a}) || m.ActiveIn(1) != a || len(e.closed) != 0 {
		t.Fatalf("window 1 %v active %v, closed %v", ids(m.AllIn(1)), m.ActiveIn(1), e.closed)
	}
	m.Move(a, 2, 2)
	if !slices.Equal(ids(m.AllIn(2)), []engine.TabID{b, c, a}) || !slices.Equal(emptied, []engine.WindowID{1}) || len(e.closed) != 0 {
		t.Fatalf("window 2 %v, emptied %v, closed %v", ids(m.AllIn(2)), emptied, e.closed)
	}
}

// A tab is in the workspace it was made in for its life: a new one, one a page opened, a
// copy, and one moved to another window.
func TestATabKeepsItsWorkspace(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	plain := m.CreateIn(1, "")
	work := m.CreateIn(1, "0a1b2c3d")
	m.HandleTabOpened(work, 9, "https://p.test/", "0a1b2c3d", true)
	m.Duplicate()
	workspace := func(id engine.TabID) string { t, _ := m.Get(id); return t.Workspace }
	got := []string{workspace(plain), workspace(work), workspace(9), workspace(m.ActiveID())}
	if !slices.Equal(got, []string{"", "0a1b2c3d", "0a1b2c3d", "0a1b2c3d"}) || m.ActiveID() == 9 {
		t.Fatalf("workspaces %q, the copy %v", got, m.ActiveID())
	}
	m.AddWindow(2)
	m.Move(work, 2, 0)
	if m.WindowOf(work) != 2 || workspace(work) != "0a1b2c3d" || m.AllIn(2)[0].Workspace != "0a1b2c3d" {
		t.Fatalf("after the move: window %v, %+v", m.WindowOf(work), m.AllIn(2))
	}
}

// New Tab opens in the workspace of its window's active tab, and in the first one in a
// window with no tab.
func TestANewTabTakesTheActiveTabsWorkspace(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	workspace := func(id engine.TabID) string { t, _ := m.Get(id); return t.Workspace }
	first := m.Create(1)
	main := m.CreateIn(1, "")
	fromMain := m.Create(1)
	m.Activate(first)
	fromFirst := m.Create(1)
	m.AddWindow(2)
	inNewWindow := m.Create(2)
	got := []string{workspace(first), workspace(main), workspace(fromMain), workspace(fromFirst), workspace(inNewWindow)}
	if !slices.Equal(got, []string{work, "", "", work, work}) {
		t.Fatalf("workspaces %q", got)
	}
}

// A URL opened with no tab to take a workspace from, as a link from another app is, opens
// in the first workspace, whichever tab is active.
func TestAURLOpensInTheFirstWorkspace(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	m.CreateIn(1, "")
	id := m.OpenIn("https://p.test/", true)
	if tab, _ := m.Get(id); tab.Workspace != work || e.in[id] != work || tab.URL != "https://p.test/" {
		t.Fatalf("the tab is %+v, in %q in the engine", tab, e.in[id])
	}
}

// Closing the active tab to the recent one makes the tab used before it active, not its
// neighbour; a plain close keeps the neighbour.
func TestClosingToTheRecentTab(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	a, b, c := m.Create(1), m.Create(1), m.Create(1)
	m.Activate(a)
	m.Activate(c)
	// c was opened last, a used before it; b is c's neighbour.
	m.CloseToRecent(c)
	if m.ActiveIn(1) != a {
		t.Fatalf("active %d, want the tab used before, %d", m.ActiveIn(1), a)
	}
	m.Activate(b)
	m.Close(b)
	if got := ids(m.AllIn(1)); !slices.Equal(got, []engine.TabID{a}) || m.ActiveIn(1) != a {
		t.Fatalf("tabs %v, active %d", got, m.ActiveIn(1))
	}
	m.CloseToRecent(a)
	if len(m.AllIn(1)) != 0 || m.ActiveIn(1) != 0 {
		t.Fatalf("tabs %v, active %d", ids(m.AllIn(1)), m.ActiveIn(1))
	}
}

func TestPageOf(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	for in, want := range map[string]string{
		"chrome://settings/":            "settings",
		"chrome://settings/keyboard":    "settings",
		"chrome://shell/standard/":      "",
		"chrome://picker/":              "",
		"https://settings/":             "",
		"https://example.com/settings/": "",
		"about:blank":                   "",
	} {
		if got := m.PageOf(in); got != want {
			t.Errorf("PageOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// Opening one of our pages opens a tab of ours beside the active tab and shows it; a
// window has one for each page, so opening it again shows that one, and another window
// gets its own.
func TestOurPageIsOneTabAWindowBesideTheActiveOne(t *testing.T) {
	e := &loads{}
	m := oneWindow(e)
	a, b := m.CreateIn(1, ""), m.Create(1)
	m.Activate(a)
	s := m.OpenPage(1, "settings")
	tab, _ := m.Get(s)
	if !slices.Equal(ids(m.AllIn(1)), []engine.TabID{a, s, b}) || m.ActiveID() != s || !slices.Equal(e.ours, []engine.TabID{s}) {
		t.Fatalf("order %v, active %v, made in our context %v", ids(m.AllIn(1)), m.ActiveID(), e.ours)
	}
	if tab.Page != "settings" || tab.URL != "chrome://settings/" || e.urls[len(e.urls)-1] != "chrome://settings/" || tab.Workspace != "" {
		t.Fatalf("the tab is %+v, the last load %q", tab, e.urls[len(e.urls)-1])
	}
	m.Activate(b)
	if again := m.OpenPage(1, "settings"); again != s || m.ActiveID() != s || len(m.AllIn(1)) != 3 || len(e.ours) != 1 {
		t.Fatalf("opened again: %v, active %v, tabs %v", again, m.ActiveID(), ids(m.AllIn(1)))
	}
	m.AddWindow(2)
	if other := m.OpenPage(2, "settings"); other == s || m.WindowOf(other) != 2 || len(e.ours) != 2 {
		t.Fatalf("in window 2: %v, in window %v", other, m.WindowOf(other))
	}
}

// One of our tabs remembers the workspace of the tab it was opened from: New Tab from it
// goes there, or to the first workspace once that one is gone.
func TestANewTabFromOurTabGoesToTheWorkspaceItWasOpenedFrom(t *testing.T) {
	l := &list{work, ""}
	m := New(&fakeEngine{}, l, &pinList{})
	m.AddWindow(1)
	m.CreateIn(1, "")
	s := m.OpenPage(1, "settings")
	workspace := func(id engine.TabID) string { t, _ := m.Get(id); return t.Workspace }
	if fromIt := m.Create(1); workspace(fromIt) != "" {
		t.Fatalf("from our tab, in %q", workspace(fromIt))
	}
	m.Activate(s)
	if beside := m.CreateWith(s, true); workspace(beside) != "" {
		t.Fatalf("beside it, in %q", workspace(beside))
	}
	l.remove("")
	m.Activate(s)
	if gone := m.Create(1); workspace(gone) != work || workspace(s) != "" {
		t.Fatalf("with Main gone, in %q; our tab remembers %q", workspace(gone), workspace(s))
	}
}

// One of our tabs is in no workspace, so in no tab group.
func TestOurTabIsInNoGroup(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	a := m.CreateIn(1, work)
	s := m.OpenPage(1, "settings")
	b := m.CreateIn(1, work)
	if g := m.CreateGroup([]engine.TabID{s, a}, "", ""); g != "" {
		t.Fatalf("a group led by our tab: %q", g)
	}
	g := m.CreateGroup([]engine.TabID{a, s, b}, "", "")
	m.AddToGroup(s, g)
	if groups := m.GroupsIn(1); len(groups) != 1 || !slices.Equal(groups[0].Tabs, []engine.TabID{a, b}) {
		t.Fatalf("groups %+v", groups)
	}
	if beside := m.CreateWith(s, true); len(m.GroupsIn(1)[0].Tabs) != 2 || m.ActiveID() != beside {
		t.Fatalf("a new tab beside ours joined a group: %+v", m.GroupsIn(1))
	}
}

// A window has one tab of each of our pages: one of ours has no copy, and one moved to a
// window that has its page shows that window's tab and stays where it was. Moved to a
// window without, it moves as any tab, keeping its kind.
func TestOurTabStaysOneAWindow(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	m.CreateIn(1, "")
	s := m.OpenPage(1, "settings")
	m.Duplicate()
	if len(m.AllIn(1)) != 2 || m.ActiveID() != s || e.next != 2 {
		t.Fatalf("a duplicate of ours: tabs %v, active %v", ids(m.AllIn(1)), m.ActiveID())
	}
	m.AddWindow(2)
	m.CreateIn(2, "")
	there := m.OpenPage(2, "settings")
	other := m.Create(2)
	m.Move(s, 2, 0)
	if m.WindowOf(s) != 1 || m.ActiveIn(2) != there || len(m.AllIn(2)) != 3 || m.ActiveIn(1) != s {
		t.Fatalf("moved where its page is: in window %v, window 2 %v active %v", m.WindowOf(s), ids(m.AllIn(2)), m.ActiveIn(2))
	}
	m.AddWindow(3)
	m.Move(s, 3, 0)
	if moved, _ := m.Get(s); m.WindowOf(s) != 3 || moved.Page != "settings" || m.ActiveIn(2) != there || other == 0 {
		t.Fatalf("moved where its page isn't: %+v in window %v", moved, m.WindowOf(s))
	}
}

// The one navigation rule. A tab of ours goes on to our pages, without the web's gates; any
// other address, however its navigation started, goes nowhere there and opens in a new
// web tab beside it, in the workspace it was opened from. A web tab asked for one of our
// pages stays as it was, and the window's tab of that page shows.
func TestOurPagesNavigateInPlaceAndAnythingElseOpensBeside(t *testing.T) {
	e := &loads{}
	m := oneWindow(e)
	m.AddGate(func(string) bool { return false })
	web := m.CreateIn(1, "")
	ours := m.OpenPage(1, "settings")
	last := m.Create(1)
	m.Activate(ours)

	if got := m.HandleNavigationStarting(ours, 1, "chrome://settings/keyboard"); got != engine.NavigationProceed {
		t.Fatalf("to our page: %v", got)
	}
	if tab, _ := m.Get(ours); tab.URL != "chrome://settings/keyboard" || tab.Blocked {
		t.Fatalf("our tab after its page: %+v", tab)
	}

	m.HandleNavigationFinished(ours, 1, "chrome://settings/")
	before := len(e.urls)
	if got := m.HandleNavigationStarting(ours, 2, "https://a.test/"); got != engine.NavigationElsewhere {
		t.Fatalf("to the web: %v", got)
	}
	opened := m.Active()
	if !slices.Equal(ids(m.AllIn(1)), []engine.TabID{web, ours, opened.ID, last}) || opened.Page != "" || opened.Workspace != "" ||
		opened.URL != "https://a.test/" || !slices.Equal(e.urls[before:], []string{"https://a.test/"}) {
		t.Fatalf("order %v, opened %+v, loads %v", ids(m.AllIn(1)), opened, e.urls[before:])
	}
	if tab, _ := m.Get(ours); tab.URL != "chrome://settings/" {
		t.Fatalf("our tab went with it: %+v", tab)
	}

	m.Activate(web)
	if got := m.HandleNavigationStarting(web, 3, "chrome://settings/"); got != engine.NavigationElsewhere || m.ActiveID() != ours || len(e.ours) != 1 {
		t.Fatalf("a web tab to our page: %v, active %v, ours made %v", got, m.ActiveID(), e.ours)
	}
	if tab, _ := m.Get(web); tab.URL != "" {
		t.Fatalf("the web tab went with it: %+v", tab)
	}
}

// An address of one of our pages asked for in a new tab, or typed in a window with no
// tabs, shows the window's tab of it, and opens no web tab for it.
func TestOpeningOurAddressShowsOurTab(t *testing.T) {
	e := &fakeEngine{}
	m := oneWindow(e)
	m.CreateIn(1, "")
	id := m.OpenIn("chrome://settings/", true)
	if tab, _ := m.Get(id); tab.Page != "settings" || len(m.AllIn(1)) != 2 || len(e.in) != 1 {
		t.Fatalf("opened %+v; tabs %v", tab, ids(m.AllIn(1)))
	}
	m.AddWindow(2)
	m.Navigate(2, "chrome://settings", work)
	if in := m.AllIn(2); len(in) != 1 || in[0].Page != "settings" || len(e.in) != 1 {
		t.Fatalf("typed in a window with no tabs: %+v", in)
	}
}

func TestATabSaysHowItsPageReachedIt(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	id := m.Create(1)
	changes := 0
	m.OnChanged(func(Tab) { changes++ })
	m.HandleConnectionChanged(id, engine.Connection{State: engine.ConnectionInsecure, Reason: "not-encrypted"})
	if tab, _ := m.Get(id); tab.Connection != "insecure" || !strings.Contains(tab.ConnectionNote, "isn't encrypted") {
		t.Fatalf("insecure: %+v", tab)
	}
	m.HandleConnectionChanged(id, engine.Connection{State: engine.ConnectionInsecure, Reason: "not-encrypted"})
	m.HandleConnectionChanged(id, engine.Connection{State: engine.ConnectionSecure})
	if tab, _ := m.Get(id); tab.Connection != "secure" || tab.ConnectionNote != "" || changes != 2 {
		t.Fatalf("secure: %+v after %d changes", tab, changes)
	}
}
