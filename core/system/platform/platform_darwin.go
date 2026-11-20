package platform

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UserNotifications -framework LocalAuthentication
#include <stdlib.h>
#include "platform_darwin.h"
*/
import "C"

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"projectb/crash"
	"projectb/system"
	"projectb/timeline"
	"projectb/tones"
)

func withPath(path string, call func(*C.char)) {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	call(c)
}

func (s *System) OpenFile(path string) { withPath(path, func(c *C.char) { C.pb_system_open_file(c) }) }
func (s *System) RevealFile(path string) {
	withPath(path, func(c *C.char) { C.pb_system_reveal_file(c) })
}

// CopySecret puts text on the pasteboard with the concealed and transient types beside it.
func (s *System) CopySecret(text string) {
	withPath(text, func(c *C.char) { C.pb_system_copy_secret(c) })
}

func (s *System) IsDefaultBrowser() bool { return C.pb_system_is_default_browser() != 0 }

func (s *System) AskDefaultBrowser(answered func(bool)) {
	s.defaultBrowser.Ask(answered, func() { C.pb_system_ask_default_browser() })
}

// A Mac's notifications are the app's own to show: allowed, with nothing to ask.
func (s *System) NotificationsAllowed() bool { return true }

func (s *System) AskNotifications(answered func(bool)) {
	s.notifications.Ask(answered, func() { C.pb_system_answer_notifications() })
}

// verifyPerson asks macOS's own sheet: Touch ID, else the Mac's password.
func (s *System) verifyPerson(id int32, check system.PersonCheck) {
	withPath(check.Reason, func(c *C.char) { C.pb_system_verify_person(C.int32_t(id), c) })
}

func (s *System) cancelPerson(id int32) { C.pb_system_cancel_person(C.int32_t(id)) }

func (s *System) SetDockColor(color string) {
	if color == tones.None {
		C.pb_system_set_dock_color(-1)
		return
	}
	rgb, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32)
	if err != nil {
		timeline.Errorf("dock colour %q is not #rrggbb", color)
		return
	}
	C.pb_system_set_dock_color(C.int32_t(rgb))
}

func (s *System) ShowNotification(n system.Notification) {
	id, title, subtitle, body := C.CString(n.ID), C.CString(n.Title), C.CString(n.Subtitle), C.CString(n.Body)
	defer C.free(unsafe.Pointer(id))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(subtitle))
	defer C.free(unsafe.Pointer(body))
	C.pb_system_show_notification(id, title, subtitle, body)
}

func (s *System) RemoveNotification(id string) {
	withPath(id, func(c *C.char) { C.pb_system_remove_notification(c) })
}

func (s *System) OnOpenURLs(opened func(urls []string)) {
	s.opened = opened
	C.pb_system_receive_links()
}

// fullDiskAccess is System Settings' Privacy & Security, at Full Disk Access.
const fullDiskAccess = "x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"

func (s *System) OpenFullDiskAccess() {
	if out, err := exec.Command("/usr/bin/open", fullDiskAccess).CombinedOutput(); err != nil {
		timeline.Errorf("can't open System Settings at Full Disk Access: %v %s", err, out)
	}
}

// Report is what a crash report says of this Mac: the release, macOS's version and the model.
// macOS's own report of a crash is added from the crash manager's.
func (s *System) Report(version string) []crash.Section {
	run := func(name string, args ...string) string {
		out, _ := exec.Command(name, args...).Output()
		return strings.TrimSpace(string(out))
	}
	return []crash.Section{
		{Title: "Version", Text: "Project B " + version + ", " + runtime.GOOS + "/" + runtime.GOARCH + ", pid " + strconv.Itoa(os.Getpid())},
		{Title: "macOS", Text: run("/usr/bin/sw_vers", "-productVersion") + " (" + run("/usr/bin/sw_vers", "-buildVersion") + ")"},
		{Title: "Device", Text: run("/usr/sbin/sysctl", "-n", "hw.model")},
	}
}

// DownloadsDir is where downloads go on a Mac: the user's own Downloads folder.
func DownloadsDir(home string) string { return filepath.Join(home, "Downloads") }
