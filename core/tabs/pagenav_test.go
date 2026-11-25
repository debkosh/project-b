package tabs

import (
	"testing"

	"projectb/engine"
)

// postNow runs what is posted at once, as the main loop would next.
type postNow struct{ *fakeEngine }

func (p postNow) Post(f func()) { f() }

// blockOthers is Links to other sites set to Block everywhere: a page may go only to its
// own host.
func blockOthers(asked *[]PageNavigation) func(PageNavigation) bool {
	return func(n PageNavigation) bool {
		*asked = append(*asked, n)
		return n.To != "" && host(n.From) == host(n.To)
	}
}

func host(u string) string {
	for i := len("https://"); i < len(u); i++ {
		if u[i] == '/' {
			return u[:i]
		}
	}
	return u
}

// What the person asks for goes without the guard; what the page starts is put to it, a
// redirect going by its navigation's start.
func TestOnlyThePagesOwnNavigationsMeetTheGuard(t *testing.T) {
	m := oneWindow(postNow{&fakeEngine{}})
	var asked []PageNavigation
	m.SetPageGuard(blockOthers(&asked))
	id := m.Create(1)
	m.Navigate(1, "https://shop.test/")
	if m.HandleNavigationStarting(id, 1, "https://shop.test/") != engine.NavigationProceed || len(asked) != 0 {
		t.Fatalf("the person's navigation met the guard: %v", asked)
	}
	// Its redirect to another site is still the person's.
	if m.HandleNavigationStarting(id, 1, "https://login.test/") != engine.NavigationProceed {
		t.Fatal("the person's navigation redirected elsewhere was refused")
	}
	m.HandleNavigationFinished(id, 1, "https://shop.test/")
	m.HandleTitleChanged(id, "Shop")
	if m.HandleNavigationStarting(id, 2, "https://ads.test/") != engine.NavigationElsewhere {
		t.Fatal("the page's link to another site went")
	}
	if tab, _ := m.Get(id); tab.URL != "https://shop.test/" || tab.Title != "Shop" {
		t.Fatalf("the refused navigation changed the tab: %q %q", tab.URL, tab.Title)
	}
	if m.HandleNavigationStarting(id, 3, "https://shop.test/cart") != engine.NavigationProceed {
		t.Fatal("the page's link to its own site was refused")
	}
	// A redirect of the page's own navigation is the page's.
	if m.HandleNavigationStarting(id, 3, "https://ads.test/") != engine.NavigationElsewhere {
		t.Fatal("the page's navigation redirected to another site went")
	}
}

// A tab the page opens goes by its opener's page until it has a page of its own: one on
// another site, or the blank page, closes at once, counted on the opener, which shows
// again; one on the blank page that the page later sends elsewhere closes then.
func TestATabThePageOpensGoesByItsOpener(t *testing.T) {
	f := &fakeEngine{}
	m := oneWindow(postNow{f})
	var asked []PageNavigation
	m.SetPageGuard(blockOthers(&asked))
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://shop.test/")
	m.HandleNavigationFinished(id, 1, "https://shop.test/")

	m.HandleTabOpened(id, 50, "https://ads.test/", "", true, true)
	if _, ok := m.Get(50); ok {
		t.Fatal("a popup to another site stayed open")
	}
	if m.HandleNavigationStarting(50, 7, "https://ads.test/") == engine.NavigationProceed {
		t.Fatal("the closed popup loaded")
	}
	if tab, _ := m.Get(id); tab.PopupsBlocked != 1 || m.ActiveID() != id {
		t.Fatalf("the opener counts %d, the active tab is %d", tab.PopupsBlocked, m.ActiveID())
	}

	m.HandleTabOpened(id, 51, "https://shop.test/help", "", true, true)
	if _, ok := m.Get(51); !ok || m.HandleNavigationStarting(51, 8, "https://shop.test/help") != engine.NavigationProceed {
		t.Fatal("a popup to the page's own site was refused")
	}

	// window.open(), on the blank page: refused at once under Block.
	m.HandleTabOpened(id, 52, "", "", true, true)
	if _, ok := m.Get(52); ok {
		t.Fatal("a blank popup stayed open")
	}
	if tab, _ := m.Get(id); tab.PopupsBlocked != 2 {
		t.Fatalf("the opener counts %d", tab.PopupsBlocked)
	}
	// A new page in the opener starts its count again.
	m.HandleNavigationStarting(id, 2, "https://shop.test/next")
	m.HandleNavigationFinished(id, 2, "https://shop.test/next")
	if tab, _ := m.Get(id); tab.PopupsBlocked != 0 {
		t.Fatalf("the count outlived its page: %d", tab.PopupsBlocked)
	}
}

