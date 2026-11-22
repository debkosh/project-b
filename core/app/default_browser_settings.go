package app

import (
	"projectb/engine"
	"projectb/settings"
	"projectb/timeline"
)

// isDefault is whether the app is the default browser now, as the system says, kept for
// defaultBrowserChanged.
func (a *App) isDefault() bool {
	a.wasDefault = a.System.IsDefaultBrowser()
	return a.wasDefault
}

// defaultBrowserChanged asks the system again as a window comes to the front, so a person
// who set the default browser in the OS's own settings, as Linux's desktops have them do,
// comes back to pages that say so; nothing is sent when nothing changed.
func (a *App) defaultBrowserChanged() {
	if was := a.wasDefault; a.isDefault() != was {
		a.defaultBrowserAnswered(a.wasDefault)
	}
}

// defaultBrowserSettingsPart is the settings page's status line's half about the default
// browser (docs/settings.md): whether the app is it, read as the page opens, and Make
// default browser, which asks the system as the welcome's button does. How the asking ended
// reaches the page as it reaches the welcome (defaultBrowserAnswered).
func (a *App) defaultBrowserSettingsPart() settings.Part {
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["isDefault"] = a.isDefault()
			into["desktop"] = a.System.Desktop()
		},
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
