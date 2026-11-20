package main

import (
	"os"
	"path/filepath"

	"github.com/sudiptadeb/amarit"
	"github.com/sudiptadeb/amarit/appupdate"
	"github.com/sudiptadeb/amarit/macapp"

	"projectb/profiles"
	"projectb/relaunch"
	"projectb/timeline"
	"projectb/update"
)

// signingLeafSHA1 is the certificate tools/package.sh signs with (its SHA-1, as in the trust
// profile): a downloaded bundle must be signed with it.
const signingLeafSHA1 = "59b20dfe02ee393a05173eec84daa8c662c845be"

// installedBundle is the app bundle this binary runs from, or "" for a binary in a build
// directory.
func installedBundle() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return macapp.BundleOf(exe)
}

// appVersion is the version the installed bundle says it is, or "" for a build directory.
func appVersion() string {
	bundle := installedBundle()
	if bundle == "" {
		return ""
	}
	info, _ := macapp.Info(bundle)
	return info.Version
}

// updateConfig describes this build to the updater, for a copy on the profile c. A copy on
// a listed profile shares the root's update folder with every copy on the root, and
// relaunches them all; one on a folder named outright keeps its update state in that folder
// and relaunches alone, so tests and ChromeDriver never touch the copies on the root. A
// binary running from a build directory rather than an installed bundle has no version, and
// never updates.
func updateConfig(c profiles.Choice) update.Config {
	cfg := update.Config{Project: releaseProject, Releases: releaseManifest, StateDir: filepath.Join(c.Dir, "updates"), AutoUpdate: releaseAutoUpdate}
	var copies macapp.Copies
	if c.ID != "" {
		cfg.StateDir = filepath.Join(list.Root(), "update")
		copies = relaunch.Copies{List: list, Profile: c.ID}
	}
	bundle := installedBundle()
	if bundle == "" {
		return cfg
	}
	info, err := macapp.Info(bundle)
	if err != nil {
		timeline.Errorf("update: %v; updates are off", err)
		return cfg
	}
	cfg.Version = info.Version
	installer := func(quit func()) *macapp.Installer {
		return &macapp.Installer{App: bundle, StateDir: cfg.StateDir, Requirement: macapp.SignedBy(signingLeafSHA1), Copies: copies, Quit: quit}
	}
	cfg.Installer = func(quit func()) appupdate.Installer { return installer(quit) }
	cfg.Settle = func(cl *amarit.Client) (bool, error) { return installer(nil).Settle(cl) }
	return cfg
}
