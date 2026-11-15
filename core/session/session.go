// Package session keeps the profile's windows and their tabs as they are, in session.json in
// its folder, so the app can open them again after a crash (docs/daily-driver.md). It
// writes only when what it would write changed.
package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"projectb/engine"
	"projectb/timeline"
)

// Session is the profile's windows, the one in front first, and what the person closed
// recently, newest first.
type Session struct {
	Windows []Window `json:"windows"`
	Closed  []Closed `json:"closed,omitempty"`
}

// Closed is a tab or a window the person closed. A tab says its window, by Go's id for it
// in the run that closed it, and its place there; a window is all of it.
type Closed struct {
	Tab    *Tab    `json:"tab,omitempty"`
	Window *Window `json:"window,omitempty"`
	In     int64   `json:"in,omitempty"`
	Index  int     `json:"index,omitempty"`
}

// ClosedLimit is how many closed tabs and windows are kept.
const ClosedLimit = 25

// Window is one window: where it was on screen, its tabs in order, the index of the active
// one, and its tab groups.
type Window struct {
	Bounds engine.Rect `json:"bounds"`
	Tabs   []Tab       `json:"tabs"`
	Active int         `json:"active"`
	Groups []Group     `json:"groups,omitempty"`
}

// Tab is one tab: its address, title and workspace, and its group's id, "" for none.
type Tab struct {
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Workspace string `json:"workspace"`
	Group     string `json:"group,omitempty"`
}

// Group is one tab group, by the id its tabs name it with.
type Group struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	Collapsed bool   `json:"collapsed,omitempty"`
}

// Manager keeps one profile's session file. Save encodes on the caller's thread and a writer
// of its own puts it on disk, so a tab closing never waits for the disk.
type Manager struct {
	path string
	last Session
	// written is the file as last handed to the writer, or read at start.
	written []byte
	writes  chan []byte
	done    chan struct{}
	// closed says the app has stopped: nothing is written after.
	closed bool
}

// New reads what the last run kept in the profile folder dir.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "session.json")}
	b, err := os.ReadFile(m.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		timeline.Errorf("session: %v", err)
	default:
		if err := json.Unmarshal(b, &m.last); err != nil {
			timeline.Errorf("session: %s: %v", m.path, err)
		}
		m.written = bytes.TrimSpace(b)
	}
	return m
}

// Last is what the last run kept, read at start.
func (m *Manager) Last() Session { return m.last }

// Save keeps s, when it differs from what the file holds: the writer has it at once, and
// only the newest waits when the disk is behind.
func (m *Manager) Save(s Session) {
	b, err := json.Marshal(s)
	if err != nil {
		timeline.Errorf("session: %v", err)
		return
	}
	if m.closed || bytes.Equal(b, m.written) {
		return
	}
	m.written = b
	if m.writes == nil {
		m.writes = make(chan []byte, 1)
		m.done = make(chan struct{})
		go m.writer(m.writes, m.done)
	}
	select {
	case <-m.writes:
	default:
	}
	m.writes <- b
}

// Close writes what is waiting and stops the writer, for good: the app is stopping.
func (m *Manager) Close() {
	m.closed = true
	if m.writes == nil {
		return
	}
	close(m.writes)
	<-m.done
	m.writes = nil
}

func (m *Manager) writer(writes <-chan []byte, done chan<- struct{}) {
	defer close(done)
	for b := range writes {
		m.write(b)
	}
}

// write puts b on disk, a new file renamed over the old, so no crash leaves half of one.
func (m *Manager) write(b []byte) {
	tmp := m.path + ".tmp"
	err := os.WriteFile(tmp, append(b, '\n'), 0o600)
	if err == nil {
		err = os.Rename(tmp, m.path)
	}
	if err != nil {
		timeline.Errorf("session: %v", err)
	}
}
