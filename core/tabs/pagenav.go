package tabs

import (
	"net/url"
	"strings"

	"projectb/engine"
)

// A tab's navigations are the person's or the page's (docs/site-features.md, Links to
// other sites). The person's reach the engine through Go: an address, Back, Forward,
// Reload, a tab opened from our menus, a tab restored. Every other navigation that starts
// in a tab is the page's: a link pressed in it, a script, a form, a redirect, a window it
// opens. A page's navigation is put to the page guard, which says whether it may go.

// PageNavigation is a navigation a page started: in its own tab, or in a tab it opened
// (Opener, the tab whose page opened it; its page is From). Untapped is a tab the page
// opened with no real tap first, a pop-up, as it opens.
type PageNavigation struct {
	Tab      engine.TabID
	From     string
	To       string
	Opener   engine.TabID
	Untapped bool
}

// SetPageGuard sets what decides each page's navigation: false drops it. A navigation in
// place is dropped with nothing shown and nothing kept, the page staying as it was; a tab
// the page opened is closed, as if it never opened, and its opener counts it in
// PopupsBlocked.
func (m *Manager) SetPageGuard(f func(PageNavigation) bool) { m.guard = f }

// own is the engine as the tabs manager calls it: each navigation it asks for in a tab is
// the person's, marked so that its start isn't put to the guard. A load's mark is its
// address's host, and only a start there is it: a start Go never hears (one the engine began
// before Go listened) leaves a mark behind, which a page's start elsewhere drops. A move in
// the history, whose address Go doesn't know, is any start's.
type own struct {
	engine.Engine
	issued map[engine.TabID]string
}

// anyStart marks a move in a tab's history: Back, Forward, Reload.
const anyStart = "*"

func (o own) LoadURL(tab engine.TabID, url string) {
	o.issued[tab] = normal(url)
	o.Engine.LoadURL(tab, url)
}
func (o own) GoBack(tab engine.TabID)    { o.issued[tab] = anyStart; o.Engine.GoBack(tab) }
func (o own) GoForward(tab engine.TabID) { o.issued[tab] = anyStart; o.Engine.GoForward(tab) }
func (o own) Reload(tab engine.TabID)    { o.issued[tab] = anyStart; o.Engine.Reload(tab) }
func (o own) ReloadBypassingCache(tab engine.TabID) {
	o.issued[tab] = anyStart
	o.Engine.ReloadBypassingCache(tab)
}

// asked says whether a start at u in tab is the one Go asked for, and drops the mark.
func (o own) asked(tab engine.TabID, u string) bool {
	mark, ok := o.issued[tab]
	delete(o.issued, tab)
	return ok && (mark == anyStart || mark == normal(u))
}

// normal is what a load's mark keeps of its address: its host, lower case, which the
// engine reports as Go asked for it however it writes the rest. A left-over mark a page's
// start to the same host takes is no harm: that is the page's own site, which goes anyway.
func normal(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.ToLower(u.Host)
}

// GoBackIn and GoForwardIn move tab's page back and forward: the person's, as from a menu
// on the page.
func (m *Manager) GoBackIn(tab engine.TabID)    { m.eng.GoBack(tab) }
func (m *Manager) GoForwardIn(tab engine.TabID) { m.eng.GoForward(tab) }

// pageMay puts a navigation starting in tab t to the guard, unless the person asked for
// it, and says whether it may go. A tab a page opened goes by its opener's page until it
// has a page of its own; a refused one closes.
func (m *Manager) pageMay(t *Tab, nav int64, u string) bool {
	if nav == t.started {
		// A redirect is its navigation's: the person's stays theirs.
		if !t.byPage {
			return true
		}
	} else {
		t.byPage = !m.own.asked(t.ID, u)
	}
	if t.refused {
		return false
	}
	if !t.byPage || m.guard == nil {
		return true
	}
	return m.pageMayOpen(t, u, false)
}

// pageMayOpen puts a page's navigation of t to u to the guard, closing t when it is a tab
// the page opened that the guard refuses. untapped is a pop-up as it opens.
func (m *Manager) pageMayOpen(t *Tab, u string, untapped bool) bool {
	n := PageNavigation{Tab: t.ID, From: t.URL, To: u, Untapped: untapped}
	if t.opener != 0 && !t.ownPage {
		n.From, n.Opener = t.openerURL, t.opener
	}
	if m.guard(n) {
		return true
	}
	if n.Opener != 0 {
		t.refused = true
		m.blockPopup(t.ID, n.Opener, u)
	}
	return false
}

// blockPopup closes a tab a page opened, after the engine's question it was asked in, and
// counts it on its opener, which shows again and keeps u for Open it.
func (m *Manager) blockPopup(id, opener engine.TabID, u string) {
	if o := m.tabs[opener]; o != nil {
		o.PopupsBlocked++
		if u != "" && u != "about:blank" {
			o.BlockedPopup = u
		}
		m.changed(o)
	}
	m.post(func() {
		m.Close(id)
		if _, ok := m.tabs[opener]; ok {
			m.Activate(opener)
		}
	})
}

func (m *Manager) post(f func()) { m.eng.Post(f) }

// OpenBlockedPopup opens the latest pop-up tab's page blocked, beside it, as the person's:
// Open it. It counts as blocked no longer.
func (m *Manager) OpenBlockedPopup(tab engine.TabID) {
	t := m.tabs[tab]
	if t == nil || t.BlockedPopup == "" {
		return
	}
	u := t.BlockedPopup
	t.BlockedPopup = ""
	t.PopupsBlocked = max(t.PopupsBlocked-1, 0)
	m.changed(t)
	m.OpenBeside(tab, u)
}
