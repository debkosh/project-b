package platform

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_system_android.h"

static inline void call_show_update(const PBSystemCalls* c, const char* json) { c->show_update(json); }
static inline void call_verify_person(const PBSystemCalls* c, int32_t id, const char* title, const char* subtitle) { c->verify_person(id, title, subtitle); }
static inline void call_cancel_person(const PBSystemCalls* c, int32_t id) { c->cancel_person(id); }
static inline void call_scan_code(const PBSystemCalls* c, int32_t id, const char* prompt) { c->scan_code(id, prompt); }
static inline char* call_process_ends(const PBSystemCalls* c) { return c->process_ends(); }
static inline void call_relaunch_after_quit(const PBSystemCalls* c) { c->relaunch_after_quit(); }
*/
import "C"

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/debkosh/project-b/oskit/clip"

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

// CopySecret puts text on the clipboard as a clip marked sensitive (oskit/clip).
func (s *System) CopySecret(text string) { clip.New().CopySecret(text) }

// ShowUpdate hands the updater's state, as its JSON, to amarit's UpdateNotification, the
// system's notification of a download, a release ready, or a failure a retry can help.
func ShowUpdate(stateJSON string) {
	c := C.CString(stateJSON)
	defer C.free(unsafe.Pointer(c))
	C.call_show_update(calls.Load(), c)
}

// RelaunchAfterQuit has Android start the app again once it has quit: relaunch.Self on a
// phone, through the activity, as Chrome restarts, with no program started here.
func RelaunchAfterQuit() { C.call_relaunch_after_quit(calls.Load()) }

func (s *System) Desktop() string { return "" }

func (s *System) DefaultBrowserLive() bool { return true }

// MediaOS is Android: its runtime permissions let the app use the camera and microphone.
func (s *System) MediaOS() string { return "android" }

// verifyPerson asks Android's own sheet, BiometricPrompt: a fingerprint, a face or the
// screen lock.
func (s *System) verifyPerson(id int32, check system.PersonCheck) {
	title, subtitle := C.CString(check.Title), C.CString(check.Subtitle)
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(subtitle))
	C.call_verify_person(calls.Load(), C.int32_t(id), title, subtitle)
}

func (s *System) cancelPerson(id int32) { C.call_cancel_person(calls.Load(), C.int32_t(id)) }

// CanVerifyPerson: a phone has BiometricPrompt.
func (s *System) CanVerifyPerson() bool { return true }

// CanScanCode: a phone has the scanner (engine/android/third_party/zxing).
func (s *System) CanScanCode() bool { return true }

// scanCode opens the scanner's own screen over the app, which asks for the camera the
// first time.
func (s *System) scanCode(id int32, prompt string) {
	p := C.CString(prompt)
	defer C.free(unsafe.Pointer(p))
	C.call_scan_code(calls.Load(), C.int32_t(id), p)
}

// otherNote is none on a phone: amarit's update notification is Java's (ShowUpdate).
func otherNote(string, string) bool { return false }

// SetDockColor does nothing: a phone has no Dock.
func (s *System) SetDockColor(string) {}

// OpenFullDiskAccess does nothing: a phone's data is each app's own.
func (s *System) OpenFullDiskAccess() {}

// Report is what a crash report says of this phone: the release, Android's version and the
// device, the end of the app's own log, where content says why it ended a process (logcat
// shows an app its own lines), the app's last crashes Android kept, and Android's record of
// each process of the app that ended. Android keeps those across runs, days back: apart, each
// with its date, an old one reads as old rather than as the top of this run's log. The record
// says why the app's process or the GPU process ended; not a renderer's crash, which Android
// files under the renderer's own isolated user id (ProcessEnds.java).
func (s *System) Report(version string, ended []int) []crash.Section {
	prop := func(name string) string {
		out, _ := run("/system/bin/getprop", name)
		return strings.TrimSpace(out)
	}
	// The app's own lines: once the app may read every app's log (READ_LOGS, which only adb
	// grants), logcat would give the whole phone's.
	// logcat's -t counts lines before --uid keeps the app's, so the end is cut here.
	own := "--uid=" + strconv.Itoa(os.Getuid())
	mine := lastLines(logcat("-b", "main,system", own), 600)
	return []crash.Section{
		{Title: "Version", Text: "Project B " + version + ", " + runtime.GOOS + "/" + runtime.GOARCH + ", pid " + strconv.Itoa(os.Getpid())},
		{Title: "Android", Text: "Android " + prop("ro.build.version.release") + " (API " + prop("ro.build.version.sdk") + "), " +
			prop("ro.build.fingerprint")},
		{Title: "Device", Text: prop("ro.product.manufacturer") + " " + prop("ro.product.model") + ", page size " + strconv.Itoa(os.Getpagesize())},
		{Title: "The app's log, its end (logcat)", Text: mine},
		{Title: "The app's last crashes Android kept, newest last (logcat crash buffer)", Text: lastLines(logcat("-b", "crash", own), 50)},
		{Title: "The children that crashed in this run, their own lines, the newest three (logcat, by pid)", Text: childLines(ended)},
		{Title: "How Android says the app's processes ended, newest first (ApplicationExitInfo)", Text: processEnds()},
	}
}

