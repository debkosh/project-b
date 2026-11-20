package sync_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sudiptadeb/synckosh"

	"projectb/sync"
	"projectb/timeline"
)

// inspect is whether the bucket is new, as the page asks before the passphrase.
func inspect(t *testing.T, p *profile, address, token string) bool {
	t.Helper()
	type answer struct {
		isNew bool
		err   error
	}
	got := make(chan answer, 1)
	p.main(func() { p.sync.Inspect(address, token, func(isNew bool, err error) { got <- answer{isNew, err} }) })
	a := <-got
	if a.err != nil {
		t.Fatal(a.err)
	}
	return a.isNew
}

func run(t *testing.T, ps ...*profile) {
	t.Helper()
	for _, p := range ps {
		if err := p.sync.RunNow(); err != nil {
			t.Fatal(err)
		}
		p.settle()
	}
}

// Two profiles sync bookmarks, history and typed addresses, through a bucket at either of its
// addresses: one starts it, the other joins it with its passphrase.
func TestTwoProfilesSync(t *testing.T) {
	for _, form := range []string{"bearer", "single"} {
		t.Run(form, func(t *testing.T) {
			s := newServer(t)
			address, token := s.bearer(), "tok"
			if form == "single" {
				address, token = s.single("tok"), ""
			}
			a, b := newProfile(t), newProfile(t)
			a.startSync(s)
			b.startSync(s)
			a.main(func() { a.bookmarks.Add("The Go site", "https://go.dev/") })
			a.history.AddTypedVisit("https://example.com/", "Example Domain")
			b.history.AddVisit("https://example.org/", "From b")
			a.settle()
			b.settle()

			if !inspect(t, a, address, token) {
				t.Fatal("an empty bucket isn't new")
			}
			if err := a.connect(address, token, "correct horse"); err != nil {
				t.Fatal(err)
			}
			if inspect(t, b, address, token) {
				t.Fatal("a bucket with items is new")
			}
			if err := b.connect(address, token, "correct horse"); err != nil {
				t.Fatal(err)
			}
			run(t, a)
			if got := b.bookmarked(); !slices.Equal(got, []string{"The Go site"}) {
				t.Fatalf("b's bookmarks: %v", got)
			}
			if title, visits := b.page("https://example.com/"); title != "Example Domain" || visits != 1 {
				t.Fatalf("b's history: %q, %d", title, visits)
			}
			if n := b.count(`SELECT count(*) FROM typed`); n != 1 {
				t.Fatalf("b's typed: %d", n)
			}
			if title, _ := a.page("https://example.org/"); title != "From b" {
				t.Fatalf("a's history from b: %q", title)
			}
			if st := b.sync.Status(); !st.On || st.Error != "" || !strings.HasPrefix(s.ts.URL, "https://"+st.Host) {
				t.Fatalf("b's status: %+v", st)
			}
		})
	}
}

// One address visited on both devices is one item, and one row on each with the larger count.
func TestOneAddressOnBothIsOneRowWithTheLargerCount(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	for range 3 {
		a.history.AddTypedVisit("https://example.com/", "On a")
	}
	for range 5 {
		b.history.AddTypedVisit("https://example.com/", "On b")
	}
	a.settle()
	b.settle()
	a.connect(s.bearer(), "tok", "pass phrase")
	if err := b.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	run(t, a, b, a)
	titleA, _ := a.page("https://example.com/")
	for name, p := range map[string]*profile{"a": a, "b": b} {
		if title, visits := p.page("https://example.com/"); visits != 5 || title != titleA {
			t.Fatalf("%s: %q, %d visits; a has %q", name, title, visits, titleA)
		}
		if n := p.count(`SELECT count FROM typed WHERE url = 'https://example.com/'`); n != 5 {
			t.Fatalf("%s: typed %d times", name, n)
		}
		if n := p.count(`SELECT count(*) FROM visits`); n != map[string]int{"a": 3, "b": 5}[name] {
			t.Fatalf("%s has %d visits: visits don't sync", name, n)
		}
	}
	pages := 0
	for _, it := range s.items("tok") {
		if it.Collection == "history" {
			pages++
		}
	}
	if pages != 1 {
		t.Fatalf("the server has %d history items for one address", pages)
	}
}

