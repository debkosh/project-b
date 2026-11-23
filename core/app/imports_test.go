package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sudiptadeb/project-b/browserdata"

	"projectb/app"
	"projectb/bookmarks"
	"projectb/engine"
	"projectb/history"
	"projectb/profiles"
	"projectb/update"
)

// importPage is the welcome's page, as the import wizard talks to Go from it.
type importPage struct {
	t   *testing.T
	dir string
	r   *rig
	got <-chan []byte
	say func(string)
}

func startImports(t *testing.T, home string) *importPage {
	t.Helper()
	dir := t.TempDir()
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	t.Cleanup(stop)
	p := &importPage{t: t, dir: dir, r: r, got: got, say: func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }}
	p.say(`{"t":"ready"}`)
	if page := next(t, got, "snapshot"); page["browsers"] != nil {
		t.Fatalf("before a look the list is null: %v", page["browsers"])
	}
	return p
}

// browser is the browser named in the list the page has, by its id.
func browserIn(t *testing.T, list []any, id string) map[string]any {
	t.Helper()
	for _, b := range list {
		if b := b.(map[string]any); b["id"] == id {
			return b
		}
	}
	t.Fatalf("no %s in %v", id, list)
	return nil
}

func profilesOf(b map[string]any) []map[string]any {
	var out []map[string]any
	for _, p := range b["profiles"].([]any) {
		out = append(out, p.(map[string]any))
	}
	return out
}

// bring imports what from the profile key and returns each kind's result, by kind, as the
// page hears them: each "doing", then "done", then imported.
func (p *importPage) bring(key string, what ...string) map[string]json.RawMessage {
	p.t.Helper()
	b, _ := json.Marshal(map[string]any{"t": "import", "key": key, "what": what})
	p.say(string(b))
	out := map[string]json.RawMessage{}
	var seen []string
	deadline := time.After(10 * time.Second)
	for {
		select {
		case raw := <-p.got:
			var m struct {
				T, Key, Step, State, Error string
				Result                     json.RawMessage
			}
			json.Unmarshal(raw, &m)
			switch m.T {
			case "importStep":
				seen = append(seen, m.Step+" "+m.State)
				if m.State == "done" {
					out[m.Step] = m.Result
				}
			case "imported":
				// Each kind came in order, said first to be coming.
				for i, k := range what {
					if !slices.Contains(seen, k+" doing") || slices.Index(seen, k+" done") < slices.Index(seen, k+" doing") {
						p.t.Fatalf("kind %d %s: the steps were %q", i, k, seen)
					}
				}
				return out
			}
		case <-deadline:
			p.t.Fatalf("no end to the import; steps %q", seen)
		}
	}
}

// The wizard finds the browsers in a home folder (browserdata's testdata, made by its
// make.sh), each with its profiles and what each holds, forks among them, and brings one
// profile's things over one kind after another: Chrome · Personal's bookmarks into From
// Chrome · Personal with their folders, then its history. Brought again, nothing is
// counted twice. Nothing about an import is kept in the profile.
func TestBringingThingsOverFromAnotherBrowser(t *testing.T) {
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	p := startImports(t, home)
	p.say(`{"t":"findBrowsers"}`)
	list := next(t, p.got, "browsers")["browsers"].([]any)
	var ids []string
	for _, b := range list {
		ids = append(ids, b.(map[string]any)["id"].(string))
	}
	if !slices.Equal(ids, []string{"chrome", "opera", "firefox", "zen", "safari"}) {
		t.Fatalf("found %q", ids)
	}
	chrome := profilesOf(browserIn(t, list, "chrome"))
	if len(chrome) != 2 || chrome[0]["name"] != "Personal" || chrome[1]["name"] != "Work" {
		t.Fatalf("Chrome's profiles %v", chrome)
	}
	counts := chrome[0]["counts"].(map[string]any)
	if counts["bookmarks"] != 4.0 || counts["folders"] != 2.0 || counts["pages"] != 3.0 || counts["extensions"] != nil {
		t.Fatalf("Chrome · Personal holds %v", counts)
	}

	key := chrome[0]["key"].(string)
	got := p.bring(key, "bookmarks", "history")
	var bm struct {
		bookmarks.Imported
		Folder string
	}
	json.Unmarshal(got["bookmarks"], &bm)
	if bm.Imported != (bookmarks.Imported{Bookmarks: 4, Folders: 5}) || bm.Folder != "From Chrome · Personal" || string(got["history"]) != `{"pages":3}` {
		t.Fatalf("brought %s and %s", got["bookmarks"], got["history"])
	}
	look := make(chan []string)
	p.r.e.Post(func() {
		var titles []string
		for _, e := range p.r.a.Bookmarks.All() {
			titles = append(titles, e.Title)
		}
		p.r.a.History.Search("example.org/old", 1, func(es []history.Entry) {
			if len(es) == 1 {
				titles = append(titles, "visited")
			}
			look <- titles
		})
	})
	// The top's folder, its folders, then theirs, and the history.
	if titles := <-look; len(titles) != 10 || titles[9] != "visited" {
		t.Fatalf("bookmarks and history %q", titles)
	}

	got = p.bring(key, "bookmarks")
	json.Unmarshal(got["bookmarks"], &bm)
	if bm.Bookmarks != 0 || bm.Skipped != 4 {
		t.Fatalf("again: %s", got["bookmarks"])
	}
	if _, err := os.Stat(filepath.Join(p.dir, "imports.json")); !os.IsNotExist(err) {
		t.Fatalf("imports.json is kept: %v", err)
	}
}

