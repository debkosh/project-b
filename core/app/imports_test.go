package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"projectb/app"
	"projectb/bookmarks"
	"projectb/engine"
	"projectb/profiles"
	"projectb/update"
)

// The welcome finds the browsers in a home folder (browserdata's testdata, made by its
// make.sh), each profile with what it holds, and brings over what is asked: Chrome's
// bookmarks into From Chrome · Personal with their folders, and its history. imports.json keeps what was brought, and the list says so.
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
	if len(titles) != 4 || titles[0] != "Chrome · Personal" || titles[2] != "Firefox · default-release" || titles[3] != "Safari" {
		t.Fatalf("found %q", titles)
	}
	counts := list[0].(map[string]any)["counts"].(map[string]any)
	if counts["bookmarks"] != 4.0 || counts["folders"] != 2.0 || counts["pages"] != 3.0 {
		t.Fatalf("Chrome · Personal holds %v", counts)
	}

	say(`{"t":"import","key":"chrome/Default","what":["bookmarks","history"]}`)
	if m := next(t, got, "importing"); len(m["importing"].([]any)) != 1 {
		t.Fatalf("importing %v", m)
	}
	imported := next(t, got, "imported")
	b, _ := json.Marshal(imported["imported"])
	var brought struct {
		Bookmarks bookmarks.Imported
		Folder    string
		Pages     int
		Errors    []string
	}
	json.Unmarshal(b, &brought)
	if brought.Bookmarks != (bookmarks.Imported{Bookmarks: 4, Folders: 5}) || brought.Folder != "From Chrome · Personal" || brought.Pages != 3 || len(brought.Errors) != 0 {
		t.Fatalf("brought over %s", b)
	}
	if l := next(t, got, "browsers")["browsers"].([]any)[0].(map[string]any)["last"]; l == nil {
		t.Fatal("the list doesn't say what Chrome · Personal brought")
	}

	type seen struct {
		titles  []string
		visited bool
	}
	look := make(chan seen)
	r.e.Post(func() {
		var s seen
		for _, e := range r.a.Bookmarks.All() {
			s.titles = append(s.titles, e.Title)
		}
		s.visited = len(r.a.History.Search("example.org/old", 1)) == 1
		look <- s
	})
	s := <-look
	// The top's folder, its folders, then theirs.
	if len(s.titles) != 9 || !s.visited {
		t.Fatalf("bookmarks %q, visited %v", s.titles, s.visited)
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

// Safari's files are behind Full Disk Access: until the app has it, Safari's row is locked,
// with nothing counted, and can't be brought over. Once a page has opened System Settings,
// Go tries the files each second and, when they can be read, looks again: the row opens,
// and brings Safari's bookmarks into From Safari. A folder
// with no permissions stands in for macOS's refusal.
func TestSafariWaitsForFullDiskAccess(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	if err := os.CopyFS(home, os.DirFS("../../browserdata/testdata/home")); err != nil {
		t.Fatal(err)
	}
	safari := filepath.Join(home, "Library/Safari")
	if err := os.Chmod(safari, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(safari, 0o755)
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	safariRow := func(list []any) map[string]any {
		for _, b := range list {
			if b := b.(map[string]any); b["key"] == "safari/Safari" {
				return b
			}
		}
		t.Fatalf("no Safari in %v", list)
		return nil
	}
	say(`{"t":"findBrowsers"}`)
	row := safariRow(next(t, got, "browsers")["browsers"].([]any))
	if row["locked"] != true || row["counts"].(map[string]any)["bookmarks"] != 0.0 {
		t.Fatalf("Safari's row, without access: %v", row)
	}
	say(`{"t":"import","key":"safari/Safari","what":["bookmarks"]}`)
	say(`{"t":"openAccess"}`)
	// The import was refused: nothing is being brought over.
	select {
	case b := <-got:
		t.Fatalf("a locked row was brought over, or the list sent before access: %s", b)
	case <-time.After(1500 * time.Millisecond):
	}
	os.Chmod(safari, 0o755)
	row = safariRow(next(t, got, "browsers")["browsers"].([]any))
	if row["locked"] != nil || row["counts"].(map[string]any)["bookmarks"] != 4.0 || row["title"] != "Safari" {
		t.Fatalf("Safari's row, with access: %v", row)
	}
	say(`{"t":"import","key":"safari/Safari","what":["bookmarks","history"]}`)
	next(t, got, "importing")
	b, _ := json.Marshal(next(t, got, "imported")["imported"])
	var brought struct {
		Bookmarks bookmarks.Imported
		Folder    string
		Pages     int
		Errors    []string
	}
	json.Unmarshal(b, &brought)
	// From Safari, Favorites, Recipes and Reading List.
	if brought.Bookmarks != (bookmarks.Imported{Bookmarks: 4, Folders: 4}) || brought.Folder != "From Safari" || brought.Pages != 2 || len(brought.Errors) != 0 {
		t.Fatalf("brought over %s", b)
	}
}
