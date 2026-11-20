package suggest

import (
	"fmt"
	"strings"
	"testing"

	"projectb/engine"
	"projectb/history"
	"projectb/store"
)

// hist is a history in a store of its own; its answers come on the store's worker.
func hist(t testing.TB) *history.Manager {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return history.New(dir, st, func(f func()) { f() })
}

// query is Query, waiting for its answer, with the default number of rows.
func query(m *Manager, w engine.WindowID, text string, kinds []string) []Row {
	return queryRows(m, w, text, kinds, 0)
}

// queryRows is Query for a shell with room for most rows.
func queryRows(m *Manager, w engine.WindowID, text string, kinds []string, most int) []Row {
	got := make(chan []Row, 1)
	m.Query(w, text, kinds, most, func(rows []Row) { got <- rows })
	return <-got
}

func manager(t testing.TB, visits ...string) *Manager {
	h := hist(t)
	for _, v := range visits {
		u, title, _ := strings.Cut(v, " ")
		h.AddVisit(u, title)
	}
	m := New(h)
	m.SetSources(Sources{
		Tabs: func(engine.WindowID) []Tab {
			return []Tab{{ID: 7, Title: "Mail inbox", URL: "https://mail.test/", Detail: "Work"}, {ID: 8, Title: "", URL: ""}}
		},
		Actions: func() []Action {
			return []Action{{ID: "tab.new", Title: "New Tab", Detail: "⌘T"}, {ID: "app.settings", Title: "Settings…"}, {ID: "view.tabsTop", Title: "Tabs on Top"}}
		},
		Engine: func() (string, func(string) string) {
			return "Find", func(w string) string { return "https://find.test/?q=" + strings.ReplaceAll(w, " ", "+") }
		},
		Bookmarks: func() []Bookmark { return marks },
		Closed:    func() []Closed { return closed },
	})
	return m
}

var all = []string{KindAddress, KindSearch, KindTab, KindHistory, KindAction, KindBookmark}

// marks are the bookmarks the tests' manager has, the newest first; closed is the tabs
// closed lately, the newest first.
var (
	marks  []Bookmark
	closed []Closed
)

func kinds(rows []Row) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Kind)
	}
	return strings.Join(out, " ")
}

func TestWordsSearchFirstThenTabsHistoryAndActions(t *testing.T) {
	marks = nil
	m := manager(t, "https://mail.test/ Mail", "https://news.test/mail-digest Digest")
	rows := query(m, 0, "news", all)
	if kinds(rows) != "address history search" || rows[0].URL != "https://news.test/" || rows[0].Inline != ".test" {
		t.Fatalf("a visited host the text starts is completed first, the search last: %+v", rows)
	}
	rows = query(m, 0, "inbox", all)
	if kinds(rows) != "search tab" {
		t.Fatalf("kinds = %q: %+v", kinds(rows), rows)
	}
	if rows[0].URL != "https://find.test/?q=inbox" || rows[0].Detail != "Search Find" {
		t.Fatalf("search row = %+v", rows[0])
	}
	if rows[1].Tab != 7 || rows[1].Detail != "Work" {
		t.Fatalf("tab row = %+v", rows[1])
	}
	// The page open in a tab is listed once, as the tab.
	rows = query(m, 0, "mail-", all)
	if kinds(rows) != "search history" || rows[1].URL != "https://news.test/mail-digest" || rows[1].Detail != "news.test" {
		t.Fatalf("history row = %+v", rows)
	}
	rows = query(m, 0, "Mail", []string{KindSearch, KindTab, KindHistory})
	if kinds(rows) != "tab history search" || rows[0].Tab != 7 || rows[0].Inline != ".test" || rows[1].URL != "https://news.test/mail-digest" {
		t.Fatalf("a completed page open in a tab is the tab's row, listed once: %+v", rows)
	}
	if rows := query(m, 0, "Mail", []string{KindSearch, KindHistory}); kinds(rows) != "history history search" || rows[0].URL != "https://mail.test/" {
		t.Fatalf("with no tab rows, the page's own: %+v", rows)
	}

	rows = query(m, 0, "tab", all)
	if kinds(rows) != "search action action" || rows[1].Action != "view.tabsTop" || rows[2].Action != "tab.new" || rows[2].Detail != "⌘T" {
		t.Fatalf("actions: a title that starts with the text first: %+v", rows)
	}
}

