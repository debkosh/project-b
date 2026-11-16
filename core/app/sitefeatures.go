package app

import (
	"encoding/json"
	"slices"

	"projectb/adblock"
	"projectb/engine"
	"projectb/settings"
	"projectb/sites"
)

// The ad blocker and Sparks (docs/site-features.md): Go keeps the rules and the scripts and
// decides what each page gets; the engine carries it out.

// startSiteFeatures hands the engine the ad blocker's rules, again whenever a site switches
// it, and gives settings the Sparks section.
func (a *App) startSiteFeatures(e engine.Engine) {
	a.post = e.Post
	hosts := adblock.Hosts()
	var off []string
	setRules := func() {
		now := a.adsOff()
		if off != nil && slices.Equal(now, off) {
			return
		}
		off = now
		e.SetAdRules(engine.AdRules{Hosts: hosts, Off: now})
	}
	setRules()
	a.Sites.OnSitesChanged(setRules)

	a.Sparks.OnChanged(func() {
		a.Settings.Send(map[string]any{"t": "sparks", "sparks": a.Sparks.List()})
	})
	a.Settings.AddPart(settings.Part{
		Snapshot: func(into map[string]any) { into["sparks"] = a.Sparks.List() },
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			var c struct {
				ID     string `json:"id"`
				Source string `json:"source"`
				On     bool   `json:"on"`
			}
			json.Unmarshal(msg, &c)
			// What a change says back to the page: "" when it took, else why not.
			var err error
			switch t {
			case "sparkAdd":
				_, err = a.Sparks.Add(c.Source)
			case "sparkUpdate":
				err = a.Sparks.Update(c.ID, c.Source)
			case "sparkRemove":
				a.Sparks.Remove(c.ID)
				return true
			case "sparkSetOn":
				a.Sparks.SetOn(c.ID, c.On)
				return true
			case "sparkSource":
				// For the editor and for export: the file as the person wrote it.
				source, _ := a.Sparks.Source(c.ID)
				a.Settings.Send(map[string]any{"t": "sparkSource", "id": c.ID, "source": source, "fileName": a.Sparks.FileName(c.ID)})
				return true
			default:
				return false
			}
			problem := ""
			if err != nil {
				problem = err.Error()
			}
			a.Settings.Send(map[string]any{"t": "sparkSaved", "problem": problem})
			return true
		},
	})
}

// adsOff is the origins where the person switched the ad blocker off, in order.
func (a *App) adsOff() []string {
	off := []string{}
	for _, s := range a.Sites.Sites() {
		if a.Sites.Value(s.Origin, sites.Ads) == sites.Off {
			off = append(off, s.Origin)
		}
	}
	return off
}

// sparksOn adds to list each spark that matches url, on or off, and isn't in it yet: on
// is whether it runs on origin.
func (a *App) sparksOn(origin, url string, list []siteSpark) []siteSpark {
	for _, s := range a.Sparks.Matching(url, true) {
		if slices.ContainsFunc(list, func(x siteSpark) bool { return x.ID == s.ID }) {
			continue
		}
		on := s.On && a.Sites.Value(origin, sites.Spark+s.ID) != sites.Off
		list = append(list, siteSpark{ID: s.ID, Name: s.Name, On: on})
	}
	return list
}

// SparksFor is the scripts a frame of the tab runs at url: the sparks that are on, match,
// and aren't switched off for the tab's site. A frame inside the page goes by the tab's
// site, as the panel shows it.
func (a *App) SparksFor(tab engine.TabID, url string, mainFrame bool) []engine.Script {
	site := url
	if t, ok := a.Tabs.Get(tab); ok && !mainFrame {
		site = t.URL
	}
	origin := sites.Origin(site)
	var out []engine.Script
	for _, s := range a.Sparks.Scripts(url, mainFrame, func(id string) bool {
		return a.Sites.Value(origin, sites.Spark+id) == sites.Off
	}) {
		out = append(out, engine.Script{ID: s.ID, Source: s.Source, RunAt: s.RunAt})
	}
	return out
}

func (a *App) AdBlocked(tab engine.TabID) { a.Tabs.HandleAdBlocked(tab) }