// A folder the person chooses is read as a browser's and listed with its profiles, which
// then come over as any found do; one cancelled says so, and one with no browser's
// profiles says why.
func TestAFolderChosenIsABrowser(t *testing.T) {
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	p := startImports(t, home)
	choose := func(path string) map[string]any {
		t.Helper()
		p.r.e.Fire(1, 0, "choose", path)
		p.say(`{"t":"chooseFolder"}`)
		return next(t, p.got, "folder")
	}
	f := choose(filepath.Join(home, "Elsewhere/Thorium/User Data"))
	// The dialog opens where the browsers keep their folders, as it can't show ~/Library.
	if starts := p.r.e.FolderStarts(); len(starts) != 1 || starts[0] != filepath.Join(home, "Library/Application Support") {
		t.Fatalf("the dialog opened at %q", starts)
	}
	b, _ := f["browser"].(map[string]any)
	if b == nil || b["id"] != "other-chromium" || b["name"] != "Thorium" || len(profilesOf(b)) != 1 || profilesOf(b)[0]["name"] != "Me" {
		t.Fatalf("Thorium's folder: %v", f)
	}
	got := p.bring(profilesOf(b)[0]["key"].(string), "bookmarks")
	var bm struct {
		bookmarks.Imported
		Folder string
	}
	json.Unmarshal(got["bookmarks"], &bm)
	if bm.Bookmarks != 1 || bm.Folder != "From Thorium · Me" {
		t.Fatalf("from Thorium: %s", got["bookmarks"])
	}
	if f := choose(""); f["cancelled"] != true {
		t.Fatalf("none chosen: %v", f)
	}
	if f := choose(filepath.Join(home, "Library")); f["error"] == nil || f["browser"] != nil {
		t.Fatalf("a folder of no browser: %v", f)
	}
}

