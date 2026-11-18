package app

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"

	"projectb/engine"
	"projectb/settings"
	"projectb/sites"
	"projectb/timeline"
)

// Settings' Sites (docs/settings.md): one row per site, its registrable domain, as Safari's
// Website Data and Chrome's All sites have it. Each row joins the permissions of the
// site's hosts with what every workspace's storage keeps for them, the cookies and bytes
// the engine counts by host. A site's Clear erases all of it for every host of the site,
// in every workspace, as the engine erases a site: its cookies are kept by domain, so no
// host's can go alone.

// siteRow is one row of Sites.
type siteRow struct {
	Site string `json:"site"`
	// Origins are the site's origins that have a setting of their own.
	Origins []string `json:"origins"`
	// Hosts are those whose storage keeps something, in any workspace.
	Hosts   []string `json:"hosts"`
	Cookies int      `json:"cookies"`
	Bytes   int64    `json:"bytes"`
}

// webHost reports whether a host the engine counted is a site's: an extension's storage
// is under its id, which has no dot.
func webHost(h string) bool {
	return strings.Contains(h, ".") || strings.Contains(h, ":") || h == "localhost"
}

// siteRows is Sites as it is now, by site.
func (a *App) siteRows() []siteRow {
	rows := map[string]*siteRow{}
	row := func(site string) *siteRow {
		if rows[site] == nil {
			rows[site] = &siteRow{Site: site, Origins: []string{}, Hosts: []string{}}
		}
		return rows[site]
	}
	for _, hosts := range a.siteData {
		for _, h := range hosts {
			if !webHost(h.Host) || (h.Cookies == 0 && h.Bytes == 0) {
				continue
			}
			r := row(sites.SiteOf(h.Host))
			if !slices.Contains(r.Hosts, h.Host) {
				r.Hosts = append(r.Hosts, h.Host)
			}
			r.Cookies += h.Cookies
			r.Bytes += h.Bytes
		}
	}
	for _, s := range a.Sites.Sites() {
		if u, err := url.Parse(s.Origin); err == nil && u.Hostname() != "" {
			r := row(sites.SiteOf(u.Hostname()))
			r.Origins = append(r.Origins, s.Origin)
		}
	}
	list := []siteRow{}
	for _, r := range rows {
		slices.Sort(r.Hosts)
		slices.Sort(r.Origins)
		list = append(list, *r)
	}
	slices.SortFunc(list, func(x, y siteRow) int { return strings.Compare(x.Site, y.Site) })
	return list
}

// storageIDs are the storage of every workspace, and the profile's own, "", whether or
// not Main is still listed: the engine's own tabs, an extension's, open there.
func (a *App) storageIDs() []string {
	ids := []string{""}
	for _, w := range a.Workspaces.List() {
		if !slices.Contains(ids, w.ID) {
			ids = append(ids, w.ID)
		}
	}
	return ids
}

// SiteDataListed takes what one workspace's storage holds, and Sites shows it.
func (a *App) SiteDataListed(workspace string, hosts []engine.SiteData) {
	if a.siteData == nil {
		a.siteData = map[string][]engine.SiteData{}
	}
	a.siteData[workspace] = hosts
	a.sendSiteRows()
}

func (a *App) sendSiteRows() {
	a.Settings.Send(map[string]any{"t": "sites", "sites": a.siteRows()})
}

// forgetSiteData drops what Sites shows of the sites named, or of every site for none,
// once the engine has been asked to erase it.
func (a *App) forgetSiteData(site ...string) {
	for id, hosts := range a.siteData {
		if len(site) == 0 {
			delete(a.siteData, id)
			continue
		}
		a.siteData[id] = slices.DeleteFunc(hosts, func(h engine.SiteData) bool { return slices.Contains(site, sites.SiteOf(h.Host)) })
	}
	a.sendSiteRows()
}

// siteDataSettingsPart lists each workspace's storage as the page opens, and clears a
// site's, or every site's.
func (a *App) siteDataSettingsPart(e engine.Engine) settings.Part {
	a.Sites.OnSitesChanged(a.sendSiteRows)
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["sites"] = a.siteRows()
			for _, id := range a.storageIDs() {
				e.ListSiteData(id)
			}
		},
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			switch t {
			case "clearSiteData":
				var c struct {
					Site string `json:"site"`
				}
				if err := json.Unmarshal(msg, &c); err != nil || c.Site == "" {
					timeline.Warnf("settings: clearSiteData: %v %q", err, c.Site)
					return true
				}
				for _, id := range a.storageIDs() {
					e.EraseWorkspace(id, engine.Erasure{Kinds: engine.EraseAll, Sites: []string{c.Site}})
				}
				timeline.Infof("settings: cleared %s's site data", c.Site)
				a.forgetSiteData(c.Site)
			case "clearAllSiteData":
				for _, id := range a.storageIDs() {
					e.EraseWorkspace(id, engine.Erasure{Kinds: engine.EraseAll})
				}
				timeline.Infof("settings: cleared every site's data")
				a.forgetSiteData()
			default:
				return false
			}
			return true
		},
	}
}
