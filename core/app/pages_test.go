package app_test

import (
	"encoding/json"
	"fmt"
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

func TestBookmarksFromTheShellAndThePage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://a.test/")
	r.e.Fire(1, 0, "menu", "bookmarks.toggle")
	m := r.waitFor(func(m map[string]any) bool {
		b, _ := m["bookmarks"].([]any)
		return m["t"] == "bookmarks" && len(b) == 1
	})
	first := m["bookmarks"].([]any)[0].(map[string]any)
	if first["url"] != "https://a.test/" {
		t.Fatalf("bookmark = %v", first)
	}
	r.shell(`{"t":"bookmark","url":"https://b.test/","title":"B","on":true}`)
	r.waitFor(func(m map[string]any) bool {
		b, _ := m["bookmarks"].([]any)
		return m["t"] == "bookmarks" && len(b) == 2
	})

	tab, got := r.openPage("bookmarks.show", "bookmarks")
	if b := next(t, got, "snapshot")["bookmarks"].([]any); len(b) != 2 {
		t.Fatalf("page's bookmarks = %v", b)
	}
	r.e.Post(func() {
		r.a.PageMessage(tab, "bookmarks", []byte(fmt.Sprintf(`{"t":"editBookmark","id":%v,"title":"Home","url":"https://a.test/home"}`, first["id"])))
	})
	if b := next(t, got, "bookmarks")["bookmarks"].([]any); b[1].(map[string]any)["title"] != "Home" {
		t.Fatalf("after the edit: %v", b)
	}
	r.e.Post(func() {
		r.a.PageMessage(tab, "bookmarks", []byte(fmt.Sprintf(`{"t":"removeBookmark","id":%v}`, first["id"])))
	})
	if b := next(t, got, "bookmarks")["bookmarks"].([]any); len(b) != 1 {
		t.Fatalf("after the removal: %v", b)
	}
}
