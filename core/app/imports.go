package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/debkosh/project-b/browserdata"

	"projectb/bookmarks"
	"projectb/engine"
	"projectb/history"
	"projectb/shells"
	"projectb/timeline"
)

// Bringing things over from another browser (docs/onboarding.md, docs/import.md), a wizard
// in the welcome's step and in Settings' Import alike: the browsers on this Mac, each with
// its profiles and what each holds, or a folder the person chooses for one this package
// doesn't find; then one profile's things, one kind after another, each told to the pages
// as it lands. browserdata reads, on a worker; what it read joins the profile here, on the
// main thread. Nothing is kept between imports: the pages show what an import brought
// while it is on screen, and bookmarks and history say for themselves what is there.
// A phone keeps each app's data from the others, so there is nothing to find there.
//
// Safari's files are behind macOS's Full Disk Access: its profile is locked until the
// person gives the app that in System Settings, which a page opens with openAccess. Nothing
// says when they have, so from then Go tries the locked profiles' files each second, for
// ten minutes, and looks again once they can be read.

type importer struct {
	a    *App
	e    engine.Engine
	home string
	// profiles are the profiles the pages have been shown, by key, their folder: the last
	// look's and each folder chosen.
	profiles map[string]shownFrom
	// looking says a look is on; runs are the imports going on, by the profile's key.
	looking bool
	runs    map[string]*importRun
	// waiting says Go is trying the locked profiles' files until they can be read.
	waiting bool
	locked  []browserdata.Profile
	// send tells every page that shows the wizard.
	send func(any)
	// store installs extensions from the store, and says which are installed: set while
	// the Extensions flag is on, in a build with extensions.
	store extensionStore
	// passwords is the passwords store, once there is one (UsePasswordStore); keychain
	// reads a Chromium browser's key for its passwords, and keychainItems lists the
	// keychain's items of those keys, for a browser known only by its folder.
	passwords     PasswordStore
	keychain      func(service, account string) ([]byte, error)
	keychainItems func() ([]browserdata.KeychainItem, error)
}

// PasswordStore is what bringing passwords over needs of the passwords store
// (core/passwords): it adds what it hasn't, keeping a saved login for the same origin and
// username as it is, and says how many it added, had already, and left out.
type PasswordStore interface {
	Import(logins []browserdata.Login) (added, already, skipped int)
}

// UsePasswordStore lets passwords come over into s, from other browsers and from files.
func (a *App) UsePasswordStore(s PasswordStore) { a.imports.passwords = s }

// broughtPasswords is what passwords brought.
type broughtPasswords struct {
	Added   int `json:"added"`
	Already int `json:"already"`
	Skipped int `json:"skipped"`
}

// extensionStore is what bringing extensions over needs of the extension manager: whether
// one is installed, an install from the store by id, which calls done with why it failed,
// or "", and dropping installs asked for that haven't started.
type extensionStore interface {
	Installed(id string) bool
	Install(id string, done func(failed string))
	Drop(ids []string)
}

// stallAfter is how long an import's step may go with nothing new before it ends as
// failed, as every progress does (docs/ux-guide.md): Go's timers count awake time, so a
// Mac's sleep doesn't count. Waiting on the person, for macOS's keychain question, doesn't
// either.
var stallAfter = 30 * time.Second

// importRun is an import going on: the step it is at, whether that step waits on the
// person, the extensions it asked the store for that haven't ended, and the clock that
// ends it when nothing new comes. A late answer for a run that has ended is dropped.
type importRun struct {
	step    string
	asking  bool
	pending []string
	stall   *time.Timer
}

// shownBrowser is a browser as the wizard lists it: its id, which names its logo, its
// name, and its profiles. Locked says macOS keeps its files from the app until it has Full
// Disk Access, so its profiles have nothing counted.
type shownBrowser struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Locked   bool           `json:"locked,omitempty"`
	Profiles []shownProfile `json:"profiles"`
}

// shownFrom is a profile shown, with the name of its browser. For a browser known only by
// its folder, keychain is the keychain's items its passwords' key may be in: the one it is
// when sure, else those the person was shown to choose from.
type shownFrom struct {
	browserdata.Profile
	browser      string
	keychain     []browserdata.KeychainItem
	keychainSure bool
	// keychainAll is every item the keychain listed, which the person chooses from once the
	// one its folder named turned out to be another browser's.
	keychainAll []browserdata.KeychainItem
}

