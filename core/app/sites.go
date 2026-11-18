package app

import (
	"encoding/json"
	"slices"

	"projectb/engine"
	"projectb/prefs"
	"projectb/settings"
	"projectb/shell"
	"projectb/sites"
	"projectb/sparks"
	"projectb/tabs"
)

// Each site's settings (docs/daily-driver.md): the manager decides what a page may use,
// every shell hears the open prompts and draws those of its own tabs, and settings lists
// each site's settings and removes them.

// tabURLs is what the sites manager reads of the tabs.
type tabURLs struct{ t *tabs.Manager }

func (u tabURLs) URLOf(tab engine.TabID) (string, bool) {
	t, ok := u.t.Get(tab)
	return t.URL, ok
}

func (a *App) startSites(e engine.Engine, dir string) {
	a.Sites = sites.New(dir, tabURLs{a.Tabs}, e.AnswerPermission)
	a.Sparks = sparks.New(dir)
	// A profile's first start on a version with the built-in sparks gives it them, off.
	a.Sparks.AddBuiltins()
	a.startSiteFeatures(e)
	a.Sites.OnPromptsChanged(func() {
		a.Shell.Send(map[string]any{"t": "permissionPrompts", "permissionPrompts": a.Sites.Prompts()})
	})
	// Every shell hears the settings of each site a tab is open on, for its site panel.
	var open string
	sendOpen := func() {
		o := a.openSites()
		if b, _ := json.Marshal(o); string(b) != open {
			open = string(b)
			a.Shell.Send(map[string]any{"t": "openSites", "openSites": o})
		}
	}
	a.Sites.OnSitesChanged(func() {
		a.Settings.Send(map[string]any{"t": "siteSettings", "siteSettings": a.Sites.Sites()})
		sendOpen()
	})
	// The ad blocker off on every site takes its switch out of the panel.
	a.Prefs.OnChanged(func(prefs.Prefs) { sendOpen() })
	a.Sparks.OnChanged(sendOpen)
	a.Tabs.OnChanged(func(tabs.Tab) { sendOpen() })
	a.Tabs.OnListChanged(sendOpen)
	a.Tabs.OnChanged(func(t tabs.Tab) { a.Sites.TabChanged(t.ID, t.URL) })
	a.Tabs.SetPageGuard(a.pageMay)
	// The site view's connection row follows the open pages.
	var connections string
	sendConnections := func() {
		c := a.siteConnections()
		if b, _ := json.Marshal(c); string(b) != connections {
			connections = string(b)
			a.Settings.Send(map[string]any{"t": "siteConnections", "siteConnections": c})
		}
	}
	a.Tabs.OnChanged(func(tabs.Tab) { sendConnections() })
	a.Tabs.OnListChanged(sendConnections)
	a.Tabs.OnListChanged(func() {
		for _, p := range a.Sites.Prompts() {
			if _, ok := a.Tabs.Get(p.Tab); !ok {
				a.Sites.TabClosed(p.Tab)
			}
		}
	})
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) {
			into["permissionPrompts"] = a.Sites.Prompts()
			into["openSites"] = a.openSites()
		},
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			switch t {
			case "permissionAnswer":
				var c struct {
					ID     int    `json:"id"`
					Choice string `json:"choice"`
				}
				json.Unmarshal(msg, &c)
				a.Sites.Answer(c.ID, c.Choice)
			case "setSiteSetting":
				// A setting changed in the site panel, as Site settings changes one.
				var c struct {
					Origin string `json:"origin"`
					Kind   string `json:"kind"`
					Value  string `json:"value"`
				}
				json.Unmarshal(msg, &c)
				a.Sites.Set(c.Origin, c.Kind, c.Value)
			case "openSiteSettings":
				// Settings, on the site of the window's active tab: its own address.
				t, _ := a.Tabs.Get(a.Tabs.ActiveIn(w))
				if o := sites.Origin(t.URL); o != "" {
					a.Settings.OpenSection(w, "sites/"+o)
				} else {
					a.Settings.OpenSection(w, "sites")
				}
			default:
				return false
			}
			return true
		},
	})
	a.Settings.AddPart(settings.Part{
		Snapshot: func(into map[string]any) {
			into["siteSettings"] = a.Sites.Sites()
			into["siteKinds"] = sites.Settable()
			into["siteConnections"] = a.siteConnections()
		},
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			var c struct {
				Origin string `json:"origin"`
				Kind   string `json:"kind"`
				Value  string `json:"value"`
			}
			switch t {
			case "forgetSiteSetting":
				json.Unmarshal(msg, &c)
				a.Sites.Forget(c.Origin, c.Kind)
			case "setSiteSetting":
				json.Unmarshal(msg, &c)
				a.Sites.Set(c.Origin, c.Kind, c.Value)
			default:
				return false
			}
			return true
		},
	})
}

