// Package pins owns the profile's pinned tabs. A pin is a saved place in a workspace: an
// address, with the title and icon its tab had when it was pinned. A pin outlives the tabs
// open for it: closing one keeps the pin, and after a restart, which restores no tabs, it
// has none until it is opened. Only unpinning it, or deleting its workspace, removes it.
// The list is one small file in the profile, pins.json beside workspaces.json, read when
// the app starts and written when it changes: each pin's id, which Go makes and never
// changes, its workspace, address, title and icon, in the order pinned, a new pin after
// the others of its workspace. Which tab is open for a pin is the tabs' (tabs.Tab.Pin).
package pins

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"projectb/timeline"
)

// Pin is one pin, as the file holds it.
type Pin struct {
	ID        string `json:"id"`
	Workspace string `json:"workspace"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	// Favicon is a data: URL, or "" when the page had none.
	Favicon string `json:"favicon"`
}

// Manager owns the list.
type Manager struct {
	path      string
	list      []Pin
	onChanged []func()
}

// New returns a manager for the list in the profile folder dir, read here.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "pins.json"), list: []Pin{}}
	if b, err := os.ReadFile(m.path); err == nil {
		if err := json.Unmarshal(b, &m.list); err != nil {
			timeline.Errorf("pins: %s: %v", m.path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		timeline.Errorf("pins: %v", err)
	}
	return m
}

// List returns a copy of the pins, in their order.
func (m *Manager) List() []Pin { return append([]Pin{}, m.list...) }

// Get returns a pin by its id.
func (m *Manager) Get(id string) (Pin, bool) {
	if i := m.Index(id); i >= 0 {
		return m.list[i], true
	}
	return Pin{}, false
}

// Index is where a pin is in the order, or -1 when there is no such pin.
func (m *Manager) Index(id string) int {
	return slices.IndexFunc(m.list, func(p Pin) bool { return p.ID == id })
}

// OnChanged subscribes to a pin being made or removed.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// Add makes a pin of an address in a workspace, after that workspace's other pins, with an
// id Go makes: eight hex digits.
func (m *Manager) Add(workspace, url, title, favicon string) Pin {
	p := Pin{ID: m.newID(), Workspace: workspace, URL: url, Title: title, Favicon: favicon}
	at := len(m.list)
	for i, q := range m.list {
		if q.Workspace == workspace {
			at = i + 1
		}
	}
	m.list = slices.Insert(m.list, at, p)
	m.changed()
	return p
}

// Remove takes a pin out of the list.
func (m *Manager) Remove(id string) error {
	i := m.Index(id)
	if i < 0 {
		return errors.New("no pin " + id)
	}
	m.list = slices.Delete(m.list, i, i+1)
	m.changed()
	return nil
}

// RemoveWorkspace takes a workspace's pins out of the list, as the workspace is deleted.
func (m *Manager) RemoveWorkspace(workspace string) {
	n := len(m.list)
	m.list = slices.DeleteFunc(m.list, func(p Pin) bool { return p.Workspace == workspace })
	if len(m.list) != n {
		m.changed()
	}
}

func (m *Manager) newID() string {
	for {
		b := make([]byte, 4)
		rand.Read(b)
		if id := hex.EncodeToString(b); m.Index(id) < 0 {
			return id
		}
	}
}

func (m *Manager) changed() {
	m.save()
	for _, f := range m.onChanged {
		f()
	}
}

func (m *Manager) save() {
	b, _ := json.MarshalIndent(m.list, "", "  ")
	// A new file renamed over the old, so no reader and no crash sees half of one.
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		timeline.Errorf("pins: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		timeline.Errorf("pins: %v", err)
	}
}
