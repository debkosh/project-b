package sync_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"
	"projectb/timeline"
)

// newKey starts a bucket on the server, as Connect does, and returns its key.
func newKey(t *testing.T, s *testServer, token string) *synckosh.DataKey {
	t.Helper()
	key, err := client.Create(context.Background(), &client.Client{URL: s.bearer(), Token: token, HTTP: s.ts.Client()}, "a passphrase", "Passphrase")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// Rows are marked in their own write while sync is on, and only then; a run hands them to
// synckosh, and the other device's rows and tree have them.
func TestChangesAreMarkedAndRun(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	sa, sb := a.startSync(s), b.startSync(s)
	a.history.AddVisit("https://example.com/before", "Before")
	a.settle()
	if n := a.count(`SELECT count(*) FROM sync_changes`); n != 0 {
		t.Fatalf("%d marks with sync off", n)
	}
	key := newKey(t, s, "tok")
	if err := sa.Begin(s.bearer(), "tok", key); err != nil {
		t.Fatal(err)
	}
	if err := sb.Begin(s.bearer(), "tok", key); err != nil {
		t.Fatal(err)
	}
	if title, _ := b.page("https://example.com/before"); title != "Before" {
		t.Fatalf("a row from before sync was on: %q", title)
	}

	a.main(func() { a.bookmarks.Add("After", "https://example.com/after") })
	a.history.AddTypedVisit("https://example.com/after", "After")
	a.settle()
	if n := a.count(`SELECT count(*) FROM sync_changes`); n != 3 {
		t.Fatalf("%d marks for a bookmark, a page and a typed address", n)
	}
	if err := sa.RunNow(); err != nil {
		t.Fatal(err)
	}
	if n := a.count(`SELECT count(*) FROM sync_changes`); n != 0 {
		t.Fatalf("%d marks left after a run", n)
	}
	if err := sb.RunNow(); err != nil {
		t.Fatal(err)
	}
	b.settle()
	if got := b.bookmarked(); !slices.Equal(got, []string{"After"}) {
		t.Fatalf("b's tree: %v", got)
	}
	if n := b.count(`SELECT count(*) FROM typed WHERE url = 'https://example.com/after'`); n != 1 {
		t.Fatal("b has no typed address")
	}
	// What sync wrote on b marked nothing to go back.
	if n := b.count(`SELECT count(*) FROM sync_changes`); n != 0 {
		t.Fatalf("b marked %d of what it took", n)
	}
	if st := sb.Status(); !st.On || st.Syncing || st.Error != "" || st.LastSynced == 0 || st.Host != strings.TrimPrefix(s.ts.URL, "https://") {
		t.Fatalf("status %+v", st)
	}
}

// Clearing all history goes out as one delete of everything, and clears the other device's;
// clearing a range deletes those pages there.
func TestClearingHistoryReachesTheOtherDevice(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	sa, sb := a.startSync(s), b.startSync(s)
	key := newKey(t, s, "tok")
	sa.Begin(s.bearer(), "tok", key)
	sb.Begin(s.bearer(), "tok", key)
	for _, u := range []string{"https://example.com/1", "https://example.com/2", "https://example.com/3"} {
		a.history.AddTypedVisit(u, "")
	}
	a.settle()
	sa.RunNow()
	sb.RunNow()
	if n := b.count(`SELECT count(*) FROM history`); n != 3 {
		t.Fatalf("b has %d pages", n)
	}

	a.history.Delete("https://example.com/3")
	a.settle()
	sa.RunNow()
	sb.RunNow()
	if n := b.count(`SELECT count(*) FROM history`); n != 2 {
		t.Fatalf("b has %d pages after one was deleted", n)
	}

	a.history.Clear(0)
	a.settle()
	if n := a.count(`SELECT count(*) FROM sync_changes`); n != 2 {
		t.Fatalf("%d marks for clearing all, want one each for history and typed", n)
	}
	sa.RunNow()
	sb.RunNow()
	if n := b.count(`SELECT count(*) FROM history`) + b.count(`SELECT count(*) FROM typed`); n != 0 {
		t.Fatalf("b kept %d pages and typed addresses", n)
	}
	for _, it := range s.items("tok") {
		if it.Collection == "history" && it.GUID != synckosh.DeleteBeforeItem {
			t.Fatalf("the server kept %+v", it)
		}
	}
	// A page visited after the clear syncs as usual.
	a.history.AddVisit("https://example.com/later", "Later")
	a.settle()
	sa.RunNow()
	sb.RunNow()
	if title, _ := b.page("https://example.com/later"); title != "Later" {
		t.Fatalf("a page after the clear: %q", title)
	}
}

// A row changed again while its first change still waits to go out is written all the same:
// a page visited twice between two runs counts both visits, and a bookmark edited twice
// keeps the second edit. An upsert's update runs its triggers with its own way of meeting a
// conflict, which overrides the mark's, so the mark can't rely on one.
func TestARowChangedTwiceBeforeARunKeepsBoth(t *testing.T) {
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	p.settle()
	_, seen := timeline.Since(0)
	var id string
	p.main(func() {
		e, _ := p.bookmarks.Add("First", "https://example.com/mark")
		id = e.ID
		p.bookmarks.Edit(id, "Second", "https://example.com/mark")
		p.bookmarks.Edit(id, "Third", "https://example.com/mark")
	})
	for range 3 {
		p.history.AddTypedVisit("https://example.com/twice", "Twice")
	}
	p.settle()
	if _, visits := p.page("https://example.com/twice"); visits != 3 {
		t.Fatalf("three visits before a run counted %d", visits)
	}
	if n := p.count(`SELECT count FROM typed WHERE url = ?`, "https://example.com/twice"); n != 3 {
		t.Fatalf("typed three times before a run counted %d", n)
	}
	if n := p.count(`SELECT COUNT(*) FROM bookmarks WHERE id = ? AND title = 'Third'`, id); n != 1 {
		t.Fatal("the bookmark's second edit before a run was lost")
	}
	logged, _ := timeline.Since(seen)
	for _, e := range logged {
		if strings.Contains(e.Text, "constraint") {
			t.Fatalf("the store refused a write: %q", e.Text)
		}
	}
	if err := p.sync.RunNow(); err != nil {
		t.Fatal(err)
	}
	if n := p.count(`SELECT COUNT(*) FROM sync_changes`); n != 0 {
		t.Fatalf("%d marks left after the run", n)
	}
}