// A tab the page opens with no tap first is put to the guard as a pop-up, and only as it
// opens; one after a tap isn't. A refused one is counted on its opener, which keeps its
// address, and Open it opens that beside the opener as the person's, counted no more.
func TestAPopupMeetsTheGuardAsItOpensAndOpensOnce(t *testing.T) {
	m := oneWindow(postNow{&fakeEngine{}})
	var asked []PageNavigation
	m.SetPageGuard(func(n PageNavigation) bool {
		asked = append(asked, n)
		return !n.Untapped
	})
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://shop.test/")
	m.HandleNavigationFinished(id, 1, "https://shop.test/")
	asked = nil

	m.HandleTabOpened(id, 50, "https://shop.test/help", "", true, true)
	if _, ok := m.Get(50); !ok || len(asked) != 1 || asked[0].Untapped {
		t.Fatalf("a tab opened after a tap: %v", asked)
	}
	// Its own navigations later are no pop-up.
	if m.HandleNavigationStarting(50, 2, "https://shop.test/help") != engine.NavigationProceed || asked[1].Untapped {
		t.Fatalf("a tapped tab's load met the guard as a pop-up: %v", asked)
	}
	m.Activate(id)

	m.HandleTabOpened(id, 51, "https://ads.test/win", "", true, false)
	if _, ok := m.Get(51); ok {
		t.Fatal("a pop-up stayed open")
	}
	if m.HandleNavigationStarting(51, 3, "https://ads.test/win") == engine.NavigationProceed {
		t.Fatal("the closed pop-up loaded")
	}
	tab, _ := m.Get(id)
	if tab.PopupsBlocked != 1 || tab.BlockedPopup != "https://ads.test/win" || m.ActiveID() != id {
		t.Fatalf("the opener counts %d, keeps %q, the active tab is %d", tab.PopupsBlocked, tab.BlockedPopup, m.ActiveID())
	}
	// A blank one counts, and keeps the last address with a page.
	m.HandleTabOpened(id, 52, "", "", true, false)
	if tab, _ := m.Get(id); tab.PopupsBlocked != 2 || tab.BlockedPopup != "https://ads.test/win" {
		t.Fatalf("after a blank pop-up the opener counts %d, keeps %q", tab.PopupsBlocked, tab.BlockedPopup)
	}

	asked = nil
	m.OpenBlockedPopup(id)
	opened := m.ActiveID()
	if o, _ := m.Get(opened); opened == id || o.URL != "https://ads.test/win" {
		t.Fatalf("Open it showed tab %d at %q", opened, o.URL)
	}
	if m.HandleNavigationStarting(opened, 4, "https://ads.test/win") != engine.NavigationProceed || len(asked) != 0 {
		t.Fatalf("the person's Open it met the guard: %v", asked)
	}
	if tab, _ := m.Get(id); tab.PopupsBlocked != 1 || tab.BlockedPopup != "" {
		t.Fatalf("after Open it the opener counts %d, keeps %q", tab.PopupsBlocked, tab.BlockedPopup)
	}
	// Nothing left to open: nothing opens.
	n := len(m.All())
	m.OpenBlockedPopup(id)
	if len(m.All()) != n {
		t.Fatal("Open it with nothing kept opened a tab")
	}
}

// A blank popup a guard lets open, which the page then sends to another site, goes by its
// opener's page: the blank page is no page of its own.
func TestABlankPopupSentElsewhereGoesByItsOpener(t *testing.T) {
	m := oneWindow(postNow{&fakeEngine{}})
	allowBlank := func(n PageNavigation) bool { return n.To == "" || host(n.From) == host(n.To) }
	m.SetPageGuard(allowBlank)
	id := m.Create(1)
	m.HandleNavigationStarting(id, 1, "https://shop.test/")
	m.HandleNavigationFinished(id, 1, "https://shop.test/")
	m.HandleTabOpened(id, 60, "", "", true, true)
	m.HandleNavigationFinished(60, 0, BlankURL)
	if m.HandleNavigationStarting(60, 9, "https://ads.test/") != engine.NavigationElsewhere {
		t.Fatal("a blank popup sent to another site went")
	}
	if _, ok := m.Get(60); ok {
		t.Fatal("it stayed open")
	}
}

// A load Go asked for whose start it never heard, as one the engine began before Go
// listened, leaves no exemption behind: the page's next navigation, elsewhere, meets the
// guard. Back and Forward, whose address Go doesn't know, are the person's at any start.
func TestAStartGoNeverHeardLeavesNoExemption(t *testing.T) {
	m := oneWindow(postNow{&fakeEngine{}})
	var asked []PageNavigation
	m.SetPageGuard(blockOthers(&asked))
	id := m.Create(1)
	m.Navigate(1, "https://shop.test")
	m.HandleNavigationFinished(id, 2, "https://shop.test/")
	if m.HandleNavigationStarting(id, 3, "https://ads.test/") != engine.NavigationElsewhere {
		t.Fatal("the page's link went on the person's left-over mark")
	}
	m.GoBackIn(id)
	if m.HandleNavigationStarting(id, 4, "https://elsewhere.test/") != engine.NavigationProceed {
		t.Fatal("Back to another site was put to the guard")
	}
	// A load's own start, its root without the slash it was asked with.
	m.Navigate(1, "https://login.test")
	if m.HandleNavigationStarting(id, 5, "https://login.test/") != engine.NavigationProceed {
		t.Fatal("the person's load was put to the guard")
	}
}
