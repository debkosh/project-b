package crash

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projectb/timeline"
)

// Section is one titled part of a crash report.
type Section struct {
	Title, Text string
}

// WriteReport writes one plain-text report the user can hand on, in the profile's
// reports folder, and returns its path. It holds what the platform says of itself
// (System: the version, the OS and the device, and on Android the app's own log), the
// last run's crash log and the system's report of it, the child processes that crashed,
// and the ends of this run's and the last run's timelines. Nothing leaves the device
// unless the user shares the file.
func (m *Manager) WriteReport() (string, error) {
	dir := filepath.Dir(m.current)
	var b strings.Builder
	// UTC, said so: Go on Android has no time zone, and a report goes to other people.
	now := time.Now().UTC()
	fmt.Fprintf(&b, "Project B crash report, %s UTC\n", now.Format(time.DateTime))
	var sections []Section
	if m.System != nil {
		sections = append(sections, m.System()...)
	}
	if m.last != "" {
		sections = append(sections, Section{"The last run's crash log, " + filepath.Base(m.last), tail(m.last, 400)})
		if m.report != "" {
			sections = append(sections, Section{"The system's report of it, " + m.report, tail(m.report, 200)})
		}
	}
	sections = append(sections,
		Section{"Processes that crashed (crashes.log)", tail(m.children, 50)},
		Section{"This run's timeline, its end", tail(filepath.Join(dir, "timeline.log"), 300)},
		Section{"The last run's timeline, its end", tail(filepath.Join(dir, "timeline.previous.log"), 200)},
	)
	for _, s := range sections {
		text := strings.TrimRight(s.Text, "\n")
		if text == "" {
			text = "(none)"
		}
		fmt.Fprintf(&b, "\n== %s\n%s\n", s.Title, text)
	}
	reports := filepath.Join(dir, "reports")
	if err := os.MkdirAll(reports, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(reports, "project-b-report-"+now.Format("2006-01-02-150405")+"Z.txt")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", err
	}
	timeline.Mark("crash.report")
	return path, nil
}

// tail is the last n lines of the file at path, "" when there is none. A folder, as the
// system's report can be while it has none for the run yet, is no file.
func tail(path string, n int) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = append([]string{fmt.Sprintf("(the last %d of %d lines)", n, len(lines))}, lines[len(lines)-n:]...)
	}
	return strings.Join(lines, "\n")
}