// A bookmark deleted on one device goes from the other.
func TestABookmarkDeletedOnOneGoesFromTheOther(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	var keep, gone string
	a.main(func() {
		e, _ := a.bookmarks.Add("Kept", "https://example.com/kept")
		keep = e.ID
		f, _ := a.bookmarks.NewFolder("", "Folder")
		e, _ = a.bookmarks.Add("Gone", "https://example.com/gone")
		a.bookmarks.Move(e.ID, f.ID)
		gone = f.ID
	})
	a.settle()
	a.connect(s.bearer(), "tok", "pass phrase")
	b.connect(s.bearer(), "tok", "pass phrase")
	if got := b.bookmarked(); len(got) != 2 {
		t.Fatalf("b's bookmarks: %v", got)
	}
	b.main(func() { b.bookmarks.Remove(gone) })
	b.settle()
	run(t, b, a)
	if got := a.bookmarked(); !slices.Equal(got, []string{"Kept"}) {
		t.Fatalf("a's bookmarks: %v", got)
	}
	if n := a.count(`SELECT count(*) FROM bookmarks`); n != 1 {
		t.Fatalf("a has %d rows", n)
	}
	var e struct{ ok bool }
	a.main(func() { _, e.ok = a.bookmarks.Get(keep) })
	if !e.ok {
		t.Fatal("the kept bookmark went")
	}
}

// The server holds no visit, nor anything readable of a page or a bookmark: their items are
// sealed, with nothing in their public part.
func TestTheServerHoldsNothingReadable(t *testing.T) {
	s := newServer(t)
	a := newProfile(t)
	a.startSync(s)
	for range 4 {
		a.history.AddVisit("https://secret.example/path", "A secret title")
	}
	a.history.AddTypedVisit("https://typed.example/", "Typed")
	a.main(func() { a.bookmarks.Add("A secret bookmark", "https://bookmarked.example/") })
	a.settle()
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, it := range s.items("tok") {
		counts[it.Collection]++
		raw, _ := json.Marshal(it)
		for _, secret := range []string{"secret", "Typed", "typed.example", "bookmarked"} {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("the server can read %q: %s", secret, raw)
			}
		}
		if it.Collection != synckosh.KeysCollection && len(it.Public) != 0 {
			t.Fatalf("a public part: %s", raw)
		}
	}
	if counts["history"] != 2 || counts["typed"] != 1 || counts["bookmarks"] != 1 || len(counts) != 4 {
		t.Fatalf("the server holds %v", counts)
	}
}

// A wrong passphrase changes nothing; the right one joins.
func TestAWrongPassphrase(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	a.connect(s.bearer(), "tok", "the right one")
	b.history.AddVisit("https://example.com/", "")
	b.settle()
	if err := b.connect(s.bearer(), "tok", "a wrong one"); !errors.Is(err, sync.ErrPassphrase) {
		t.Fatalf("connect: %v", err)
	}
	if st := b.sync.Status(); st.On {
		t.Fatalf("b's status: %+v", st)
	}
	if n := b.count(`SELECT count(*) FROM secrets`) + b.count(`SELECT count(*) FROM sync_changes`) + b.count(`SELECT marking FROM sync_state`); n != 0 {
		t.Fatalf("a wrong passphrase left %d secrets, marks or marking", n)
	}
	if err := b.connect(s.bearer(), "tok", "the right one"); err != nil {
		t.Fatal(err)
	}
	run(t, a)
	if _, visits := a.page("https://example.com/"); visits != 1 {
		t.Fatal("b didn't join")
	}
}

