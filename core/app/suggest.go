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
	}
}

// suggestShellPart runs the row a shell's user chose: an address or a search loads in the
// window's active tab, or a new one; a tab is shown, in whichever window it is; an action
// runs.
func (a *App) suggestShellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
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
			case suggest.KindAddress, suggest.KindSearch, suggest.KindHistory:
				switch {
				case c.Row.URL == "":
				case c.NewTab || active == 0:
					// In the active tab's workspace, or the first in a window with no tab.
					workspace := a.Tabs.FirstWorkspace()
					if t, ok := a.Tabs.Get(active); ok {
						workspace = t.Workspace
					}
					a.Tabs.OpenFrom(active, c.Row.URL, workspace, true)
				default:
					a.Tabs.Load(active, c.Row.URL)
				}
			}
			return true
		},
	}
}
