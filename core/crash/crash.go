// Package crash keeps what is known about crashes. While the app runs, crash.log in the
// profile says so: it starts with the process id, and if Go crashes the runtime writes
// its own report there, the panic or fatal error and every goroutine's stack, as well as
// to stderr, whichever goroutine crashed. A clean stop deletes it. So a crash.log found
// at the next start means the last run crashed, from Go or not: it is renamed after the
// time it was last written, so no crash overwrites another, and the shell is told where
// it is and where macOS put its own report of that process. Unless the system says that
// process ended for its own reasons: Android ends an app in the background when it wants
// the memory, when it's swiped away or stopped from its app info, and leaves crash.log as
// a crash would. Only a crash, a native crash or not responding are crashes then; any other
// reason goes in the timeline, and crash.log with it. Where Android keeps no reason, before
// 11 or after the phone lost its power, its crash log says whether that process crashed.
// A desktop says nothing, and there a crash.log left is a crash: a native one leaves
// nothing in it but its first line.
//
// A child process that crashes is one line in crashes.log, which is kept: when, its kind, how
// it ended, its pid, and for a renderer the origin of its page, not the whole address, which
// can hold what was searched for, as a report goes to other people.
package crash

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"projectb/engine"
	"projectb/timeline"
)

// Manager knows whether the last run crashed.
type Manager struct {
	current  string
	children string
	last     string
	report   string
	// System is what the platform says of itself for a report: the version, the OS and
	// the device, and on Android the app's own log. Nil says nothing.
	System func() []Section
}

// crashed are the system's reasons a process ended that are crashes (engine.ExitReason).
var crashed = map[string]bool{"crash": true, "native crash": true, "not responding": true}

// New starts this run's crash.log in dir, after keeping the last run's, if there is one,
// as crash-<time>.log, unless exitReason, why the process it names ended as the system
// keeps it, says that was no crash. reports is the folder macOS writes its crash reports to, "" on an OS that keeps none.
func New(dir, reports string, exitReason func(pid int) string) *Manager {
	m := &Manager{current: filepath.Join(dir, "crash.log"), children: filepath.Join(dir, "crashes.log")}
	if pid := loggedPID(m.current); pid != 0 {
		if why := exitReason(pid); why != "" && !crashed[why] {
			timeline.Infof("crash: the last run, pid %d, ended without crashing: %s", pid, why)
			if err := os.Remove(m.current); err != nil {
				timeline.Errorf("crash: %v", err)
			}
		} else if why != "" {
			timeline.Infof("crash: the last run, pid %d, ended: %s", pid, why)
		}
	}
	if info, err := os.Stat(m.current); err == nil {
		last := filepath.Join(dir, "crash-"+info.ModTime().Format("2006-01-02-150405")+".log")
		if err := os.Rename(m.current, last); err != nil {
			timeline.Errorf("crash: %v", err)
		} else {
			m.last = last
			m.report = osReport(last, info.ModTime(), reports)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		timeline.Errorf("crash: %v", err)
		return m
	}
	f, err := os.OpenFile(m.current, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		timeline.Errorf("crash: %v", err)
		return m
	}
	// The runtime keeps its own copy of the descriptor.
	defer f.Close()
	if _, err := fmt.Fprintf(f, "pid %d, started %s\n", os.Getpid(), time.Now().Format(time.DateTime)); err != nil {
		timeline.Errorf("crash: %v", err)
	}
	if err := setCrashOutput(f); err != nil {
		timeline.Errorf("crash: %v", err)
	}
	return m
}

// loggedPID is the process id a crash.log starts with, or 0 when there is none.
func loggedPID(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	var pid int
	if _, err := fmt.Fscanf(f, "pid %d,", &pid); err != nil {
		return 0
	}
	return pid
}

// osReport is macOS's report of the process that wrote the crash log at path, or the
// folder of reports if it hasn't written one there; "" where the OS keeps none, reports "".
func osReport(path string, since time.Time, reports string) string {
	if reports == "" {
		return ""
	}
	var pid int
	if f, err := os.Open(path); err == nil {
		fmt.Fscanf(f, "pid %d,", &pid)
		f.Close()
	}
	exe, err := os.Executable()
	if pid == 0 || err != nil {
		return reports
	}
	found, _ := filepath.Glob(filepath.Join(reports, filepath.Base(exe)+"-*.ips"))
	for _, name := range found {
		if info, err := os.Stat(name); err == nil && !info.ModTime().Before(since) && reportPID(name) == pid {
			return name
		}
	}
	return reports
}

// reportPID reads the process id from a macOS crash report: a line of JSON about the
// report, then the report, one JSON object.
func reportPID(name string) int {
	f, err := os.Open(name)
	if err != nil {
		return 0
	}
	defer f.Close()
	r := bufio.NewReader(f)
	if _, err := r.ReadString('\n'); err != nil {
		return 0
	}
	var body struct {
		PID int `json:"pid"`
	}
	json.NewDecoder(r).Decode(&body)
	return body.PID
}

// Stop says this run ends cleanly: there is no crash to report next time.
func (m *Manager) Stop() {
	if err := os.Remove(m.current); err != nil && !os.IsNotExist(err) {
		timeline.Errorf("crash: %v", err)
	}
}

// ProcessCrashed adds a line about a child process that crashed to crashes.log.
func (m *Manager) ProcessCrashed(p engine.ProcessCrash) {
	f, err := os.OpenFile(m.children, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		timeline.Errorf("crash: %v", err)
		return
	}
	defer f.Close()
	line := fmt.Sprintf("%s %s %s, exit code %d, pid %d", time.Now().Format(time.DateTime), p.Type, p.Status, p.ExitCode, p.PID)
	if o := origin(p.URL); o != "" {
		line += ", " + o
	}
	if _, err := fmt.Fprintln(f, line); err != nil {
		timeline.Errorf("crash: %v", err)
	}
}

// origin is the scheme and host of address, "" for none; an address with no host, as data:
// or about:blank, by its scheme alone.
func origin(address string) string {
	u, err := url.Parse(address)
	if err != nil || u.Scheme == "" {
		return ""
	}
	if u.Host == "" {
		return u.Scheme + ":"
	}
	return u.Scheme + "://" + u.Host
}

// Last is the log of the last run's crash, or "" if it didn't crash or the notice was
// closed.
func (m *Manager) Last() string { return m.last }

// Report is macOS's report of the last run's crash, or the folder macOS keeps its
// reports in when it has none for that run yet; "" when Last is.
func (m *Manager) Report() string { return m.report }

// Dismiss forgets the last crash: the user has seen where its report is.
func (m *Manager) Dismiss() { m.last, m.report = "", "" }
