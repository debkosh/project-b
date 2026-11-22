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

func (s *System) CopySecret(string) { timeline.Errorf("system: no CopySecret on Linux yet") }

// verifyPerson says there is nothing to check the person with, until a portal does it.
func (s *System) verifyPerson(id int32, _ system.PersonCheck) {
	if answered := s.checks[id]; answered != nil {
		delete(s.checks, id)
		answered(system.NoScreenLock)
	}
}

func (s *System) cancelPerson(int32) {}

// otherNote is none on Linux: every note is a page's.
func otherNote(string, string) bool { return false }

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
