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
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"projectb/engine"
	"projectb/streams"
	"projectb/system"
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
	// Reason is why a failed one failed, as engine.Download has it, or ""; for a stream that
	// can't be saved, streams.Reason's: "live", "protected", "format", "refused" or "empty".
	Reason string `json:"reason,omitempty"`
	// NoRetry says trying a failed one again can't help, so it has no Retry: a protected
	// stream, or one of a format the app can't repack.
	NoRetry bool `json:"noRetry,omitempty"`
	// Stream is the kind of playlist of a streamed video saved as one MP4
	// (docs/video-streams.md, streams.go), "" for a file. Workspace is the workspace whose
	// storage it is fetched in, and Space where its playlist's server is, an
	// engine.AddressSpace: what Retry starts it over with.
	Stream    string `json:"stream,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Space     int    `json:"space,omitempty"`
	// Referrer is the page it came from, which Retry says again: a site may serve its
	// files only to its own pages.
	Referrer string `json:"referrer,omitempty"`
	// Original is the name its server gave it, for one named after better text instead
	// (naming.go); "" for one that kept its name.
	Original string `json:"original,omitempty"`
	// Started is when it joined the list, in Unix seconds; 0 for one listed before the
	// list kept it, which a clear from a time leaves.
	Started int64 `json:"started,omitempty"`

	// engine is the engine's number for it while this run has it.
	engine engine.DownloadID
	// retrying is set by Retry until the engine's download for it shows up.
	retrying bool
	// job is the stream being saved, while it is.
	job *streams.Job
	// asked is what whoever started it through Download asked of it, until the engine's
	// download for it shows up; into is the folder a stream's MP4 goes in when that isn't
	// the Downloads folder itself.
	asked *asked
	into  string
	// publishing is set, on a phone, from done until the engine has said where its copy in
	// the shared Downloads is (Published); waiting is what Open and Share asked meanwhile,
	// done with that copy once it is there.
	publishing bool
	waiting    []func(path string)
}

// publishes says the engine copies each finished file into the phone's shared Downloads and
// says where (Published), as Chromium's does: Android.
var publishes = runtime.GOOS == "android"

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
	// NotificationsOffered: a phone's download bar has offered, once, to show downloads in
	// the system's notifications.
	NotificationsOffered bool `json:"notificationsOffered,omitempty"`
}

// Manager keeps one profile's downloads, newest first.
type Manager struct {
	e         engine.Engine
	sys       system.System
	file      string
	dir       string
	list      []*Item
	next      int
	byEngine  map[engine.DownloadID]*Item
	onChanged []func()

	// openHere opens an address in a new tab of the app's own: Open's way for a file the
	// engine shows itself (ShowsItself).
	openHere func(address string)

	offered bool

	// began is what is known of each engine download from when it began until it ends.
	began map[engine.DownloadID]began
	// fetcher fetches a stream's pieces, and workspaceOf is the workspace a tab is in: set
	// as the app starts (Streams). Without them no download is read as a stream.
	fetcher     streams.Fetcher
	workspaceOf func(engine.TabID) string
	// naming says what a download is named by: nil, each keeps its server's name.
	naming func(engine.DownloadRequest) Naming
	// jobs counts the streams started, for each one's folder; after is the streams' timer
	// for their waits between tries, nil for a real one (tests set it).
	jobs  int
	after func(time.Duration, func()) func()
}

// asked is what Download was asked beside the address: a name under the folder, "" for the
// server's own, what to do when that name is taken, "uniquify" (a number is added, the
// default), "overwrite" or "prompt", and whether the Save panel shows first.
type asked struct {
	name     string
	conflict string
	saveAs   bool
}

// Download starts a download of an address as the browser's own, as an extension's
// chrome.downloads.download asks (docs/extensions.md), and returns its id in the list at
// once: its row is there from now, and the engine's download joins it when it shows up. A
// playlist's address becomes a stream's MP4, as from anywhere (docs/video-streams.md).
// name is a path under the folder to save it as, "" for the server's own name; conflict
// says what to do when that is taken.
func (m *Manager) Download(address, name string, saveAs bool, conflict string) int {
	shown := filepath.Base(name)
	if name == "" {
		shown = "download"
		if u, err := url.Parse(address); err == nil && filepath.Base(u.Path) != "/" && filepath.Base(u.Path) != "." {
			shown = cleanName(filepath.Base(u.Path))
		}
	}
	it := &Item{ID: m.next, Name: shown, URL: address, State: "progress", Total: -1, Started: time.Now().Unix(),
		retrying: true, asked: &asked{name: name, conflict: conflict, saveAs: saveAs}}
	m.next++
	m.list = append([]*Item{it}, m.list...)
	m.changed()
	m.e.DownloadURL(0, address, "", saveAs)
	return it.ID
}

// claim gives an engine download that began in no tab, as the browser's own do, the row
// waiting for it: one retried or asked for at its address, or, when a redirect has changed
// the address, the one asked for longest ago.
func (m *Manager) claim(r engine.DownloadRequest) *Item {
	if r.Tab != 0 {
		return nil
	}
	var it *Item
	for _, x := range m.list {
		if x.retrying && m.byEngine[x.engine] != x && (x.URL == r.URL || x.asked != nil && (it == nil || it.URL != r.URL)) {
			it = x
			if x.URL == r.URL {
				break
			}
		}
	}
	if it != nil {
		delete(m.byEngine, it.engine)
		it.engine = r.ID
		m.byEngine[r.ID] = it
	}
	return it
}

// began is an engine download as it began: the workspace of the tab it began in, and for
// one that is a stream's playlist by its type or address, the kind and the MP4's name.
type began struct {
	workspace string
	stream    streams.Kind
	name      string
	// dir is the folder the MP4 goes in, when it isn't the Downloads folder itself.
	dir string
	// original is the name its server gave it, when it goes by another.
	original string
}

// New returns a manager for the profile folder profile, saving files into dir.
func New(e engine.Engine, sys system.System, profile, dir string) *Manager {
	return &Manager{e: e, sys: sys, file: filepath.Join(profile, "downloads.json"), dir: dir, next: 1, byEngine: map[engine.DownloadID]*Item{}, began: map[engine.DownloadID]began{}}
}

// Streams has a download that is a streamed video's playlist saved as one MP4
// (docs/video-streams.md): its pieces are fetched through f, as from the page it came from,
// in the workspace of the tab it began in, which workspaceOf says. Set as the app starts.
func (m *Manager) Streams(f streams.Fetcher, workspaceOf func(engine.TabID) string) {
	m.fetcher, m.workspaceOf = f, workspaceOf
}

// OpenHere is how Open opens a file the engine shows itself: in a new tab of the app's own,
// at the address it gives. Set as the app starts; until then every file goes to the OS.
func (m *Manager) OpenHere(open func(address string)) { m.openHere = open }

// shownHere are the files the engine shows itself, by extension: pages, PDFs, images and
// plain text. Open opens these in a tab of the app's own, and everything else, a zip or a
// document, in the OS's app for it (oskit/open).
var shownHere = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true, ".svg": true,
	".pdf": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".avif": true,
	".txt": true, ".json": true,
}

// ShowsItself says whether the engine shows a file of this name itself.
func ShowsItself(name string) bool { return shownHere[strings.ToLower(filepath.Ext(name))] }

// addressOf is a file's address for a tab: a phone's content:// copy as it is, a path as a
// file:// URL.
func addressOf(path string) string {
	if strings.HasPrefix(path, "content://") {
		return path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

// SetDir is where files are saved from now on.
func (m *Manager) SetDir(dir string) { m.dir = dir }

// Dir is where files are saved.
func (m *Manager) Dir() string { return m.dir }

// Load reads the list kept from the last run.
func (m *Manager) Load() {
	// What a stream being saved when the app last ended left behind.
	os.RemoveAll(m.streamsDir())
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
	m.offered = f.NotificationsOffered
}

// Offered is whether a phone's download bar has offered, once, to show downloads in the
// system's notifications. It offers only the once, taken up or not.
func (m *Manager) Offered() bool { return m.offered }

// SetOffered keeps that it has.
func (m *Manager) SetOffered() {
	if !m.offered {
		m.offered = true
		m.save()
	}
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
	// What an extension asked of it: its name, in a folder under the Downloads folder, and
	// what to do when that is taken.
	dir, name, prompt, overwrite := m.dir, cleanName(r.Name), r.SaveAs, false
	// The file's own name, and its own ending whatever it is named after: a stream's is
	// its MP4's.
	kind := streams.None
	if m.fetcher != nil {
		kind = streams.KindOf(r.Mime, r.URL)
	}
	original := name
	if kind != streams.None {
		name = streams.NameOf(name)
	}
	claimed := m.claim(r)
	switch {
	case claimed != nil && claimed.asked != nil:
		// An extension's download is named as the extension said, or as its server did.
		original = ""
	case claimed != nil && claimed.Original != "":
		// Retried, it keeps the name it had.
		name, original = claimed.Name, claimed.Original
	case m.naming != nil:
		if better, renamed := m.naming(r).Name(name); renamed {
			name = better
		} else {
			original = ""
		}
	default:
		original = ""
	}
	if it := claimed; it != nil && it.asked != nil {
		if it.asked.name != "" {
			dir, name = filepath.Join(m.dir, filepath.Dir(it.asked.name)), cleanName(it.asked.name)
			if kind != streams.None {
				name = streams.NameOf(name)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				timeline.Errorf("downloads: %v", err)
				return engine.DownloadTarget{}
			}
			_, err := os.Stat(filepath.Join(dir, name))
			overwrite = it.asked.conflict == "overwrite"
			prompt = prompt || it.asked.conflict == "prompt" && err == nil
		}
		prompt = prompt || it.asked.saveAs
	}
	place := func(name string) string {
		if overwrite {
			return filepath.Join(dir, name)
		}
		return m.freeIn(dir, name)
	}
	if m.fetcher == nil {
		if original != "" {
			m.began[r.ID] = began{original: original}
		}
		return engine.DownloadTarget{Path: place(name), Prompt: prompt}
	}
	b := began{workspace: m.workspaceOf(r.Tab), original: original}
	// A stream's playlist goes to a folder of the app's own and is read from there: the
	// Downloads folder gets the MP4, under the playlist's name. No Save panel asks about it.
	if kind != streams.None {
		lists := filepath.Join(m.streamsDir(), "playlists")
		if err := os.MkdirAll(lists, 0o700); err == nil {
			b.stream, b.name = kind, name
			if dir != m.dir {
				b.dir = dir
			}
			m.began[r.ID] = b
			return engine.DownloadTarget{Path: filepath.Join(lists, fmt.Sprintf("%d", r.ID))}
		}
	}
	m.began[r.ID] = b
	return engine.DownloadTarget{Path: place(name), Prompt: prompt}
}

// streamsDir is the folder the streams being saved work in, in the profile.
func (m *Manager) streamsDir() string { return filepath.Join(filepath.Dir(m.file), "streams") }

// NameBy has each download named by what by says of it, as it begins (naming.go): the way
// Settings and its site have, and the better texts for a name that says nothing.
func (m *Manager) NameBy(by func(engine.DownloadRequest) Naming) { m.naming = by }

// IsStream says an engine download is a stream's playlist, as its Target saw.
func (m *Manager) IsStream(id engine.DownloadID) bool { return m.began[id].stream != streams.None }

// cleanName is a file name with no folder in it.
func cleanName(name string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == ".." || name == "/" {
		return "download"
	}
	return name
}

// free is a path in the folder for name that no file and no download in the list has:
// name, then name (1), name (2) and so on, before its extension. A phone moves a finished
// download's file to the shared Downloads, so its path is empty again, but its name stays the
// download's (taken): two of one name are told apart in the list, as in the shared Downloads.
func (m *Manager) free(name string) string { return m.freeIn(m.dir, name) }

// freeIn is free in a folder under the Downloads folder.
func (m *Manager) freeIn(dir, name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 0; ; n++ {
		try := name
		if n > 0 {
			try = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		p := filepath.Join(dir, try)
		if _, err := os.Stat(p); err == nil {
			continue
		}
		if m.taken(p) {
			continue
		}
		return p
	}
}

// taken says a download in the list has the path, or on a phone had it before its file went
// to the shared Downloads and its path became the copy's address: not one being retried, whose
// new copy takes its name.
func (m *Manager) taken(path string) bool {
	for _, it := range m.list {
		if it.retrying {
			continue
		}
		if it.Path == path || strings.HasPrefix(it.Path, "content://") && it.Name == filepath.Base(path) {
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
	it.retrying, it.asked = false, nil
	if d.Referrer != "" {
		it.Referrer = d.Referrer
	}
	b, known := m.began[d.ID]
	if d.State != engine.DownloadProgress {
		delete(m.began, d.ID)
	}
	if known {
		it.Original = b.original
	}
	// A stream's playlist, by its type or address, or once it is all here by how it starts.
	if b.stream == streams.None && d.State == engine.DownloadDone && m.fetcher != nil {
		if b.stream = sniff(d.Path); b.stream != streams.None {
			b.name = streams.NameOf(filepath.Base(d.Path))
		}
	}
	if b.stream != streams.None {
		m.playlist(it, d, b)
		return it.ID
	}
	it.Path, it.Name = d.Path, filepath.Base(d.Path)
	it.State, it.Received, it.Total, it.Paused, it.Reason = states[d.State], d.Received, d.Total, d.Paused, d.Reason
	it.publishing = publishes && it.State == "done"
	// A finished file is handed to the system first, so what showing it done does, such as a
	// phone's notification that opens it, comes after the copy it opens.
	if it.State == "done" {
		m.e.PublishFile(d.Path, d.Mime)
	}
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
	it.publishing = publishes
	m.e.PublishFile(path, "")
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

// Share offers a finished download's file to the system's share sheet, on window w: on a
// phone, its copy in the shared Downloads.
func (m *Manager) Share(id int, w engine.WindowID) {
	for _, it := range m.list {
		if it.ID == id && it.State == "done" {
			it.when(func(path string) { m.e.ShareFile(w, path, engine.Rect{}) })
		}
	}
}

// when does do with the file's path now, or, while a phone makes its shared copy, with the
// copy's once it is made.
func (it *Item) when(do func(path string)) {
	if it.publishing {
		it.waiting = append(it.waiting, do)
		return
	}
	do(it.Path)
}

// Published takes a phone's copy of a finished file in its shared Downloads, by the file's
// path: its content:// address is the download's path from then on, as Chromium's is, so
// Open, Share and Delete hand that on. Its notification is shown again with it, and what was
// asked while it was made is done. An address of "" is a copy that failed: the path stays.
func (m *Manager) Published(path, address string) {
	for _, it := range m.list {
		if it.Path != path || it.State != "done" {
			continue
		}
		if address != "" {
			it.Path = address
		}
		it.publishing = false
		waiting := it.waiting
		it.waiting = nil
		m.e.ShowDownload(it.ID, engine.Download{Path: it.Path, Name: it.Name, State: engine.DownloadDone, Received: it.Received, Total: it.Total})
		for _, do := range waiting {
			do(it.Path)
		}
		m.save()
		m.changed()
		return
	}
}

// Delete deletes a finished download's file, its copy in a phone's shared Downloads, and
// takes it off the list.
func (m *Manager) Delete(id int) bool {
	it := m.find(id)
	if it == nil || it.State != "done" || it.publishing || !m.e.DeleteDownload(it.Path) {
		return false
	}
	m.drop(func(x *Item) bool { return x == it })
	return true
}

// find is the download with Go's id, or nil.
func (m *Manager) find(id int) *Item {
	for _, x := range m.list {
		if x.ID == id {
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
	if it.Stream != "" && m.actStream(it, action) {
		return
	}
	switch action {
	case "open":
		if it.State == "done" {
			it.when(func(path string) {
				if m.openHere != nil && ShowsItself(it.Name) {
					m.openHere(addressOf(path))
					return
				}
				m.sys.OpenFile(path)
			})
		}
	case "reveal":
		if it.State == "done" {
			m.sys.RevealFile(it.Path)
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
		// Only one that failed or was cancelled: a running one has Pause and Cancel, and the
		// engine's silence watch fails one that is stuck.
		if it.URL == "" || it.State != "failed" && it.State != "cancelled" {
			return
		}
		// One this run still has resumes from where it got to, or starts over when it
		// can't; any other starts over as a new download of its address, in this row.
		it.retrying = true
		if _, ours := m.byEngine[it.engine]; ours && it.State == "failed" {
			m.e.DownloadCommand(it.engine, engine.DownloadResume)
		} else {
			m.e.DownloadURL(0, it.URL, it.Referrer, false)
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
			continue
		}
		if m.byEngine[x.engine] == x {
			delete(m.byEngine, x.engine)
		}
		// A failed stream's job, held for a Retry, goes with its row.
		if x.job != nil {
			x.job.Cancel()
			x.job = nil
		}
	}
	m.list = kept
	m.save()
	m.changed()
}

// save keeps the finished downloads, the newest kept, in a new file renamed over the old.
func (m *Manager) save() {
	f := file{Version: 1, Next: m.next, Downloads: []Item{}, NotificationsOffered: m.offered}
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
