package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
)

// bookmarksShellPart gives every shell the profile's bookmarks, for a star on the address
// that is bookmarked, and takes a shell's star: bookmark {url, title, on} (docs/daily-driver.md).
func (a *App) bookmarksShellPart() shell.Part {
	a.Bookmarks.OnChanged(func() { a.Shell.Send(map[string]any{"t": "bookmarks", "bookmarks": a.Bookmarks.List()}) })
	return shell.Part{
		Snapshot: func(into map[string]any) { into["bookmarks"] = a.Bookmarks.List() },
		Handle: func(_ engine.WindowID, t string, msg []byte) bool {
			if t != "bookmark" {
				return false
			}
			var c struct {
				URL   string `json:"url"`
				Title string `json:"title"`
				On    bool   `json:"on"`
			}
			json.Unmarshal(msg, &c)
			if c.On {
				a.Bookmarks.Add(c.Title, c.URL)
			} else {
				a.Bookmarks.RemoveURL(c.URL)
			}
			return true
		},
	}
}

// toggleBookmark bookmarks the active tab's page, or takes its bookmark away.
func (a *App) toggleBookmark() {
	t := a.Tabs.Active()
	if t.URL == "" {
		return
	}
	if a.Bookmarks.Has(t.URL) {
		a.Bookmarks.RemoveURL(t.URL)
	} else {
		a.Bookmarks.Add(t.Title, t.URL)
	}
}
