//go:build linux && !android

package main

import (
	"os"
	"path/filepath"
)

// defaultRoot is Linux's folder for the profiles, the XDG data folder's project-b:
// ~/.local/share/project-b, and in the Flatpak ~/.var/app/<id>/data/project-b, which Flatpak
// keeps between runs where the rest of the sandbox's home is thrown away.
func defaultRoot() string {
	if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
		return filepath.Join(d, "project-b")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "project-b")
}

// crashReports is "": Linux keeps no crash reports of its own for the crash notice to name.
func crashReports(string) string { return "" }