func TestActionsAskedForAloneFillTheList(t *testing.T) {
	m := manager(t)
	if rows := query(m, 0, "t", []string{KindAction}); kinds(rows) != "action action action" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestAnAddressGoesFirstAndTheSearchLast(t *testing.T) {
	m := manager(t, "https://mail.test/ Mail")
	rows := query(m, 0, "mail.test", all)
	if kinds(rows) != "address tab search" || rows[0].URL != "https://mail.test" {
		t.Fatalf("rows = %+v", rows)
	}
}

// Typed, the bookmarks whose title or address holds the text join the pages from history by
// their visits, the newest first among those with as many, past what a tab already shows. A
// page from history that is bookmarked is one row, a bookmark's.
func TestTypedBookmarks(t *testing.T) {
	marks = []Bookmark{
		{Title: "Recipes", URL: "https://food.test/recipes"},
		{Title: "Mail inbox", URL: "https://mail.test/"},
		{Title: "Old recipes", URL: "https://food.test/old"},
		{Title: "Recipe index", URL: "https://cook.test/"},
		{Title: "More recipes", URL: "https://more.test/"},
		{Title: "Seen", URL: "https://seen.test/recipes"},
	}
	defer func() { marks = nil }()
	m := manager(t, "https://seen.test/recipes Seen")
	rows := query(m, 0, "recipe", all)
	if kinds(rows) != "search bookmark bookmark bookmark bookmark bookmark" || rows[1].URL != "https://seen.test/recipes" || rows[2].URL != "https://food.test/recipes" || rows[3].Title != "Old recipes" || rows[4].Detail != "cook.test" {
		t.Fatalf("rows = %+v", rows)
	}
	// The bookmark open in a tab is listed once, as the tab.
	if rows := query(m, 0, "inbox", all); kinds(rows) != "search tab" {
		t.Fatalf("rows = %+v", rows)
	}
	// Not asked for, none.
	if rows := query(m, 0, "recipe", []string{KindSearch, KindHistory}); kinds(rows) != "search history" {
		t.Fatalf("rows = %+v", rows)
	}
}

// Inline completion: a visited or bookmarked address the text starts, without its scheme
// and www., goes first, with the rest of it in Inline.
func TestInlineCompletion(t *testing.T) {
	marks = nil
	defer func() { marks = nil }()
	m := manager(t,
		"https://www.wikipedia.org/ Wikipedia",
		"https://en.wikipedia.org/wiki/Web_browser Web browser",
		"https://en.wikipedia.org/wiki/Web_browser Web browser",
		"https://docs.test/Guide/Start Start",
		"https://docs.test/Guide/Start Start",
		"http://plain.test/a/b B",
	)
	first := func(text string, kinds []string) Row {
		t.Helper()
		rows := query(m, 0, text, kinds)
		if len(rows) == 0 {
			t.Fatalf("%q: no rows", text)
		}
		return rows[0]
	}

	// Inside the host, to the host alone: its own page when it was visited.
	if r := first("wikip", all); r.Kind != KindHistory || r.URL != "https://www.wikipedia.org/" || r.Inline != "edia.org" || r.Title != "Wikipedia" {
		t.Fatalf("wikip = %+v", r)
	}
	// The host's own page not visited: an address row for the host, in the scheme the
	// page had.
	if r := first("en.wik", all); r.Kind != KindAddress || r.URL != "https://en.wikipedia.org/" || r.Inline != "ipedia.org" || r.Title != "en.wikipedia.org" {
		t.Fatalf("en.wik = %+v", r)
	}
	if r := first("pla", all); r.Kind != KindAddress || r.URL != "http://plain.test/" || r.Inline != "in.test" {
		t.Fatalf("pla = %+v", r)
	}
	// A shell that draws no address rows gets the page itself, completed whole.
	if r := first("en.wik", []string{KindSearch, KindHistory}); r.Kind != KindHistory || r.Inline != "ipedia.org/wiki/Web_browser" {
		t.Fatalf("en.wik, no address rows = %+v", r)
	}
	// Past the host, the whole address, in its own case.
	if r := first("docs.test/gu", all); r.URL != "https://docs.test/Guide/Start" || r.Inline != "ide/Start" {
		t.Fatalf("docs.test/gu = %+v", r)
	}
	// A typed scheme or www. is kept: the rest follows what was typed.
	if r := first("https://www.wiki", all); r.URL != "https://www.wikipedia.org/" || r.Inline != "pedia.org" {
		t.Fatalf("https://www.wiki = %+v", r)
	}
	if r := first("WIKIP", all); r.Inline != "edia.org" {
		t.Fatalf("WIKIP = %+v", r)
	}
	// Nothing to add, a space, or a text inside an address but not at its start: none.
	for _, text := range []string{"wikipedia.org", "wiki pedia", "wikip ", "pedia", "https://"} {
		for _, r := range query(m, 0, text, all) {
			if r.Inline != "" {
				t.Fatalf("%q completes: %+v", text, r)
			}
		}
	}
	// The completed page isn't listed again from history.
	rows := query(m, 0, "wikip", all)
	for _, r := range rows[1:] {
		if r.URL == "https://www.wikipedia.org/" {
			t.Fatalf("listed twice: %+v", rows)
		}
	}
	// A bookmark completes too, when history has nothing; Enter's row comes second.
	marks = []Bookmark{{Title: "Notes", URL: "https://notes.test/today"}, {Title: "Not web", URL: "javascript:void(0)"}}
	if rows := query(m, 0, "notes.test/t", all); kinds(rows) != "bookmark address search" || rows[0].Inline != "oday" {
		t.Fatalf("notes.test/t = %+v", rows)
	}
	if r := first("java", all); r.Inline != "" {
		t.Fatalf("only web addresses complete: %+v", r)
	}
	// History asked for alone, as a shell that names no kinds is, never completes.
	for _, r := range query(m, 0, "wikip", nil) {
		if r.Inline != "" {
			t.Fatalf("no kinds: %+v", r)
		}
	}
}

func TestNothingTypedListsTheSitesVisitedMost(t *testing.T) {
	marks = nil
	m := manager(t, "https://a.test/ A", "https://a.test/ A", "https://b.test/x B", "https://b.test/y B", "https://b.test/y B")
	rows := query(m, 0, "", all)
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
	if rows := query(m, 0, "", all); kinds(rows) != "history history history history history bookmark bookmark bookmark" || rows[5].URL != "https://docs.test/" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows := query(m, 0, "", []string{KindBookmark}); len(rows) != 5 || rows[0].Title != "Site 0" {
		t.Fatalf("bookmarks alone = %+v", rows)
	}
	marks = marks[:2]
	if rows := query(m, 0, "", all); kinds(rows) != "history history history history history history history bookmark" {
		t.Fatalf("one bookmark to show = %+v", rows)
	}
	marks = nil
	m = manager(t)
	if rows := query(m, 0, "", all); rows == nil || len(rows) != 0 {
		t.Fatalf("nothing at all = %#v", rows)
	}
}

// With nothing typed, a shell that asks for closed tabs, as a phone's new tab page does,
// gets the tabs closed lately, the newest first, three at most, ahead of the sites and
// bookmarks it asks for, eight rows in all and each address once.
func TestNothingTypedOffersClosedTabs(t *testing.T) {
	var visits []string
	for i := range 9 {
		visits = append(visits, fmt.Sprintf("https://site%d.test/ Site %d", i, i))
	}
	marks = []Bookmark{{Title: "Docs", URL: "https://docs.test/"}}
	closed = []Closed{{Title: "Gone", URL: "https://gone.test/a"}, {URL: "https://bare.test/x"}, {Title: "Site", URL: "https://site0.test/"}, {Title: "Older", URL: "https://older.test/"}}
	defer func() { marks, closed = nil, nil }()
	m := manager(t, visits...)

	rows := query(m, 0, "", []string{KindClosed, KindHistory, KindBookmark})
	// The sites keep their five of the rows left, so the bookmark has none.
	if kinds(rows) != "closed closed closed history history history history history" {
		t.Fatalf("rows = %+v", rows)
	}
	// One with no title shows its address; a closed site isn't a site row again.
	if rows[0].URL != "https://gone.test/a" || rows[0].Detail != "gone.test" || rows[1].Title != "https://bare.test/x" || rows[2].URL != "https://site0.test/" || rows[3].URL == "https://site0.test/" {
		t.Fatalf("closed = %+v", rows)
	}
	if rows := query(m, 0, "", []string{KindClosed}); kinds(rows) != "closed closed closed" {
		t.Fatalf("closed alone = %+v", rows)
	}
	// Asked for tabs with nothing typed, none: no shell lists them before a letter.
	if rows := query(m, 0, "", []string{KindTab, KindHistory}); strings.Contains(kinds(rows), "tab") {
		t.Fatalf("tabs = %+v", rows)
	}
	// Typed, closed tabs are never rows.
	if rows := query(m, 0, "gone", []string{KindSearch, KindClosed}); kinds(rows) != "search" {
		t.Fatalf("typed = %+v", rows)
	}
}

// A shell that names no kinds gets history alone, as before there were any.
func TestNoKindsIsHistoryAlone(t *testing.T) {
	m := manager(t, "https://mail.test/ Mail")
	rows := query(m, 0, "mail", nil)
	if len(rows) != 1 || rows[0].Kind != KindHistory || rows[0].URL != "https://mail.test/" || rows[0].Title != "Mail" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows := query(m, 0, "", nil); len(rows) != 0 {
		t.Fatalf("nothing typed: %+v", rows)
	}
}

// A key in the field over a long history, 100,000 addresses: each tier one query with a
// LIMIT, through an index.
func BenchmarkAKeyOver100000Visits(b *testing.B) {
	h := hist(b)
	pages := make([]history.Entry, 100000)
	for i := range pages {
		pages[i] = history.Entry{URL: fmt.Sprintf("https://site%d.example.test/Path/%d", i%5000, i), Title: fmt.Sprintf("Page %d of Site %d", i, i%5000), Visits: 1 + i%13, LastVisit: int64(1700000000 + i)}
	}
	h.Import(pages)
	m := New(h)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query(m, 0, "site42", []string{KindHistory})
	}
}

