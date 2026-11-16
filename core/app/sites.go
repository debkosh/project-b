package app

import (
	"encoding/json"

	"projectb/engine"
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
	a.Sparks.OnChanged(sendOpen)
	a.Tabs.OnChanged(func(tabs.Tab) { sendOpen() })
	a.Tabs.OnListChanged(sendOpen)
	a.Tabs.OnChanged(func(t tabs.Tab) { a.Sites.TabChanged(t.ID, t.URL) })
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
	Sparks   []siteSpark     `json:"sparks"`
}

// siteSpark is a spark on a site's page: on is whether it runs on the site.
type siteSpark struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	On   bool   `json:"on"`
}

// openSites is, by origin, each site a web tab is open on.
func (a *App) openSites() map[string]openSite {
	out := map[string]openSite{}
	for _, t := range a.Tabs.All() {
		o := sites.Origin(t.URL)
		if t.Page != "" || o == "" {
			continue
		}
		site, ok := out[o]
		if !ok {
			site = openSite{Settings: a.Sites.SettingsOf(o), Sparks: []siteSpark{}}
		}
		site.Sparks = a.sparksOn(o, t.URL, site.Sparks)
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

// LeavingSite carries out the Links to other sites of the page it leaves, from: Allow lets
// the page go; New tab opens the address in a tab beside it when a press led there; Block,
// or New tab with no press, keeps the tab where it is, with nothing shown or kept of the
// navigation. A new tab the page opens (opener, the tab whose page asks) is a new tab
// already: New tab lets it go; Block keeps it from opening, asked before it is made (tab is
// opener), or closes one made blank first and shows its opener again, as if it never opened.
func (a *App) LeavingSite(tab engine.TabID, from, url string, pressed bool, opener engine.TabID) engine.Navigation {
	if _, ok := a.Tabs.Get(tab); !ok {
		return engine.NavigationProceed
	}
	links := a.Sites.Value(sites.Origin(from), sites.Links)
	// The blank page a popup opens on, which Android would load over the page: blocked
	// under Block, as the popup's own address would be; there is nothing to open beside.
	if url == engine.BlankURL {
		if links == sites.Block {
			return engine.NavigationElsewhere
		}
		return engine.NavigationProceed
	}
	if opener != 0 {
		if links != sites.Block {
			return engine.NavigationProceed
		}
		if tab == opener {
			return engine.NavigationElsewhere
		}
		// After the engine has its answer, not inside its question.
		a.post(func() {
			a.Tabs.Close(tab)
			if _, ok := a.Tabs.Get(opener); ok {
				a.Tabs.Activate(opener)
			}
		})
		return engine.NavigationElsewhere
	}
	switch links {
	case sites.NewTab:
		if pressed {
			a.Tabs.OpenFor(tab, url)
		}
		return engine.NavigationElsewhere
	case sites.Block:
		return engine.NavigationElsewhere
	}
	return engine.NavigationProceed
}
