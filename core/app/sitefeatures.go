package app

import (
	"encoding/json"
	"slices"

	"projectb/adblock"
	"projectb/engine"
	"projectb/prefs"
	"projectb/settings"
	"projectb/sites"
	"projectb/sparks"
)

// The ad blocker and Sparks (docs/site-features.md): Go keeps the rules and the scripts and
// decides what each page gets; the engine carries it out.

// startSiteFeatures hands the engine the ad blocker's rules, again whenever a site switches
// it or the blocker is switched on every site, and gives settings the Sparks section. Off on
// every site, the rules have no hosts, so nothing is kept from any page.
func (a *App) startSiteFeatures(e engine.Engine) {
	hosts := adblock.Hosts()
	var last *engine.AdRules
	setRules := func() {
		now := engine.AdRules{Hosts: []string{}, Off: a.adsOff()}
		if a.Prefs.All().BlockAds {
			now.Hosts = hosts
		}
		if last != nil && slices.Equal(now.Hosts, last.Hosts) && slices.Equal(now.Off, last.Off) {
			return
		}
		last = &now
		e.SetAdRules(now)
	}
	setRules()
	a.Sites.OnSitesChanged(setRules)
	a.Prefs.OnChanged(func(prefs.Prefs) { setRules() })

	a.Sparks.OnChanged(func() {
		a.Settings.Send(map[string]any{"t": "sparks", "sparks": a.Sparks.List()})
	})
	// An import waiting on the person to settle its clashes: one at a time, as settings
	// is one page.
	var importing *sparks.Import
	a.Settings.AddPart(settings.Part{
		Snapshot: func(into map[string]any) { into["sparks"] = a.Sparks.List() },
		Handle: func(tab engine.TabID, t string, msg []byte) bool {
			var c struct {
				ID     string `json:"id"`
				Source string `json:"source"`
				On     bool   `json:"on"`
				// For an import: the files picked, each as base64, or only its size when
				// it is too large to send; then a choice for each clash, by its index.
				Files []struct {
					Name string `json:"name"`
					Data []byte `json:"data"`
					Size int64  `json:"size"`
				} `json:"files"`
				Choices map[int]sparks.Choice `json:"choices"`
				// For an export: the sparks to put in the zip, all when none.
				IDs []string `json:"ids"`
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
			case "sparkImport":
				var picked []sparks.Picked
				for _, f := range c.Files {
					picked = append(picked, sparks.Picked{Name: f.Name, Data: f.Data, Size: f.Size})
				}
				importing = a.Sparks.Prepare(sparks.Unpack(picked))
				if clashes := importing.Clashes(); len(clashes) > 0 {
					a.Settings.Send(map[string]any{"t": "sparkClashes", "clashes": clashes})
					return true
				}
				fallthrough
			case "sparkImportKeep":
				if importing != nil {
					done := a.Sparks.Keep(importing, c.Choices)
					importing = nil
					a.Settings.Send(map[string]any{"t": "sparkImported", "imported": done})
				}
				return true
			case "sparkImportCancel":
				importing = nil
				return true
			case "sparkExport":
				data, n := a.Sparks.Zip(c.IDs)
				a.Settings.Send(map[string]any{"t": "sparkZip", "data": data, "count": n, "fileName": "Sparks.zip"})
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

// sparksAt adds to list the name of each spark that is on and matches url, and isn't in it
// yet.
func (a *App) sparksAt(url string, list []string) []string {
	for _, s := range a.Sparks.Matching(url, true) {
		if s.On && !slices.Contains(list, s.Name) {
			list = append(list, s.Name)
		}
	}
	return list
}

// SparksFor is the scripts a frame of the tab runs at url: the sparks that are on and
// match, unless sparks are switched off for the tab's site. A frame inside the page goes by
// the tab's site, as the panel shows it.
func (a *App) SparksFor(tab engine.TabID, url string, mainFrame bool) []engine.Script {
	site := url
	if t, ok := a.Tabs.Get(tab); ok && !mainFrame {
		site = t.URL
	}
	if a.Sites.Value(sites.Origin(site), sites.Sparks) == sites.Off {
		return nil
	}
	var out []engine.Script
	for _, s := range a.Sparks.Scripts(url, mainFrame) {
		out = append(out, engine.Script{ID: s.ID, Source: s.Source, RunAt: s.RunAt})
	}
	return out
}

func (a *App) AdBlocked(tab engine.TabID) { a.Tabs.HandleAdBlocked(tab) }
