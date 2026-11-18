package sync_test

import (
	"context"
	"strings"
	"testing"

	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/client"

	"projectb/store"
	"projectb/sync"
)

// replicaOf is a replica of the bucket at the bearer address, over the profile's store, for a
// test that drives synckosh itself.
func replicaOf(t *testing.T, p *profile, s *testServer, token string, key *synckosh.DataKey) (*client.Replica, *replicaLocal) {
	t.Helper()
	l, maps := sync.NewLocal(p.st, p.maps, key)
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
				its = append(its, synckosh.Item{Collection: m.Collection, Datatype: m.Datatype, GUID: l.GUID(m, k), Private: private})
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
	if err := ra.Delete("bookmarks", id); err != nil {
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
	handOver(t, a, ra, la)
	syncNow(t, ra)

	s.reset()
	if n := len(s.items("tok")); n != 0 {
		t.Fatalf("the server kept %d items", n)
	}
	syncNow(t, ra)
	if n := len(s.items("tok")); n != 2 {
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