// Safari's files are behind Full Disk Access: its profile is listed locked, with nothing
// counted, and won't come over; asking for the access has Go try its files each second,
// and once they can be read the list comes again with its counts, and its bookmarks come
// over into From Safari. A folder with no permissions stands in for macOS's refusal.
func TestSafariWaitsForFullDiskAccess(t *testing.T) {
	home := t.TempDir()
	if err := os.CopyFS(home, os.DirFS("../../browserdata/testdata/home")); err != nil {
		t.Fatal(err)
	}
	safari := filepath.Join(home, "Library/Safari")
	if err := os.Chmod(safari, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(safari, 0o755)
	p := startImports(t, home)
	p.say(`{"t":"findBrowsers"}`)
	s := browserIn(t, next(t, p.got, "browsers")["browsers"].([]any), "safari")
	key := profilesOf(s)[0]["key"].(string)
	if s["locked"] != true || profilesOf(s)[0]["counts"].(map[string]any)["bookmarks"] != 0.0 {
		t.Fatalf("Safari, without access: %v", s)
	}
	p.say(`{"t":"import","key":"` + key + `","what":["bookmarks"]}`)
	p.say(`{"t":"openAccess"}`)
	// The import was refused: nothing comes.
	select {
	case b := <-p.got:
		t.Fatalf("a locked profile was brought over, or the list sent before access: %s", b)
	case <-time.After(1500 * time.Millisecond):
	}
	os.Chmod(safari, 0o755)
	s = browserIn(t, next(t, p.got, "browsers")["browsers"].([]any), "safari")
	if s["locked"] != nil || profilesOf(s)[0]["counts"].(map[string]any)["bookmarks"] != 4.0 || s["name"] != "Safari" {
		t.Fatalf("Safari, with access: %v", s)
	}
	got := p.bring(key, "bookmarks", "history")
	var bm struct {
		bookmarks.Imported
		Folder      string
		ReadingList int
	}
	json.Unmarshal(got["bookmarks"], &bm)
	// From Safari, Favorites, Recipes and Reading List.
	if bm.Imported != (bookmarks.Imported{Bookmarks: 4, Folders: 4}) || bm.Folder != "From Safari" || bm.ReadingList != 1 || string(got["history"]) != `{"pages":2}` {
		t.Fatalf("brought %s and %s", got["bookmarks"], got["history"])
	}
}

// fakePasswords is a passwords store that keeps which logins it has, by origin and
// username, as the real one counts them.
type fakePasswords struct{ saved map[string]string }

func (s *fakePasswords) Import(logins []browserdata.Login) (added, already, skipped int) {
	for _, l := range logins {
		key := l.Origin + " " + l.Username
		if _, ok := s.saved[key]; ok {
			already++
			continue
		}
		s.saved[key] = l.Password
		added++
	}
	return added, already, skipped
}

// With a passwords store, passwords come over too: Chrome's, opened with the keychain's
// key, Firefox's with no primary password, and Zen's only with its primary password,
// which the page sends with the import. Files a person exported come over as well:
// bookmarks into a folder named after the file, passwords into the store; one that isn't
// such a file says so.
func TestPasswordsComeOver(t *testing.T) {
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	dir := t.TempDir()
	keychain := func(service, account string) ([]byte, error) {
		if service == "Chrome Safe Storage" {
			return []byte("test-safe-storage"), nil
		}
		return nil, browserdata.ErrNoSecret
	}
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Keychain: keychain, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	store := &fakePasswords{saved: map[string]string{}}
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	p := &importPage{t: t, dir: dir, r: r, got: got, say: func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }}
	r.e.Post(func() { r.a.UsePasswordStore(store) })
	p.say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	p.say(`{"t":"findBrowsers"}`)
	m := next(t, got, "browsers")
	list := m["browsers"].([]any)
	chrome := profilesOf(browserIn(t, list, "chrome"))[0]
	zen := profilesOf(browserIn(t, list, "zen"))[0]
	if m["passwords"] != true || chrome["counts"].(map[string]any)["passwords"] != 3.0 || chrome["primaryPassword"] != nil || zen["primaryPassword"] != true {
		t.Fatalf("passwords %v, Chrome %v, Zen %v", m["passwords"], chrome, zen)
	}

	passwords := func(key, primary string) (string, string) {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"t": "import", "key": key, "what": []string{"passwords"}, "primaryPassword": primary})
		p.say(string(b))
		for {
			s := next(t, got, "importStep")
			if s["state"] == "done" {
				r, _ := json.Marshal(s["result"])
				e, _ := s["error"].(string)
				next(t, got, "imported")
				return string(r), e
			}
		}
	}
	if res, err := passwords(chrome["key"].(string), ""); res != `{"added":3,"already":0,"skipped":0}` || err != "" {
		t.Fatalf("Chrome's: %s %s", res, err)
	}
	firefox := profilesOf(browserIn(t, list, "firefox"))[0]
	// Firefox's alice at example.com is another origin than Chrome's example.com/login.
	if res, err := passwords(firefox["key"].(string), ""); res != `{"added":2,"already":0,"skipped":0}` || err != "" {
		t.Fatalf("Firefox's: %s %s", res, err)
	}
	if _, err := passwords(zen["key"].(string), "wrong"); err != "the primary password isn't right" {
		t.Fatalf("Zen's with a wrong primary password: %q", err)
	}
	if res, err := passwords(zen["key"].(string), "letmein"); res != `{"added":1,"already":0,"skipped":0}` || err != "" {
		t.Fatalf("Zen's: %s %s", res, err)
	}
	if store.saved["https://zen.example.com zed"] != "z3n" || store.saved["https://example.org/ bob"] != "hunter2" {
		t.Fatalf("the store has %v", store.saved)
	}

	file := func(kind, name, text string) map[string]any {
		t.Helper()
		b, _ := json.Marshal(map[string]any{"t": "importFile", "kind": kind, "name": name, "text": text})
		p.say(string(b))
		return next(t, got, "fileImported")
	}
	f := file("passwords", "Passwords.csv", "Title,URL,Username,Password,Notes,OTPAuth\nnew,https://new.example.com/,nina,n3w,,\nagain,https://example.org/,bob,hunter2,,\n")
	if r, _ := json.Marshal(f["result"]); string(r) != `{"added":1,"already":1,"skipped":0}` {
		t.Fatalf("a passwords file: %v", f)
	}
	f = file("bookmarks", "bookmarks_11_18.html", "<!DOCTYPE NETSCAPE-Bookmark-file-1>\n<DL><p>\n<DT><A HREF=\"https://example.com/file\">From a file</A>\n</DL><p>\n")
	if res := f["result"].(map[string]any); res["bookmarks"] != 1.0 || res["folder"] != "From bookmarks_11_18" {
		t.Fatalf("a bookmarks file: %v", f)
	}
	if f = file("bookmarks", "notes.html", "<html>notes</html>"); f["error"] != "That file isn't a bookmarks file a browser exported." {
		t.Fatalf("not a bookmarks file: %v", f)
	}
}

