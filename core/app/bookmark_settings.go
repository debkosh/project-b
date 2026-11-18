package app

import (
	"encoding/json"
	"projectb/engine"
	"projectb/prefs"
	"projectb/settings"
	"projectb/timeline"
)

// bookmarkSettingsPart is Settings' Bookmarks: where bookmarks show and the bar's folder
// (docs/bookmark-folders.md). The page gets Go's prefs, all of them, as a shell does, and
// the bookmarks for the bar's folder list, and sets a pref by name, as a shell's setPref
// does, through the same setters.
func (a *App) bookmarkSettingsPart() settings.Part {
	a.Prefs.OnChanged(func(p prefs.Prefs) { a.Settings.Send(map[string]any{"t": "prefs", "prefs": p}) })
	a.Bookmarks.OnChanged(func() { a.Settings.Send(map[string]any{"t": "bookmarks", "bookmarks": a.Bookmarks.All()}) })
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["prefs"] = a.Prefs.All()
			into["bookmarks"] = a.Bookmarks.All()
		},
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			if t != "setPref" {
				return false
			}
			var c struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}
			if json.Unmarshal(msg, &c) != nil || !a.Prefs.Set(c.Name, c.Value) {
				timeline.Warnf("settings: setPref: no pref %q", c.Name)
			}
			return true
		},
	}
}
