package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"projectb/crash"
)

// systemReport is what a crash report says of this Mac: the release, macOS's version and
// the model. macOS's own report of a crash is added from the crash manager's.
func systemReport() []crash.Section {
	run := func(name string, args ...string) string {
		out, _ := exec.Command(name, args...).Output()
		return strings.TrimSpace(string(out))
	}
	return []crash.Section{
		{Title: "Version", Text: "Project B " + appVersion() + ", " + runtime.GOOS + "/" + runtime.GOARCH + ", pid " + strconv.Itoa(os.Getpid())},
		{Title: "macOS", Text: run("/usr/bin/sw_vers", "-productVersion") + " (" + run("/usr/bin/sw_vers", "-buildVersion") + ")"},
		{Title: "Device", Text: run("/usr/sbin/sysctl", "-n", "hw.model")},
	}
}

// downloadsDir is where downloads go: the user's own Downloads folder.
func downloadsDir(home string) string { return filepath.Join(home, "Downloads") }