// While macOS's keychain question waits on the person, a Chromium browser's passwords step
// says it is asking and its clock doesn't run: it doesn't end as stalled however long the
// person takes, and goes on once they answer.
func TestAKeychainQuestionDoesNotStall(t *testing.T) {
	restore := app.SetStallAfter(100 * time.Millisecond)
	defer restore()
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	dir := t.TempDir()
	answer := make(chan struct{})
	keychain := func(service, account string) ([]byte, error) {
		<-answer
		return []byte("test-safe-storage"), nil
	}
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Keychain: keychain, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	r.e.Post(func() { r.a.UsePasswordStore(&fakePasswords{saved: map[string]string{}}) })
	say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	say(`{"t":"findBrowsers"}`)
	key := profilesOf(browserIn(t, next(t, got, "browsers")["browsers"].([]any), "chrome"))[0]["key"].(string)
	say(`{"t":"import","key":"` + key + `","what":["passwords"]}`)
	for s := next(t, got, "importStep"); s["state"] != "asking"; s = next(t, got, "importStep") {
	}
	select {
	case b := <-got:
		t.Fatalf("while the keychain asked: %s", b)
	case <-time.After(400 * time.Millisecond):
	}
	close(answer)
	m := next(t, got, "imported")
	if m["how"] != "done" {
		t.Fatalf("after the answer: %v", m)
	}
}

// waitingPasswords is a passwords store whose Import waits, as sealing a profile's first
// password does while the OS asks the person before it gives the device key.
type waitingPasswords struct {
	fakePasswords
	began  chan struct{}
	answer chan struct{}
}

func (s *waitingPasswords) Import(logins []browserdata.Login) (added, already, skipped int) {
	s.began <- struct{}{}
	<-s.answer
	return s.fakePasswords.Import(logins)
}

