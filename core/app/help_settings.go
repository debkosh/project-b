package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/links"
	"projectb/settings"
)

// helpSettingsPart opens a help page (core/links) by its name beside Settings, in a new tab
// of its workspace, as Learn more's Read more asks; a name with no page opens nothing.
func (a *App) helpSettingsPart() settings.Part {
	return settings.Part{
		// The page knows the names; the addresses are Go's, so the snapshot has nothing.
		Snapshot: func(map[string]any) {},
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			if t != "openHelp" {
				return false
			}
			var m struct {
				Page string `json:"page"`
			}
			json.Unmarshal(msg, &m)
			if u := links.Help(m.Page); u != "" {
				if t, ok := a.Tabs.Get(tab); ok {
					a.Tabs.OpenFrom(tab, u, t.Workspace, true)
				}
			}
			return true
		},
	}
}
