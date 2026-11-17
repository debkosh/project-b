package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/bookmarks"
	"projectb/engine"
	"projectb/history"
	"projectb/search"
	"projectb/shells"
	"projectb/timeline"
)

// Bringing things over from another browser (docs/onboarding.md): the welcome's step and
// Settings' Import list the browsers on this Mac and each profile with what it holds, and
// bring over what the person ticks: its bookmarks, with their folders, into a folder named
// after it; its history; and its search engine, when it is one of ours. browserdata reads
// them, on a worker; what it read joins the profile here, on the main thread.
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
}

// foundProfile is a row: a browser's profile, its icon, what it holds, and what it last
// brought over. A locked one's files can't be read until the app has Full Disk Access, so
// it has no counts.
type foundProfile struct {
	Key     string             `json:"key"`
	Browser string             `json:"browser"`
	Title   string             `json:"title"`
	Icon    string             `json:"icon,omitempty"`
	Locked  bool               `json:"locked,omitempty"`
	Counts  browserdata.Counts `json:"counts"`
	Last    *broughtOver       `json:"last,omitempty"`
	// folder is the bookmarks' folder: From Chrome, or From Chrome · Work for a browser
	// with more than one profile.
	folder  string
	profile browserdata.Profile
}

// broughtOver is what one import brought: its bookmarks, the folder they are in, the pages
// of history, and the search engine taken, by id.
type broughtOver struct {
	At        time.Time          `json:"at"`
	Bookmarks bookmarks.Imported `json:"bookmarks"`
	Folder    string             `json:"folder,omitempty"`
	Pages     int                `json:"pages"`
	Search    string             `json:"search,omitempty"`
	Errors    []string           `json:"errors,omitempty"`
}

func newImporter(a *App, e engine.Engine, dir, home string, send func(any)) *importer {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	im := &importer{a: a, e: e, home: home, path: filepath.Join(dir, "imports.json"), busy: map[string]bool{}, last: map[string]broughtOver{}, send: send}
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
			icon := ""
			if b.App != "" {
				if png, err := browserdata.Icon(b.App, 64); err == nil {
					icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
				}
			}
			for _, p := range b.Profiles {
				f := foundProfile{Key: b.ID + "/" + filepath.Base(p.Dir), Browser: b.ID, Title: b.Name, Icon: icon, folder: "From " + b.Name, profile: p}
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
// of its things: bookmarks, history, search.
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
	bookmarks []browserdata.Bookmark
	pages     []browserdata.Page
	search    browserdata.Search
	errors    []string
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
	im.send(map[string]any{"t": "importing", "importing": im.importing()})
	timeline.Infof("imports: bringing %v over from %s", what, f.Title)
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
		if slices.Contains(what, "search") {
			r.search, err = f.profile.Search()
			note(err)
		}
		im.e.Post(func() { im.add(f, r) })
	}()
}

// add joins what was read to the profile, keeps what it brought, and tells the pages.
func (im *importer) add(f foundProfile, r read) {
	delete(im.busy, f.Key)
	got := broughtOver{At: time.Now().UTC(), Errors: r.errors}
	if len(r.bookmarks) > 0 {
		imported, err := im.a.Bookmarks.Import(bookmarks.Top, []bookmarks.Node{{Title: f.folder, Children: nodesOf(r.bookmarks)}})
		if err != nil {
			got.Errors = append(got.Errors, err.Error())
		}
		got.Bookmarks, got.Folder = imported, f.folder
	}
	if len(r.pages) > 0 {
		pages := make([]history.Entry, len(r.pages))
		for i, p := range r.pages {
			pages[i] = history.Entry{URL: p.URL, Title: p.Title, Visits: p.Visits, LastVisit: p.Last.Unix()}
		}
		im.a.History.Import(pages)
		got.Pages = len(pages)
	}
	if r.search.Name != "" {
		if e, ok := search.ByAddress(r.search.URL); ok {
			im.a.Prefs.SetSearchEngine(e.ID)
			got.Search = e.ID
		} else {
			got.Errors = append(got.Errors, r.search.Name+" isn't a search engine this browser has")
		}
	}
	timeline.Infof("imports: from %s, %d bookmarks in %d folders (%d bookmarked already), %d pages, search %q, errors %q",
		f.Title, got.Bookmarks.Bookmarks, got.Bookmarks.Folders, got.Bookmarks.Skipped, got.Pages, got.Search, got.Errors)
	im.last[f.Key] = got
	if b, err := json.MarshalIndent(im.last, "", "  "); err == nil {
		if err := os.WriteFile(im.path, append(b, '\n'), 0o600); err != nil {
			timeline.Errorf("imports: %v", err)
		}
	}
	im.send(map[string]any{"t": "imported", "key": f.Key, "imported": got, "importing": im.importing()})
	im.send(map[string]any{"t": "browsers", "browsers": im.list()})
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
