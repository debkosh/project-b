package sync_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/debkosh/synckosh"
	"github.com/debkosh/synckosh/client"
	"projectb/bookmarks"
	"projectb/history"
	"projectb/store"
	"projectb/sync"
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
		if it.Collection == "history" {
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

// A row that can't be sent is set aside, with why, and everything else syncs: synckosh takes a
// batch whole or not at all, and one row it refuses mustn't hold the rest back, run after
// run. The status counts it, Sync details names it in the queue and in the run's row, sync's
// own runs leave it be, a run a person asks for tries it again, and the row changed again
// goes like any other.
func TestARowThatCantBeSentIsSetAside(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	if err := a.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	a.settle()
	// A bookmark too big for one item, beside two that aren't, and a page.
	a.main(func() {
		a.bookmarks.Add("Before", "https://example.com/before")
	})
	if err := a.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO bookmarks (id, parent, kind, title, url, added, edited, place) VALUES ('big one', '', 'bookmark', 'Stuck one', 'https://example.com/' || hex(zeroblob(1500000)), 1, 1, 'z')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	a.main(func() {
		a.bookmarks.Add("After", "https://example.com/after")
	})
	a.history.AddVisit("https://example.com/page", "A page")
	a.settle()
	if err := a.sync.RunAs(sync.WhyChange); err != nil {
		t.Fatalf("the run with a row that can't go: %v", err)
	}
	a.settle()
	if st := a.sync.Status(); st.Error != "" || st.Unsent != 1 || st.LastSynced == 0 {
		t.Fatalf("the status: %+v", st)
	}
	d := a.details()
	r := d.Runs[0]
	if len(r.Unsent) != 1 || r.Unsent[0].Collection != bookmarks.Collection || r.Unsent[0].Name != "Stuck one" || !strings.Contains(r.Unsent[0].Error, "over the item limit") ||
		r.Sent[bookmarks.Collection] != 2 || r.Sent[history.Collection] != 1 || r.Error != "" {
		t.Fatalf("the run's row: %+v", r)
	}
	stuck := 0
	for _, w := range d.Waiting {
		if w.Error != "" {
			stuck++
			if w.Name != "Stuck one" || !strings.Contains(w.Error, "over the item limit") {
				t.Fatalf("the queue's stuck row: %+v", w)
			}
		}
	}
	if stuck != 1 || len(d.Waiting) != 1 {
		t.Fatalf("the queue: %+v", d.Waiting)
	}
	// The other device has all but it.
	if err := b.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	b.settle()
	if got := b.bookmarked(); !slices.Contains(got, "Before") || !slices.Contains(got, "After") || slices.Contains(got, "Stuck one") {
		t.Fatalf("the other device's bookmarks: %v", got)
	}
	if _, visits := b.page("https://example.com/page"); visits != 1 {
		t.Fatal("the page didn't reach the other device")
	}

	// Sync's own runs leave it aside: nothing is marked for them, and one says nothing of it.
	if n := a.count(`SELECT COUNT(*) FROM sync_changes WHERE stuck = ''`); n != 0 {
		t.Fatalf("%d marks a run of sync's own would take", n)
	}
	if err := a.sync.RunAs(sync.WhyTimer); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if r := a.details().Runs[0]; len(r.Unsent) != 0 {
		t.Fatalf("a run of sync's own tried it again: %+v", r)
	}
	if st := a.sync.Status(); st.Unsent != 1 {
		t.Fatalf("the status after it: %+v", st)
	}
	// A run a person asks for tries it again, and says so again.
	if err := a.sync.RunNow(); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if r := a.details().Runs[0]; len(r.Unsent) != 1 || r.Unsent[0].Name != "Stuck one" {
		t.Fatalf("a run a person asked for: %+v", r)
	}
	// Its row gone, there is nothing left to send or to say.
	if err := a.st.Wait(func(tx *store.Tx) error {
		_, err := tx.Exec(`DELETE FROM bookmarks WHERE id = 'big one'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.sync.RunAs(sync.WhyChange); err != nil {
		t.Fatal(err)
	}
	a.settle()
	if st := a.sync.Status(); st.Unsent != 0 {
		t.Fatalf("the status once the row is gone: %+v", st)
	}
	if n := a.count(`SELECT COUNT(*) FROM sync_changes`); n != 0 {
		t.Fatalf("%d marks left", n)
	}
}

// An item the server refuses is named, with the server's reason, and the run still counts:
// everything else it sent and received stands.
func TestAnItemTheServerRefusesIsNamed(t *testing.T) {
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	p.settle()
	// The server takes smaller items than the device thinks it does.
	s.mu.Lock()
	s.srv.Limits.ItemBytes = 2000
	s.mu.Unlock()
	p.main(func() {
		p.bookmarks.Add("Small", "https://example.com/small")
		p.bookmarks.Add("Too big for this server", "https://example.com/"+strings.Repeat("x", 3000))
	})
	p.settle()
	if err := p.sync.RunAs(sync.WhyChange); err != nil {
		t.Fatalf("the run with an item the server refuses: %v", err)
	}
	p.settle()
	st := p.sync.Status()
	r := p.details().Runs[0]
	if st.Error != "" || st.Unsent != 1 || len(r.Unsent) != 1 || r.Unsent[0].Name != "Too big for this server" || r.Unsent[0].Error == "" || r.Sent[bookmarks.Collection] != 1 {
		t.Fatalf("the status %+v, the run %+v", st, r)
	}
	n := 0
	for _, it := range s.items("tok") {
		if it.Collection == bookmarks.Collection {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("the server holds %d bookmarks, want the small one", n)
	}
}
