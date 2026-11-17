package history

import (
	"strings"
	"testing"
)

func newManager(t *testing.T) *Manager {
	t.Helper()
	m := New(t.TempDir(), func(fn func()) { fn() })
	t.Cleanup(m.Close)
	return m
}

func TestOnlyWebPagesAreRecorded(t *testing.T) {
	m := newManager(t)
	m.AddVisit("about:blank", "")
	m.AddVisit("chrome://shell/", "")
	m.AddVisit("https://example.com/", "")
	if got := m.Search("e", 10); len(got) != 1 || got[0].URL != "https://example.com/" {
		t.Fatalf("got %v", got)
	}
}

func TestHostPrefixRanksAboveTitleMatch(t *testing.T) {
	m := newManager(t)
	m.AddVisit("https://notes.test/", "docs about go")
	m.AddVisit("https://www.go.dev/", "")
	got := m.Search("go", 10)
	if len(got) != 2 || got[0].URL != "https://www.go.dev/" {
		t.Fatalf("got %v", got)
	}
}

func TestVisitsAddUp(t *testing.T) {
	m := newManager(t)
	m.AddVisit("https://example.com/", "")
	m.AddVisit("https://example.com/", "Example")
	got := m.Search("example", 10)
	if len(got) != 1 || got[0].Visits != 2 || got[0].Title != "Example" {
		t.Fatalf("got %v", got)
	}
}

// The history page's list is the latest first, searchable, and pages through by time; what
// is deleted or cleared stays gone after a restart.
func TestListDeleteAndClearOutliveARestart(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, func(f func()) { f() })
	for i, u := range []string{"https://a.test/", "https://b.test/news", "https://c.test/"} {
		m.AddVisit(u, "Page "+u)
		m.entries[u].LastVisit = int64(100 + i)
	}
	if got := m.List("", 0, 10); len(got) != 3 || got[0].URL != "https://c.test/" || got[2].URL != "https://a.test/" {
		t.Fatalf("list = %+v", got)
	}
	if got := m.List("news", 0, 10); len(got) != 1 || got[0].URL != "https://b.test/news" {
		t.Fatalf("search = %+v", got)
	}
	if got := m.List("", 102, 10); len(got) != 2 || got[0].URL != "https://b.test/news" {
		t.Fatalf("before = %+v", got)
	}
	m.Delete("https://b.test/news")
	m.Clear(102)
	m.Close()

	again := New(dir, func(f func()) { f() })
	defer again.Close()
	done := make(chan struct{})
	again.Load(func() { close(done) })
	<-done
	if got := again.List("", 0, 10); len(got) != 1 || got[0].URL != "https://a.test/" {
		t.Fatalf("after a restart: %+v", got)
	}
}

// The sites visited most: a site's visits add up across its pages, www. or not, and the
// page it is listed by is the one visited most.
func TestTopSites(t *testing.T) {
	m := newManager(t)
	for _, v := range []string{
		"https://mail.test/inbox", "https://mail.test/inbox", "https://www.mail.test/sent",
		"https://news.test/", "https://news.test/", "https://news.test/",
		"https://one.test/",
	} {
		m.AddVisit(v, "")
	}
	got := []string{}
	for _, e := range m.TopSites(2) {
		got = append(got, e.URL)
	}
	if strings.Join(got, " ") != "https://mail.test/inbox https://news.test/" {
		t.Fatalf("top sites = %v", got)
	}
}

// Forgetting a site takes every page of it out, www. or not, and no other.
func TestDeleteSite(t *testing.T) {
	m := newManager(t)
	for _, v := range []string{"https://mail.test/inbox", "https://www.mail.test/sent", "https://news.test/"} {
		m.AddVisit(v, "")
	}
	m.DeleteSite("https://mail.test/inbox")
	if got := m.TopSites(5); len(got) != 1 || got[0].URL != "https://news.test/" {
		t.Fatalf("left = %+v", got)
	}
}

// Pages from another browser join the history and outlive a restart; made again, an
// import counts nothing twice.
func TestImport(t *testing.T) {
	dir := t.TempDir()
	m := New(dir, func(f func()) { f() })
	m.AddVisit("https://a.test/", "A")
	pages := []Entry{
		{URL: "https://a.test/", Title: "A there", Visits: 5, LastVisit: 100},
		{URL: "https://b.test/", Title: "B", Visits: 2, LastVisit: 200},
		{URL: "chrome://settings/", Title: "Settings", Visits: 9, LastVisit: 300},
	}
	if n := m.Import(pages); n != 1 {
		t.Fatalf("added %d", n)
	}
	if n := m.Import(pages); n != 0 {
		t.Fatalf("again, added %d", n)
	}
	m.Close()
	again := New(dir, func(f func()) { f() })
	loaded := make(chan struct{})
	again.Load(func() { close(loaded) })
	<-loaded
	got := again.List("", 0, 10)
	if len(got) != 2 || got[1].URL != "https://b.test/" || got[1].Visits != 2 || got[1].LastVisit != 200 {
		t.Fatalf("after a restart: %+v", got)
	}
	if got[0].URL != "https://a.test/" || got[0].Visits != 5 || got[0].Title != "A" {
		t.Fatalf("a page here already: %+v", got[0])
	}
}
