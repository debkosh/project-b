package app

import (
	"encoding/json"
	"time"

	"projectb/engine"
	"projectb/history"
	"projectb/timeline"
	"projectb/tones"
)

// The History, Bookmarks and Downloads pages (docs/daily-driver.md): pages of ours, each in
// a tab of ours, one per window. Go keeps what they show; each page asks for it and sends back what
// the user does.

// historyPage is how many visits the history page shows at a time.
const historyPage = 100

// ourPages keeps which tabs show the History, Bookmarks and Downloads pages, to send them
// changes.
type ourPages struct {
	a     *App
	e     engine.Engine
	shown map[engine.TabID]engine.Page
}

func newOurPages(a *App, e engine.Engine) *ourPages {
	p := &ourPages{a: a, e: e, shown: map[engine.TabID]engine.Page{}}
	a.Prefs.OnThemeChanged(func(t string) { p.sendAll("", map[string]any{"t": "theme", "theme": t}) })
	a.Window.OnColorChanged(func(c string) { p.sendAll("", map[string]any{"t": "tones", "tones": tones.Of(c)}) })
	a.Bookmarks.OnChanged(func() { p.sendAll("bookmarks", map[string]any{"t": "bookmarks", "bookmarks": a.Bookmarks.All()}) })
	a.Downloads.OnChanged(func() { p.sendAll("downloads", map[string]any{"t": "downloads", "downloads": a.Downloads.List()}) })
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
// items sends the history page its first page of items for text, after a change.
func (p *ourPages) items(tab engine.TabID, text string) {
	p.a.History.List(text, 0, historyPage, func(items []history.Entry) {
		p.send(tab, map[string]any{"t": "items", "text": text, "items": items})
	})
}

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

// pageMessage is what the pages send: t, and what each needs of text, a time, the
// addresses, a download's id, an address, and what to do to a download. What the
// Bookmarks page sends is a bookmarkMessage.
type pageMessage struct {
	T      string   `json:"t"`
	Text   string   `json:"text"`
	Before int64    `json:"before"`
	Since  int64    `json:"since"`
	URLs   []string `json:"urls"`
	ID     int      `json:"id"`
	URL    string   `json:"url"`
	Action string   `json:"action"`
}

func (p *ourPages) handle(tab engine.TabID, page engine.Page, msg []byte) {
	if page == "bookmarks" && p.handleBookmarks(tab, msg) {
		return
	}
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
		switch page {
		case "history":
			p.a.History.List("", 0, historyPage, func(items []history.Entry) {
				snapshot["items"] = items
				snapshot["now"] = time.Now().Unix()
				p.send(tab, snapshot)
			})
			return
		case "downloads":
			snapshot["downloads"] = p.a.Downloads.List()
		default:
			snapshot["bookmarks"] = p.a.Bookmarks.All()
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
		p.a.History.List(m.Text, m.Before, historyPage, func(items []history.Entry) {
			p.send(tab, map[string]any{"t": "items", "text": m.Text, "before": m.Before, "items": items})
		})
	case "delete":
		// The list comes after the delete: history's jobs run in the order asked.
		p.a.History.Delete(m.URLs...)
		p.items(tab, m.Text)
	case "clear":
		p.a.History.Clear(m.Since)
		p.items(tab, m.Text)
	case "downloadAction":
		p.a.Downloads.Act(m.ID, m.Action)
	case "clearDownloads":
		p.a.Downloads.Clear(0)
	default:
		timeline.Warnf("pages: %s sent %q, which it has no use for", page, m.T)
	}
}

// handleBookmarks does what the Bookmarks page asks of the tree (docs/bookmark-folders.md):
// editBookmark {id, title, url} renames a folder or edits a bookmark, moveBookmark {id,
// parent} moves an entry first into a folder, newFolder {parent, title} makes one there,
// and removeBookmark {id} takes an entry away with all it holds. A refusal goes back to
// the page as error {id, error}. It says whether the message was one of these.
func (p *ourPages) handleBookmarks(tab engine.TabID, msg []byte) bool {
	var m bookmarkMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		timeline.Warnf("pages: bad message from bookmarks: %v", err)
		return true
	}
	var err error
	switch m.T {
	case "editBookmark":
		err = p.a.Bookmarks.Edit(m.ID, m.Title, m.URL)
	case "moveBookmark":
		err = p.a.Bookmarks.Move(m.ID, m.Parent)
	case "newFolder":
		_, err = p.a.Bookmarks.NewFolder(m.Parent, m.Title)
	case "removeBookmark":
		p.a.Bookmarks.Remove(m.ID)
	default:
		return false
	}
	if err != nil {
		p.send(tab, map[string]any{"t": "error", "id": m.ID, "error": err.Error()})
	}
	return true
}
