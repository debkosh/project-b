package app

import (
	"encoding/json"
	"slices"
	"time"

	"projectb/engine"
	"projectb/settings"
	"projectb/timeline"
)

// clearRanges are the times Clear browsing data clears from, by the page's names for
// them: how far back each reaches, 0 for from the start.
var clearRanges = map[string]time.Duration{
	"hour":      time.Hour,
	"day":       24 * time.Hour,
	"week":      7 * 24 * time.Hour,
	"fourWeeks": 28 * 24 * time.Hour,
	"all":       0,
}

// clearSince is the Unix time a range starts at, now, and 0 for from the start or a name
// Go doesn't know.
func clearSince(name string, now time.Time) int64 {
	d := clearRanges[name]
	if d == 0 {
		return 0
	}
	return now.Add(-d).Unix()
}

// clearDataSettingsPart is Settings' Clear browsing data (docs/settings.md): what the person
// ticked, from the time they chose. History and the downloads list are Go's own. The
// cookies and site storage, and apart from them the cache, are each workspace's, and the
// profile's own, in the engine, erased as a deleted workspace's are but with their tabs
// left open, as every browser's Clear browsing data leaves them: their pages stay, with
// nothing kept. The page asks how many pages each range would take, and hears what went.
func (a *App) clearDataSettingsPart(e engine.Engine) settings.Part {
	return settings.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			switch t {
			case "clearDataCounts":
				now := time.Now()
				pages := map[string]int{}
				for name := range clearRanges {
					pages[name] = a.History.Count(clearSince(name, now))
				}
				a.Settings.Send(map[string]any{"t": "clearDataCounts", "pages": pages})
				return true
			case "clearData":
			default:
				return false
			}
			var c struct {
				Since     string `json:"since"`
				History   bool   `json:"history"`
				SiteData  bool   `json:"siteData"`
				Cache     bool   `json:"cache"`
				Downloads bool   `json:"downloads"`
			}
			if err := json.Unmarshal(msg, &c); err != nil {
				timeline.Warnf("settings: clearData: %v", err)
				return true
			}
			since := clearSince(c.Since, time.Now())
			if c.History {
				a.History.Clear(since)
			}
			if c.Downloads {
				a.Downloads.Clear(since)
			}
			var r engine.Erasure
			if c.SiteData {
				r.Kinds |= engine.EraseSiteData
			}
			if c.Cache {
				r.Kinds |= engine.EraseCache
			}
			if r.Kinds != 0 {
				if since != 0 {
					r.Since = time.Unix(since, 0)
				}
				// The profile's own storage, "", whether or not Main is still listed: the
				// engine's own tabs, an extension's, open there.
				ids := []string{""}
				for _, w := range a.Workspaces.List() {
					if !slices.Contains(ids, w.ID) {
						ids = append(ids, w.ID)
					}
				}
				for _, id := range ids {
					e.EraseWorkspace(id, r)
				}
			}
			timeline.Infof("settings: cleared since %q: history %v, site data %v, cache %v, the downloads list %v", c.Since, c.History, c.SiteData, c.Cache, c.Downloads)
			a.Settings.Send(map[string]any{"t": "dataCleared", "since": c.Since, "history": c.History, "siteData": c.SiteData, "cache": c.Cache, "downloads": c.Downloads})
			return true
		},
	}
}
