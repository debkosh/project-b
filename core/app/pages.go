package app

import (
	"encoding/json"
	"time"

	"projectb/engine"
	"projectb/timeline"
	"projectb/tones"
)

// The History and Bookmarks pages (docs/daily-driver.md): pages of ours, each in a tab of
// ours, one per window. Go keeps what they show; each page asks for it and sends back what
// the user does.

// historyPage is how many visits the history page shows at a time.
const historyPage = 100

// ourPages keeps which tabs show the History and Bookmarks pages, to send them changes.
type ourPages struct {
	a     *App
	e     engine.Engine
	shown map[engine.TabID]engine.Page
}

func newOurPages(a *App, e engine.Engine) *ourPages {
	p := &ourPages{a: a, e: e, shown: map[engine.TabID]engine.Page{}}
	a.Prefs.OnThemeChanged(func(t string) { p.sendAll("", map[string]any{"t": "theme", "theme": t}) })
	a.Window.OnColorChanged(func(c string) { p.sendAll("", map[string]any{"t": "tones", "tones": tones.Of(c)}) })
	a.Bookmarks.OnChanged(func() { p.sendAll("bookmarks", map[string]any{"t": "bookmarks", "bookmarks": a.Bookmarks.List()}) })
	return p
}

func (p *ourPages) send(tab engine.TabID, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		timeline.Errorf("pages: %v", err)
		return
	}
	p.e.SendToPage(tab, b)
}

// sendAll sends to every tab showing page, or every one of these pages for "", forgetting
// the tabs gone.
func (p *ourPages) sendAll(page engine.Page, v any) {
	for tab, shown := range p.shown {
		if _, ok := p.a.Tabs.Get(tab); !ok {
			delete(p.shown, tab)
			continue
		}
		if page == "" || shown == page {
			p.send(tab, v)
		}
	}
}

// pageMessage is what the two pages send: t, and what each needs of text, a time, the
// addresses, a bookmark's id, title and address, and whether to open in a new tab.
type pageMessage struct {
	T      string   `json:"t"`
	Text   string   `json:"text"`
	Before int64    `json:"before"`
	Since  int64    `json:"since"`
	URLs   []string `json:"urls"`
	ID     int      `json:"id"`
	Title  string   `json:"title"`
	URL    string   `json:"url"`
}

func (p *ourPages) handle(tab engine.TabID, page engine.Page, msg []byte) {
	var m pageMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		timeline.Warnf("pages: bad message from %s: %v", page, err)
		return
	}
	if m.T == "timeline" {
		timeline.FromPage(string(page), msg)
		return
	}
	if m.T == "ready" {
		p.shown[tab] = page
		snapshot := map[string]any{"t": "snapshot", "theme": p.a.Prefs.Theme(), "tones": tones.Of(p.a.Window.Color())}
		if page == "history" {
			snapshot["items"] = p.a.History.List("", 0, historyPage)
			snapshot["now"] = time.Now().Unix()
		} else {
			snapshot["bookmarks"] = p.a.Bookmarks.List()
		}
		p.send(tab, snapshot)
		return
	}
	switch m.T {
	case "open":
		// A page of ours opens an address beside itself, in a new tab of the workspace.
		if t, ok := p.a.Tabs.Get(tab); ok && m.URL != "" {
			p.a.Tabs.OpenFrom(tab, m.URL, t.Workspace, true)
		}
	case "search", "more":
		p.send(tab, map[string]any{"t": "items", "text": m.Text, "before": m.Before, "items": p.a.History.List(m.Text, m.Before, historyPage)})
	case "delete":
		p.a.History.Delete(m.URLs...)
		p.send(tab, map[string]any{"t": "items", "text": m.Text, "items": p.a.History.List(m.Text, 0, historyPage)})
	case "clear":
		p.a.History.Clear(m.Since)
		p.send(tab, map[string]any{"t": "items", "text": m.Text, "items": p.a.History.List(m.Text, 0, historyPage)})
	case "editBookmark":
		if err := p.a.Bookmarks.Edit(m.ID, m.Title, m.URL); err != nil {
			p.send(tab, map[string]any{"t": "error", "id": m.ID, "error": err.Error()})
		}
	case "removeBookmark":
		p.a.Bookmarks.Remove(m.ID)
	default:
		timeline.Warnf("pages: %s sent %q, which it has no use for", page, m.T)
	}
}
