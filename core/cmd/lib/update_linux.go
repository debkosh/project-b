//go:build !android

package main

import (
	"path/filepath"

	"github.com/sudiptadeb/amarit/appupdate"
	"github.com/sudiptadeb/amarit/flatpak"

	"projectb/links"
	"projectb/profiles"
	"projectb/relaunch"
	"projectb/timeline"
	"projectb/update"
)

// version is the release this build is, from VERSION, set at build time by
// engine/build_go.py.
var version string

// appVersion is the release this build is.
func appVersion() string { return version }

// updateConfig describes this build to the updater (docs/research/linux.md, 5): the Flatpak's
// update portal, through amarit's Flatpak source, which finds releases in the signed
// repository, puts them in place and starts each running copy again on the new one. A copy on
// a listed profile shares the root's update folder and relaunches every copy on the root; one
// on a folder named outright keeps its own and relaunches alone. Outside the Flatpak, as a
// build directory's binary, the portal refuses and the copy doesn't update.
func updateConfig(c profiles.Choice) update.Config {
	cfg := update.Config{Project: releaseProject, Releases: links.Releases, StateDir: filepath.Join(c.Dir, "updates"), AutoUpdate: releaseAutoUpdate, Version: version}
	opts := flatpak.Options{Args: []string{"/app/bin/project-b", "--user-data-dir=" + c.Dir, "--after-update"}}
	if c.ID != "" {
		cfg.StateDir = filepath.Join(list.Root(), "update")
		opts = flatpak.Options{Copies: relaunch.Copies{List: list, Profile: c.ID}}
	}
	var src *flatpak.Source
	commits := &update.Commits{URL: links.LinuxVersions, Channel: update.Stable}
	cfg.Source = func(quit func()) appupdate.Source {
		opts.Quit = quit
		opts.VersionOf = commits.VersionOf
		s, err := flatpak.New(opts)
		if err != nil {
			timeline.Infof("update: %v; this copy doesn't update", err)
			return nil
		}
		src = s
		return s
	}
	cfg.Present = func(u *appupdate.Updater) {
		if err := src.Present(u, flatpak.Words{Title: "An update to Project B is ready", Body: "Relaunch to update.", Relaunch: "Relaunch"}); err != nil {
			timeline.Errorf("update: the ready notification: %v", err)
		}
	}
	return cfg
}
