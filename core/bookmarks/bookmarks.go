// Package bookmarks keeps a profile's bookmarks and their folders, one tree in bookmarks.json
// in the profile folder (docs/bookmark-folders.md). Each entry keeps an id made once, the
// folder it is in, when it was added and last changed, and its place in that folder, so an
// import and a later sync can tell one entry from another and a move changes that one alone.
package bookmarks

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"projectb/timeline"
)

// The kinds of entry.
const (
	KindBookmark = "bookmark"
	KindFolder   = "folder"
)

// Top is the folder at the top of the tree, All bookmarks, which is no entry.
const Top = ""

// Entry is a bookmark or a folder: its id, which never changes; the folder it is in; its
// title, and a bookmark's address; when it was added and last changed, in Unix
// milliseconds; and its place among its folder's entries, a key that sorts as text.
type Entry struct {
	ID       string `json:"id"`
	Parent   string `json:"parent"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	URL      string `json:"url,omitempty"`
	Added    int64  `json:"added"`
	Modified int64  `json:"modified"`
	Place    string `json:"place"`
}

// file is bookmarks.json: the tree, and the folder a bookmark was last saved in.
type file struct {
	Version int     `json:"version"`
	Last    string  `json:"last"`
	Entries []Entry `json:"entries"`
}

// v1 is bookmarks.json before folders: a flat list, newest first, added in Unix seconds.
type v1 struct {
	Bookmarks []struct {
		Title string `json:"title"`
		URL   string `json:"url"`
		Added int64  `json:"added"`
	} `json:"bookmarks"`
}

// Manager keeps one profile's bookmarks.
type Manager struct {
	path      string
	entries   []Entry
	last      string
	onChanged []func()
}

// New returns the bookmarks saved in the profile folder dir, read here: the file is small.
// A file from before folders becomes the top folder's bookmarks, newest first, saved so.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "bookmarks.json")}
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
	if f.Version >= 2 {
		m.entries, m.last = f.Entries, f.Last
		return m
	}
	var old v1
	json.Unmarshal(b, &old)
	places := spread(len(old.Bookmarks))
	for i, o := range old.Bookmarks {
		added := o.Added * 1000
		m.entries = append(m.entries, Entry{ID: newID(), Parent: Top, Kind: KindBookmark, Title: o.Title, URL: o.URL, Added: added, Modified: added, Place: places[i]})
	}
	timeline.Infof("bookmarks: %d moved into the top folder from the list before folders, in %s", len(m.entries), m.path)
	m.save()
	return m
}

// OnChanged subscribes to any change.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

// All is every entry, each folder's in their places.
func (m *Manager) All() []Entry {
	out := append([]Entry{}, m.entries...)
	slices.SortStableFunc(out, func(a, b Entry) int { return cmp.Compare(a.Place, b.Place) })
	return out
}

// List is every bookmark, in no folder in particular, the newest first.
func (m *Manager) List() []Entry {
	var out []Entry
	for _, e := range m.entries {
		if e.Kind == KindBookmark {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return cmp.Compare(b.Added, a.Added) })
	return out
}

// Get is the entry id.
func (m *Manager) Get(id string) (Entry, bool) {
	if i := m.index(id); i >= 0 {
		return m.entries[i], true
	}
	return Entry{}, false
}

// Has says whether an address is bookmarked.
func (m *Manager) Has(url string) bool { return m.find(url) >= 0 }

// Last is the folder a bookmark was last saved in, where the next goes: the top when that
// folder is gone or none was chosen.
func (m *Manager) Last() string {
	if i := m.index(m.last); i >= 0 && m.entries[i].Kind == KindFolder {
		return m.last
	}
	return Top
}

// Add bookmarks an address with a title, first in the folder last saved in, and returns
// it; an address bookmarked already is returned as it is.
func (m *Manager) Add(title, url string) (Entry, error) {
	if url == "" {
		return Entry{}, errors.New("bookmarks: a bookmark needs an address")
	}
	if i := m.find(url); i >= 0 {
		return m.entries[i], nil
	}
	e := m.put(Entry{Kind: KindBookmark, Title: or(title, url), URL: url}, m.Last())
	m.changed()
	return e, nil
}

// NewFolder makes a folder, first in folder parent, and returns it.
func (m *Manager) NewFolder(parent, title string) (Entry, error) {
	if !m.isFolder(parent) {
		return Entry{}, errors.New("bookmarks: no such folder")
	}
	if title == "" {
		return Entry{}, errors.New("bookmarks: a folder needs a name")
	}
	e := m.put(Entry{Kind: KindFolder, Title: title}, parent)
	m.changed()
	return e, nil
}

// Edit changes an entry's title, and a bookmark's address: an address another bookmark has
// is refused, and a folder takes none.
func (m *Manager) Edit(id, title, url string) error {
	i := m.index(id)
	if i < 0 {
		return errors.New("bookmarks: no such bookmark")
	}
	e := &m.entries[i]
	if e.Kind == KindFolder {
		if title == "" {
			return errors.New("bookmarks: a folder needs a name")
		}
		e.Title = title
	} else {
		if url == "" {
			return errors.New("bookmarks: a bookmark needs an address")
		}
		if j := m.find(url); j >= 0 && j != i {
			return errors.New("bookmarks: that address is bookmarked already")
		}
		e.Title, e.URL = or(title, url), url
	}
	e.Modified = now()
	m.changed()
	return nil
}

// Move puts an entry first in folder parent. A folder doesn't go into itself or a folder
// it holds.
func (m *Manager) Move(id, parent string) error {
	if err := m.move(id, parent); err != nil {
		return err
	}
	m.changed()
	return nil
}

// Save is what the star's panel saves: a bookmark's title and folder, which the next
// bookmark goes into too.
func (m *Manager) Save(id, title, parent string) error {
	i := m.index(id)
	if i < 0 || m.entries[i].Kind != KindBookmark {
		return errors.New("bookmarks: no such bookmark")
	}
	if err := m.move(id, parent); err != nil {
		return err
	}
	e := &m.entries[i]
	if t := or(title, e.URL); t != e.Title {
		e.Title, e.Modified = t, now()
	}
	m.last = parent
	m.changed()
	return nil
}

// Remove takes an entry away, and a folder with all it holds.
func (m *Manager) Remove(id string) {
	if m.index(id) < 0 {
		return
	}
	gone := map[string]bool{id: true}
	for grew := true; grew; {
		grew = false
		for _, e := range m.entries {
			if gone[e.Parent] && !gone[e.ID] {
				gone[e.ID], grew = true, true
			}
		}
	}
	m.entries = slices.DeleteFunc(m.entries, func(e Entry) bool { return gone[e.ID] })
	m.changed()
}

// RemoveURL takes away the bookmark of an address.
func (m *Manager) RemoveURL(url string) {
	if i := m.find(url); i >= 0 {
		m.Remove(m.entries[i].ID)
	}
}

// put adds e first in folder parent, with a new id and times, and returns it.
func (m *Manager) put(e Entry, parent string) Entry {
	e.ID, e.Parent, e.Place = newID(), parent, between("", m.first(parent, ""))
	e.Added = now()
	e.Modified = e.Added
	m.entries = append(m.entries, e)
	return e
}

func (m *Manager) move(id, parent string) error {
	i := m.index(id)
	if i < 0 {
		return errors.New("bookmarks: no such bookmark")
	}
	if !m.isFolder(parent) {
		return errors.New("bookmarks: no such folder")
	}
	for p := parent; p != Top; {
		if p == id {
			return errors.New("bookmarks: a folder can't go into itself")
		}
		j := m.index(p)
		if j < 0 {
			break
		}
		p = m.entries[j].Parent
	}
	if m.entries[i].Parent == parent {
		return nil
	}
	e := &m.entries[i]
	e.Parent, e.Place, e.Modified = parent, between("", m.first(parent, id)), now()
	return nil
}

// first is the first place in folder parent, but entry skip's, or "" for an empty folder.
func (m *Manager) first(parent, skip string) string {
	first := ""
	for _, e := range m.entries {
		if e.Parent == parent && e.ID != skip && (first == "" || e.Place < first) {
			first = e.Place
		}
	}
	return first
}

func (m *Manager) isFolder(id string) bool {
	if id == Top {
		return true
	}
	i := m.index(id)
	return i >= 0 && m.entries[i].Kind == KindFolder
}

func (m *Manager) index(id string) int {
	if id == "" {
		return -1
	}
	return slices.IndexFunc(m.entries, func(e Entry) bool { return e.ID == id })
}

func (m *Manager) find(url string) int {
	return slices.IndexFunc(m.entries, func(e Entry) bool { return e.Kind == KindBookmark && e.URL == url })
}

// changed saves the tree and tells the subscribers.
func (m *Manager) changed() {
	m.save()
	for _, f := range m.onChanged {
		f()
	}
}

// save writes the tree, a new file renamed over the old.
func (m *Manager) save() {
	b, err := json.MarshalIndent(file{Version: 2, Last: m.last, Entries: m.All()}, "", "  ")
	if err == nil {
		tmp := m.path + ".tmp"
		if err = os.WriteFile(tmp, append(b, '\n'), 0o600); err == nil {
			err = os.Rename(tmp, m.path)
		}
	}
	if err != nil {
		timeline.Errorf("bookmarks: %v", err)
	}
}

// newID is 16 random hex digits.
func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func now() int64 { return time.Now().UnixMilli() }

func or(s, otherwise string) string {
	if s == "" {
		return otherwise
	}
	return s
}

// Node is a bookmark, or a folder, which has no URL, with what it holds, as an import from
// another browser brings it; Added is in Unix milliseconds.
type Node struct {
	Title    string
	URL      string
	Added    int64
	Children []Node
}

// Imported is what an import added: bookmarks, and folders made for them, and the bookmarks
// left out as their address was bookmarked already.
type Imported struct {
	Bookmarks int `json:"bookmarks"`
	Folders   int `json:"folders"`
	Skipped   int `json:"skipped"`
}

// Import adds nodes to folder parent after what it holds, in their order. A folder goes
// into the folder of its name already there, so an import made again adds only what is
// new; an address bookmarked already is left out, and a folder left with nothing isn't made.
func (m *Manager) Import(parent string, nodes []Node) (Imported, error) {
	if !m.isFolder(parent) {
		return Imported{}, errors.New("bookmarks: no such folder")
	}
	var got Imported
	m.importInto(parent, nodes, &got)
	if got.Bookmarks > 0 {
		m.changed()
	}
	return got, nil
}

func (m *Manager) importInto(parent string, nodes []Node, got *Imported) {
	// Places come at the end: spread out in a folder that held nothing, else after its last.
	from := len(m.entries)
	last := ""
	for _, e := range m.entries {
		if e.Parent == parent && e.Place > last {
			last = e.Place
		}
	}
	t := now()
	for _, n := range nodes {
		if n.URL != "" {
			if m.find(n.URL) >= 0 {
				got.Skipped++
				continue
			}
			m.entries = append(m.entries, Entry{ID: newID(), Parent: parent, Kind: KindBookmark, Title: or(n.Title, n.URL), URL: n.URL, Added: or64(n.Added, t), Modified: t})
			got.Bookmarks++
			continue
		}
		title := or(n.Title, "Folder")
		if i := slices.IndexFunc(m.entries, func(e Entry) bool { return e.Parent == parent && e.Kind == KindFolder && e.Title == title }); i >= 0 {
			m.importInto(m.entries[i].ID, n.Children, got)
			continue
		}
		mark, before := len(m.entries), *got
		f := Entry{ID: newID(), Parent: parent, Kind: KindFolder, Title: title, Added: or64(n.Added, t), Modified: t}
		m.entries = append(m.entries, f)
		m.importInto(f.ID, n.Children, got)
		if got.Bookmarks == before.Bookmarks {
			// Nothing in it was new: what it would have held is gone with it.
			m.entries, *got = m.entries[:mark], Imported{Bookmarks: before.Bookmarks, Folders: before.Folders, Skipped: got.Skipped}
			continue
		}
		got.Folders++
	}
	// What this folder gained, its own entries only, in their order.
	var added []int
	for i := from; i < len(m.entries); i++ {
		if m.entries[i].Parent == parent {
			added = append(added, i)
		}
	}
	if last == "" {
		for j, p := range spread(len(added)) {
			m.entries[added[j]].Place = p
		}
		return
	}
	for _, i := range added {
		last = between(last, "")
		m.entries[i].Place = last
	}
}

func or64(n, otherwise int64) int64 {
	if n == 0 {
		return otherwise
	}
	return n
}