// safeStorage is the keychain's item that holds the profile's passwords' key: the one its
// folder names, or the person's choice, by its service, of those they were shown. None
// leaves it to the names the browser goes by.
func (p shownFrom) safeStorage(chosen string) browserdata.KeychainItem {
	if p.keychainSure {
		return p.keychain[0]
	}
	for _, i := range p.keychain {
		if i.Service == chosen {
			return i
		}
	}
	return browserdata.KeychainItem{}
}

// shownProfile is a profile: its key, its name ("" for a browser of one), what it holds,
// and whether its passwords are behind a primary password, Firefox's. Keychains are the
// keychain's items, by service, for the person to choose its passwords' key from, when
// its folder doesn't say which.
type shownProfile struct {
	Key             string             `json:"key"`
	Name            string             `json:"name"`
	Counts          browserdata.Counts `json:"counts"`
	PrimaryPassword bool               `json:"primaryPassword,omitempty"`
	Keychains       []string           `json:"keychains,omitempty"`
}

// The kinds of things an import brings, in the order it brings them.
var importSteps = []string{"bookmarks", "history", "passwords", "extensions"}

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

func newImporter(a *App, e engine.Engine, home string, keychain func(string, string) ([]byte, error), keychainItems func() ([]browserdata.KeychainItem, error), send func(any)) *importer {
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if keychain == nil {
		keychain = browserdata.KeychainSecret
	}
	if keychainItems == nil {
		keychainItems = browserdata.KeychainItems
	}
	return &importer{a: a, e: e, home: home, keychain: keychain, keychainItems: keychainItems, profiles: map[string]shownFrom{}, runs: map[string]*importRun{}, send: send}
}

// snapshot is what a page starts with: no browsers until a look ends.
func (im *importer) snapshot(into map[string]any) {
	into["browsers"] = nil
	into["importing"] = im.importing()
}

