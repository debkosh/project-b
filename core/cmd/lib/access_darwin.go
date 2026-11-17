package main

import (
	"os/exec"

	"projectb/timeline"
)

// fullDiskAccess is System Settings' Privacy & Security, at Full Disk Access.
const fullDiskAccess = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

func (e *nativeEngine) OpenFullDiskAccess() {
	if out, err := exec.Command("/usr/bin/open", fullDiskAccess).CombinedOutput(); err != nil {
		timeline.Errorf("can't open System Settings at Full Disk Access: %v %s", err, out)
	}
}
