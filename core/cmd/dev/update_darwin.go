package main

import (
	"github.com/debkosh/amarit/appupdate"
	"github.com/debkosh/amarit/macapp"

	"projectb/update"
)

// devUpdate is the mock's updater: the release server and the installed bundle the tests
// name, a bundle of their own that isn't signed, so the signature isn't asked for. With no
// version the mock is a development build, which doesn't update.
func devUpdate(version, releases, dir, app string) update.Config {
	cfg := update.Config{Project: "project-b", Version: version, Releases: releases, StateDir: dir, AutoUpdate: true}
	if app != "" {
		cfg.Installer = func(_ func(func()), quit func()) appupdate.Installer {
			return &macapp.Installer{App: app, StateDir: dir, Quit: quit}
		}
	}
	return cfg
}