// A changed passphrase needs no old one; a third device joins with it, and not with the old.
func TestChangingThePassphrase(t *testing.T) {
	s := newServer(t)
	a, b, c := newProfile(t), newProfile(t), newProfile(t)
	for _, p := range []*profile{a, b, c} {
		p.startSync(s)
	}
	a.main(func() { a.bookmarks.Add("Shared", "https://example.com/") })
	a.settle()
	a.connect(s.bearer(), "tok", "old passphrase")
	b.connect(s.bearer(), "tok", "old passphrase")
	if err := b.call(func(done func(error)) { b.sync.ChangePassphrase("new passphrase", done) }); err != nil {
		t.Fatal(err)
	}
	if err := c.connect(s.bearer(), "tok", "old passphrase"); !errors.Is(err, sync.ErrPassphrase) {
		t.Fatalf("the old passphrase: %v", err)
	}
	if err := c.connect(s.bearer(), "tok", "new passphrase"); err != nil {
		t.Fatalf("the new passphrase: %v", err)
	}
	if got := c.bookmarked(); !slices.Equal(got, []string{"Shared"}) {
		t.Fatalf("c's bookmarks: %v", got)
	}
	// a, which holds the key, syncs on.
	a.history.AddVisit("https://example.com/later", "Later")
	a.settle()
	run(t, a, c)
	if title, _ := c.page("https://example.com/later"); title != "Later" {
		t.Fatalf("c from a: %q", title)
	}
}

// Disconnect stops syncing and forgets the bucket, its address too; every row stays.
func TestDisconnectKeepsTheRows(t *testing.T) {
	s := newServer(t)
	a, b := newProfile(t), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	a.history.AddVisit("https://example.com/", "Kept")
	a.main(func() { a.bookmarks.Add("Kept", "https://example.com/") })
	a.settle()
	a.connect(s.bearer(), "tok", "pass phrase")
	b.connect(s.bearer(), "tok", "pass phrase")
	if got := b.sync.Address(); got != s.bearer() {
		t.Fatalf("b's address while on: %q", got)
	}
	if err := b.call(b.sync.Disconnect); err != nil {
		t.Fatal(err)
	}
	b.settle()
	if got := b.bookmarked(); !slices.Equal(got, []string{"Kept"}) {
		t.Fatalf("b's bookmarks: %v", got)
	}
	if title, _ := b.page("https://example.com/"); title != "Kept" {
		t.Fatalf("b's history: %q", title)
	}
	if st := b.sync.Status(); st.On || st.Host != "" || b.sync.Address() != "" {
		t.Fatalf("b's status: %+v, address %q", st, b.sync.Address())
	}
	for _, table := range []string{"secrets", "sync_items", "sync_meta", "sync_changes"} {
		if n := b.count(`SELECT count(*) FROM ` + table); n != 0 {
			t.Fatalf("b's %s has %d rows", table, n)
		}
	}
	b.history.AddVisit("https://example.com/b", "")
	b.settle()
	if n := b.count(`SELECT count(*) FROM sync_changes`); n != 0 {
		t.Fatal("a change was marked after Disconnect")
	}
}

// A server that lost everything is filled again by the device, which a new one then joins.
func TestAServerResetIsRefilled(t *testing.T) {
	s := newServer(t)
	a, c := newProfile(t), newProfile(t)
	a.startSync(s)
	c.startSync(s)
	a.history.AddVisit("https://example.com/", "Kept")
	a.main(func() { a.bookmarks.Add("Kept", "https://example.com/") })
	a.settle()
	a.connect(s.bearer(), "tok", "pass phrase")
	s.reset()
	run(t, a)
	if err := c.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if got := c.bookmarked(); !slices.Equal(got, []string{"Kept"}) {
		t.Fatalf("c's bookmarks: %v", got)
	}
	if title, _ := c.page("https://example.com/"); title != "Kept" {
		t.Fatalf("c's history: %q", title)
	}
}

