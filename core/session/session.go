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

// Session is the profile's windows, the one in front first.
type Session struct {
	Windows []Window `json:"windows"`
}

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

// Manager keeps one profile's session file.
type Manager struct {
	path string
	last Session
	// written is the file as last written, or read at start.
	written []byte
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

// Save keeps s, when it differs from what the file holds.
func (m *Manager) Save(s Session) {
	b, err := json.Marshal(s)
	if err != nil {
		timeline.Errorf("session: %v", err)
		return
	}
	if bytes.Equal(b, m.written) {
		return
	}
	// A new file renamed over the old, so no crash leaves half of one.
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err == nil {
		err = os.Rename(tmp, m.path)
	}
	if err != nil {
		timeline.Errorf("session: %v", err)
		return
	}
	m.written = b
}
