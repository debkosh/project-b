package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/bookmarks"
	"projectb/engine"
	"projectb/history"
	"projectb/shells"
	"projectb/timeline"
)

// Bringing things over from another browser (docs/onboarding.md, docs/import.md): the
// welcome's step and Settings' Import list the browsers on this Mac and each profile with
// what it holds, and bring over what the person ticks: its bookmarks, with their folders,
// into a folder named after it; its history; and,
// in Settings while the Extensions flag is on, its extensions, installed again from the
// Chrome Web Store by their ids, one at a time. browserdata reads them, on a worker; what it
// read joins the profile here, on the main thread.
// imports.json in the profile keeps what each profile last brought over, and when.
// A phone keeps each app's data from the others, so there is nothing to find there.
//
// Safari's files are behind macOS's Full Disk Access: its row is locked until the person
// gives the app that in System Settings, which a page opens with openAccess. Nothing says
// when they have, so from then Go tries the locked rows' files each second, for ten
// minutes, and looks again once they can be read.

type importer struct {
	a    *App
	e    engine.Engine
	home string
	path string
	// found is what the last look found, nil before one ended; looking says one is on.
	found   []foundProfile
	looking bool
	// busy are the profiles, by key, whose things are being read.
	busy map[string]bool
	// waiting says Go is trying the locked rows' files until they can be read.
	waiting bool
	// last is what each profile last brought over, by key.
	last map[string]broughtOver
	// send tells every page that shows the list.
	send func(any)
	// store installs extensions from the store, and says which are installed: set while
	// the Extensions flag is on, in a build with extensions.
	store extensionStore
	// installing is each profile's extensions being installed, by key.
	installing map[string]*installing
}

// extensionStore is what bringing extensions over needs of the extension manager: whether
// one is installed, and an install from the store by id, which calls done with why it
// failed, or "".
type extensionStore interface {
	Installed(id string) bool
	Install(id string, done func(failed string))
}

// installing is an import whose extensions are being installed: what it has brought so
// far, and how many of its extensions have ended.
type installing struct {
	got         broughtOver
	done, total int
}

// foundProfile is a row: a browser's profile, its icon, what it holds, and what it last
// brought over. A locked one's files can't be read until the app has Full Disk Access, so
// it has no counts.
type foundProfile struct {
	Key     string             `json:"key"`
	Browser string             `json:"browser"`
	Title   string             `json:"title"`
	Locked  bool               `json:"locked,omitempty"`
	Counts  browserdata.Counts `json:"counts"`
	Last    *broughtOver       `json:"last,omitempty"`
	// folder is the bookmarks' folder: From Chrome, or From Chrome · Work for a browser
	// with more than one profile.
	folder  string
	profile browserdata.Profile
}

// broughtOver is what one import brought: its bookmarks, the folder they are in and how
// many of them were Safari's Reading List, the pages of history, and its extensions.
type broughtOver struct {
	At          time.Time          `json:"at"`
	Bookmarks   bookmarks.Imported `json:"bookmarks"`
	Folder      string             `json:"folder,omitempty"`
	ReadingList int                `json:"readingList,omitempty"`
	Pages       int                `json:"pages"`
	Extensions  *broughtExtensions `json:"extensions,omitempty"`
	Errors      []string           `json:"errors,omitempty"`
}

// broughtExtensions is what became of a profile's extensions, by name: installed from the
// store, installed here already, failed to install and why, and left out and why.
type broughtExtensions struct {
	Installed []string        `json:"installed"`
	Already   []string        `json:"already"`
	Failed    []extensionNote `json:"failed"`
	Skipped   []extensionNote `json:"skipped"`
}

type extensionNote struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// skipWhy is why an extension can't be installed again from the store, in words.
var skipWhy = map[browserdata.Skip]string{
	browserdata.SkipUnpacked:   "loaded from a folder",
	browserdata.SkipNotInStore: "not from the Chrome Web Store",
	browserdata.SkipManifestV2: "Manifest V2, which no longer runs",
}

func newImporter(a *App, e engine.Engine, dir, home string, send func(any)) *importer {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	im := &importer{a: a, e: e, home: home, path: filepath.Join(dir, "imports.json"), busy: map[string]bool{}, last: map[string]broughtOver{}, send: send, installing: map[string]*installing{}}
	if b, err := os.ReadFile(im.path); err == nil {
		if err := json.Unmarshal(b, &im.last); err != nil {
			timeline.Errorf("imports: %s: %v", im.path, err)
		}
	}
	return im
}

// snapshot is the list as a page starts with it: null until a look has ended.
func (im *importer) snapshot(into map[string]any) {
	into["browsers"] = im.list()
	into["importing"] = im.importing()
	into["installing"] = im.progress()
}

