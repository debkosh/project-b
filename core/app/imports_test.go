package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

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
