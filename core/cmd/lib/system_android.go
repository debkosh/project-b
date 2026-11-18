package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"projectb/crash"
)

// systemReport is what a crash report says of this phone: the release, Android's version
// and the device, the end of the app's own log, where content says why it ended a process
// (logcat shows an app its own lines), and the app's last crashes Android kept. Android
// keeps those across runs, days back: apart, each with its date, an old one reads as old
// rather than as the top of this run's log.
func systemReport() []crash.Section {
	prop := func(name string) string {
		out, _ := exec.Command("/system/bin/getprop", name).Output()
		return strings.TrimSpace(string(out))
	}
	logcat := func(buffers, lines string) string {
		out, err := exec.Command("/system/bin/logcat", "-d", "-b", buffers, "-t", lines, "-v", "time").CombinedOutput()
		if err != nil {
			out = append(out, []byte("\n(logcat: "+err.Error()+")")...)
		}
		return string(out)
	}
	return []crash.Section{
		{Title: "Version", Text: "Project B " + appVersion() + ", " + runtime.GOOS + "/" + runtime.GOARCH + ", pid " + strconv.Itoa(os.Getpid())},
		{Title: "Android", Text: "Android " + prop("ro.build.version.release") + " (API " + prop("ro.build.version.sdk") + "), " +
			prop("ro.build.fingerprint")},
		{Title: "Device", Text: prop("ro.product.manufacturer") + " " + prop("ro.product.model") + ", page size " + strconv.Itoa(os.Getpagesize())},
		{Title: "The app's log, its end (logcat)", Text: logcat("main,system", "600")},
		{Title: "The app's last crashes Android kept, newest last (logcat crash buffer)", Text: logcat("crash", "50")},
	}
}

// downloadsDir is "" on a phone: a download lands in the profile's own Downloads folder, and
// the engine copies it, once done, into the phone's shared Downloads (docs/daily-driver.md).
func downloadsDir(string) string { return "" }
