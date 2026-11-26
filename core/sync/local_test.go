package sync_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/debkosh/synckosh"
	"github.com/debkosh/synckosh/client"

	"projectb/store"
	"projectb/sync"
)

// replicaOf is a replica of the bucket at the bearer address, over the profile's store, for a
// test that drives synckosh itself.
func replicaOf(t *testing.T, p *profile, s *testServer, token string, key *synckosh.DataKey) (*client.Replica, *replicaLocal) {
	t.Helper()
	l, maps := sync.NewLocal(p.st, p.maps)
	r, err := client.Open(sync.ReplicaConfig(&client.Client{URL: s.bearer(), Token: token, HTTP: s.ts.Client()}, l, key, maps))
	if err != nil {
		t.Fatal(err)
	}
	return r, &replicaLocal{l, maps}
}

type replicaLocal struct {
	*sync.Local
	maps map[string]*sync.Mapping
}

// handOver puts every row of the profile's synced tables into the replica, as turning sync on
// does.
func handOver(t *testing.T, p *profile, r *client.Replica, l *replicaLocal) {
	t.Helper()
	var its []synckosh.Item
	for _, m := range l.maps {
		err := p.st.Wait(func(tx *store.Tx) error {
			rows, err := tx.Query(`SELECT ` + m.Key + ` FROM ` + m.Table)
			if err != nil {
				return err
			}
			var keys []string
			for rows.Next() {
				var k string
				rows.Scan(&k)
				keys = append(keys, k)
			}
			rows.Close()
			for _, k := range keys {
				private, _, err := m.Read(tx, k)
				if err != nil {
					return err
				}
				guid, err := sync.GuidOf(tx, m, k)
				if err != nil {
					return err
				}
				its = append(its, synckosh.Item{Collection: m.Collection, Datatype: m.Datatype, GUID: guid, Private: private})
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Put(its...); err != nil {
		t.Fatal(err)
	}
}

func syncNow(t *testing.T, r *client.Replica) {
	t.Helper()
	if err := r.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Rows go through synckosh as items read from them, and come out as rows on another device;
// the bookkeeping holds none of their content.
func TestRowsSyncThroughTheReplica(t *testing.T) {
	s := newServer(t)
	key, _ := synckosh.NewDataKey()
	a, b := newProfile(t), newProfile(t)
	a.main(func() { a.bookmarks.Add("The Go site", "https://go.dev/") })
	a.history.AddTypedVisit("https://example.com/", "Example Domain")
	a.history.AddVisit("https://example.com/", "")
	a.settle()

	ra, la := replicaOf(t, a, s, "tok", key)
	handOver(t, a, ra, la)
	syncNow(t, ra)
	rb, _ := replicaOf(t, b, s, "tok", key)
	syncNow(t, rb)

	if title, visits := b.page("https://example.com/"); title != "Example Domain" || visits != 2 {
		t.Fatalf("history on b: %q, %d visits", title, visits)
	}
	if n := b.count(`SELECT count(*) FROM typed WHERE url = 'https://example.com/' AND count = 1`); n != 1 {
		t.Fatalf("typed on b: %d", n)
	}
	if n := b.count(`SELECT count(*) FROM bookmarks WHERE title = 'The Go site' AND url = 'https://go.dev/'`); n != 1 {
		t.Fatalf("bookmarks on b: %d", n)
	}
	if n := b.count(`SELECT count(*) FROM visits`); n != 0 {
		t.Fatalf("b has %d visits", n)
	}
	for _, p := range []*profile{a, b} {
		p.st.Wait(func(tx *store.Tx) error {
			rows, _ := tx.Query(`SELECT item FROM sync_items`)
			defer rows.Close()
			for rows.Next() {
				var item string
				rows.Scan(&item)
				if strings.Contains(item, "example.com") || strings.Contains(item, "Go site") {
					t.Errorf("the bookkeeping holds a row's content: %s", item)
				}
			}
			return nil
		})
	}
}

// A delete still reaches synckosh after the feature deleted its row, and takes the row on the
// other device.
func TestADeleteReachesSynckoshAfterItsRowIsGone(t *testing.T) {
	s := newServer(t)
	key, _ := synckosh.NewDataKey()
	a, b := newProfile(t), newProfile(t)
	var id string
	a.main(func() {
		e, _ := a.bookmarks.Add("Gone soon", "https://example.org/")
		id = e.ID
	})
	a.settle()
	ra, la := replicaOf(t, a, s, "tok", key)
	handOver(t, a, ra, la)
	syncNow(t, ra)
	rb, _ := replicaOf(t, b, s, "tok", key)
	syncNow(t, rb)
	if n := b.count(`SELECT count(*) FROM bookmarks`); n != 1 {
		t.Fatalf("b has %d bookmarks", n)
	}

	a.main(func() { a.bookmarks.Remove(id) })
	a.settle()
	// The item the row was: sync_items remembers its guid by the row's key.
	var guid string
	if err := a.st.Wait(func(tx *store.Tx) (err error) {
		guid, err = sync.GuidOf(tx, la.maps["bookmarks"], id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := ra.Delete("bookmarks", guid); err != nil {
		t.Fatal(err)
	}
	syncNow(t, ra)
	syncNow(t, rb)
	if n := b.count(`SELECT count(*) FROM bookmarks`); n != 0 {
		t.Fatalf("b still has %d bookmarks", n)
	}
}

// A server that lost everything is filled again from the rows.
func TestAServerResetIsRefilledFromTheRows(t *testing.T) {
	s := newServer(t)
	key, _ := synckosh.NewDataKey()
	a := newProfile(t)
	a.history.AddVisit("https://example.com/kept", "Kept")
	a.main(func() { a.bookmarks.Add("Kept too", "https://example.net/") })
	a.settle()
	ra, la := replicaOf(t, a, s, "tok", key)
	// The bucket's sealed copy of the key: a device refills only a bucket that still has it.
	copy, err := key.SealedCopy("pass phrase", "Passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ra.Put(copy); err != nil {
		t.Fatal(err)
	}
	handOver(t, a, ra, la)
	syncNow(t, ra)

	s.loseAllButTheKey("tok")
	if n := len(s.items("tok")); n != 1 {
		t.Fatalf("the server kept %d items, want the key's copy alone", n)
	}
	syncNow(t, ra)
	if n := len(s.items("tok")); n != 3 {
		t.Fatalf("the server has %d items after the refill", n)
	}
	c := newProfile(t)
	rc, _ := replicaOf(t, c, s, "tok", key)
	syncNow(t, rc)
	if title, _ := c.page("https://example.com/kept"); title != "Kept" {
		t.Fatalf("c's history: %q", title)
	}
	if n := c.count(`SELECT count(*) FROM bookmarks WHERE title = 'Kept too'`); n != 1 {
		t.Fatalf("c's bookmarks: %d", n)
	}
}

// An address deleted on one device and visited again on another is a new item, under a guid
// of its own: the old item's delete, whenever it arrives, leaves the row that is the new
// item's, and the new item's own delete takes it.
func TestADeleteOfAnOldItemLeavesARowMadeAgain(t *testing.T) {
	p := newProfile(t)
	l, maps := sync.NewLocal(p.st, p.maps)
	m := maps["history"]
	// As history's Read gives a row, so the row isn't taken to hold something newer.
	private := json.RawMessage(`{"url":"https://example.com/","title":"Kept","visits":1,"last":5}`)
	old := synckosh.Item{Collection: m.Collection, Datatype: m.Datatype, GUID: synckosh.NewGUID(), Timestamp: 1, Sequence: 1, Private: private}
	again := old
	again.GUID, again.Timestamp, again.Sequence = synckosh.NewGUID(), 3, 3
	gone := synckosh.Item{Collection: m.Collection, Datatype: m.Datatype, GUID: old.GUID, Timestamp: 2, Sequence: 2, Deleted: true}
	for _, it := range []synckosh.Item{old, again, gone} {
		if err := l.Put(client.Entry{Item: it}); err != nil {
			t.Fatal(err)
		}
	}
	if title, _ := p.page("https://example.com/"); title != "Kept" {
		t.Fatal("the old item's delete took the row that is the new item's")
	}
	// The row goes out under the live item's guid.
	if err := p.st.Wait(func(tx *store.Tx) error {
		guid, err := sync.GuidOf(tx, m, "https://example.com/")
		if err == nil && guid != again.GUID {
			t.Errorf("the row goes out as %s, want the live item %s", guid, again.GUID)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	again.Deleted, again.Private, again.Sequence = true, nil, 4
	if err := l.Put(client.Entry{Item: again}); err != nil {
		t.Fatal(err)
	}
	if n := p.count(`SELECT count(*) FROM history`); n != 0 {
		t.Fatal("the item's own delete left its row")
	}
}

// Synckosh has a device take the item another device sent first for a row both have: it
// stores that one, then removes this device's. The row is then the taken item's, and isn't
// marked to go out again.
func TestTakingAnotherDevicesItemForARow(t *testing.T) {
	p := newProfile(t)
	l, maps := sync.NewLocal(p.st, p.maps)
	m := maps["history"]
	p.history.AddVisit("https://example.com/", "Here")
	p.settle()
	private, mine := json.RawMessage(nil), synckosh.NewGUID()
	if err := p.st.Wait(func(tx *store.Tx) (err error) {
		if _, err := tx.Exec(`DELETE FROM sync_changes`); err != nil {
			return err
		}
		private, _, err = m.Read(tx, "https://example.com/")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := l.Put(client.Entry{Item: synckosh.Item{Collection: m.Collection, Datatype: m.Datatype, GUID: mine, Timestamp: 1, Private: private}, Pending: true}); err != nil {
		t.Fatal(err)
	}
	theirs := synckosh.Item{Collection: m.Collection, Datatype: m.Datatype, GUID: synckosh.NewGUID(), Timestamp: 2, Sequence: 7, Private: private}
	if err := l.Put(client.Entry{Item: theirs}); err != nil {
		t.Fatal(err)
	}
	if err := l.Remove(m.Collection, mine); err != nil {
		t.Fatal(err)
	}
	if n := p.count(`SELECT count(*) FROM sync_changes`); n != 0 {
		t.Fatalf("the row is marked to go out again: %d marks", n)
	}
	if n := p.count(`SELECT count(*) FROM sync_items WHERE collection = 'history' AND key = 'https://example.com/' AND guid = ?`, theirs.GUID); n != 1 {
		t.Fatal("the row isn't the taken item's")
	}
	if title, _ := p.page("https://example.com/"); title != "Here" {
		t.Fatal("the row went")
	}
	// Forgetting the one item a row is, as a device starting over does, marks it to go again.
	if err := l.Remove(m.Collection, theirs.GUID); err != nil {
		t.Fatal(err)
	}
	if n := p.count(`SELECT count(*) FROM sync_changes WHERE key = 'https://example.com/'`); n != 1 {
		t.Fatalf("a forgotten item's row has %d marks, want 1", n)
	}
}
