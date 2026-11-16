package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
	"projectb/suggest"
)

// suggestSources gives the suggestion list its sources beside history: every open tab of
// the profile, with its workspace's name; the actions that can run now, with their keys;
// and the search engine (docs/daily-driver.md).
func (a *App) suggestSources() suggest.Sources {
	return suggest.Sources{
		Tabs: func() []suggest.Tab {
			names := map[string]string{}
			for _, w := range a.Workspaces.List() {
				names[w.ID] = w.Name
			}
			all := a.Tabs.All()
			out := make([]suggest.Tab, len(all))
			for i, t := range all {
				out[i] = suggest.Tab{ID: int64(t.ID), Title: t.Title, URL: t.URL, Detail: names[t.Workspace]}
			}
			return out
		},
		Actions: func() []suggest.Action {
			keys := a.Commands.ShownKeys()
			titles := a.Commands.Titles()
			out := make([]suggest.Action, len(titles))
			for i, t := range titles {
				out[i] = suggest.Action{ID: t.ID, Title: t.Title, Detail: keys[t.ID]}
			}
			return out
		},
		Engine: func() (string, func(string) string) {
			e := a.Prefs.SearchEngine()
			return e.Name, e.For
		},
		Bookmarks: func() []suggest.Bookmark {
			list := a.Bookmarks.List()
			out := make([]suggest.Bookmark, len(list))
			for i, b := range list {
				out[i] = suggest.Bookmark{Title: b.Title, URL: b.URL}
			}
			return out
		},
	}
}

// suggestShellPart runs the row a shell's user chose: an address, a search, a page or a
// bookmark loads in the window's active tab, or a new one; a tab is shown, in whichever
// window it is; an action runs. forgetSite takes a site the new tab offers out of history,
// every page of it.
func (a *App) suggestShellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			if t == "forgetSite" {
				var c struct {
					URL string `json:"url"`
				}
				json.Unmarshal(msg, &c)
				a.History.DeleteSite(c.URL)
				return true
			}
			if t != "chooseSuggestion" {
				return false
			}
			var c struct {
				Row    suggest.Row `json:"row"`
				NewTab bool        `json:"newTab"`
			}
			json.Unmarshal(msg, &c)
			active := a.Tabs.ActiveIn(w)
			switch c.Row.Kind {
			case suggest.KindTab:
				if _, ok := a.Tabs.Get(engine.TabID(c.Row.Tab)); ok {
					a.ActivateRequested(engine.TabID(c.Row.Tab))
				}
			case suggest.KindAction:
				a.Commands.Run(c.Row.Action)
			case suggest.KindAddress, suggest.KindSearch, suggest.KindHistory, suggest.KindBookmark:
				switch {
				case c.Row.URL == "":
				case c.NewTab || active == 0:
					// Where New Tab opens one: the active tab's workspace, or for a tab of ours
					// the one it was opened from, or the first when that is gone or the window
					// has no tab.
					a.Tabs.OpenFrom(active, c.Row.URL, a.Tabs.NewTabWorkspace(w), true)
				default:
					a.Tabs.Load(active, c.Row.URL)
				}
			}
			return true
		},
	}
}