func (im *importer) importing() []string {
	out := []string{}
	for k := range im.runs {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// shown is a browser as the pages hear of it, its profiles counted, which reads their
// files: called on a worker. Extensions are counted only while they can be installed, and
// passwords only once there is a store for them.
func shown(b browserdata.Browser, extensions, passwords bool) (shownBrowser, []shownFrom) {
	s := shownBrowser{ID: b.ID, Name: b.Name, Profiles: []shownProfile{}}
	for _, p := range b.Profiles {
		sp := shownProfile{Key: p.Dir, Name: p.Name}
		if p.Locked() {
			s.Locked = true
		} else {
			var err error
			if sp.Counts, err = p.Counts(); err != nil {
				timeline.Warnf("imports: %s %s: %v", b.Name, p.Name, err)
			}
			if !extensions {
				sp.Counts.Extensions = 0
			}
			if !passwords {
				sp.Counts.Passwords = 0
			} else if sp.Counts.Passwords > 0 {
				sp.PrimaryPassword = p.HasPrimaryPassword()
			}
		}
		s.Profiles = append(s.Profiles, sp)
	}
	from := make([]shownFrom, len(b.Profiles))
	for i, p := range b.Profiles {
		from[i] = shownFrom{Profile: p, browser: b.Name}
	}
	return s, from
}

// look finds the browsers again, with what each profile holds now, and tells the pages.
func (im *importer) look() {
	if im.looking {
		return
	}
	if shells.OnAndroid {
		im.send(map[string]any{"t": "browsers", "browsers": []shownBrowser{}})
		return
	}
	im.looking = true
	home, extensions, passwords := im.home, im.store != nil, im.passwords != nil
	go func() {
		start := time.Now()
		list := []shownBrowser{}
		var profiles []shownFrom
		var locked []browserdata.Profile
		for _, b := range browserdata.Find(home) {
			s, ps := shown(b, extensions, passwords)
			list = append(list, s)
			profiles = append(profiles, ps...)
			if s.Locked {
				for _, p := range ps {
					locked = append(locked, p.Profile)
				}
			}
		}
		took := time.Since(start)
		im.e.Post(func() {
			im.looking = false
			for _, p := range profiles {
				im.profiles[p.Dir] = p
			}
			im.locked = locked
			timeline.Infof("imports: found %d browsers on this Mac in %v", len(list), took.Round(time.Millisecond))
			im.send(map[string]any{"t": "browsers", "browsers": list, "passwords": im.passwords != nil})
		})
	}()
}

// importMessage is what a page sends: the profile to bring over, by key, and which of its
// things, of importSteps, with Firefox's primary password for its passwords, used once and
// kept nowhere, and the keychain's item the person chose for its passwords' key, by its
// service. A file's: its kind, bookmarks or passwords, its name and what it holds.
type importMessage struct {
	Key             string   `json:"key"`
	What            []string `json:"what"`
	PrimaryPassword string   `json:"primaryPassword"`
	Keychain        string   `json:"keychain"`
	Kind            string   `json:"kind"`
	Name            string   `json:"name"`
	Text            string   `json:"text"`
}

// handle takes a page's message about bringing things over, from the tab it is in, and
// says whether it was one.
func (im *importer) handle(tab engine.TabID, t string, msg []byte) bool {
	switch t {
	case "findBrowsers":
		im.look()
	case "chooseFolder":
		if win := im.a.Tabs.WindowOf(tab); win != 0 {
			// Where the browsers keep their folders, which shows them at once: the dialog
			// can't show hidden files, as ~/Library is (docs/layers.md, 2b).
			im.e.ChooseFolder(win, browserdata.Support(im.home))
		}
	case "openAccess":
		timeline.Infof("imports: opening System Settings at Full Disk Access")
		im.a.System.OpenFullDiskAccess()
		im.wait()
	case "import":
		var m importMessage
		if err := json.Unmarshal(msg, &m); err != nil {
			timeline.Warnf("imports: bad message: %v", err)
			return true
		}
		im.bring(m.Key, m.What, m.PrimaryPassword, m.Keychain)
	case "cancelImport":
		var m importMessage
		if json.Unmarshal(msg, &m) == nil {
			im.end(m.Key, "cancelled")
		}
	case "importFile":
		var m importMessage
		if err := json.Unmarshal(msg, &m); err != nil {
			timeline.Warnf("imports: bad message: %v", err)
			return true
		}
		im.bringFile(m.Kind, m.Name, m.Text)
	default:
		return false
	}
	return true
}

// folderChosen reads the folder the person chose as a browser's and tells the pages: the
// browser with its profiles, or why it isn't one. None chosen tells them so.
func (im *importer) folderChosen(path string) {
	if path == "" {
		im.send(map[string]any{"t": "folder", "cancelled": true})
		return
	}
	home, extensions, passwords, list := im.home, im.store != nil, im.passwords != nil, im.keychainItems
	go func() {
		b, err := browserdata.Open(home, path)
		s, ps := shown(b, extensions, passwords)
		if err == nil && passwords && b.ID == browserdata.OtherChromium {
			findSafeStorage(&s, ps, list)
		}
		im.e.Post(func() {
			if err != nil {
				why := "Couldn't read that folder: " + err.Error()
				if errors.Is(err, browserdata.ErrNoBrowser) {
					why = "That folder holds no Chrome-based or Firefox-based browser's profiles."
				}
				timeline.Infof("imports: %s: %v", path, err)
				im.send(map[string]any{"t": "folder", "error": why})
				return
			}
			for _, p := range ps {
				im.profiles[p.Dir] = p
			}
			timeline.Infof("imports: %s is %s's, with %d profiles", path, s.Name, len(s.Profiles))
			im.send(map[string]any{"t": "folder", "browser": s})
		})
	}()
}

// findSafeStorage finds, for each profile with passwords of a browser known only by its
// folder, the keychain's item of their key: listing the items asks the person nothing.
// One its folder names for sure is kept for the import; else the profile shows those to
// choose from. Called on a worker.
func findSafeStorage(s *shownBrowser, ps []shownFrom, list func() ([]browserdata.KeychainItem, error)) {
	var items []browserdata.KeychainItem
	listed := false
	for i := range ps {
		if s.Profiles[i].Counts.Passwords == 0 {
			continue
		}
		if !listed {
			var err error
			if items, err = list(); err != nil {
				timeline.Warnf("imports: listing the keychain's Safe Storage items: %v", err)
			}
			listed = true
		}
		ps[i].keychain, ps[i].keychainSure = ps[i].SafeStorage(items)
		ps[i].keychainAll = items
		switch {
		case ps[i].keychainSure:
			timeline.Infof("imports: %s's passwords' key is the keychain's %q", s.Name, ps[i].keychain[0].Service)
		case len(ps[i].keychain) > 0:
			for _, it := range ps[i].keychain {
				s.Profiles[i].Keychains = append(s.Profiles[i].Keychains, it.Service)
			}
			timeline.Infof("imports: %s's passwords' key is one of %d keychain items, for the person to choose", s.Name, len(ps[i].keychain))
		default:
			timeline.Infof("imports: no Safe Storage item in the keychain for %s; its folder's names are tried", s.Name)
		}
	}
}

// wait tries the locked profiles' files each second, on a worker, and looks again once
// none is locked, or gives up after ten minutes.
func (im *importer) wait() {
	locked := slices.Clone(im.locked)
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

// step tells the pages how one kind of thing is going: "doing", with how far for
// extensions, or "done" with what it brought, or an error.
func (im *importer) step(key, step, state string, result any, err error) {
	m := map[string]any{"t": "importStep", "key": key, "step": step, "state": state}
	if result != nil {
		m["result"] = result
	}
	if err != nil {
		m["error"] = err.Error()
	}
	// The keychain's item didn't open the passwords: which it was, and those to choose
	// another from.
	var wrong *wrongKeyError
	if errors.As(err, &wrong) {
		m["wrongKey"], m["keychain"], m["keychains"] = true, wrong.service, wrong.choices
	}
	im.send(m)
}

// bring brings over what was asked from the profile key, one kind after another: each
// read on a worker, then added here, and told to the pages as it lands. Asked again while
// it runs, as Retry does, the run going on ends and a new one starts at once; what came
// already isn't brought twice.
func (im *importer) bring(key string, what []string, primaryPassword, keychain string) {
	p, ok := im.profiles[key]
	if !ok || p.Locked() {
		timeline.Warnf("imports: %q is not a profile shown, or is locked", key)
		return
	}
	if im.runs[key] != nil {
		im.end(key, "retried")
	}
	var steps []string
	for _, s := range importSteps {
		if slices.Contains(what, s) && (s != "extensions" || im.store != nil) && (s != "passwords" || im.passwords != nil) {
			steps = append(steps, s)
		}
	}
	r := &importRun{}
	im.runs[key] = r
	r.stall = time.AfterFunc(stallAfter, func() { im.e.Post(func() { im.stalled(key, r) }) })
	im.send(map[string]any{"t": "importing", "importing": im.importing()})
	folder := folderFor(p)
	timeline.Infof("imports: bringing %v over from %s", steps, folder)
	alive := func() bool { return im.runs[key] == r }
	var next func(i int)
	next = func(i int) {
		if i == len(steps) {
			im.end(key, "done")
			return
		}
		s := steps[i]
		r.step = s
		im.progress(key, r, s, "doing", nil, nil)
		done := func(result any, err error) {
			if !alive() {
				return
			}
			if err != nil {
				timeline.Warnf("imports: %s from %s: %v", s, folder, err)
			}
			im.progress(key, r, s, "done", result, err)
			next(i + 1)
		}
		switch s {
		case "bookmarks":
			read(p.Bookmarks, func(bs []browserdata.Bookmark, err error) {
				if alive() {
					done(im.addBookmarks(p, folder, bs), err)
				}
			}, im.e)
		case "history":
			read(p.History, func(pages []browserdata.Page, err error) {
				if alive() {
					done(im.addHistory(pages), err)
				}
			}, im.e)
		case "passwords":
			// A Chromium browser's key is in the login keychain, which macOS asks the
			// person about: the worker waits for the answer, and the step's clock with it.
			if p.UsesKeychain() {
				r.asking = true
				im.progress(key, r, s, "asking", nil, nil)
			}
			keys := browserdata.Keys{Secret: im.keychain, Item: p.safeStorage(keychain), PrimaryPassword: primaryPassword}
			logins := func() ([]browserdata.Login, error) { return p.Logins(keys) }
			read(logins, func(ls []browserdata.Login, err error) {
				if !alive() {
					return
				}
				r.asking = false
				// A key that is another browser's brings nothing, and the person picks another:
				// of those they were shown, or, when the folder's own name led to this one, of
				// every item the keychain lists.
				if errors.Is(err, browserdata.ErrWrongKey) {
					if p.keychainSure && len(p.keychainAll) > 1 {
						p.keychainSure, p.keychain = false, p.keychainAll
						im.profiles[key] = p
					}
					wrong := &wrongKeyError{service: keys.Item.Service}
					if !p.keychainSure {
						for _, it := range p.keychain {
							wrong.choices = append(wrong.choices, it.Service)
						}
					}
					done(nil, wrong)
					return
				}
				// Keeping them may wait on the person too, where the OS asks before it gives
				// the device key: the step's clock waits with it.
				r.asking = true
				im.addPasswords(ls, func(b broughtPasswords) {
					if alive() {
						r.asking = false
						done(b, passwordError(err))
					}
				})
			}, im.e)
		case "extensions":
			read(p.Extensions, func(xs []browserdata.Extension, err error) {
				if alive() {
					im.installExtensions(key, r, xs, func(x *broughtExtensions) { done(x, err) })
				}
			}, im.e)
		}
	}
	next(0)
}

// progress tells the pages how a step is going, and starts its clock again: something new
// came. A step waiting on the person has no clock.
func (im *importer) progress(key string, r *importRun, step, state string, result any, err error) {
	if r.asking {
		r.stall.Stop()
	} else {
		r.stall.Reset(stallAfter)
	}
	im.step(key, step, state, result, err)
}

// stalled ends a run whose step went stallAfter with nothing new, as failed.
func (im *importer) stalled(key string, r *importRun) {
	if im.runs[key] != r || r.asking {
		return
	}
	timeline.Warnf("imports: %s from %s: nothing for %v; giving up", r.step, key, stallAfter)
	im.step(key, r.step, "done", nil, errors.New("Stopped: nothing arrived for 30 seconds"))
	im.end(key, "failed")
}

// end ends the run of key, if one is going: done, cancelled, failed, or retried, which a
// new run follows at once. Extension installs it asked for and that haven't started are
// dropped; a step's late answer finds the run gone and is dropped too.
func (im *importer) end(key, how string) {
	r := im.runs[key]
	if r == nil {
		return
	}
	delete(im.runs, key)
	r.stall.Stop()
	if len(r.pending) > 0 && im.store != nil {
		im.store.Drop(r.pending)
	}
	timeline.Infof("imports: %s from %s", how, key)
	if how != "retried" {
		im.send(map[string]any{"t": "imported", "key": key, "how": how, "importing": im.importing()})
	}
}

// read runs f on a worker and hands what it returned to then on the main thread.
func read[T any](f func() (T, error), then func(T, error), e engine.Engine) {
	go func() {
		v, err := f()
		e.Post(func() { then(v, err) })
	}()
}

// folderFor is the folder a profile's bookmarks go in: "From Chrome", or "From Chrome ·
// Work" for a profile with a name.
func folderFor(p shownFrom) string {
	if p.Name != "" {
		return "From " + p.browser + " · " + p.Name
	}
	return "From " + p.browser
}

// broughtBookmarks is what bookmarks brought: how many, in how many folders, how many were
// there already, the folder they are under, and how many were Safari's Reading List.
type broughtBookmarks struct {
	bookmarks.Imported
	Folder      string `json:"folder"`
	ReadingList int    `json:"readingList,omitempty"`
}

func (im *importer) addBookmarks(p shownFrom, folder string, bs []browserdata.Bookmark) broughtBookmarks {
	got := broughtBookmarks{Folder: folder}
	if len(bs) == 0 {
		return got
	}
	imported, err := im.a.Bookmarks.Import(bookmarks.Top, []bookmarks.Node{{Title: folder, Children: nodesOf(bs)}})
	if err != nil {
		timeline.Errorf("imports: bookmarks: %v", err)
	}
	got.Imported = imported
	for _, top := range bs {
		if p.Browser == "safari" && top.Title == browserdata.ReadingList {
			got.ReadingList = leaves(top.Children)
		}
	}
	return got
}

// addPasswords keeps logins and hands what came of them to then, on the main thread.
// Keeping them is a worker's: sealing the first password of a profile asks the OS for the
// device key, which may wait on the person, as a Linux keyring to unlock does and the Mac's
// keychain after an update (oskit/secret), and the main thread must never wait on that.
func (im *importer) addPasswords(ls []browserdata.Login, then func(broughtPasswords)) {
	if len(ls) == 0 {
		then(broughtPasswords{})
		return
	}
	read(func() (broughtPasswords, error) {
		added, already, skipped := im.passwords.Import(ls)
		return broughtPasswords{Added: added, Already: already, Skipped: skipped}, nil
	}, func(b broughtPasswords, _ error) { then(b) }, im.e)
}

// wrongKeyError is the keychain's item not opening a browser's passwords: the item's
// service, "" when it was one of the names the browser goes by, and the items the person
// can choose another from, none when there is no other.
type wrongKeyError struct {
	service string
	choices []string
}

func (e *wrongKeyError) Error() string {
	if e.service == "" {
		return "the keychain's key didn't open them"
	}
	return "they didn't open with " + e.service
}

// passwordError says why passwords didn't come, in words a page shows.
func passwordError(err error) error {
	switch {
	case errors.Is(err, browserdata.ErrPrimaryPassword):
		return errors.New("the primary password isn't right")
	case errors.Is(err, browserdata.ErrNoSecret):
		return errors.New("the keychain didn't give the browser's key")
	}
	return err
}

// bringFile brings over a file the person exported: bookmarks as an HTML file, into a
// folder named after it, or passwords as CSV. The page read the file and sent what it
// holds; a passwords file's text is in memory only, until the store has what it held.
func (im *importer) bringFile(kind, name, text string) {
	reply := func(result any, err error) {
		m := map[string]any{"t": "fileImported", "kind": kind, "name": name}
		if result != nil {
			m["result"] = result
		}
		if err != nil {
			m["error"] = err.Error()
			timeline.Infof("imports: %s file %s: %v", kind, name, err)
		} else {
			timeline.Infof("imports: %s from the file %s", kind, name)
		}
		im.send(m)
	}
	notAFile := func(err error, what string) error {
		if errors.Is(err, browserdata.ErrNotAFile) {
			return errors.New("That file isn't " + what + ".")
		}
		return err
	}
	switch kind {
	case "bookmarks":
		read(func() ([]browserdata.Bookmark, error) { return browserdata.ReadBookmarksHTML(strings.NewReader(text)) }, func(bs []browserdata.Bookmark, err error) {
			if err != nil {
				reply(nil, notAFile(err, "a bookmarks file a browser exported"))
				return
			}
			title := strings.TrimSuffix(name, filepath.Ext(name))
			reply(im.addBookmarks(shownFrom{browser: title}, "From "+title, bs), nil)
		}, im.e)
	case "passwords":
		if im.passwords == nil {
			return
		}
		read(func() ([]browserdata.Login, error) { return browserdata.ReadPasswordsCSV(strings.NewReader(text)) }, func(ls []browserdata.Login, err error) {
			if err != nil {
				reply(nil, notAFile(err, "a passwords file a browser or the Passwords app exported"))
				return
			}
			im.addPasswords(ls, func(b broughtPasswords) { reply(b, nil) })
		}, im.e)
	}
}

// broughtHistory is how many pages of history came.
type broughtHistory struct {
	Pages int `json:"pages"`
}

func (im *importer) addHistory(pages []browserdata.Page) broughtHistory {
	entries := make([]history.Entry, len(pages))
	for i, p := range pages {
		entries[i] = history.Entry{URL: p.URL, Title: p.Title, Visits: p.Visits, LastVisit: p.Last.Unix()}
	}
	if len(entries) > 0 {
		im.a.History.Import(entries)
	}
	return broughtHistory{Pages: len(entries)}
}

// installExtensions installs each extension the store has and this browser hasn't, in
// turn, telling the pages how far it is, and calls done once the last has ended.
func (im *importer) installExtensions(key string, r *importRun, xs []browserdata.Extension, done func(*broughtExtensions)) {
	x := &broughtExtensions{Installed: []string{}, Already: []string{}, Failed: []extensionNote{}, Skipped: []extensionNote{}}
	var want []browserdata.Extension
	for _, e := range xs {
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
		done(x)
		return
	}
	ended := 0
	for _, e := range want {
		r.pending = append(r.pending, e.ID)
	}
	im.progress(key, r, "extensions", "doing", map[string]int{"done": 0, "total": len(want)}, nil)
	for _, e := range want {
		im.store.Install(e.ID, func(failed string) {
			if im.runs[key] != r {
				return
			}
			r.pending = slices.DeleteFunc(r.pending, func(id string) bool { return id == e.ID })
			if failed == "" {
				x.Installed = append(x.Installed, e.Name)
			} else {
				x.Failed = append(x.Failed, extensionNote{e.Name, failed})
			}
			ended++
			if ended < len(want) {
				im.progress(key, r, "extensions", "doing", map[string]int{"done": ended, "total": len(want)}, nil)
				return
			}
			done(x)
		})
	}
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
