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
		if len(p.r.a.History.Search("example.org/old", 1)) == 1 {
			titles = append(titles, "visited")
		}
		look <- titles
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