// An address reached from the address field comes first for the text it starts, ahead of
// pages visited more, and completes; every row of a list for typed text is marked typed, so
// choosing one counts as typing it, and none of the list for nothing typed is.
func TestTypedAddressesComeFirstAndComplete(t *testing.T) {
	marks = nil
	m := manager(t, "https://gist.test/ Gist", "https://gist.test/ Gist", "https://gist.test/ Gist")
	m.hist.AddTypedVisit("https://www.github.test/", "GitHub")
	rows := query(m, 0, "g", all)
	if rows[0].URL != "https://www.github.test/" || rows[0].Inline != "ithub.test" || kinds(rows) != "history history search" || rows[1].URL != "https://gist.test/" {
		t.Fatalf("g = %+v", rows)
	}
	for _, r := range rows {
		if !r.Typed {
			t.Fatalf("not marked typed: %+v", r)
		}
	}
	for _, r := range query(m, 0, "", all) {
		if r.Typed {
			t.Fatalf("nothing typed, marked typed: %+v", r)
		}
	}
}

// Under three letters a row matches only where its host or a word of its title starts with
// the text, in history, the open tabs and the bookmarks alike; from three on, anywhere.
func TestShortTextMatchesStartsOnly(t *testing.T) {
	marks = []Bookmark{{Title: "Kitchen", URL: "https://food.test/am"}, {Title: "Big maps", URL: "https://atlas.test/"}}
	defer func() { marks = nil }()
	m := manager(t, "https://maps.test/ Maps", "https://news.test/am Gamma", "https://shop.test/ Summer sale")
	urls := func(rows []Row) string {
		var out []string
		for _, r := range rows {
			if r.Kind != KindSearch {
				out = append(out, r.URL)
			}
		}
		return strings.Join(out, " ")
	}
	// "ma": the host maps.test, the open tab's title "Mail inbox", the bookmark "Big maps";
	// not news.test/am "Gamma" or the Kitchen at food.test/am, which only hold it.
	if got := urls(query(m, 0, "ma", all)); got != "https://maps.test/ https://mail.test/ https://atlas.test/" {
		t.Fatalf("ma: %s", got)
	}
	if got := urls(query(m, 0, "s", []string{KindSearch, KindHistory})); got != "https://shop.test/" {
		t.Fatalf("s, a title's word: %s", got)
	}
	// Three letters match anywhere.
	if got := urls(query(m, 0, "amm", []string{KindSearch, KindHistory})); got != "https://news.test/am" {
		t.Fatalf("amm: %s", got)
	}
}