// openSite is what a shell's site panel shows of one site: each kind it can set, with the
// site's value ("" for a permission it hasn't set), and the sparks its open pages match.
type openSite struct {
	Settings []sites.Setting `json:"settings"`
	// Sparks is the names of the sparks that are on and match a page of the site's open,
	// which its one switch for sparks runs or not.
	Sparks []string `json:"sparks"`
}

// openSites is, by origin, each site a web tab is open on. With the ad blocker off on every
// site, a site's own switch for it means nothing, and isn't there.
func (a *App) openSites() map[string]openSite {
	out := map[string]openSite{}
	for _, t := range a.Tabs.All() {
		o := sites.Origin(t.URL)
		if t.Page != "" || o == "" {
			continue
		}
		site, ok := out[o]
		if !ok {
			list := a.Sites.SettingsOf(o)
			if !a.Prefs.All().BlockAds {
				list = slices.DeleteFunc(list, func(s sites.Setting) bool { return s.Kind == sites.Ads })
			}
			site = openSite{Settings: list, Sparks: []string{}}
		}
		site.Sparks = a.sparksAt(t.URL, site.Sparks)
		out[o] = site
	}
	return out
}

func (a *App) PermissionRequested(r engine.PermissionRequest) { a.Sites.Request(r) }

func (a *App) PermissionStatusOf(tab engine.TabID, origin, kind string) engine.PermissionStatus {
	return a.Sites.Status(tab, origin, kind)
}

// NotificationShown shows a page's notification on the system when its site may notify,
// and keeps the page it came from, for its click.
func (a *App) NotificationShown(n engine.Notification) bool {
	if a.Sites.Status(0, n.Origin, sites.Notifications) != engine.PermissionGranted {
		return false
	}
	if a.notified == nil {
		a.notified = map[string]string{}
	}
	a.notified[n.ID] = n.Page
	return true
}

// NotificationClicked brings the tab that showed the notification to the front: the one
// at its page's address, or else the first on its site. A service worker's has no page; its
// own click handler opens one, as the web does it.
func (a *App) NotificationClicked(id string) {
	page, ok := a.notified[id]
	delete(a.notified, id)
	if !ok || page == "" {
		return
	}
	var found engine.TabID
	for _, w := range a.Tabs.Windows() {
		for _, t := range a.Tabs.AllIn(w) {
			if t.URL == page {
				a.ActivateRequested(t.ID)
				return
			}
			if found == 0 && sites.Origin(t.URL) == sites.Origin(page) {
				found = t.ID
			}
		}
	}
	if found != 0 {
		a.ActivateRequested(found)
	}
}

// siteConnection is how a site's open pages reached it, for its site view.
type siteConnection struct {
	Connection string `json:"connection"`
	Note       string `json:"note,omitempty"`
}

// siteConnections is, by origin, how the open pages of each site reached them: insecure
// when any of them did, and why.
func (a *App) siteConnections() map[string]siteConnection {
	out := map[string]siteConnection{}
	for _, t := range a.Tabs.All() {
		o := sites.Origin(t.URL)
		if o == "" || t.Connection == "" {
			continue
		}
		if have, ok := out[o]; !ok || have.Connection == "secure" {
			out[o] = siteConnection{Connection: t.Connection, Note: t.ConnectionNote}
		}
	}
	return out
}

// pageMay is Links to other sites (docs/site-features.md): a page's navigation, in its own
// tab or a tab it opened, may go to its own site whatever the setting, and anywhere under
// Allow; under Block, nowhere else, the blank page included.
func (a *App) pageMay(n tabs.PageNavigation) bool {
	o := sites.Origin(n.From)
	if o == "" || a.Sites.Value(o, sites.Links) != sites.Block {
		return true
	}
	return sites.SameSite(n.From, n.To)
}