// progress is each import installing extensions, by key: how many have ended, of how many.
func (im *importer) progress() map[string]map[string]int {
	out := map[string]map[string]int{}
	for k, in := range im.installing {
		out[k] = map[string]int{"done": in.done, "total": in.total}
	}
	return out
}

// sendImporting tells the pages which profiles are being brought over, and how far their
// extensions are.
func (im *importer) sendImporting() {
	im.send(map[string]any{"t": "importing", "importing": im.importing(), "installing": im.progress()})
}

func (im *importer) list() []foundProfile {
	if im.found == nil {
		return nil
	}
	out := slices.Clone(im.found)
	for i := range out {
		if l, ok := im.last[out[i].Key]; ok {
			out[i].Last = &l
		}
		// Extensions come over only while they can be installed.
		if im.store == nil {
			out[i].Counts.Extensions = 0
		}
	}
	return out
}

func (im *importer) importing() []string {
	out := []string{}
	for k := range im.busy {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// look finds the browsers again, with what each profile holds now, and tells the pages.
func (im *importer) look() {
	if im.looking {
		return
	}
	if shells.OnPhone {
		im.found = []foundProfile{}
		return
	}
	im.looking = true
	home := im.home
	go func() {
		start := time.Now()
		found := []foundProfile{}
		for _, b := range browserdata.Find(home) {
			for _, p := range b.Profiles {
				f := foundProfile{Key: b.ID + "/" + filepath.Base(p.Dir), Browser: b.ID, Title: b.Name, folder: "From " + b.Name, profile: p}
				if p.Name != "" {
					f.Title += " · " + p.Name
				}
				if len(b.Profiles) > 1 {
					f.folder += " · " + p.Name
				}
				if f.Locked = p.Locked(); !f.Locked {
					var err error
					if f.Counts, err = p.Counts(); err != nil {
						timeline.Warnf("imports: %s: %v", f.Title, err)
					}
				}
				found = append(found, f)
			}
		}
		took := time.Since(start)
		im.e.Post(func() {
			im.looking = false
			im.found = found
			timeline.Infof("imports: found %d profiles of other browsers in %v", len(found), took.Round(time.Millisecond))
			im.send(map[string]any{"t": "browsers", "browsers": im.list()})
		})
	}()
}

// importMessage is what a page sends to bring things over: the profile, by key, and which
// of its things: bookmarks, history, extensions.
type importMessage struct {
	Key  string   `json:"key"`
	What []string `json:"what"`
}

// handle takes a page's message about bringing things over, and says whether it was one.
func (im *importer) handle(t string, msg []byte) bool {
	switch t {
	case "findBrowsers":
		im.look()
	case "openAccess":
		timeline.Infof("imports: opening System Settings at Full Disk Access")
		im.e.OpenFullDiskAccess()
		im.wait()
	case "import":
		var m importMessage
		if err := json.Unmarshal(msg, &m); err != nil {
			timeline.Warnf("imports: bad message: %v", err)
			return true
		}
		im.bring(m.Key, m.What)
	default:
		return false
	}
	return true
}

// wait tries the locked rows' files each second, on a worker, and looks again once none is
// locked, or gives up after ten minutes.
func (im *importer) wait() {
	var locked []browserdata.Profile
	for _, f := range im.found {
		if f.Locked {
			locked = append(locked, f.profile)
		}
	}
	if im.waiting || len(locked) == 0 {
		return
	}
	im.waiting = true
	go func() {
		start := time.Now()
		for time.Since(start) < 10*time.Minute {
			time.Sleep(time.Second)
			if !slices.ContainsFunc(locked, browserdata.Profile.Locked) {
				im.e.Post(func() {
					im.waiting = false
					timeline.Infof("imports: Full Disk Access given after %v; looking again", time.Since(start).Round(time.Second))
					im.look()
				})
				return
			}
		}
		im.e.Post(func() {
			im.waiting = false
			timeline.Infof("imports: no Full Disk Access after ten minutes; no longer waiting for it")
		})
	}()
}

// read is what was read from a profile, on the worker.
type read struct {
	bookmarks  []browserdata.Bookmark
	pages      []browserdata.Page
	extensions []browserdata.Extension
	errors     []string
}

// bring reads what was asked for from the profile key on a worker, then adds it here.
func (im *importer) bring(key string, what []string) {
	i := slices.IndexFunc(im.found, func(f foundProfile) bool { return f.Key == key })
	if i < 0 || im.busy[key] || im.found[i].Locked {
		timeline.Warnf("imports: %q is not a profile found, is locked, or is being brought over", key)
		return
	}
	f := im.found[i]
	im.busy[key] = true
	im.sendImporting()
	timeline.Infof("imports: bringing %v over from %s", what, f.Title)
	extensions := slices.Contains(what, "extensions") && im.store != nil
	go func() {
		var r read
		note := func(err error) {
			if err != nil {
				r.errors = append(r.errors, err.Error())
			}
		}
		var err error
		if slices.Contains(what, "bookmarks") {
			r.bookmarks, err = f.profile.Bookmarks()
			note(err)
		}
		if slices.Contains(what, "history") {
			r.pages, err = f.profile.History()
			note(err)
		}
		if extensions {
			r.extensions, err = f.profile.Extensions()
			note(err)
		}
		im.e.Post(func() { im.add(f, r) })
	}()
}

// add joins what was read to the profile, then installs its extensions, if it brought
// any, before it is done.
func (im *importer) add(f foundProfile, r read) {
	got := broughtOver{At: time.Now().UTC(), Errors: r.errors}
	if len(r.bookmarks) > 0 {
		imported, err := im.a.Bookmarks.Import(bookmarks.Top, []bookmarks.Node{{Title: f.folder, Children: nodesOf(r.bookmarks)}})
		if err != nil {
			got.Errors = append(got.Errors, err.Error())
		}
		got.Bookmarks, got.Folder = imported, f.folder
		for _, top := range r.bookmarks {
			if f.Browser == "safari" && top.Title == browserdata.ReadingList {
				got.ReadingList = leaves(top.Children)
			}
		}
	}
	if len(r.pages) > 0 {
		pages := make([]history.Entry, len(r.pages))
		for i, p := range r.pages {
			pages[i] = history.Entry{URL: p.URL, Title: p.Title, Visits: p.Visits, LastVisit: p.Last.Unix()}
		}
		im.a.History.Import(pages)
		got.Pages = len(pages)
	}
	if r.extensions == nil {
		im.done(f, got)
		return
	}
	// Each extension the store has and this browser hasn't is installed in turn; the import
	// is done once the last has ended.
	x := &broughtExtensions{Installed: []string{}, Already: []string{}, Failed: []extensionNote{}, Skipped: []extensionNote{}}
	got.Extensions = x
	var want []browserdata.Extension
	for _, e := range r.extensions {
		switch {
		case e.Skip != "":
			x.Skipped = append(x.Skipped, extensionNote{e.Name, skipWhy[e.Skip]})
		case im.store.Installed(e.ID):
			x.Already = append(x.Already, e.Name)
		default:
			want = append(want, e)
		}
	}
	if len(want) == 0 {
		im.done(f, got)
		return
	}
	in := &installing{got: got, total: len(want)}
	im.installing[f.Key] = in
	im.sendImporting()
	for _, e := range want {
		im.store.Install(e.ID, func(failed string) {
			if failed == "" {
				x.Installed = append(x.Installed, e.Name)
			} else {
				x.Failed = append(x.Failed, extensionNote{e.Name, failed})
			}
			in.done++
			if in.done < in.total {
				im.sendImporting()
				return
			}
			delete(im.installing, f.Key)
			im.done(f, in.got)
		})
	}
}

// done keeps what an import brought and tells the pages.
func (im *importer) done(f foundProfile, got broughtOver) {
	delete(im.busy, f.Key)
	extensions := ""
	if x := got.Extensions; x != nil {
		extensions = fmt.Sprintf(", extensions: %d installed, %d already, %d failed, %d skipped", len(x.Installed), len(x.Already), len(x.Failed), len(x.Skipped))
	}
	timeline.Infof("imports: from %s, %d bookmarks in %d folders (%d bookmarked already, %d of the Reading List), %d pages%s, errors %q",
		f.Title, got.Bookmarks.Bookmarks, got.Bookmarks.Folders, got.Bookmarks.Skipped, got.ReadingList, got.Pages, extensions, got.Errors)
	im.last[f.Key] = got
	if b, err := json.MarshalIndent(im.last, "", "  "); err == nil {
		if err := os.WriteFile(im.path, append(b, '\n'), 0o600); err != nil {
			timeline.Errorf("imports: %v", err)
		}
	}
	im.send(map[string]any{"t": "imported", "key": f.Key, "imported": got, "importing": im.importing(), "installing": im.progress()})
	im.send(map[string]any{"t": "browsers", "browsers": im.list()})
}

// leaves counts the bookmarks in bs and the folders it holds.
func leaves(bs []browserdata.Bookmark) int {
	n := 0
	for _, b := range bs {
		if b.Folder() {
			n += leaves(b.Children)
		} else {
			n++
		}
	}
	return n
}

// nodesOf is another browser's bookmarks as an import takes them, times in milliseconds.
func nodesOf(bs []browserdata.Bookmark) []bookmarks.Node {
	out := make([]bookmarks.Node, len(bs))
	for i, b := range bs {
		out[i] = bookmarks.Node{Title: b.Title, URL: b.URL, Children: nodesOf(b.Children)}
		if !b.Added.IsZero() {
			out[i].Added = b.Added.UnixMilli()
		}
	}
	return out
}
