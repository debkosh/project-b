package main

/*
#include <stdlib.h>
#include "pb_abi.h"

static inline void call_install_update(const PBEngineCalls* c, const char* path) { c->install_update(path); }
*/
import "C"

import (
	"path/filepath"
	"unsafe"

	"github.com/debkosh/amarit/appupdate"

	"projectb/links"
	"projectb/profiles"
	"projectb/update"
)

// version is the release this APK is, from VERSION, set at build time by
// engine/build_go.py. The APK's own versionName is the same.
var version string

// appVersion is the release this APK is.
func appVersion() string { return version }

// updateConfig describes this build to the updater. The APK is the release: a newer one is
// downloaded into the profile and handed to Android's package installer, which refuses one
// signed with another key and asks the user.
func updateConfig(c profiles.Choice) update.Config {
	dir := filepath.Join(c.Dir, "updates")
	return update.Config{
		Project:    releaseProject,
		Version:    version,
		Releases:   links.Releases,
		AutoUpdate: releaseAutoUpdate,
		StateDir:   dir,
		// The system's notification shows the download's progress too, with Cancel, Install
		// when ready, and Try again (core/update's notes).
		ProgressNotes: true,
		Installer: func(main func(func()), _ func()) appupdate.Installer {
			return appupdate.File{Dir: dir, Open: func(apk string) error {
				// On the main thread: amarit opens the release on its own goroutine, and the
				// engine's call off the main thread ended the app (a Dev build's Install).
				done := make(chan struct{})
				main(func() {
					defer close(done)
					// The installer ends this process to replace the app, which isn't a
					// crash: the run counts as stopped cleanly from here. A user who cancels
					// goes on without crash.log, so a crash later in that run goes
					// unreported.
					if theApp != nil {
						theApp.Crash.Stop()
					}
					c := C.CString(apk)
					defer C.free(unsafe.Pointer(c))
					C.call_install_update(native.calls, c)
				})
				<-done
				return nil
			}}
		},
	}
}

