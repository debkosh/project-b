package app

import (
	"slices"

	"projectb/engine"
	"projectb/shell"
	"projectb/shells"
)

// menuShellPart answers a shell that opens its own menu with the menu bar's tree as it is
// then, for this window and its profile ("The shell's menu" in docs/menus-shortcuts.md),
// as the menu bar reads its lists when it opens. The shell runs an item with run.
func (a *App) menuShellPart() shell.Part {
	return shell.Part{
		Snapshot: func(map[string]any) {},
		Handle: func(w engine.WindowID, t string, _ []byte) bool {
			if t != "menu" {
				return false
			}
			id := a.Prefs.Shell()
			phone := slices.ContainsFunc(shells.All(), func(s shells.Shell) bool { return s.ID == id && s.Phone })
			a.Shell.SendTo(w, map[string]any{"t": "menu", "menu": a.Commands.ShellMenu(id, phone)})
			return true
		},
	}
}
