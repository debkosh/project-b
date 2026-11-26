//go:build !js && !android

// Package relaunch starts copies of the app: this one again once this process has gone,
// since starting it any earlier would run two copies on one profile, or another copy now,
// on another profile.
package relaunch

import (
	"os"
	"os/exec"
	"syscall"

	amarit "github.com/debkosh/amarit/relaunch"

	"projectb/timeline"
)

// After starts executable with args once this process has gone (amarit's relaunch). The
// caller then ends the app.
func After(executable string, args []string) {
	if err := amarit.After(executable, args); err != nil {
		timeline.Errorf("relaunch: %v", err)
	}
}

// Copy starts another copy of this program now, with args, apart from this process: it
// goes on when this one ends. In the Flatpak it starts in a sandbox of its own (command).
func Copy(args ...string) {
	argv, err := command(args)
	if err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	cmd.Process.Release()
}

// Self starts this program again, with the arguments it was started with.
func Self() {
	// Nothing outlives the Flatpak's sandbox to wait for this process: the new copy starts
	// now, in its own, and waits for the profile instead (--after-update).
	if sandboxed() {
		Copy(append([]string{"--after-update"}, os.Args[1:]...)...)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		timeline.Errorf("relaunch: %v", err)
		return
	}
	After(exe, os.Args[1:])
}
