// Package downloads keeps a profile's downloads (docs/daily-driver.md): where each goes,
// what each is now, and the finished ones in downloads.json in the profile folder, the
// last 100, so the list is there after a restart. The engine keeps no list; it asks
// where a download goes and reports each change, and acts on one only while it runs.
package downloads

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"projectb/engine"
	"projectb/timeline"
)

// kept is how many finished downloads the list keeps.
const kept = 100

// Item is one download as the shells see it. ID is Go's, the same across restarts.
type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	// From says what a file the app made itself is, such as a crash report, which has no
	// address: URL is "" then.
	From     string `json:"from,omitempty"`
	Path     string `json:"path"`
	State    string `json:"state"` // progress, done, cancelled or failed
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Paused   bool   `json:"paused"`
	// Started is when it joined the list, in Unix seconds; 0 for one listed before the
	// list kept it, which a clear from a time leaves.
	Started int64 `json:"started,omitempty"`

	// engine is the engine's number for it while this run has it.
	engine engine.DownloadID
	// retrying is set by Retry until the engine's download for it shows up.
	retrying bool
}

var states = map[engine.DownloadState]string{
	engine.DownloadProgress:  "progress",
	engine.DownloadDone:      "done",
	engine.DownloadCancelled: "cancelled",
	engine.DownloadFailed:    "failed",
}

// file is downloads.json.
type file struct {
	Version   int    `json:"version"`
	Next      int    `json:"next"`
	Downloads []Item `json:"downloads"`
}

// Manager keeps one profile's downloads, newest first.
type Manager struct {
	e         engine.Engine
	file      string
	dir       string
	list      []*Item
	next      int
	byEngine  map[engine.DownloadID]*Item
	onChanged []func()
}

// New returns a manager for the profile folder profile, saving files into dir.
func New(e engine.Engine, profile, dir string) *Manager {
	return &Manager{e: e, file: filepath.Join(profile, "downloads.json"), dir: dir, next: 1, byEngine: map[engine.DownloadID]*Item{}}
}

// SetDir is where files are saved from now on.
func (m *Manager) SetDir(dir string) { m.dir = dir }

// Dir is where files are saved.
func (m *Manager) Dir() string { return m.dir }

// Load reads the list kept from the last run.
func (m *Manager) Load() {
	b, err := os.ReadFile(m.file)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			timeline.Errorf("downloads: %v", err)
		}
		return
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		timeline.Errorf("downloads: %s: %v", m.file, err)
		return
	}
	for i := range f.Downloads {
		m.list = append(m.list, &f.Downloads[i])
	}
	m.next = max(f.Next, 1)
}

// OnChanged subscribes to any change to the list.
func (m *Manager) OnChanged(f func()) { m.onChanged = append(m.onChanged, f) }

func (m *Manager) changed() {
	for _, f := range m.onChanged {
		f()
	}
}

// List is the list now, newest first.
func (m *Manager) List() []Item {
	out := make([]Item, len(m.list))
	for i, it := range m.list {
		out[i] = *it
	}
	return out
}

// Target is where a download goes: the folder, under the name the engine suggests,
// numbered when that is taken, through the Save panel when it asked for one.
func (m *Manager) Target(r engine.DownloadRequest) engine.DownloadTarget {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		timeline.Errorf("downloads: %v", err)
		return engine.DownloadTarget{}
	}
	return engine.DownloadTarget{Path: m.free(cleanName(r.Name)), Prompt: r.SaveAs}
}

// cleanName is a file name with no folder in it.
func cleanName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == ".." || name == "/" {
		return "download"
	}
	return name
}

// free is a path in the folder for name that no file and no running download has:
// name, then name (1), name (2) and so on, before its extension.
func (m *Manager) free(name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		try := name
		if n > 0 {
			try = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		p := filepath.Join(m.dir, try)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if m.running(p) {
			continue
		}
		return p
	}
}

func (m *Manager) running(path string) bool {
	for _, it := range m.list {
		if it.State == "progress" && it.Path == path {
			return true
		}
	}
	return false
}

// Updated takes what the engine says a download is now, and returns its id in the list.
func (m *Manager) Updated(d engine.Download) int {
	it := m.byEngine[d.ID]
	if it == nil {
		it = m.retried(d.URL)
	}
	if it == nil {
		it = &Item{ID: m.next, URL: d.URL, Started: time.Now().Unix()}
		m.next++
		m.list = append([]*Item{it}, m.list...)
	}
	if it.engine != d.ID {
		delete(m.byEngine, it.engine)
		it.engine = d.ID
		m.byEngine[d.ID] = it
	}
	it.retrying = false
	it.Path, it.Name = d.Path, filepath.Base(d.Path)
	it.State, it.Received, it.Total, it.Paused = states[d.State], d.Received, d.Total, d.Paused
	m.e.ShowDownload(it.ID, d)
	// A failed download stays the engine's: it may resume, by itself or by Retry.
	if it.State == "done" || it.State == "cancelled" {
		delete(m.byEngine, d.ID)
	}
	if it.State != "progress" {
		m.save()
	}
	m.changed()
	return it.ID
}

