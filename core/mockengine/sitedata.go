package mockengine

import (
	"fmt"
	"slices"

	"golang.org/x/net/publicsuffix"

	"projectb/engine"
)

// What each workspace's storage holds, by host, as a real engine's would after pages set
// cookies and stored things: a test, or a mock page's siteData button, says what. An erase
// of site data takes it, all of it or the sites named. Main thread.

// AddSiteData says a workspace's storage holds a host's cookies and bytes, on top of what
// it held. Main thread.
func (e *Engine) AddSiteData(workspace string, d engine.SiteData) {
	if e.siteData == nil {
		e.siteData = map[string][]engine.SiteData{}
	}
	hosts := e.siteData[workspace]
	if i := slices.IndexFunc(hosts, func(h engine.SiteData) bool { return h.Host == d.Host }); i >= 0 {
		hosts[i].Cookies += d.Cookies
		hosts[i].Bytes += d.Bytes
	} else {
		e.siteData[workspace] = append(hosts, d)
	}
}

// ListSiteData answers on a later task, as a real engine counts on its own threads.
func (e *Engine) ListSiteData(workspace string) {
	hosts := slices.Clone(e.siteData[workspace])
	e.Post(func() { e.listener.SiteDataListed(workspace, hosts) })
}

// eraseSiteData takes what an erasure of site data takes from a workspace's storage.
func (e *Engine) eraseSiteData(workspace string, r engine.Erasure) {
	if r.Kinds&engine.EraseSiteData == 0 {
		return
	}
	if len(r.Sites) == 0 {
		delete(e.siteData, workspace)
		return
	}
	e.siteData[workspace] = slices.DeleteFunc(e.siteData[workspace], func(h engine.SiteData) bool {
		site, err := publicsuffix.EffectiveTLDPlusOne(h.Host)
		if err != nil {
			site = h.Host
		}
		return slices.Contains(r.Sites, site)
	})
}

// fireSiteData is a mock page's siteData button: value is "host cookies bytes", stored in
// the workspace of the tab it is in.
func (e *Engine) fireSiteData(workspace, value string) {
	var d engine.SiteData
	if n, _ := fmt.Sscan(value, &d.Host, &d.Cookies, &d.Bytes); n == 3 {
		e.AddSiteData(workspace, d)
	}
}