// Keeping passwords that came over may wait on the person, where the OS asks before it
// gives the device key: a keyring to unlock on Linux, the Mac's keychain after an update.
// The main thread doesn't wait with it, from a browser or from a file, the step's clock
// doesn't run meanwhile, and the import ends as done once they answer.
func TestKeepingPasswordsNeverHoldsTheMainThread(t *testing.T) {
	restore := app.SetStallAfter(100 * time.Millisecond)
	defer restore()
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	dir := t.TempDir()
	keychain := func(service, account string) ([]byte, error) { return []byte("test-safe-storage"), nil }
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Keychain: keychain, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	store := &waitingPasswords{fakePasswords: fakePasswords{saved: map[string]string{}}, began: make(chan struct{}, 4), answer: make(chan struct{})}
	// However the test ends, the store answers, so that nothing is left waiting on it.
	t.Cleanup(func() { close(store.answer) })
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	r.e.Post(func() { r.a.UsePasswordStore(store) })
	say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	// The store is asked for the passwords, and while it waits the main thread still runs
	// what is posted to it.
	free := func(what string) {
		t.Helper()
		select {
		case <-store.began:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the store was never asked to keep them", what)
		}
		ran := make(chan struct{})
		r.e.Post(func() { close(ran) })
		select {
		case <-ran:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: the main thread waited while the passwords were being kept", what)
		}
	}
	say(`{"t":"findBrowsers"}`)
	key := profilesOf(browserIn(t, next(t, got, "browsers")["browsers"].([]any), "chrome"))[0]["key"].(string)
	say(`{"t":"import","key":"` + key + `","what":["passwords"]}`)
	free("from a browser")
	// Four times the step's clock, and it hasn't ended as stalled.
	wait := time.After(400 * time.Millisecond)
	for waiting := true; waiting; {
		select {
		case b := <-got:
			var m map[string]any
			json.Unmarshal(b, &m)
			if m["t"] == "imported" {
				t.Fatalf("while the passwords were being kept: %s", b)
			}
		case <-wait:
			waiting = false
		}
	}
	store.answer <- struct{}{}
	if m := next(t, got, "imported"); m["how"] != "done" {
		t.Fatalf("after the answer: %v", m)
	}
	b, _ := json.Marshal(map[string]any{"t": "importFile", "kind": "passwords", "name": "Passwords.csv", "text": "Title,URL,Username,Password,Notes,OTPAuth\nnew,https://new.example.com/,nina,n3w,,\n"})
	say(string(b))
	free("from a file")
	store.answer <- struct{}{}
	if f, _ := json.Marshal(next(t, got, "fileImported")["result"]); string(f) != `{"added":1,"already":0,"skipped":0}` {
		t.Fatalf("a passwords file: %s", f)
	}
}

