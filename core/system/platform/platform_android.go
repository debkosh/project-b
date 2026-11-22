package platform

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_system_android.h"

static inline void call_open_file(const PBSystemCalls* c, const char* path) { c->open_file(path); }
static inline void call_reveal_file(const PBSystemCalls* c) { c->reveal_file(); }
static inline void call_copy_secret(const PBSystemCalls* c, const char* text) { c->copy_secret(text); }
static inline void call_show_update(const PBSystemCalls* c, const char* json) { c->show_update(json); }
static inline int call_is_default_browser(const PBSystemCalls* c) { return c->is_default_browser(); }
static inline void call_ask_default_browser(const PBSystemCalls* c) { c->ask_default_browser(); }
static inline void call_verify_person(const PBSystemCalls* c, int32_t id, const char* title, const char* subtitle) { c->verify_person(id, title, subtitle); }
static inline void call_cancel_person(const PBSystemCalls* c, int32_t id) { c->cancel_person(id); }
*/
import "C"

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	"projectb/crash"
	"projectb/system"
)

// calls is go_table.c's table of Java's calls, set before Go starts and never after.
var calls atomic.Pointer[C.PBSystemCalls]

// PBSetSystemCalls is go_table.c giving Go its way to Java's GoSystem. The table lives as long
// as the process.
//
//export PBSetSystemCalls
func PBSetSystemCalls(c *C.PBSystemCalls) { calls.Store(c) }

func (s *System) OpenFile(path string) {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	C.call_open_file(calls.Load(), c)
}

// CopySecret puts text on the clipboard as a clip marked sensitive.
func (s *System) CopySecret(text string) {
	c := C.CString(text)
	defer C.free(unsafe.Pointer(c))
	C.call_copy_secret(calls.Load(), c)
}

// ShowUpdate hands the updater's state, as its JSON, to amarit's UpdateNotification, the
// system's notification of a download, a release ready, or a failure a retry can help.
func ShowUpdate(stateJSON string) {
	c := C.CString(stateJSON)
	defer C.free(unsafe.Pointer(c))
	C.call_show_update(calls.Load(), c)
}

// RevealFile shows the system's Downloads, where a phone's finished downloads are.
func (s *System) RevealFile(string) { C.call_reveal_file(calls.Load()) }

func (s *System) IsDefaultBrowser() bool { return C.call_is_default_browser(calls.Load()) != 0 }

func (s *System) Desktop() string { return "" }

func (s *System) DefaultBrowserLive() bool { return true }

func (s *System) AskDefaultBrowser(answered func(bool)) {
	s.defaultBrowser.Ask(answered, func() { C.call_ask_default_browser(calls.Load()) })
}

// verifyPerson asks Android's own sheet, BiometricPrompt: a fingerprint, a face or the
// screen lock.
func (s *System) verifyPerson(id int32, check system.PersonCheck) {
	title, subtitle := C.CString(check.Title), C.CString(check.Subtitle)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(subtitle))
	C.call_verify_person(calls.Load(), C.int32_t(id), title, subtitle)
}

func (s *System) cancelPerson(id int32) { C.call_cancel_person(calls.Load(), C.int32_t(id)) }

// otherNote is none on a phone: amarit's update notification is Java's (ShowUpdate).
func otherNote(string, string) bool { return false }

// OnOpenURLs takes the links other apps send, which Java's BrowserActivity hands go_table.c,
// once Go has started.
func (s *System) OnOpenURLs(opened func(urls []string)) { s.opened = opened }

// SetDockColor does nothing: a phone has no Dock.
func (s *System) SetDockColor(string) {}

// OpenFullDiskAccess does nothing: a phone's data is each app's own.
func (s *System) OpenFullDiskAccess() {}

// Report is what a crash report says of this phone: the release, Android's version and the
// device, the end of the app's own log, where content says why it ended a process (logcat
// shows an app its own lines), and the app's last crashes Android kept. Android keeps those
// across runs, days back: apart, each with its date, an old one reads as old rather than as
// the top of this run's log.
func (s *System) Report(version string) []crash.Section {
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
		{Title: "Version", Text: "Project B " + version + ", " + runtime.GOOS + "/" + runtime.GOARCH + ", pid " + strconv.Itoa(os.Getpid())},
		{Title: "Android", Text: "Android " + prop("ro.build.version.release") + " (API " + prop("ro.build.version.sdk") + "), " +
			prop("ro.build.fingerprint")},
		{Title: "Device", Text: prop("ro.product.manufacturer") + " " + prop("ro.product.model") + ", page size " + strconv.Itoa(os.Getpagesize())},
		{Title: "The app's log, its end (logcat)", Text: logcat("main,system", "600")},
		{Title: "The app's last crashes Android kept, newest last (logcat crash buffer)", Text: logcat("crash", "50")},
	}
}

// DownloadsDir is "" on a phone: a download lands in the profile's own Downloads folder, and
// the engine copies it, once done, into the phone's shared Downloads (docs/daily-driver.md).
func DownloadsDir(string) string { return "" }
