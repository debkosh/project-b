package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
	"projectb/timeline"
)

// bookmarkMessage is what the Bookmarks page sends about an entry: its id, a title and
// address, and a folder. The star's panel names its bookmark as bookmark, as a shell's
// message has a number for id.
type bookmarkMessage struct {
	T        string `json:"t"`
	ID       string `json:"id"`
	Bookmark string `json:"bookmark"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Parent   string `json:"parent"`
}

// bookmarksShellPart gives every shell the profile's bookmarks and folders, for the star on
// an address that is bookmarked and the panel it opens, and takes what they do
// (docs/bookmark-folders.md): bookmark {url, title}, the star, bookmarks the page into the
// folder last saved in and shows the panel; saveBookmark {bookmark, title, parent}, the panel's
// Enter; removeBookmark {bookmark}, its Remove.
func (a *App) bookmarksShellPart(e engine.Engine) shell.Part {
	a.Bookmarks.OnChanged(func() { a.Shell.Send(map[string]any{"t": "bookmarks", "bookmarks": a.Bookmarks.All()}) })
	return shell.Part{
		Snapshot: func(into map[string]any) { into["bookmarks"] = a.Bookmarks.All() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var m bookmarkMessage
			switch t {
			case "bookmark":
				json.Unmarshal(msg, &m)
				a.bookmarkPage(e, w, m.Title, m.URL)
			case "saveBookmark":
				json.Unmarshal(msg, &m)
				if err := a.Bookmarks.Save(m.Bookmark, m.Title, m.Parent); err != nil {
					timeline.Warnf("%v", err)
				}
			case "removeBookmark":
				json.Unmarshal(msg, &m)
				a.Bookmarks.Remove(m.Bookmark)
			default:
				return false
			}
			return true
		},
	}
}

// bookmarkPage bookmarks an address into the folder last saved in, or finds its bookmark,
// and has window w's shell show the star's panel for it, bookmarkPanel {id, new}, with the
// focus, so the panel's Enter and Escape reach it. A shell with no panel has the bookmark
// saved all the same.
func (a *App) bookmarkPage(e engine.Engine, w engine.WindowID, title, url string) {
	added := !a.Bookmarks.Has(url)
	b, err := a.Bookmarks.Add(title, url)
	if err != nil {
		timeline.Warnf("%v", err)
		return
	}
	e.FocusShell(w)
	a.Shell.SendTo(w, map[string]any{"t": "bookmarkPanel", "bookmarkPanel": map[string]any{"id": b.ID, "new": added}})
}
