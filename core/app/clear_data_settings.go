package app

import (
	"encoding/json"
	"slices"

	"projectb/engine"
	"projectb/settings"
	"projectb/timeline"
)

// clearDataSettingsPart is Settings' Clear browsing data (docs/settings.md): what the person
// ticked, all of it, from the start. History and the downloads list are Go's own. The
// cookies, site storage and cache are each workspace's, and the profile's own, in the
// engine, erased as a deleted workspace's are but with their tabs left open, as every
// browser's Clear browsing data leaves them: their pages stay, with nothing kept. The page
// hears what went.
func (a *App) clearDataSettingsPart(e engine.Engine) settings.Part {
	return settings.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			if t != "clearData" {
				return false
			}
			var c struct {
				History   bool `json:"history"`
				SiteData  bool `json:"siteData"`
				Downloads bool `json:"downloads"`
			}
			if err := json.Unmarshal(msg, &c); err != nil {
				timeline.Warnf("settings: clearData: %v", err)
				return true
			}
			if c.History {
				a.History.Clear(0)
			}
			if c.Downloads {
				a.Downloads.Clear()
			}
			if c.SiteData {
				// The profile's own storage, "", whether or not Main is still listed: the
				// engine's own tabs, an extension's, open there.
				ids := []string{""}
				for _, w := range a.Workspaces.List() {
					if !slices.Contains(ids, w.ID) {
						ids = append(ids, w.ID)
					}
				}
				for _, id := range ids {
					e.EraseWorkspace(id)
				}
			}
			timeline.Infof("settings: cleared history %v, site data %v, the downloads list %v", c.History, c.SiteData, c.Downloads)
			a.Settings.Send(map[string]any{"t": "dataCleared", "history": c.History, "siteData": c.SiteData, "downloads": c.Downloads})
			return true
		},
	}
}
