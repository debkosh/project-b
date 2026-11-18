package history

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"projectb/store"
)

// newManager is a history in a store of its own, in dir. Its answers come on the worker,
// which the helpers below wait for.
func newManager(t testing.TB, dir string) *Manager {
	m := managerIn(t, dir)
	t.Cleanup(m.st.Close)
	return m
}

// managerIn is newManager whose store the test closes, to start again on the same folder.
func managerIn(t testing.TB, dir string) *Manager {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return New(dir, st, func(f func()) { f() })
}

func wait[T any](ask func(func(T))) T {
	got := make(chan T, 1)
	ask(func(v T) { got <- v })
	return <-got
}

func find(m *Manager, text string, limit int) []Entry {
	return wait(func(f func([]Entry)) { m.Search(text, limit, f) })
}

func list(m *Manager, text string, before int64, limit int) []Entry {
	return wait(func(f func([]Entry)) { m.List(text, before, limit, f) })
}

func topSites(m *Manager, limit int) []Entry {
	return wait(func(f func([]Entry)) { m.TopSites(limit, f) })
}

func urls(es []Entry) string {
	var out []string
	for _, e := range es {
		out = append(out, e.URL)
	}
	return strings.Join(out, " ")
}

// rows is how many rows a table has.
func rows(t *testing.T, m *Manager, table string) int {
	n := 0
	if err := m.st.Wait(func(tx *store.Tx) error { return tx.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestOnlyWebPagesAreRecorded(t *testing.T) {
	m := newManager(t, t.TempDir())
	m.AddVisit("about:blank", "")
	m.AddVisit("chrome://shell/", "")
	m.AddVisit("https://example.com/", "")
	if got := find(m, "e", 10); len(got) != 1 || got[0].URL != "https://example.com/" {
		t.Fatalf("got %v", got)
	}
}

func TestHostPrefixRanksAboveTitleMatch(t *testing.T) {
	m := newManager(t, t.TempDir())
	m.AddVisit("https://notes.test/", "docs about go")
	m.AddVisit("https://www.go.dev/", "")
	got := find(m, "go", 10)
	if len(got) != 2 || got[0].URL != "https://www.go.dev/" {
		t.Fatalf("got %v", got)
	}
}

func TestVisitsAddUp(t *testing.T) {
	m := newManager(t, t.TempDir())
	m.AddVisit("https://example.com/", "")
	m.AddVisit("https://example.com/", "Example")
	m.AddVisit("https://example.com/", "")
	got := find(m, "example", 10)
	if len(got) != 1 || got[0].Visits != 3 || got[0].Title != "Example" {
		t.Fatalf("got %v", got)
	}
	if n := rows(t, m, "visits"); n != 3 {
		t.Fatalf("%d visits kept", n)
	}
}

// A title and an address are compared lowercased as Go lowercases them, past ASCII too.
func TestNonASCIITitleAndAddress(t *testing.T) {
	m := newManager(t, t.TempDir())
	m.AddVisit("https://ÜBER.test/", "")
	m.AddVisit("https://notes.test/", "ÄRGER und Über")
	if got := urls(find(m, "über", 10)); got != "https://ÜBER.test/ https://notes.test/" {
		t.Fatalf("über: %s", got)
	}
	if got := urls(find(m, "ärger", 10)); got != "https://notes.test/" {
		t.Fatalf("ärger: %s", got)
	}
	if got := urls(find(m, "är", 10)); got != "https://notes.test/" {
		t.Fatalf("är: %s", got)
	}
}

// The history page's list is the latest first, searchable, and pages through by time; what
// is deleted or cleared stays gone after a restart.
func TestListDeleteAndClearOutliveARestart(t *testing.T) {
	dir := t.TempDir()
	m := managerIn(t, dir)
	for i, u := range []string{"https://a.test/", "https://b.test/news", "https://c.test/"} {
		m.Import([]Entry{{URL: u, Title: "Page " + u, Visits: 1, LastVisit: int64(100 + i)}})
	}
	if got := urls(list(m, "", 0, 10)); got != "https://c.test/ https://b.test/news https://a.test/" {
		t.Fatalf("list = %s", got)
	}
	if got := urls(list(m, "NEWS", 0, 10)); got != "https://b.test/news" {
		t.Fatalf("search = %s", got)
	}
	if got := urls(list(m, "", 102, 10)); got != "https://b.test/news https://a.test/" {
		t.Fatalf("before = %s", got)
	}
	if n := wait(func(f func(int)) { m.Count(102, f) }); n != 1 {
		t.Fatalf("count = %d", n)
	}
	m.Delete("https://b.test/news")
	m.Clear(102)
	m.st.Close()

	again := newManager(t, dir)
	if got := urls(list(again, "", 0, 10)); got != "https://a.test/" {
		t.Fatalf("after a restart: %s", got)
	}
}

// The sites visited most: a site's visits add up across its pages, www. or not, and the
// page it is listed by is the one visited most.
func TestTopSites(t *testing.T) {
	m := newManager(t, t.TempDir())
	for _, v := range []string{
		"https://mail.test/inbox", "https://mail.test/inbox", "https://www.mail.test/sent",
		"https://news.test/", "https://news.test/", "https://news.test/",
		"https://one.test/",
	} {
		m.AddVisit(v, "")
	}
	if got := urls(topSites(m, 2)); got != "https://mail.test/inbox https://news.test/" {
		t.Fatalf("top sites = %s", got)
	}
	// Its page taken out, a site is listed by the page left.
	m.Delete("https://mail.test/inbox")
	if got := urls(topSites(m, 3)); got != "https://news.test/ https://one.test/ https://www.mail.test/sent" {
		t.Fatalf("after a delete = %s", got)
	}
}

// Forgetting a site takes every page of it out, www. or not, and no other.
func TestDeleteSite(t *testing.T) {
	m := newManager(t, t.TempDir())
	for _, v := range []string{"https://mail.test/inbox", "https://www.mail.test/sent", "https://mail.test.other/", "https://news.test/"} {
		m.AddVisit(v, "")
	}
	m.DeleteSite("https://mail.test/inbox")
	if got := urls(topSites(m, 5)); got != "https://mail.test.other/ https://news.test/" {
		t.Fatalf("left = %s", got)
	}
}

// Clearing history takes its pages out of every table: the visits, the sites and the
// typed addresses go with them.
func TestClearTakesEveryTable(t *testing.T) {
	m := newManager(t, t.TempDir())
	m.AddTypedVisit("https://mail.test/", "Mail")
	m.AddVisit("https://news.test/a", "News")
	m.AddVisit("https://news.test/b", "News")
	if rows(t, m, "history") != 3 || rows(t, m, "visits") != 3 || rows(t, m, "sites") != 2 || rows(t, m, "typed") != 1 {
		t.Fatal("the visits were not all kept")
	}
	m.Clear(0)
	for _, table := range []string{"history", "visits", "sites", "typed", "history_tri"} {
		if n := rows(t, m, table); n != 0 {
			t.Fatalf("%s has %d rows left", table, n)
		}
	}
	if got := find(m, "mail", 10); len(got) != 0 {
		t.Fatalf("still suggested: %v", got)
	}
}

// Pages from another browser join the history and outlive a restart; made again, an
// import counts nothing twice.
func TestImport(t *testing.T) {
	dir := t.TempDir()
	m := managerIn(t, dir)
	m.AddVisit("https://a.test/", "A")
	pages := []Entry{
		{URL: "https://a.test/", Title: "A there", Visits: 5, LastVisit: 100},
		{URL: "https://b.test/", Title: "B", Visits: 2, LastVisit: 200},
		{URL: "chrome://settings/", Title: "Settings", Visits: 9, LastVisit: 300},
	}
	m.Import(pages)
	m.Import(pages)
	m.st.Close()
	again := newManager(t, dir)
	got := list(again, "", 0, 10)
	if len(got) != 2 || got[1].URL != "https://b.test/" || got[1].Visits != 2 || got[1].LastVisit != 200 {
		t.Fatalf("after a restart: %+v", got)
	}
	if got[0].URL != "https://a.test/" || got[0].Visits != 5 || got[0].Title != "A" {
		t.Fatalf("a page here already: %+v", got[0])
	}
	if s := urls(topSites(again, 5)); s != "https://a.test/ https://b.test/" {
		t.Fatalf("top sites = %s", s)
	}
}

// The first start with the store reads history.jsonl once, the last line for an address
// winning and a removed one gone, and leaves the file as it was.
func TestHistoryJSONLIsReadOnce(t *testing.T) {
	dir := t.TempDir()
	old := `{"url":"https://a.test/","title":"A","visits":1,"lastVisit":100}
{"url":"https://b.test/","title":"B","visits":4,"lastVisit":150}
{"url":"https://a.test/","title":"A again","visits":2,"lastVisit":300}
{"url":"https://b.test/","removed":true}
{"url":"https://c.test/","title":"C","visits":1,"lastVisit":200}
{"url":"https://www.c.test/x","title":"C x","visits":3,"lastVisit":50}
not a line
`
	path := filepath.Join(dir, "history.jsonl")
	os.WriteFile(path, []byte(old), 0o600)
	m := managerIn(t, dir)
	got := list(m, "", 0, 10)
	if urls(got) != "https://a.test/ https://c.test/ https://www.c.test/x" || got[0].Title != "A again" || got[0].Visits != 2 || got[0].LastVisit != 300 {
		t.Fatalf("read: %+v", got)
	}
	if s := urls(find(m, "again", 10)); s != "https://a.test/" {
		t.Fatalf("its title isn't searched: %s", s)
	}
	if s := urls(topSites(m, 5)); s != "https://www.c.test/x https://a.test/" {
		t.Fatalf("its sites: %s", s)
	}
	// And they are kept in step from then on.
	m.AddVisit("https://a.test/", "")
	m.AddVisit("https://a.test/", "")
	if s := urls(topSites(m, 5)); s != "https://a.test/ https://www.c.test/x" {
		t.Fatalf("its sites after two visits: %s", s)
	}
	m.st.Close()
	if b, _ := os.ReadFile(path); string(b) != old {
		t.Fatal("history.jsonl changed")
	}
	os.WriteFile(path, []byte(`{"url":"https://d.test/","title":"D","visits":1,"lastVisit":400}`+"\n"), 0o600)
	again := newManager(t, dir)
	if s := urls(list(again, "", 0, 10)); s != "https://a.test/ https://c.test/ https://www.c.test/x" {
		t.Fatalf("read again: %s", s)
	}
}

// Typed addresses that start with the text come first, the most typed first, then history
// as always, each address once.
func TestTypedAddressesFirst(t *testing.T) {
	m := newManager(t, t.TempDir())
	m.Import([]Entry{
		{URL: "https://gist.test/", Title: "Gist", Visits: 40, LastVisit: 500},
		{URL: "https://notes.test/g", Title: "Going places", Visits: 50, LastVisit: 600},
	})
	m.AddTypedVisit("https://www.github.test/", "GitHub")
	m.AddTypedVisit("https://gitlab.test/", "GitLab")
	m.AddTypedVisit("https://gitlab.test/", "GitLab")
	// Typed, but not an address the text starts.
	m.AddTypedVisit("https://mail.test/g", "Mail")
	if got := urls(find(m, "g", 10)); got != "https://gitlab.test/ https://www.github.test/ https://gist.test/ https://notes.test/g https://mail.test/g" {
		t.Fatalf("g: %s", got)
	}
	if got := urls(find(m, "g", 1)); got != "https://gitlab.test/" {
		t.Fatalf("one row: %s", got)
	}
	// A scheme typed is matched as part of the address, as before.
	if got := urls(find(m, "https://www.git", 10)); got != "https://www.github.test/" {
		t.Fatalf("with a scheme: %s", got)
	}
	if got := urls(find(m, "github", 10)); got != "https://www.github.test/" {
		t.Fatalf("github: %s", got)
	}
}

// today is the search before the store, in memory: score every entry, best first.
func today(es []Entry, text string, limit int) []Entry {
	q := strings.ToLower(strings.TrimSpace(text))
	type hit struct {
		e     Entry
		score int
	}
	var hits []hit
	for _, e := range es {
		low, title := strings.ToLower(e.URL), strings.ToLower(e.Title)
		base := 0
		switch {
		case strings.HasPrefix(bareURL(low), q):
			base = 1000
		case strings.Contains(low, q):
			base = 100
		case strings.Contains(title, q):
			base = 50
		default:
			continue
		}
		hits = append(hits, hit{e, base + min(e.Visits, 50)*2})
	}
	slices.SortFunc(hits, func(a, b hit) int {
		switch {
		case a.score != b.score:
			return b.score - a.score
		case a.e.LastVisit != b.e.LastVisit:
			return int(b.e.LastVisit - a.e.LastVisit)
		}
		return strings.Compare(a.e.URL, b.e.URL)
	})
	out := []Entry{}
	for _, h := range hits[:min(limit, len(hits))] {
		out = append(out, h.e)
	}
	return out
}

// The history tiers give what the search in memory gave, in its order, for texts of every
// length, few matches and many, every path the search takes.
func TestTiersMatchTheSearchBefore(t *testing.T) {
	m := newManager(t, t.TempDir())
	r := rand.New(rand.NewSource(7))
	words := []string{"mail", "news", "maps", "docs", "shop", "blog", "wiki", "über", "café", "git", "go", "notes", "tea", "trip"}
	hosts := []string{"mail.test", "www.news.test", "maps.example", "docs.example.org", "shop.test", "blog.test", "wiki.test", "café.test", "gitea.test", "github.test", "go.dev", "travel.test"}
	var es []Entry
	seen := map[string]bool{}
	for len(es) < 3000 {
		u := fmt.Sprintf("%s://%s/%s/%d", []string{"https", "http"}[r.Intn(2)], hosts[r.Intn(len(hosts))], words[r.Intn(len(words))], r.Intn(400))
		if seen[u] {
			continue
		}
		seen[u] = true
		title := strings.ToUpper(words[r.Intn(len(words))][:1]) + words[r.Intn(len(words))] + " and " + words[r.Intn(len(words))]
		es = append(es, Entry{URL: u, Title: title, Visits: 1 + r.Intn(80), LastVisit: int64(1000 + r.Intn(500))})
	}
	m.Import(es)
	texts := []string{"m", "g", "z", "é", "ma", "gi", "go", "ne", "qx", "mai", "news", "git", "tea", "über", "café", "doc", "wik", "trip", "test", "example", "mail.test/n", "news.test/go", "/1", "and m", "and git", "gitea and", "s and t", "ips", "http", "www.n"}
	for _, text := range texts {
		for _, limit := range []int{1, 8} {
			if got, want := urls(find(m, text, limit)), urls(today(es, text, limit)); got != want {
				t.Errorf("%q, %d:\n got %s\nwant %s", text, limit, got, want)
			}
		}
	}
}

// A store.db that is no database goes aside, and history.jsonl, still in the profile, is read
// into the new one.
func TestHistoryJSONLIsReadIntoANewStore(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte(`{"url":"https://a.test/","title":"A","visits":1,"lastVisit":100}`+"\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "store.db"), []byte(strings.Repeat("garbage ", 1000)), 0o600)
	m := newManager(t, dir)
	if got := urls(list(m, "", 0, 10)); got != "https://a.test/" {
		t.Fatalf("read: %s", got)
	}
}
