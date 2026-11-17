package app

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"

	"projectb/timeline"
	"projectb/tones"
)

// What a start found in the profile, and where each thing keeps its files, as lines of
// log, so a person running the app from a terminal sees the folders and when each thing
// happens (docs/logging.md, The lifecycle). Workspaces and profiles are named by their ids,
// which are their folders' names, never by the names the user gave them.

// sayStart prints where the profile keeps its files and what this start found there.
func (a *App) sayStart(dir string) {
	timeline.Infof("profile: the engine's storage in %s", filepath.Join(dir, "chromium"))
	p := a.Prefs.All()
	colour := p.FrameColor
	if colour == tones.None {
		colour = "none"
	}
	timeline.Infof("prefs: shell %s, tabs %s, theme %s, colour %s, in %s", p.Shell, p.TabLayout, p.Theme, colour, filepath.Join(dir, "prefs.json"))
	list := a.Workspaces.List()
	timeline.Infof("workspaces: %d, listed in %s", len(list), filepath.Join(dir, "workspaces.json"))
	for _, w := range list {
		timeline.Infof("workspace %s: its sites in %s", workspaceName(w.ID), workspaceStorage(dir, w.ID))
	}
}

// count is n things: "1 tab", "3 tabs".
func count(n int, thing string) string {
	if n == 1 {
		return "1 " + thing
	}
	return fmt.Sprintf("%d %ss", n, thing)
}

// workspaceName is how a workspace is named in a line of log: its id, or "the first" for
// the one with none, which keeps its sites in the profile's own storage.
func workspaceName(id string) string {
	if id == "" {
		return "the first"
	}
	return id
}

// workspaceStorage is the folder in the profile dir where the engine keeps a workspace's
// sites: the profile's own for the first, and for the others a partition in the domain
// "workspace" (engine.cc), which content names by the first 6 bytes of the SHA-256 of
// the workspace's id, in capitals (storage_partition_impl_map.cc).
func workspaceStorage(dir, id string) string {
	if id == "" {
		return filepath.Join(dir, "chromium")
	}
	sum := sha256.Sum256([]byte(id))
	return filepath.Join(dir, "chromium", "Storage", "ext", "workspace", fmt.Sprintf("%X", sum[:6]))
}
