// Package bookmarks keeps a profile's bookmarks and their folders, one tree in the profile's
// store (docs/bookmark-folders.md, docs/store.md). Each entry keeps an id made once, the
// folder it is in, when it was added and last changed, and its place in that folder, so an
// import and a later sync can tell one entry from another and a move changes that one alone.
// The tree is read whole at start and kept in memory; each change writes the rows it changed,
// on the store's worker.
package bookmarks

import (
	"cmp"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"projectb/store"
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

// file is the bookmarks.json of a version before the store: the tree, and the folder a
// bookmark was last saved in.
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
	st        *store.Store
	entries   []Entry
	last      string
	onChanged []func()
}

// New returns the bookmarks of the profile folder dir, kept in st and read here, as they are
// few: the main thread waits for the store's worker. The first start with the store reads
// the bookmarks.json an earlier version kept, once.
func New(dir string, st *store.Store) *Manager {
	m := &Manager{st: st}
	st.Schema("bookmarks", func(tx *store.Tx) error { return made(tx, filepath.Join(dir, "bookmarks.json")) })
	st.Wait(func(tx *store.Tx) error {
		r, err := tx.Query(`SELECT id, parent, kind, title, url, added, edited, place FROM bookmarks ORDER BY place, id`)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			var e Entry
			if err := r.Scan(&e.ID, &e.Parent, &e.Kind, &e.Title, &e.URL, &e.Added, &e.Modified, &e.Place); err != nil {
				return err
			}
			m.entries = append(m.entries, e)
		}
		if err := r.Err(); err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT folder FROM bookmarks_last`).Scan(&m.last); err != sql.ErrNoRows {
			return err
		}
		return nil
	})
	return m
}

// made is the first step of the schema: the tables, and what bookmarks.json held, in either
// of its versions. One from before folders becomes the top folder's bookmarks, newest first.
func made(tx *store.Tx, old string) error {
	if _, err := tx.Exec(`
CREATE TABLE bookmarks (
  id TEXT PRIMARY KEY,
  parent TEXT NOT NULL,
  kind TEXT NOT NULL,
  title TEXT NOT NULL,
  url TEXT NOT NULL,
  added INTEGER NOT NULL,
  edited INTEGER NOT NULL,
  place TEXT NOT NULL
) WITHOUT ROWID;
-- One row: the folder a bookmark was last saved in.
CREATE TABLE bookmarks_last (folder TEXT NOT NULL);`); err != nil {
		return err
	}
	b, err := os.ReadFile(old)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		timeline.Errorf("bookmarks: %s: %v", old, err)
		return nil
	}
	if f.Version < 2 {
		var v v1
		json.Unmarshal(b, &v)
		places := spread(len(v.Bookmarks))
		for i, o := range v.Bookmarks {
			added := o.Added * 1000
			f.Entries = append(f.Entries, Entry{ID: newID(), Parent: Top, Kind: KindBookmark, Title: o.Title, URL: o.URL, Added: added, Modified: added, Place: places[i]})
		}
	}
	if err := put(tx, f.Entries...); err != nil {
		return err
	}
	if err := last(tx, f.Last); err != nil {
		return err
	}
	timeline.Infof("bookmarks: %d read once from %s, version %d", len(f.Entries), old, max(f.Version, 1))
	return nil
}

// put writes entries, new or changed.
func put(tx *store.Tx, es ...Entry) error {
	for _, e := range es {
		if _, err := tx.Exec(`INSERT INTO bookmarks (id, parent, kind, title, url, added, edited, place) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET parent = excluded.parent, title = excluded.title, url = excluded.url, edited = excluded.edited, place = excluded.place`,
			e.ID, e.Parent, e.Kind, e.Title, e.URL, e.Added, e.Modified, e.Place); err != nil {
			return err
		}
	}
	return nil
}

func last(tx *store.Tx, folder string) error {
	if _, err := tx.Exec(`DELETE FROM bookmarks_last`); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO bookmarks_last VALUES (?)`, folder)
	return err
}

// write saves entries ids, as they are now, and tells the subscribers.
func (m *Manager) write(ids ...string) {
	var es []Entry
	for _, id := range ids {
		if i := m.index(id); i >= 0 {
			es = append(es, m.entries[i])
		}
	}
	m.st.Do(func(tx *store.Tx) error { return put(tx, es...) })
	m.changed()
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
	m.write(e.ID)
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
	m.write(e.ID)
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
	m.write(id)
	return nil
}

// Move puts an entry first in folder parent. A folder doesn't go into itself or a folder
// it holds.
func (m *Manager) Move(id, parent string) error {
	if err := m.move(id, parent); err != nil {
		return err
	}
	m.write(id)
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
	saved := *e
	m.st.Do(func(tx *store.Tx) error {
		if err := put(tx, saved); err != nil {
			return err
		}
		return last(tx, parent)
	})
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
	m.st.Do(func(tx *store.Tx) error {
		for id := range gone {
			if _, err := tx.Exec(`DELETE FROM bookmarks WHERE id = ?`, id); err != nil {
				return err
			}
		}
		return nil
	})
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

// changed tells the subscribers.
func (m *Manager) changed() {
	for _, f := range m.onChanged {
		f()
	}
}

// newID is 16 random hex digits, made once: the id a later sync knows the entry by.
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
	from := len(m.entries)
	m.importInto(parent, nodes, &got)
	if got.Bookmarks > 0 {
		// An import only adds entries, after those there were.
		added := append([]Entry{}, m.entries[from:]...)
		m.st.Do(func(tx *store.Tx) error { return put(tx, added...) })
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
