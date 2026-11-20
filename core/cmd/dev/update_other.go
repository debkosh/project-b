//go:build !darwin

package main

import "projectb/update"

// devUpdate is the mock's updater, which only a Mac's mock installs with.
func devUpdate(version, releases, dir, app string) update.Config {
	return update.Config{Project: "project-b", Version: version, Releases: releases, StateDir: dir, AutoUpdate: true}
}
