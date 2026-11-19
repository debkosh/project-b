package app

import (
	"projectb/engine"
	"projectb/settings"
	"projectb/timeline"
)

// defaultBrowserSettingsPart is the settings page's status line's half about the default
// browser (docs/settings.md): whether the app is it, read as the page opens, and Make
// default browser, which asks the system as the welcome's button does. How the asking ended
// reaches the page as it reaches the welcome (defaultBrowserAnswered).
func (a *App) defaultBrowserSettingsPart() settings.Part {
	return settings.Part{
		Snapshot: func(into map[string]any) { into["isDefault"] = a.System.IsDefaultBrowser() },
		Handle: func(_ engine.TabID, t string, _ []byte) bool {
			if t != "makeDefault" {
				return false
			}
			timeline.Infof("settings: asking the system to make this the default browser")
			a.System.AskDefaultBrowser(a.defaultBrowserAnswered)
			return true
		},
	}
}
