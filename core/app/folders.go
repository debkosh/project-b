package app

import "projectb/engine"

// FolderChosen takes the answer of the folder panel the engine's ChooseFolder opened, which
// two features ask for: Save to's Change…, whose answer it is when it asked on that window
// (download_folder_settings.go), and otherwise the importer's Choose a folder (imports.go).
func (a *App) FolderChosen(w engine.WindowID, path string) {
	if a.downloadFolderChosen(w, path) {
		return
	}
	a.imports.folderChosen(path)
}