// A profile whose key doesn't open has sync off, saying why, and its rows as they were, until
// StartFresh; then it connects again.
func TestALockedProfileKeyAndStartingFresh(t *testing.T) {
	s := newServer(t)
	dir := t.TempDir()
	a := openProfile(t, dir)
	a.startSync(s)
	a.history.AddVisit("https://example.com/", "Kept")
	a.settle()
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	a.close()
	// The folder's key is another now, as a folder copied from another device's would be.
	key := filepath.Join(dir, "folder.key")
	os.Remove(key)
	os.WriteFile(key, []byte(strings.Repeat("k", 32)), 0o600)

	a = openProfile(t, dir)
	a.startSync(s)
	if st := a.sync.Status(); st.On || !st.Locked || st.Error == "" {
		t.Fatalf("status: %+v", st)
	}
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err == nil {
		t.Fatal("connected with its keys locked")
	}
	if title, _ := a.page("https://example.com/"); title != "Kept" {
		t.Fatalf("the history: %q", title)
	}
	if err := a.call(a.sync.StartFresh); err != nil {
		t.Fatal(err)
	}
	if st := a.sync.Status(); st.On || st.Locked {
		t.Fatalf("status after starting fresh: %+v", st)
	}
	if n := a.count(`SELECT count(*) FROM secrets`); n != 0 {
		t.Fatalf("%d secrets after starting fresh", n)
	}
	if err := a.connect(s.bearer(), "tok", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	if st := a.sync.Status(); !st.On || st.Error != "" {
		t.Fatalf("status after connecting again: %+v", st)
	}
}

// Sync on at start runs at once, with what was kept.
func TestSyncOnAtStartRuns(t *testing.T) {
	s := newServer(t)
	dir := t.TempDir()
	a, b := openProfile(t, dir), newProfile(t)
	a.startSync(s)
	b.startSync(s)
	a.connect(s.bearer(), "tok", "pass phrase")
	b.connect(s.bearer(), "tok", "pass phrase")
	a.close()
	b.history.AddVisit("https://example.com/while", "While a was away")
	b.settle()
	run(t, b)

	a = openProfile(t, dir)
	a.startSync(s)
	if err := a.sync.RunNow(); err != nil {
		t.Fatal(err)
	}
	if title, _ := a.page("https://example.com/while"); title != "While a was away" {
		t.Fatalf("a: %q", title)
	}
}

func TestProposeAndLink(t *testing.T) {
	for address, want := range map[string]sync.Proposal{
		"https://memd.example/sync":          {Address: "https://memd.example/sync", Host: "memd.example", NeedsToken: true},
		" https://memd.example/sync/ ":       {Address: "https://memd.example/sync", Host: "memd.example", NeedsToken: true},
		"https://memd.example:8443/sync/t0k": {Address: "https://memd.example:8443/sync/t0k", Host: "memd.example:8443"},
	} {
		if got, err := sync.Propose(address); err != nil || got != want {
			t.Errorf("%q: %+v, %v", address, got, err)
		}
	}
	for _, bad := range []string{"http://memd.example/sync", "https:///sync", "https://u:p@memd.example/sync", "https://memd.example/sync?t=1", "https://memd.example/sync#t", "memd.example/sync"} {
		if _, err := sync.Propose(bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
	got, err := sync.Link("project-b://sync?address=" + "https%3A%2F%2Fmemd.example%2Fsync%2Ft0k")
	if err != nil || got != "https://memd.example/sync/t0k" {
		t.Fatalf("link: %q, %v", got, err)
	}
	for _, bad := range []string{"project-b://other?address=x", "https://sync?address=x", "project-b://sync"} {
		if _, err := sync.Link(bad); err == nil {
			t.Errorf("%q was taken", bad)
		}
	}
}

// What goes wrong reaching a bucket is named for the page: an address that isn't one, one
// that needs its token, a server that can't be reached, an answer that isn't a bucket's, and a
// token the server refuses, at once and later, when the server signs a device out.
func TestTheProblemsReachingABucket(t *testing.T) {
	s := newServer(t)
	gone := httptest.NewTLSServer(http.NotFoundHandler())
	gone.Close()
	p := newProfile(t)
	p.startSync(s)
	problem := func(address, token string) string {
		got := make(chan error, 1)
		p.main(func() { p.sync.Inspect(address, token, func(_ bool, err error) { got <- err }) })
		return sync.Problem(<-got)
	}
	for _, c := range []struct{ address, token, want string }{
		{"http://memd.example/sync/tok", "", "address"},
		{s.bearer(), "", "needsToken"},
		{gone.URL + "/sync/tok", "", "unreachable"},
		{s.ts.URL + "/elsewhere/sync/tok", "", "notBucket"},
		{s.bearer(), "refused", "token"},
	} {
		s.refuse("refused")
		if got := problem(c.address, c.token); got != c.want {
			t.Errorf("%s with %q: %q, want %q", c.address, c.token, got, c.want)
		}
	}
	if err := p.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	s.refuse("tok")
	if err := p.sync.RunNow(); sync.Problem(err) != "token" {
		t.Fatalf("a run signed out: %v", err)
	}
	p.settle()
	if st := p.sync.Status(); !st.On || st.Problem != "token" || st.Error == "" {
		t.Fatalf("the status signed out: %+v", st)
	}
}

// A run that fails says what it was, in the status the page reads: a server that answers that
// it failed was reached, so it is "server" and not "unreachable"; one that gives no answer is
// "unreachable"; and a failure that is neither, as a request the server won't take, has no
// name, with its words in Error. None of them is the other.
func TestAFailedRunSaysWhatItWas(t *testing.T) {
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		status int
		want   string
	}{{http.StatusInternalServerError, "server"}, {http.StatusBadGateway, "server"}, {http.StatusBadRequest, ""}} {
		s.fail(c.status)
		err := p.sync.RunNow()
		if err == nil || sync.Problem(err) != c.want {
			t.Fatalf("a run answered %d: problem %q, want %q (%v)", c.status, sync.Problem(err), c.want, err)
		}
		p.settle()
		if st := p.sync.Status(); st.Problem != c.want || !strings.HasPrefix(st.Error, "GET v1/pull?after=") || !strings.Contains(st.Error, strconv.Itoa(c.status)) || strings.Contains(st.Error, "sync:") || st.Syncing {
			t.Fatalf("the status after %d: %+v", c.status, st)
		}
	}
	s.fail(0)
	s.ts.CloseClientConnections()
	s.ts.Close()
	if err := p.sync.RunNow(); sync.Problem(err) != "unreachable" {
		t.Fatalf("a run with no answer: %v", err)
	}
	p.settle()
	if st := p.sync.Status(); st.Problem != "unreachable" || !strings.HasPrefix(st.Error, "GET v1/pull?after=") || strings.Contains(st.Error, "couldn't reach") {
		t.Fatalf("the status with no answer: %+v", st)
	}
}

// A run stalls when nothing arrives for StalledAfter, and ends saying so; one that takes
// longer than that while its requests are answered doesn't.
func TestARunStallsOnlyWithNothingArriving(t *testing.T) {
	was := sync.StalledAfter
	sync.StalledAfter = 300 * time.Millisecond
	t.Cleanup(func() { sync.StalledAfter = was })
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	// Every request answered, each after most of the wait: longer than it in all, with a
	// page to send.
	p.history.AddVisit("https://example.org/slow", "Slow")
	s.delay(200 * time.Millisecond)
	began := time.Now()
	if err := p.sync.RunNow(); err != nil {
		t.Fatalf("a slow run whose requests are answered: %v", err)
	}
	if took := time.Since(began); took < sync.StalledAfter {
		t.Fatalf("the slow run took %s, which tests nothing", took)
	}
	s.delay(0)
	// Sync details says what the run waits for while it goes on, and what it was after.
	sync.StalledAfter = time.Second
	release := s.hold()
	began = time.Now()
	ended := make(chan error, 1)
	go func() { ended <- p.sync.RunNow() }()
	var going *sync.Run
	for going == nil || going.Asking == nil {
		if time.Since(began) > 900*time.Millisecond {
			t.Fatalf("the run going on never showed its request: %+v", going)
		}
		time.Sleep(20 * time.Millisecond)
		going = p.details().Running
	}
	if a := going.Asking; a.Path != "pull" || a.Method != "GET" || a.At == 0 || going.Why != sync.WhyAsked || going.Began == 0 {
		t.Fatalf("the run going on: %+v, asking %+v", going, a)
	}
	err := <-ended
	if sync.Problem(err) != "stalled" {
		t.Fatalf("a run with nothing arriving: %v", err)
	}
	if took := time.Since(began); took > 5*time.Second {
		t.Fatalf("it took %s to end", took)
	}
	p.settle()
	if st := p.sync.Status(); st.Problem != "stalled" || st.Syncing {
		t.Fatalf("the status: %+v", st)
	}
	d := p.details()
	if d.Running != nil || len(d.Runs) == 0 {
		t.Fatalf("after it: %+v", d)
	}
	if r := d.Runs[0]; r.Problem != "stalled" || !strings.HasPrefix(r.Error, "GET v1/pull?after=") || !strings.HasSuffix(r.Error, ": nothing arrived for 30 seconds") || r.Asking != nil ||
		len(r.Requests) != 1 || r.Requests[0].Status != 0 || r.Requests[0].Took < 900 {
		t.Fatalf("the stalled run's row: %+v", r)
	}
	release()
	if err := p.sync.RunNow(); err != nil {
		t.Fatalf("the run after: %v", err)
	}
}

// A failed run's words, in the status and in the timeline, are the request and then the cause,
// the request named from v1 on: the address before it may carry the bucket's token, and Go's
// own error names the whole address.
func TestAFailedRunsWordsLeaveTheTokenOut(t *testing.T) {
	const token = "k3v9x2m7q8rd"
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single(token), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	_, seen := timeline.Since(0)
	s.refuse(token)
	err := p.sync.RunNow()
	if err == nil || !strings.Contains(err.Error(), token) {
		t.Fatalf("Go's error of a refused run should name the whole address: %v", err)
	}
	p.settle()
	if st := p.sync.Status(); strings.Contains(st.Error, token) || !strings.HasPrefix(st.Error, "GET v1/pull?after=") || !strings.HasSuffix(st.Error, ": the server didn't accept the token") {
		t.Fatalf("the status's error: %q", st.Error)
	}
	logged, _ := timeline.Since(seen)
	said := false
	for _, e := range logged {
		if strings.Contains(e.Text, token) {
			t.Fatalf("the timeline has the token: %q", e.Text)
		}
		said = said || strings.HasPrefix(e.Text, "sync: ")
	}
	if !said {
		t.Fatal("the timeline says nothing of the failed run")
	}
}

// A run cut short ends with no error, and the next goes on as before.
func TestARunCutShort(t *testing.T) {
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	if err := p.connect(s.single("tok"), "", "pass phrase"); err != nil {
		t.Fatal(err)
	}
	before := p.sync.Status().LastSynced
	release := s.hold()
	ran := make(chan error, 1)
	go func() { ran <- p.sync.RunNow() }()
	for !p.sync.Status().Syncing {
		time.Sleep(5 * time.Millisecond)
	}
	p.sync.CancelRun()
	if err := <-ran; err == nil {
		t.Fatal("a run cut short ended well")
	}
	release()
	p.settle()
	if st := p.sync.Status(); st.Syncing || st.Error != "" || st.LastSynced != before {
		t.Fatalf("the status after a run cut short: %+v", st)
	}
	run(t, p)
	if st := p.sync.Status(); st.Error != "" || st.LastSynced == before {
		t.Fatalf("the status after the next run: %+v", st)
	}
}

// Connect answers once sync is on, before its first run, which the status then shows.
func TestConnectAnswersBeforeItsFirstRun(t *testing.T) {
	s := newServer(t)
	p := newProfile(t)
	p.startSync(s)
	// Create's look at the bucket and its sealed copy go through; the run is held.
	release := s.holdAfter(2)
	got := make(chan error, 1)
	p.main(func() { p.sync.Connect(s.single("tok"), "", "pass phrase", func(err error) { got <- err }) })
	if err := <-got; err != nil {
		t.Fatal(err)
	}
	if st := p.sync.Status(); !st.On || st.LastSynced != 0 {
		t.Fatalf("the status as Connect answered: %+v", st)
	}
	release()
	p.sync.Idle()
	p.settle()
	if st := p.sync.Status(); !st.On || st.Syncing || st.LastSynced == 0 {
		t.Fatalf("the status after the first run: %+v", st)
	}
}
