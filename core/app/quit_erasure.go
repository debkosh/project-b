package app

import (
	"encoding/json"
	"slices"

	"projectb/engine"
	"projectb/prefs"
	"projectb/settings"
)

// Clear cookies when the app quits (docs/settings.md): the engine is told at start, and
// again whenever the switch, the sites kept or the workspaces change, what a quit erases
// first: every workspace's cookies and site storage, but the kept sites'; the cache stays.
// The engine runs it as the app quits, however the quit comes, and waits for it. Off, a
// quit erases nothing.
func (a *App) startQuitErasure(e engine.Engine) {
	var told []byte
	tell := func() {
		var r engine.Erasure
		if p := a.Prefs.All(); p.ClearCookiesOnQuit {
			r = engine.Erasure{Kinds: engine.EraseSiteData, Keep: p.KeepCookies}
		}
		ids := a.storageIDs()
		if b, _ := json.Marshal([]any{ids, r}); !slices.Equal(b, told) {
			told = b
			e.SetQuitErasure(ids, r)
		}
	}
	tell()
	a.Prefs.OnChanged(func(prefs.Prefs) { tell() })
	a.Workspaces.OnChanged(tell)
}

// keepCookiesSettingsPart keeps a site's cookies as the app quits, from its page in Sites,
// or stops keeping them.
func (a *App) keepCookiesSettingsPart() settings.Part {
	return settings.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			if t != "keepCookies" {
				return false
			}
			var c struct {
				Site string `json:"site"`
				Keep bool   `json:"keep"`
			}
			if json.Unmarshal(msg, &c) == nil {
				a.Prefs.KeepCookies(c.Site, c.Keep)
			}
			return true
		},
	}
}
