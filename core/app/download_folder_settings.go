package app

import (
	"os"

	"projectb/engine"
	"projectb/prefs"
	"projectb/settings"
	"projectb/timeline"
)

// Settings' Downloads, Save to (docs/settings.md): the folder downloads are saved in, the
// pref downloadFolder, or the Downloads folder, standard, while it is "". Change… asks for
// a folder in the system's panel on the settings tab's window, through the engine; the
// one chosen becomes the pref, and the downloads manager saves there from then on. A
// desktop's: a phone saves to its shared Downloads.
func (a *App) downloadFolderSettingsPart(e engine.Engine, standard string) settings.Part {
	follow := func(p prefs.Prefs) {
		dir := p.DownloadFolder
		if dir == "" {
			dir = standard
		}
		if dir != a.Downloads.Dir() {
			a.Downloads.SetDir(dir)
			timeline.Infof("downloads: saved in %s", dir)
		}
	}
	follow(a.Prefs.All())
	a.Prefs.OnChanged(follow)
	return settings.Part{
		Snapshot: func(into map[string]any) {
			into["downloadsFolder"] = standard
			into["homeFolder"], _ = os.UserHomeDir()
		},
		Handle: func(tab engine.TabID, t string, _ []byte) bool {
			if t != "chooseDownloadFolder" {
				return false
			}
			w := a.Tabs.WindowOf(tab)
			a.choosingFolder = w
			// The dialog opens at the folder downloads go to now.
			e.ChooseFolder(w, a.Downloads.Dir())
			return true
		},
	}
}

// downloadFolderChosen takes the folder Save to's Change… chose on its window, an existing
// folder, as the one downloads are saved in, and says the answer was Save to's. Any other
// panel's is the importer's (FolderChosen, folders.go).
func (a *App) downloadFolderChosen(w engine.WindowID, folder string) bool {
	if a.choosingFolder == 0 || w != a.choosingFolder {
		return false
	}
	a.choosingFolder = 0
	if folder == "" {
		return true
	}
	if fi, err := os.Stat(folder); err != nil || !fi.IsDir() {
		timeline.Warnf("downloads: %s is no folder: %v", folder, err)
		return true
	}
	a.Prefs.SetDownloadFolder(folder)
	return true
}
