package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"projectb/app"
	"projectb/bookmarks"
	"projectb/engine"
	"projectb/profiles"
	"projectb/update"
)

// The welcome finds the browsers in a home folder (browserdata's testdata, made by its
// make.sh), each profile with what it holds, and brings over what is asked: Chrome's
// bookmarks into From Chrome · Personal with their folders, its history, and its search
// engine, Bing, as the profile's. imports.json keeps what was brought, and the list says so.
func TestBringingThingsOverFromAnotherBrowser(t *testing.T) {
	dir := t.TempDir()
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	say(`{"t":"ready"}`)
	if page := next(t, got, "snapshot"); page["browsers"] != nil {
		t.Fatalf("before a look the list is null: %v", page["browsers"])
	}
	say(`{"t":"findBrowsers"}`)
	list := next(t, got, "browsers")["browsers"].([]any)
	var titles []string
	for _, b := range list {
		titles = append(titles, b.(map[string]any)["title"].(string))
	}
	if len(titles) != 3 || titles[0] != "Chrome · Personal" || titles[2] != "Firefox · default-release" {
		t.Fatalf("found %q", titles)
	}
	counts := list[0].(map[string]any)["counts"].(map[string]any)
	if counts["bookmarks"] != 4.0 || counts["folders"] != 2.0 || counts["pages"] != 3.0 || counts["search"] != "Bing" {
		t.Fatalf("Chrome · Personal holds %v", counts)
	}

	say(`{"t":"import","key":"chrome/Default","what":["bookmarks","history","search"]}`)
	if m := next(t, got, "importing"); len(m["importing"].([]any)) != 1 {
		t.Fatalf("importing %v", m)
	}
	imported := next(t, got, "imported")
	b, _ := json.Marshal(imported["imported"])
	var brought struct {
		Bookmarks bookmarks.Imported
		Folder    string
		Pages     int
		Search    string
		Errors    []string
	}
	json.Unmarshal(b, &brought)
	if brought.Bookmarks != (bookmarks.Imported{Bookmarks: 4, Folders: 5}) || brought.Folder != "From Chrome · Personal" || brought.Pages != 3 || brought.Search != "bing" || len(brought.Errors) != 0 {
		t.Fatalf("brought over %s", b)
	}
	if l := next(t, got, "browsers")["browsers"].([]any)[0].(map[string]any)["last"]; l == nil {
		t.Fatal("the list doesn't say what Chrome · Personal brought")
	}

	type seen struct {
		titles  []string
		visited bool
		search  string
	}
	look := make(chan seen)
	r.e.Post(func() {
		var s seen
		for _, e := range r.a.Bookmarks.All() {
			s.titles = append(s.titles, e.Title)
		}
		s.visited = len(r.a.History.Search("example.org/old", 1)) == 1
		s.search = r.a.Prefs.SearchEngine().ID
		look <- s
	})
	s := <-look
	// The top's folder, its folders, then theirs.
	if len(s.titles) != 9 || !s.visited || s.search != "bing" {
		t.Fatalf("bookmarks %q, visited %v, search %q", s.titles, s.visited, s.search)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "imports.json")); err != nil || len(b) < 10 {
		t.Fatalf("imports.json: %s, %v", b, err)
	}

	// Made again, it adds nothing.
	say(`{"t":"import","key":"chrome/Default","what":["bookmarks"]}`)
	again := next(t, got, "imported")["imported"].(map[string]any)["bookmarks"].(map[string]any)
	if again["bookmarks"] != 0.0 || again["skipped"] != 4.0 {
		t.Fatalf("again: %v", again)
	}
}
