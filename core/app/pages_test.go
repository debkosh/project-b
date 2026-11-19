package app_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"projectb/engine"
)

// openPage runs the action that shows a page of ours, says the page is ready, and returns
// its tab and what it hears.
func (r *rig) openPage(action, page string) (engine.TabID, <-chan []byte) {
	r.t.Helper()
	r.e.Fire(1, 0, "menu", action)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "tabs" && len(m["tabs"].([]any)) == 2 })
	tab := engine.TabID(m["active"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	r.t.Cleanup(stop)
	r.e.Post(func() { r.a.PageMessage(tab, engine.Page(page), []byte(`{"t":"ready"}`)) })
	return tab, got
}

func next(t *testing.T, ch <-chan []byte, kind string) map[string]any {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case b := <-ch:
			var m map[string]any
			json.Unmarshal(b, &m)
			if m["t"] == kind {
				return m
			}
		case <-deadline:
			t.Fatalf("no %s message", kind)
		}
	}
}

func TestTheHistoryPageListsSearchesAndForgets(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.visit("https://b.test/news")
	tab, got := r.openPage("history.show", "history")
	snap := next(t, got, "snapshot")
	if items := snap["items"].([]any); len(items) != 2 {
		t.Fatalf("items = %v", items)
	}
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "history", []byte(msg)) }) }
	say(`{"t":"search","text":"news"}`)
	if items := next(t, got, "items")["items"].([]any); len(items) != 1 || items[0].(map[string]any)["url"] != "https://b.test/news" {
		t.Fatalf("search = %v", items)
	}
	say(`{"t":"delete","urls":["https://b.test/news"]}`)
	if items := next(t, got, "items")["items"].([]any); len(items) != 1 || items[0].(map[string]any)["url"] != "https://a.test/" {
		t.Fatalf("after the delete: %v", items)
	}
	say(`{"t":"clear","since":0}`)
	if items := next(t, got, "items")["items"].([]any); len(items) != 0 {
		t.Fatalf("after clear: %v", items)
	}
	// A page of ours opens an address beside itself.
	say(`{"t":"open","url":"https://c.test/"}`)
	r.waitForTab(func(tab map[string]any) bool {
		return tab["url"] == "https://c.test/" && tab["loading"] == false && tab["title"] != ""
	})
}

// Cmd+D and the star bookmark the page into the folder last saved in and have the window's
// shell show the panel for it; the panel's Enter saves a name and a folder, which the next
// bookmark goes into too. The Bookmarks page makes folders, moves entries, edits and
// removes them, a folder with all it holds (docs/bookmark-folders.md).
func TestBookmarksFromTheShellAndThePage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.e.Fire(1, 0, "menu", "bookmarks.toggle")
	shown := func() map[string]any {
		return r.waitFor(func(m map[string]any) bool { return m["t"] == "bookmarkPanel" })["bookmarkPanel"].(map[string]any)
	}
	p := shown()
	panel := p["id"].(string)
	a, _ := r.a.Bookmarks.Get(panel)
	if a.URL != "https://a.test/" || a.Parent != "" || p["new"] != true {
		t.Fatalf("Cmd+D bookmarked %+v, panel %v", a, p)
	}
	// Again on a bookmarked page, the panel shows for the same bookmark.
	r.e.Fire(1, 0, "menu", "bookmarks.toggle")
	if again := shown(); again["id"] != panel || again["new"] != false || len(r.a.Bookmarks.List()) != 1 {
		t.Fatalf("a second Cmd+D: panel %v, %d bookmarks", again, len(r.a.Bookmarks.List()))
	}

	tab, got := r.openPage("bookmarks.show", "bookmarks")
	if b := next(t, got, "snapshot")["bookmarks"].([]any); len(b) != 1 {
		t.Fatalf("page's bookmarks = %v", b)
	}
	say := func(msg string, args ...any) {
		r.e.Post(func() { r.a.PageMessage(tab, "bookmarks", []byte(fmt.Sprintf(msg, args...))) })
	}
	say(`{"t":"newFolder","parent":"","title":"Reading"}`)
	next(t, got, "bookmarks")
	var reading string
	for _, e := range r.a.Bookmarks.All() {
		if e.Kind == "folder" {
			reading = e.ID
		}
	}
	r.shell(fmt.Sprintf(`{"t":"saveBookmark","bookmark":%q,"title":"A page","parent":%q}`, a.ID, reading))
	next(t, got, "bookmarks")
	r.shell(`{"t":"bookmark","url":"https://b.test/","title":"B"}`)
	b := shown()["id"].(string)
	if e, _ := r.a.Bookmarks.Get(b); e.Parent != reading {
		t.Fatalf("the star after a save into Reading put B in %q", e.Parent)
	}
	next(t, got, "bookmarks")

	say(`{"t":"moveBookmark","id":%q,"parent":""}`, b)
	next(t, got, "bookmarks")
	say(`{"t":"editBookmark","id":%q,"title":"Home","url":"https://a.test/"}`, b)
	if m := next(t, got, "error"); m["id"] != b {
		t.Fatalf("an address bookmarked already: %v", m)
	}
	say(`{"t":"editBookmark","id":%q,"title":"Bee","url":"https://b.test/"}`, b)
	next(t, got, "bookmarks")
	if e, _ := r.a.Bookmarks.Get(b); e.Parent != "" || e.Title != "Bee" {
		t.Fatalf("after the move and edit: %+v", e)
	}
	say(`{"t":"removeBookmark","id":%q}`, reading)
	if l := next(t, got, "bookmarks")["bookmarks"].([]any); len(l) != 1 || l[0].(map[string]any)["id"] != b {
		t.Fatalf("after removing Reading: %v", l)
	}
}