// logcat is the end of the phone's log as the app may read it, each line with its time.
func logcat(args ...string) string {
	out, err := run(append([]string{"/system/bin/logcat", "-d", "-v", "time"}, args...)...)
	if err != nil {
		out += "\n(logcat: " + err.Error() + ")"
	}
	return out
}

// lastLines is text's last n lines.
func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// childLines is what the children in ended wrote before they crashed, found by their pids:
// a page's renderer's abort message and stack among them, as "Check failed: ... Unable to
// find resource: 47513" was on Google's page. ended is the crash log's, content's word of
// each, since a phone's own log may name none of them. A renderer runs under a user id of
// its own, so Android shows the app its lines only once adb has granted it READ_LOGS
// (docs/android.md, "Crash logs").
func childLines(ended []int) string {
	if len(ended) == 0 {
		return "(no child of the app crashed in this run)"
	}
	var b strings.Builder
	for _, pid := range ended {
		lines := strings.TrimSpace(logcat("-b", "main,crash", "-t", "120", "--pid="+strconv.Itoa(pid)))
		if !strings.Contains(lines, "\n") {
			lines = "(none the app may read: Android shows it a renderer's lines once adb has run " +
				"pm grant " + packageName() + " android.permission.READ_LOGS)"
		}
		fmt.Fprintf(&b, "pid %d:\n%s\n\n", pid, lines)
	}
	return strings.TrimSpace(b.String())
}

// packageName is the app's, its main process's name.
func packageName() string {
	b, _ := os.ReadFile("/proc/self/cmdline")
	name, _, _ := strings.Cut(string(b), "\x00")
	return name
}

// run runs one of the phone's programs and answers what it wrote, its output and its errors
// together. It starts it with syscall.ForkExec, never os/exec: before its first child, Go's os
// package tries pidfd_open, which Android 10's seccomp answers with SIGSYS, and Go lets that
// pass only when its own handler has SIGSYS, which a shared library's Go never installs, so
// the app ended (Go issue 69065, mended for programs only). ForkExec and Wait4 use no pidfd.
func run(argv ...string) (string, error) {
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_CLOEXEC); err != nil {
		return "", err
	}
	r := os.NewFile(uintptr(p[0]), "output")
	defer r.Close()
	none := ^uintptr(0) // no input: the child's 0 is closed
	pid, err := syscall.ForkExec(argv[0], argv, &syscall.ProcAttr{
		Env:   os.Environ(),
		Files: []uintptr{none, uintptr(p[1]), uintptr(p[1])},
	})
	syscall.Close(p[1])
	if err != nil {
		return "", err
	}
	out, _ := io.ReadAll(r)
	var status syscall.WaitStatus
	for {
		_, err = syscall.Wait4(pid, &status, 0, nil)
		if err != syscall.EINTR {
			break
		}
	}
	if err == nil && (!status.Exited() || status.ExitStatus() != 0) {
		err = fmt.Errorf("%s ended with %v", argv[0], status)
	}
	return string(out), err
}

// processEnds is ProcessEnds.java's text, or says Java couldn't give it.
func processEnds() string {
	c := C.call_process_ends(calls.Load())
	if c == nil {
		return "(Java gave nothing: its exception is in the app's log)"
	}
	defer C.free(unsafe.Pointer(c))
	return C.GoString(c)
}

// DownloadsDir is "" on a phone: a download lands in the profile's own Downloads folder, and
// the engine copies it, once done, into the phone's shared Downloads (docs/daily-driver.md).
func DownloadsDir(string) string { return "" }