// A browser known only by the folder the person chose keeps its passwords' key in the
// keychain under its own "<name> Safe Storage", which the keychain lists without asking:
// a folder named Quokka has its key in Quokka's item, read once with nothing to choose; a
// folder whose name says nothing, Numbat's, shows the items to choose from, and the one
// the person chose opens its passwords. A choice not among those shown opens nothing.
func TestAFolderChosenFindsItsSafeStorage(t *testing.T) {
	home := t.TempDir()
	chrome := os.DirFS("../../browserdata/testdata/home/Library/Application Support/Google/Chrome")
	for _, dir := range []string{"Library/Application Support/Bramblewood/Quokka", "Elsewhere/Numbat", "Elsewhere/Chrome"} {
		if err := os.CopyFS(filepath.Join(home, dir), chrome); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	var asked []string
	keychain := func(service, account string) ([]byte, error) {
		asked = append(asked, service)
		if service == "Quokka Safe Storage" && account == "Quokka" || service == "Wombat Safe Storage" && account == "Wombat" {
			return []byte("test-safe-storage"), nil
		}
		// Another browser's key, which opens none of these.
		if service == "Chrome Safe Storage" {
			return []byte("another-browsers-key"), nil
		}
		return nil, browserdata.ErrNoSecret
	}
	items := func() ([]browserdata.KeychainItem, error) {
		return []browserdata.KeychainItem{{Service: "Chrome Safe Storage", Account: "Chrome"}, {Service: "Quokka Safe Storage", Account: "Quokka"}, {Service: "Wombat Safe Storage", Account: "Wombat"}}, nil
	}
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Keychain: keychain, KeychainItems: items, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	r.e.Post(func() { r.a.UsePasswordStore(&fakePasswords{saved: map[string]string{}}) })
	say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	personal := func(path string) map[string]any {
		t.Helper()
		r.e.Fire(1, 0, "choose", filepath.Join(home, path))
		say(`{"t":"chooseFolder"}`)
		b := next(t, got, "folder")["browser"].(map[string]any)
		for _, p := range profilesOf(b) {
			if p["name"] == "Personal" {
				return p
			}
		}
		t.Fatalf("no Personal in %v", b)
		return nil
	}
	// step is the passwords step's last word, and passwords what it brought and its error.
	step := func(key, chosen string) map[string]any {
		t.Helper()
		asked = nil
		b, _ := json.Marshal(map[string]any{"t": "import", "key": key, "what": []string{"passwords"}, "keychain": chosen})
		say(string(b))
		for {
			s := next(t, got, "importStep")
			if s["state"] == "done" {
				next(t, got, "imported")
				return s
			}
		}
	}
	passwords := func(key, chosen string) (string, string) {
		t.Helper()
		s := step(key, chosen)
		res, _ := json.Marshal(s["result"])
		e, _ := s["error"].(string)
		return string(res), e
	}

	quokka := personal("Library/Application Support/Bramblewood/Quokka")
	if quokka["keychains"] != nil {
		t.Fatalf("Quokka's folder names its item, yet there is a choice: %v", quokka)
	}
	if res, err := passwords(quokka["key"].(string), ""); res != `{"added":3,"already":0,"skipped":0}` || err != "" || !slices.Equal(asked, []string{"Quokka Safe Storage"}) {
		t.Fatalf("Quokka's: %s %q, the keychain asked for %q", res, err, asked)
	}

	numbat := personal("Elsewhere/Numbat")
	if choices, _ := json.Marshal(numbat["keychains"]); string(choices) != `["Chrome Safe Storage","Quokka Safe Storage","Wombat Safe Storage"]` {
		t.Fatalf("Numbat's choices: %s", choices)
	}
	if res, err := passwords(numbat["key"].(string), "Wombat Safe Storage"); res != `{"added":0,"already":3,"skipped":0}` || err != "" || !slices.Equal(asked, []string{"Wombat Safe Storage"}) {
		t.Fatalf("Numbat's, with Wombat's item chosen: %s %q, the keychain asked for %q", res, err, asked)
	}
	if _, err := passwords(numbat["key"].(string), "Elsewhere Safe Storage"); err != "the keychain didn't give the browser's key" || slices.Contains(asked, "Elsewhere Safe Storage") {
		t.Fatalf("Numbat's, with an item not shown: %q, the keychain asked for %q", err, asked)
	}

	// An item that is another browser's opens nothing: no password is brought, the step
	// says which item it was, and names those to pick another from.
	all := `["Chrome Safe Storage","Quokka Safe Storage","Wombat Safe Storage"]`
	s := step(numbat["key"].(string), "Chrome Safe Storage")
	if choices, _ := json.Marshal(s["keychains"]); s["error"] != "they didn't open with Chrome Safe Storage" || s["wrongKey"] != true || s["keychain"] != "Chrome Safe Storage" || string(choices) != all || s["result"] != nil {
		t.Fatalf("Numbat's, with another browser's item chosen: %v", s)
	}
	// A folder whose own name leads to another browser's item: the same, and every item
	// is then the person's to pick from, the right one opening them.
	named := personal("Elsewhere/Chrome")
	if named["keychains"] != nil {
		t.Fatalf("a folder named Chrome has a choice before its item failed: %v", named)
	}
	s = step(named["key"].(string), "")
	if choices, _ := json.Marshal(s["keychains"]); s["wrongKey"] != true || s["keychain"] != "Chrome Safe Storage" || string(choices) != all || s["result"] != nil {
		t.Fatalf("a folder whose name leads to another browser's item: %v", s)
	}
	if res, err := passwords(named["key"].(string), "Quokka Safe Storage"); res != `{"added":0,"already":3,"skipped":0}` || err != "" || !slices.Equal(asked, []string{"Quokka Safe Storage"}) {
		t.Fatalf("the same folder, with the right item picked: %s %q, the keychain asked for %q", res, err, asked)
	}
}