func TestTheDownloadsPageListsAndActs(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })

	tab, got := r.openPage("downloads.show", "downloads")
	list := next(t, got, "snapshot")["downloads"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["name"] != "report.pdf" {
		t.Fatalf("the page's downloads = %v", list)
	}
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "downloads", []byte(msg)) }) }
	say(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"open"}`, list[0].(map[string]any)["id"]))
	r.e.Settle()
	if opened := r.sys.Opened(); len(opened) != 1 {
		t.Fatalf("Open from the page opened %v", opened)
	}
	// Share, from a phone's ⋮, hands the file to the system's share sheet.
	say(fmt.Sprintf(`{"t":"downloadAction","id":%v,"action":"share"}`, list[0].(map[string]any)["id"]))
	r.e.Settle()
	if shared := r.e.Shared(); !strings.HasSuffix(shared, "report.pdf") {
		t.Fatalf("Share from the page shared %q", shared)
	}
	// Clear empties the list, and the page hears of it as of any change.
	say(`{"t":"clearDownloads"}`)
	if list := next(t, got, "downloads")["downloads"].([]any); len(list) != 0 {
		t.Fatalf("after Clear: %v", list)
	}
	// A phone's ← closes the page's tab, the tab used before taking its place.
	say(`{"t":"close"}`)
	r.e.Settle()
	var still bool
	r.e.Post(func() { _, still = r.a.Tabs.Get(tab) })
	r.e.Settle()
	if still {
		t.Fatal("← left the Downloads page's tab open")
	}
}

// A phone's Delete, from a download's ⋮: it deletes the download's copy and takes it off the
// list.
func TestTheDownloadsPageDeletes(t *testing.T) {
	profile, dir := t.TempDir(), t.TempDir()
	r := startDownloading(t, profile, dir)
	defer r.stop()
	r.visit("https://a.test/")
	r.show(1)
	r.e.Fire(1, 0, "download", "https://a.test/files/report.pdf")
	r.downloads(func(l []any) bool { return len(l) == 1 && first(l)["state"] == "done" })
	tab, got := r.openPage("downloads.show", "downloads")
	id := next(t, got, "snapshot")["downloads"].([]any)[0].(map[string]any)["id"]
	r.e.Post(func() { r.a.PageMessage(tab, "downloads", []byte(fmt.Sprintf(`{"t":"deleteDownload","id":%v}`, id))) })
	if list := next(t, got, "downloads")["downloads"].([]any); len(list) != 0 {
		t.Fatalf("after Delete: %v", list)
	}
	if deleted := r.e.Deleted(); len(deleted) != 1 || !strings.HasSuffix(deleted[0], "report.pdf") {
		t.Fatalf("the engine deleted %v", deleted)
	}
}