// Keep copies a file the app made into the folder, under its own name, numbered when that
// is taken, and lists it as a finished download that from says what it is, with the
// system's notice of a finished download. It returns its id in the list.
func (m *Manager) Keep(src, from string) (int, error) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return 0, err
	}
	path := m.free(cleanName(src))
	size, err := copyFile(src, path)
	if err != nil {
		return 0, err
	}
	it := &Item{ID: m.next, Name: filepath.Base(path), Path: path, From: from, State: "done", Received: size, Total: size, Started: time.Now().Unix()}
	m.next++
	m.list = append([]*Item{it}, m.list...)
	m.e.PublishFile(path)
	m.e.ShowDownload(it.ID, engine.Download{Path: path, State: engine.DownloadDone, Received: size, Total: size})
	m.save()
	m.changed()
	return it.ID, nil
}

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
	}
	return n, err
}

// retried is the item Retry started again as a new download of url, moved to the top.
func (m *Manager) retried(url string) *Item {
	for i, x := range m.list {
		if x.retrying && x.URL == url {
			m.list = append(append([]*Item{x}, m.list[:i]...), m.list[i+1:]...)
			return x
		}
	}
	return nil
}

// Act does what a shell asks to one download: open or reveal its file, cancel, pause or
// resume it, retry one that failed or was cancelled, copy its address, or remove it from
// the list, which leaves the file where it is. A file the app made has no address to
// retry or copy.
func (m *Manager) Act(id int, action string) {
	var it *Item
	for _, x := range m.list {
		if x.ID == id {
			it = x
		}
	}
	if it == nil {
		return
	}
	live := it.State == "progress"
	switch action {
	case "open":
		if it.State == "done" {
			m.e.OpenFile(it.Path)
		}
	case "reveal":
		if it.State == "done" {
			m.e.RevealFile(it.Path)
		}
	case "cancel":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadCancel)
		}
	case "pause":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadPause)
		}
	case "resume":
		if live {
			m.e.DownloadCommand(it.engine, engine.DownloadResume)
		}
	case "retry":
		if it.URL == "" || it.State != "failed" && it.State != "cancelled" {
			return
		}
		// One this run still has resumes from where it got to, or starts over when it
		// can't; any other starts over as a new download of its address, in this row.
		it.retrying = true
		if _, ours := m.byEngine[it.engine]; ours && it.State == "failed" {
			m.e.DownloadCommand(it.engine, engine.DownloadResume)
		} else {
			m.e.DownloadURL(0, it.URL, false)
		}
	case "copyLink":
		if it.URL != "" {
			m.e.WriteClipboard(it.URL)
		}
	case "remove":
		if live || m.byEngine[it.engine] == it {
			m.e.DownloadCommand(it.engine, engine.DownloadCancel)
			m.e.DownloadCommand(it.engine, engine.DownloadRemove)
			delete(m.byEngine, it.engine)
		}
		m.drop(func(x *Item) bool { return x == it })
	}
}

// Clear removes every finished download that joined the list since a time in Unix
// seconds, 0 for all of them, as History.Clear does.
func (m *Manager) Clear(since int64) {
	m.drop(func(x *Item) bool { return x.State != "progress" && (since == 0 || x.Started >= since) })
}

func (m *Manager) drop(gone func(*Item) bool) {
	kept := m.list[:0]
	for _, x := range m.list {
		if !gone(x) {
			kept = append(kept, x)
		} else if m.byEngine[x.engine] == x {
			delete(m.byEngine, x.engine)
		}
	}
	m.list = kept
	m.save()
	m.changed()
}

// save keeps the finished downloads, the newest kept, in a new file renamed over the old.
func (m *Manager) save() {
	f := file{Version: 1, Next: m.next, Downloads: []Item{}}
	for _, it := range m.list {
		if it.State != "progress" && len(f.Downloads) < kept {
			f.Downloads = append(f.Downloads, *it)
		}
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err == nil {
		tmp := m.file + ".tmp"
		if err = os.WriteFile(tmp, append(b, '\n'), 0o600); err == nil {
			err = os.Rename(tmp, m.file)
		}
	}
	if err != nil {
		timeline.Errorf("downloads: %v", err)
	}
}
