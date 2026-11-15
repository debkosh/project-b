// Package bookmarks keeps a profile's bookmarks: a flat list, newest first, each a title and
// an address, in bookmarks.json in the profile folder (docs/daily-driver.md). Folders are a
// later step.
package bookmarks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projectb/timeline"
)

// Bookmark is one: its id, which never changes, its title and address, and when it was
// made, in Unix seconds.
type Bookmark struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Added int64  `json:"added"`
}

type file struct {
	Version   int        `json:"version"`
	Next      int        `json:"next"`
	Bookmarks []Bookmark `json:"bookmarks"`
}

// Manager keeps one profile's bookmarks.
type Manager struct {
	path      string
	list      []Bookmark
	next      int
	onChanged []func()
}

// New returns the bookmarks saved in the profile folder dir, read here: the file is small.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "bookmarks.json"), next: 1}
	b, err := os.ReadFile(m.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("bookmarks: %v", err)
		}
		return m
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		timeline.Errorf("bookmarks: %s: %v", m.path, err)
		return m
	}
	m.list, m.next = f.Bookmarks, max(f.Next, 1)
	return m
}

// OnChanged subscribes to any change.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// List is every bookmark, newest first.
func (m *Manager) List() []Bookmark { return append([]Bookmark{}, m.list...) }

// Has says whether an address is bookmarked.
func (m *Manager) Has(url string) bool { return m.find(url) >= 0 }

// Search is the bookmarks whose title or address holds text, newest first.
func (m *Manager) Search(text string, limit int) []Bookmark {
	q := strings.ToLower(strings.TrimSpace(text))
	var out []Bookmark
	for _, b := range m.list {
		if len(out) == limit {
			break
		}
		if q == "" || strings.Contains(strings.ToLower(b.Title), q) || strings.Contains(strings.ToLower(b.URL), q) {
			out = append(out, b)
		}
	}
	return out
}

// Add bookmarks an address with a title, or does nothing when it is bookmarked already.
func (m *Manager) Add(title, url string) {
	if url == "" || m.Has(url) {
		return
	}
	if title == "" {
		title = url
	}
	m.list = append([]Bookmark{{ID: m.next, Title: title, URL: url, Added: time.Now().Unix()}}, m.list...)
	m.next++
	m.changed()
}

// Edit changes a bookmark's title and address. An address another bookmark has is refused.
func (m *Manager) Edit(id int, title, url string) error {
	i := m.index(id)
	if i < 0 {
		return errors.New("bookmarks: no such bookmark")
	}
	if url == "" {
		return errors.New("bookmarks: a bookmark needs an address")
	}
	if j := m.find(url); j >= 0 && j != i {
		return errors.New("bookmarks: that address is bookmarked already")
	}
	if title == "" {
		title = url
	}
	m.list[i].Title, m.list[i].URL = title, url
	m.changed()
	return nil
}

// Remove takes bookmark id away.
func (m *Manager) Remove(id int) {
	if i := m.index(id); i >= 0 {
		m.list = append(m.list[:i:i], m.list[i+1:]...)
		m.changed()
	}
}

// RemoveURL takes away the bookmark of an address, as the star does.
func (m *Manager) RemoveURL(url string) {
	if i := m.find(url); i >= 0 {
		m.Remove(m.list[i].ID)
	}
}

func (m *Manager) index(id int) int {
	for i, b := range m.list {
		if b.ID == id {
			return i
		}
	}
	return -1
}

func (m *Manager) find(url string) int {
	for i, b := range m.list {
		if b.URL == url {
			return i
		}
	}
	return -1
}

// changed saves the list, a new file renamed over the old, and tells the subscribers.
func (m *Manager) changed() {
	b, err := json.MarshalIndent(file{Version: 1, Next: m.next, Bookmarks: m.List()}, "", "  ")
	if err == nil {
		tmp := m.path + ".tmp"
		if err = os.WriteFile(tmp, append(b, '\n'), 0o600); err == nil {
			err = os.Rename(tmp, m.path)
		}
	}
	if err != nil {
		timeline.Errorf("bookmarks: %v", err)
	}
	for _, f := range m.onChanged {
		f()
	}
}