// Pages rank by their visits, a typed visit counting three, a bookmark ahead of a page with as
// many; an address shows once, as an open tab, then a bookmark, then a page from history.
func TestRankedByVisitsTypedCountingMore(t *testing.T) {
	marks = []Bookmark{{Title: "Mango", URL: "https://mango.test/"}, {Title: "Mail", URL: "https://mail.test/"}}
	defer func() { marks = nil }()
	m := manager(t,
		"https://maps.test/ Maps", "https://maps.test/ Maps", "https://maps.test/ Maps",
		"https://mango.test/ Mango", "https://mango.test/ Mango",
		"https://mail.test/ Mail",
	)
	m.hist.AddVisit("https://mint.test/", "Mint")
	m.hist.AddVisit("https://mint.test/", "Mint")
	// One typed visit: 1 + 3 = 4 beside the 3 clicks of maps.test.
	m.hist.AddTypedVisit("https://moss.test/", "Moss")
	rows := query(m, 0, "m ", []string{KindSearch, KindTab, KindHistory, KindBookmark})
	var got []string
	for _, r := range rows {
		got = append(got, r.Kind+":"+r.URL)
	}
	want := "search:https://find.test/?q=m+ tab:https://mail.test/ history:https://moss.test/ history:https://maps.test/ bookmark:https://mango.test/ history:https://mint.test/"
	if g := strings.Join(got, " "); g != want {
		t.Fatalf("got  %s\nwant %s", g, want)
	}
}

