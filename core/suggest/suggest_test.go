package suggest

import (
	"fmt"
	"strings"
	"testing"

	"projectb/history"
)

func manager(t testing.TB, visits ...string) *Manager {
	h := history.New(t.TempDir(), func(f func()) { f() })
	t.Cleanup(h.Close)
	for _, v := range visits {
		u, title, _ := strings.Cut(v, " ")
		h.AddVisit(u, title)
	}
	m := New(h)
	m.SetSources(Sources{
		Tabs: func() []Tab {
			return []Tab{{ID: 7, Title: "Mail inbox", URL: "https://mail.test/", Detail: "Work"}, {ID: 8, Title: "", URL: ""}}
		},
		Actions: func() []Action {
			return []Action{{ID: "tab.new", Title: "New Tab", Detail: "⌘T"}, {ID: "app.settings", Title: "Settings…"}, {ID: "view.tabsTop", Title: "Tabs on Top"}}
		},
		Engine: func() (string, func(string) string) {
			return "Find", func(w string) string { return "https://find.test/?q=" + strings.ReplaceAll(w, " ", "+") }
		},
		Bookmarks: func() []Bookmark { return marks },
	})
	return m
}

var all = []string{KindAddress, KindSearch, KindTab, KindHistory, KindAction, KindBookmark}

// marks are the bookmarks the tests' manager has, the newest first.
var marks []Bookmark

func kinds(rows []Row) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Kind)
	}
	return strings.Join(out, " ")
}

func TestWordsSearchFirstThenTabsHistoryAndActions(t *testing.T) {
	m := manager(t, "https://mail.test/ Mail", "https://news.test/mail-digest Digest")
	rows := m.Query("mail", all)
	if kinds(rows) != "search tab history" {
		t.Fatalf("kinds = %q: %+v", kinds(rows), rows)
	}
	if rows[0].URL != "https://find.test/?q=mail" || rows[0].Detail != "Search Find" {
		t.Fatalf("search row = %+v", rows[0])
	}
	if rows[1].Tab != 7 || rows[1].Detail != "Work" {
		t.Fatalf("tab row = %+v", rows[1])
	}
	// The page open in a tab is listed once, as the tab.
	if rows[2].URL != "https://news.test/mail-digest" || rows[2].Detail != "news.test" {
		t.Fatalf("history row = %+v", rows[2])
	}

	rows = m.Query("tab", all)
	if kinds(rows) != "search action action" || rows[1].Action != "view.tabsTop" || rows[2].Action != "tab.new" || rows[2].Detail != "⌘T" {
		t.Fatalf("actions: a title that starts with the text first: %+v", rows)
	}
}

func TestActionsAskedForAloneFillTheList(t *testing.T) {
	m := manager(t)
	if rows := m.Query("t", []string{KindAction}); kinds(rows) != "action action action" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestAnAddressGoesFirstAndTheSearchLast(t *testing.T) {
	m := manager(t, "https://mail.test/ Mail")
	rows := m.Query("mail.test", all)
	if kinds(rows) != "address tab search" || rows[0].URL != "https://mail.test" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestNothingTypedListsTheSitesVisitedMost(t *testing.T) {
	marks = nil
	m := manager(t, "https://a.test/ A", "https://a.test/ A", "https://b.test/x B", "https://b.test/y B", "https://b.test/y B")
	rows := m.Query("", all)
	if len(rows) != 2 || rows[0].URL != "https://b.test/y" || rows[0].Kind != KindHistory || rows[0].Detail != "b.test" {
		t.Fatalf("rows = %+v", rows)
	}
}

// With nothing typed the sites visited most come first, then the newest bookmarks not
// among them, eight rows at most: the sites keep five when there are bookmarks to show, and
// either fills what the other leaves.
func TestNothingTypedAddsTheNewestBookmarks(t *testing.T) {
	var visits []string
	for i := range 9 {
		visits = append(visits, fmt.Sprintf("https://site%d.test/ Site %d", i, i))
	}
	marks = []Bookmark{{Title: "Site 0", URL: "https://site0.test/"}, {Title: "Docs", URL: "https://docs.test/"}, {Title: "Spec", URL: "https://spec.test/"}, {Title: "Go", URL: "https://go.test/"}, {Title: "Blog", URL: "https://blog.test/"}}
	m := manager(t, visits...)
	if rows := m.Query("", all); kinds(rows) != "history history history history history bookmark bookmark bookmark" || rows[5].URL != "https://docs.test/" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows := m.Query("", []string{KindBookmark}); len(rows) != 5 || rows[0].Title != "Site 0" {
		t.Fatalf("bookmarks alone = %+v", rows)
	}
	marks = marks[:2]
	if rows := m.Query("", all); kinds(rows) != "history history history history history history history bookmark" {
		t.Fatalf("one bookmark to show = %+v", rows)
	}
	marks = nil
	m = manager(t)
	if rows := m.Query("", all); rows == nil || len(rows) != 0 {
		t.Fatalf("nothing at all = %#v", rows)
	}
}

// A shell that names no kinds gets history alone, as before there were any.
func TestNoKindsIsHistoryAlone(t *testing.T) {
	m := manager(t, "https://mail.test/ Mail")
	rows := m.Query("mail", nil)
	if len(rows) != 1 || rows[0].Kind != KindHistory || rows[0].URL != "https://mail.test/" || rows[0].Title != "Mail" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows := m.Query("", nil); len(rows) != 0 {
		t.Fatalf("nothing typed: %+v", rows)
	}
}

// A key in the field over a long history: one pass, nothing lowercased again.
func BenchmarkAKeyOver100000Visits(b *testing.B) {
	h := history.New(b.TempDir(), func(f func()) { f() })
	b.Cleanup(h.Close)
	for i := 0; i < 100000; i++ {
		h.AddVisit(fmt.Sprintf("https://site%d.example.test/Path/%d", i%5000, i), fmt.Sprintf("Page %d of Site %d", i, i%5000))
	}
	m := New(h)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Query("site42", []string{KindHistory})
	}
}
