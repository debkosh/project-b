//go:build !android

package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"projectb/crash"
	"projectb/system"
	"projectb/timeline"
)

// Linux's calls are desktop portals over D-Bus, from pure Go (docs/research/linux.md, 4), most
// of them oskit's. What has no portal yet says it isn't on this system, and every question is
// answered at once, so the managers run as they do elsewhere.

// writeClipboard is the engine's clipboard, plain text, which the library hands over at start
// (UseClipboard). Main thread.
var writeClipboard func(text string)

// UseClipboard gives CopySecret the engine's clipboard. Once, as the app starts.
func (s *System) UseClipboard(write func(text string)) { writeClipboard = write }

// CopySecret puts the text on the clipboard as Copy Link does, through the engine: no portal
// marks a clipboard entry as a secret, so it is plain text, as the desktop's other apps copy a
// password.
func (s *System) CopySecret(text string) {
	if writeClipboard == nil {
		timeline.Errorf("system: no clipboard to copy to")
		return
	}
	writeClipboard(text)
}

// DefaultBrowserLive is false in the Flatpak: its permission to read mimeapps.list mounts the
// file as it was when the sandbox started, so a change the desktop's settings make meanwhile,
// which replace the file, shows at the next start.
func (s *System) DefaultBrowserLive() bool { return os.Getenv("FLATPAK_ID") == "" }

// Desktop is GNOME or KDE, by XDG_CURRENT_DESKTOP, which may list several ("ubuntu:GNOME"),
// and other for the rest.
func (s *System) Desktop() string {
	for _, d := range strings.Split(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		switch strings.ToUpper(d) {
		case "GNOME":
			return "gnome"
		case "KDE":
			return "kde"
		}
	}
	return "other"
}

// verifyPerson says there is nothing to check the person with, until a portal does it.
func (s *System) verifyPerson(id int32, _ system.PersonCheck) {
	if answered := s.checks[id]; answered != nil {
		delete(s.checks, id)
		answered(system.NoScreenLock)
	}
}

func (s *System) cancelPerson(int32) {}

// otherNote is amarit's update notice on Linux, which its flatpak package posts and answers
// itself (id "amarit-update"): a press on it is no page's.
func otherNote(id, _ string) bool { return id == "amarit-update" }

// SetDockColor and OpenFullDiskAccess are the Mac's: nothing on Linux.
func (s *System) SetDockColor(string) {}
func (s *System) OpenFullDiskAccess() {}

// Report is what a crash report says of this machine: the release, the distribution and the
// kernel, and whether it runs in the Flatpak.
func (s *System) Report(version string) []crash.Section {
	// Inside the Flatpak, /etc/os-release is the runtime's; the host's is under /run/host.
	osName := "unknown"
	release, err := os.ReadFile("/run/host/os-release")
	if err != nil {
		release, err = os.ReadFile("/etc/os-release")
	}
	if err == nil {
		for _, line := range strings.Split(string(release), "\n") {
			if v, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
				osName = strings.Trim(v, `"`)
			}
		}
	}
	kernel, _ := os.ReadFile("/proc/sys/kernel/osrelease")
	inside := "no"
	if id := os.Getenv("FLATPAK_ID"); id != "" {
		inside = id
	}
	return []crash.Section{
		{Title: "Version", Text: "Project B " + version + ", " + runtime.GOOS + "/" + runtime.GOARCH + ", pid " + strconv.Itoa(os.Getpid())},
		{Title: "Linux", Text: osName + ", kernel " + strings.TrimSpace(string(kernel)) + ", Flatpak " + inside},
	}
}

// DownloadsDir is the user's Downloads folder; in the Flatpak, xdg-download (5) makes it the
// same path as outside. A desktop that names it otherwise (user-dirs.dirs) comes with step 4.
func DownloadsDir(home string) string { return filepath.Join(home, "Downloads") }
