package tabs

import (
	"slices"
	"testing"

	"projectb/engine"
)

// A page made ahead loads out of sight and is no tab to anyone: not in its window's tabs or
// recent ones, not activated by its page taking focus, not closed by Close, and nobody hears
// of it. OpenPage shows it beside the active tab, without loading it again, and from then on
// it is the window's tab of that page, as one opened at once is.
func TestAPageMadeAheadIsNoTabUntilOpened(t *testing.T) {
	e := &loads{}
	m := oneWindow(e)
	a, b := m.CreateIn(1, ""), m.Create(1)
	m.Activate(a)
	lists, changes := 0, 0
	m.OnListChanged(func() { lists++ })
	m.OnChanged(func(Tab) { changes++ })
	heard := 0
	m.OnNavigationFinished(func(Tab) { heard++ })
	m.OnTitleChanged(func(Tab) { heard++ })

	m.PreparePage(1, "settings")
	held := m.Prepared(1, "settings")
	if held == 0 || !slices.Equal(e.ours, []engine.TabID{held}) || e.urls[len(e.urls)-1] != "chrome://settings/" {
		t.Fatalf("made ahead %v, in our context %v, loads %v", held, e.ours, e.urls)
	}
	m.PreparePage(1, "settings")
	if len(e.ours) != 1 {
		t.Fatalf("made ahead twice: %v", e.ours)
	}
	m.HandleNavigationStarting(held, 1, "chrome://settings/")
	m.HandleLoadingChanged(held, true)
	m.HandleNavigationFinished(held, 1, "chrome://settings/")
	m.HandleTitleChanged(held, "Settings")
	m.HandleLoadingChanged(held, false)
	m.HandleTabFocused(held)
	m.Close(held)
	if got := ids(m.AllIn(1)); !slices.Equal(got, []engine.TabID{a, b}) || m.ActiveID() != a || len(m.RecentIn(1)) != 1 || m.PageIn(1, "settings") != 0 {
		t.Fatalf("tabs %v, active %v, recent %v", got, m.ActiveID(), ids(m.RecentIn(1)))
	}
	if lists != 0 || changes != 0 || heard != 0 || len(e.closed) != 0 || len(m.All()) != 2 || m.CountByWorkspace()[""] != 2 {
		t.Fatalf("heard of it: lists %d, changes %d, navigations and titles %d, closed %v", lists, changes, heard, e.closed)
	}

	loads := len(e.urls)
	s := m.OpenPage(1, "settings")
	tab, _ := m.Get(s)
	if s != held || !slices.Equal(ids(m.AllIn(1)), []engine.TabID{a, s, b}) || m.ActiveID() != s || m.WindowOf(s) != 1 || len(e.urls) != loads || m.Prepared(1, "settings") != 0 {
		t.Fatalf("opened %v (made ahead %v): tabs %v, active %v, loads %v", s, held, ids(m.AllIn(1)), m.ActiveID(), e.urls[loads:])
	}
	if tab.Title != "Settings" || tab.Page != "settings" || tab.Workspace != "" || lists == 0 {
		t.Fatalf("the tab is %+v, lists %d", tab, lists)
	}
	m.PreparePage(1, "settings")
	if m.Prepared(1, "settings") != 0 || len(e.ours) != 1 {
		t.Fatalf("made ahead beside the shown one: %v", e.ours)
	}
	// Closed, it goes as any tab does; the next is made ahead again.
	m.Close(s)
	if len(m.AllIn(1)) != 2 || !slices.Equal(e.closed, []engine.TabID{s}) {
		t.Fatalf("closed: tabs %v, engine closed %v", ids(m.AllIn(1)), e.closed)
	}
	m.PreparePage(1, "settings")
	if again := m.Prepared(1, "settings"); again == 0 || again == s {
		t.Fatalf("made ahead after closing: %v", again)
	}
}

// A page made ahead takes the workspace of the tab it is opened from, which is the active
// one when OpenPage runs, not when it was made.
func TestAPageMadeAheadRemembersTheTabItWasOpenedFrom(t *testing.T) {
	m := oneWindow(&fakeEngine{})
	inMain := m.CreateIn(1, "")
	inWork := m.CreateIn(1, work)
	m.PreparePage(1, "settings")
	m.Activate(inMain)
	s := m.OpenPage(1, "settings")
	if tab, _ := m.Get(s); tab.Workspace != "" || inWork == 0 {
		t.Fatalf("opened from Main, it remembers %q", tab.Workspace)
	}
}

// A page made ahead goes with its window, closed with nothing heard but its page's own
// OnClosed; one whose page crashed meanwhile is closed when opened, and a new one opens.
// A window that gets a tab of the page by a move drops its own made ahead.
func TestAPageMadeAheadGoesWithItsWindowOrItsCrash(t *testing.T) {
	e := &loads{}
	m := oneWindow(e)
	m.CreateIn(1, "")
	var closed []engine.TabID
	m.OnClosed(func(t Tab) { closed = append(closed, t.ID) })
	closedAt := 0
	m.OnClosedAt(func(Tab, engine.WindowID, int) { closedAt++ })

	m.PreparePage(1, "settings")
	crashed := m.Prepared(1, "settings")
	m.HandlePageCrashed(crashed)
	s := m.OpenPage(1, "settings")
	if s == crashed || !slices.Contains(e.closed, crashed) || !slices.Equal(closed, []engine.TabID{crashed}) || m.ActiveID() != s {
		t.Fatalf("opened %v after %v crashed: engine closed %v, heard closed %v", s, crashed, e.closed, closed)
	}

	m.AddWindow(2)
	m.CreateIn(2, "")
	m.PreparePage(2, "settings")
	held := m.Prepared(2, "settings")
	m.Move(s, 2, 0)
	if m.Prepared(2, "settings") != 0 || !slices.Contains(e.closed, held) || m.PageIn(2, "settings") != s {
		t.Fatalf("moved in: made ahead %v, engine closed %v", m.Prepared(2, "settings"), e.closed)
	}

	m.PreparePage(1, "settings")
	held = m.Prepared(1, "settings")
	m.RemoveWindow(1)
	if !slices.Contains(e.closed, held) || !slices.Contains(closed, held) || closedAt != 0 {
		t.Fatalf("its window closed: engine closed %v, heard closed %v, closed tabs kept %d", e.closed, closed, closedAt)
	}
}
