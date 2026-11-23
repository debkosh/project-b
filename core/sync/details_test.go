package sync_test

import (
	"strings"
	"testing"

	"projectb/bookmarks"
	"projectb/history"
	"projectb/sync"
)

func kind(d sync.Details, collection string) sync.Kind {
	for _, k := range d.Kinds {
		if k.Collection == collection {
			return k
		}
	}
	return sync.Kind{}
}

func names(items []sync.Moved) string {
	var out []string
	for _, it := range items {
		switch {
		case it.All:
			out = append(out, it.Collection+": all")
		case it.Deleted:
			out = append(out, it.Collection+": deleted")
		default:
			out = append(out, it.Collection+": "+it.Name)
		}
	}
	return strings.Join(out, ", ")
}

// Sync details: the counts and the queue come from the bookkeeping, a run says what it
// received and sent, by name, and why it ran, and a delete counts as a deletion kept.
func TestDetailsCountsQueueAndRuns(t *testing.T) {
	const token = "k3v9x2m7q8rd"
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	var gone string
	a.main(func() {
		a.bookmarks.Add("The Go site", "https://go.dev/")
		e, _ := a.bookmarks.Add("Gone soon", "https://example.com/gone")
		gone = e.ID
	})
	a.history.AddVisit("https://example.com/", "Example Domain")
	a.settle()
	if err := a.connect(s.single(token), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	d := a.details()
	if d.Scheme != "https://" || !strings.HasPrefix(s.ts.URL, d.Scheme+d.Host) || d.Rest != "/•••" {
		t.Fatalf("the server, given with no token, its path left out: %q %q %q", d.Scheme, d.Host, d.Rest)
	}
	if k := kind(d, bookmarks.Collection); k.Items != 2 || k.Deletions != 0 || k.Waiting != 0 {
		t.Fatalf("a's bookmarks after its first run: %+v", k)
	}
	if k := kind(d, history.Collection); k.Items != 1 || k.Waiting != 0 {
		t.Fatalf("a's history after its first run: %+v", k)
	}
	if len(d.Runs) != 1 || d.Runs[0].Why != sync.WhyOn || d.Runs[0].Sent[bookmarks.Collection] != 2 || d.Runs[0].Sent[history.Collection] != 1 || len(d.Runs[0].Received) != 0 {
		t.Fatalf("a's first run: %+v", d.Runs)
	}
	if got := names(d.Runs[0].SentItems); !strings.Contains(got, "bookmarks: The Go site") || !strings.Contains(got, "history: https://example.com/") {
		t.Fatalf("what a's first run sent, by name: %s", got)
	}
	if d.Cursor == 0 || d.Next == 0 || d.Running != nil || len(d.Waiting) != 0 {
		t.Fatalf("the cursor, the next run, none going on, nothing waiting: %+v", d)
	}
	r := d.Runs[0]
	if r.RequestCount < 2 || len(r.Requests) != r.RequestCount || r.Out == 0 || r.In == 0 {
		t.Fatalf("the run's requests: %+v", r)
	}
	for _, q := range r.Requests {
		if q.Path != "pull" && q.Path != "push" || q.Status != 200 || strings.Contains(q.Path, token) {
			t.Fatalf("a request: %+v", q)
		}
	}

	// b joins and receives them.
	if err := b.connect(s.single(token), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	d = b.details()
	if r := d.Runs[0]; r.Received[bookmarks.Collection] != 2 || r.Received[history.Collection] != 1 || len(r.Sent) != 0 {
		t.Fatalf("b's first run: %+v", r)
	}
	if got := names(d.Runs[0].ReceivedItems); !strings.Contains(got, "bookmarks: Gone soon") {
		t.Fatalf("what b's first run received, by name: %s", got)
	}

	// A change and a delete on a wait in its queue, named, until they go out.
	a.main(func() {
		a.bookmarks.Add("Waiting", "https://example.com/waiting")
		a.bookmarks.Remove(gone)
	})
	a.settle()
	d = a.details()
	if k := kind(d, bookmarks.Collection); k.Waiting != 2 {
		t.Fatalf("a's bookmarks with two waiting: %+v", k)
	}
	if got := names(d.Waiting); got != "bookmarks: Waiting, bookmarks: deleted" && got != "bookmarks: deleted, bookmarks: Waiting" {
		t.Fatalf("a's queue: %s", got)
	}
	run(t, a)
	d = a.details()
	if k := kind(d, bookmarks.Collection); k.Items != 2 || k.Deletions != 1 || k.Waiting != 0 || len(d.Waiting) != 0 {
		t.Fatalf("a's bookmarks after they went: %+v, %v", k, d.Waiting)
	}
	if r := d.Runs[0]; r.Why != sync.WhyAsked || r.Sent[bookmarks.Collection] != 2 || r.Times != 1 {
		t.Fatalf("the run that sent them: %+v", r)
	}
	run(t, b)
	d = b.details()
	if got := names(d.Runs[0].ReceivedItems); got != "bookmarks: Waiting, bookmarks: deleted" && got != "bookmarks: deleted, bookmarks: Waiting" {
		t.Fatalf("what b received: %s", got)
	}
}

// Runs sync starts itself that move nothing and end alike share one row; one a person asked
// for always has its own, as does one that ends another way. A failed run's row has the error
// with no token in it.
func TestDetailsRunsThatEndAlikeShareARow(t *testing.T) {
	const token = "k3v9x2m7q8rd"
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single(token), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	as := func(why string, well bool) {
		t.Helper()
		if err := p.sync.RunAs(why); (err == nil) != well {
			t.Fatalf("a run for %s: %v", why, err)
		}
		p.settle()
	}
	for i := 0; i < 3; i++ {
		as(sync.WhyTimer, true)
	}
	d := p.details()
	if len(d.Runs) != 2 || d.Runs[0].Times != 3 || d.Runs[0].Why != sync.WhyTimer || d.Runs[0].Began > d.Runs[0].Last || d.Runs[1].Why != sync.WhyOn {
		t.Fatalf("three quiet runs on the timer after the first: %+v", d.Runs)
	}
	as(sync.WhyAsked, true)
	as(sync.WhyAsked, true)
	if d = p.details(); len(d.Runs) != 4 || d.Runs[0].Times != 1 || d.Runs[1].Times != 1 || d.Runs[0].Why != sync.WhyAsked {
		t.Fatalf("two runs a person asked for, a row each: %+v", d.Runs)
	}
	s.refuse(token)
	as(sync.WhyTimer, false)
	as(sync.WhyTimer, false)
	d = p.details()
	if len(d.Runs) != 5 || d.Runs[0].Times != 2 || d.Runs[0].Problem != "token" || d.Runs[0].Error == "" {
		t.Fatalf("two refused runs on the timer: %+v", d.Runs)
	}
	if r := d.Runs[0]; strings.Contains(r.Error, token) || len(r.Requests) == 0 || r.Requests[0].Status != 401 && r.Requests[0].Status != 403 {
		t.Fatalf("the refused run's error and request: %+v", r)
	}
	if d.Next == 0 {
		t.Fatal("no next run after a failed one")
	}
	as(sync.WhyAsked, false)
	if d = p.details(); len(d.Runs) != 6 || d.Runs[0].Times != 1 {
		t.Fatalf("a refused run a person asked for, a row of its own: %+v", d.Runs)
	}
}

// Nothing is stored for the page: a new manager over the same store starts with no runs, and
// turning sync off forgets them.
func TestDetailsKeepNothing(t *testing.T) {
	s := newServer(t)
	dir := t.TempDir()
	p := openProfile(t, dir)
	p.startSync(s)
	if err := p.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if d := p.details(); len(d.Runs) != 1 {
		t.Fatalf("the runs before closing: %+v", d.Runs)
	}
	tables := p.count(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name LIKE 'sync_%'`)
	if tables != 4 {
		t.Fatalf("sync's tables: %d, want the four it had", tables)
	}
	p.close()
	p = openProfile(t, dir)
	p.startSync(s)
	p.settle()
	d := p.details()
	if len(d.Runs) != 1 || d.Runs[0].Why != sync.WhyStart {
		t.Fatalf("the runs after starting again: only the start's: %+v", d.Runs)
	}
	if err := p.call(p.sync.Disconnect); err != nil {
		t.Fatal(err)
	}
	if d := p.details(); len(d.Runs) != 0 || d.Host != "" || len(d.Kinds) != 0 {
		t.Fatalf("the details with sync off: %+v", d)
	}
}
