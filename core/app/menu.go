package app

import (
	"encoding/json"
	"slices"

	"projectb/engine"
	"projectb/shell"
	"projectb/shells"
)

// menuShellPart answers a shell that opens its own menu with the menu bar's tree as it is
// then, for this window and its profile ("The shell's menu" in docs/menus-shortcuts.md),
// as the menu bar reads its lists when it opens. The shell runs an item with run. Asked
// with bar, as a shell drawing the menu bar off the Mac asks, it answers barMenu with the
// whole bar's (BarMenu).
func (a *App) menuShellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, t string, raw []byte) bool {
			if t != "menu" {
				return false
			}
			id := a.Prefs.Shell()
			var ask struct {
				Bar bool `json:"bar"`
			}
			json.Unmarshal(raw, &ask)
			if ask.Bar {
				a.Shell.SendTo(w, map[string]any{"t": "barMenu", "barMenu": a.Commands.BarMenu(id)})
				return true
			}
			phone := slices.ContainsFunc(shells.All(), func(s shells.Shell) bool { return s.ID == id && s.Phone })
			a.Shell.SendTo(w, map[string]any{"t": "menu", "menu": a.Commands.ShellMenu(id, phone)})
			return true
		},
	}
}
