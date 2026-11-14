package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/settings"
	"projectb/shell"
	"projectb/sites"
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
	a.Sites.OnPromptsChanged(func() {
		a.Shell.Send(map[string]any{"t": "permissionPrompts", "permissionPrompts": a.Sites.Prompts()})
	})
	a.Sites.OnSitesChanged(func() {
		a.Settings.Send(map[string]any{"t": "siteSettings", "siteSettings": a.Sites.Sites()})
	})
	a.Tabs.OnChanged(func(t tabs.Tab) { a.Sites.TabChanged(t.ID, t.URL) })
	a.Tabs.OnListChanged(func() {
		for _, p := range a.Sites.Prompts() {
			if _, ok := a.Tabs.Get(p.Tab); !ok {
				a.Sites.TabClosed(p.Tab)
			}
		}
	})
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["permissionPrompts"] = a.Sites.Prompts() },
		Handle: func(_ engine.WindowID, t string, msg []byte) bool {
			if t != "permissionAnswer" {
				return false
			}
			var c struct {
				ID     int    `json:"id"`
				Choice string `json:"choice"`
			}
			json.Unmarshal(msg, &c)
			a.Sites.Answer(c.ID, c.Choice)
			return true
		},
	})
	a.Settings.AddPart(settings.Part{
		Snapshot: func(into map[string]any) { into["siteSettings"] = a.Sites.Sites() },
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			if t != "forgetSiteSetting" {
				return false
			}
			var c struct {
				Origin string `json:"origin"`
				Kind   string `json:"kind"`
			}
			json.Unmarshal(msg, &c)
			a.Sites.Forget(c.Origin, c.Kind)
			return true
		},
	})
}

func (a *App) PermissionRequested(r engine.PermissionRequest) { a.Sites.Request(r) }

func (a *App) PermissionStatusOf(tab engine.TabID, origin, kind string) engine.PermissionStatus {
	return a.Sites.Status(tab, origin, kind)
}
