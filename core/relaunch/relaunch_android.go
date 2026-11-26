// Package relaunch, on a phone: Android starts the app again through its activity, as
// Chrome restarts, and nothing here starts a program, since Go's os/exec ends the app on
// Android 10 (its seccomp refuses pidfd_open, which Go asks first). A phone runs one copy
// of the app, so there is none to start beside it.
package relaunch

import "projectb/timeline"

// again has Android start the app again once it has quit: the core's library's call into
// Java (platform.RelaunchAfterQuit), set once as it loads.
var again func()

// OnSelf names how Self has Android start the app again.
func OnSelf(f func()) { again = f }

// Self has Android start the app again once it has quit. The caller then ends the app.
func Self() {
	if again == nil {
		timeline.Errorf("relaunch: no way to start the app again")
		return
	}
	again()
}

// Copy has no copy to start: a phone runs one.
func Copy(args ...string) { timeline.Errorf("relaunch: a phone runs one copy of the app: %v", args) }