// The caps: Enter's row and five more under three letters, eight rows from three, or as many
// as the shell has room for; two open tabs at most; the search keeps its place at the end;
// up to two actions come on top.
func TestCaps(t *testing.T) {
	marks = nil
	var visits []string
	for i := range 12 {
		visits = append(visits, fmt.Sprintf("https://site%02d.test/ Site %d", i, i))
	}
	m := manager(t, visits...)
	two := []Tab{{ID: 1, Title: "Site A", URL: "https://a.test/"}, {ID: 2, Title: "Site B", URL: "https://b.test/"}, {ID: 3, Title: "Site C", URL: "https://c.test/"}}
	m.src.Tabs = func(engine.WindowID) []Tab { return two }
	m.src.Actions = func() []Action {
		return []Action{{ID: "a1", Title: "Site one"}, {ID: "a2", Title: "Site two"}, {ID: "a3", Title: "Site three"}}
	}
	count := func(rows []Row, kind string) int {
		n := 0
		for _, r := range rows {
			if r.Kind == kind {
				n++
			}
		}
		return n
	}
	words := []string{KindSearch, KindTab, KindHistory}
	if rows := query(m, 0, "si ", words); len(rows) != 6 || count(rows, KindTab) != 2 || rows[0].Kind != KindSearch {
		t.Fatalf("under three letters, six rows and two tabs: %s", kinds(rows))
	}
	if rows := query(m, 0, "site", words); len(rows) != 8 || rows[0].Kind != KindHistory || rows[7].Kind != KindSearch {
		t.Fatalf("from three, eight, the search last: %s", kinds(rows))
	}
	if rows := queryRows(m, 0, "si", words, 5); len(rows) != 5 || rows[4].Kind != KindSearch {
		t.Fatalf("a phone's five: %s", kinds(rows))
	}
	if rows := queryRows(m, 0, "site", words, 6); len(rows) != 6 || rows[5].Kind != KindSearch {
		t.Fatalf("a phone's six: %s", kinds(rows))
	}
	rows := query(m, 0, "site", append(words, KindAction))
	if len(rows) != 10 || count(rows, KindAction) != 2 || rows[9].Kind != KindSearch {
		t.Fatalf("two actions on top of eight: %s", kinds(rows))
	}
}
