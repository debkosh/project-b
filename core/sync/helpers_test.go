package sync_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"
	"time"

	"github.com/sudiptadeb/synckosh"
	"github.com/sudiptadeb/synckosh/bolt"
	"github.com/sudiptadeb/synckosh/server"

	"projectb/bookmarks"
	"projectb/history"
	"projectb/keys"
	"projectb/passkeys"
	"projectb/store"
	"projectb/sync"
)

// testServer is synckosh's server in-process, over https, as the memd server serves a bucket:
// at /sync with the token as a bearer, or at /sync/<token> alone. A token's bucket is named
// after it.
type testServer struct {
	t    *testing.T
	ts   *httptest.Server
	mu   gosync.Mutex
	srv  *server.Server
	data *bolt.Store
	n    int
	// refused are the tokens the server signs out.
	refused map[string]bool
	// held, while it is open, holds every request after the next pass.
	held chan struct{}
	pass int
	// failing, when it isn't 0, is the status every request is answered with, as synckosh's
	// server answers a failure of its own: in JSON, with the error's words.
	failing int
	// slow is how long every request waits before its answer.
	slow time.Duration
}

func newServer(t *testing.T) *testServer {
	s := &testServer{t: t, refused: map[string]bool{}}
	s.reset()
	s.ts = httptest.NewTLSServer(http.StripPrefix("/sync", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, rest, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/"); ok && token != "v1" {
			r = r.Clone(r.Context())
			r.URL.Path = "/" + rest
			r.Header.Set("Authorization", "Bearer "+token)
		}
		s.mu.Lock()
		srv, held, failing, slow := s.srv, s.held, s.failing, s.slow
		if held != nil && s.pass > 0 {
			held, s.pass = nil, s.pass-1
		}
		s.mu.Unlock()
		time.Sleep(slow)
		if failing != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(failing)
			w.Write([]byte(`{"error":"the test server fails"}`))
			return
		}
		if held != nil {
			<-held
		}
		srv.ServeHTTP(w, r)
	})))
	t.Cleanup(s.ts.Close)
	return s
}

// reset gives the server an empty store, with new store ids: it has lost every item.
func (s *testServer) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.n++
	data, err := bolt.OpenStore(filepath.Join(s.t.TempDir(), "server.db"))
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Cleanup(func() { data.Close() })
	s.data = data
	s.srv = server.New(data, server.Bearer(func(token string) (string, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.refused[token] {
			return "", errors.New("signed out")
		}
		return "bucket-" + token, nil
	}))
}

// hold holds every request until the release it returns, or the test's end.
func (s *testServer) hold() (release func()) { return s.holdAfter(0) }

// holdAfter lets the next n requests through, and holds the rest as hold does.
func (s *testServer) holdAfter(n int) (release func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := make(chan struct{})
	s.held, s.pass = held, n
	var once gosync.Once
	release = func() {
		once.Do(func() {
			s.mu.Lock()
			s.held = nil
			s.mu.Unlock()
			close(held)
		})
	}
	s.t.Cleanup(release)
	return release
}

// refuse signs a token out.
func (s *testServer) refuse(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refused[token] = true
}

// fail answers every request from now on with status, 0 for as usual.
func (s *testServer) fail(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failing = status
}

// delay answers every request from now on after d.
func (s *testServer) delay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slow = d
}

// bearer is the bucket's address without its token; single the one address with it.
func (s *testServer) bearer() string             { return s.ts.URL + "/sync" }
func (s *testServer) single(token string) string { return s.ts.URL + "/sync/" + token }

// items is every item the server holds for the token's bucket, as it holds them.
func (s *testServer) items(token string) []synckosh.Item {
	s.mu.Lock()
	data := s.data
	s.mu.Unlock()
	var out []synckosh.Item
	if err := data.Since("bucket-"+token, "", 0, func(it synckosh.Item) bool { out = append(out, it); return true }); err != nil {
		s.t.Fatal(err)
	}
	return out
}

// profile is a profile folder named outright, with its store, keys, history and bookmarks, and
// a main thread of its own.
type profile struct {
	t         *testing.T
	dir       string
	st        *store.Store
	keys      *keys.Keys
	history   *history.Manager
	bookmarks *bookmarks.Manager
	passkeys  *passkeys.Manager
	posted    chan func()
	maps      []sync.Mapping
	sync      *sync.Manager
	close     func()
}

func newProfile(t *testing.T) *profile {
	return openProfile(t, t.TempDir())
}

func openProfile(t *testing.T, dir string) *profile {
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := &profile{t: t, dir: dir, st: st, posted: make(chan func(), 1024)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for f := range p.posted {
			f()
		}
	}()
	p.keys = keys.New(dir, "", st, nil)
	p.bookmarks = bookmarks.New(dir, st)
	p.history = history.New(dir, st, p.post)
	// The device's check always passes here: core/passkeys' own tests hold it.
	p.passkeys = passkeys.New(st, p.keys, func(context.Context, passkeys.Why, string) error { return nil })
	p.maps = append(history.Synced(), p.bookmarks.Synced(p.post), p.passkeys.Synced(func() {}))
	sync.Setup(st)
	closed := false
	p.close = func() {
		if closed {
			return
		}
		closed = true
		if p.sync != nil {
			p.sync.Stop()
		}
		st.Close()
		close(p.posted)
		<-done
	}
	t.Cleanup(p.close)
	return p
}

func (p *profile) post(f func()) { p.posted <- f }

// startSync starts the profile's sync, with requests going to the test server, and the
// subscriptions history and bookmarks have in the app.
func (p *profile) startSync(s *testServer) *sync.Manager {
	defer sync.SetHTTPClient(s.ts.Client())()
	p.history.OnClearAll(sync.ClearedAll)
	p.sync = sync.New(p.st, p.keys, p.post, p.maps...)
	p.sync.Idle()
	p.settle()
	return p.sync
}

// connect connects the profile's sync, and waits for its answer and its first run.
func (p *profile) connect(address, token, passphrase string) error {
	got := make(chan error, 1)
	p.main(func() { p.sync.Connect(address, token, passphrase, func(err error) { got <- err }) })
	err := <-got
	p.sync.Idle()
	p.settle()
	return err
}

// call runs one of sync's calls that answer with an error, and waits for it.
func (p *profile) call(f func(done func(error))) error {
	got := make(chan error, 1)
	p.main(func() { f(func(err error) { got <- err }) })
	err := <-got
	p.settle()
	return err
}

// main runs f on the profile's main thread and waits for it, and for what it posted.
func (p *profile) main(f func()) {
	ran := make(chan struct{})
	p.post(func() { f(); close(ran) })
	<-ran
}

// settle waits for the store's worker and the main thread to run what was asked of them.
func (p *profile) settle() {
	for range 3 {
		p.st.Wait(func(*store.Tx) error { return nil })
		p.main(func() {})
	}
}

// count is the rows a query counts.
func (p *profile) count(query string, args ...any) int {
	n := -1
	if err := p.st.Wait(func(tx *store.Tx) error { return tx.QueryRow(query, args...).Scan(&n) }); err != nil {
		p.t.Fatal(err)
	}
	return n
}

// page is the history row of url: its title and visits, or "" and -1 for none.
func (p *profile) page(url string) (string, int) {
	title, visits := "", -1
	p.st.Wait(func(tx *store.Tx) error {
		tx.QueryRow(`SELECT title, visits FROM history WHERE url = ?`, url).Scan(&title, &visits)
		return nil
	})
	return title, visits
}

// bookmarked is the titles of the bookmarks the main thread's tree has.
func (p *profile) bookmarked() []string {
	var out []string
	p.main(func() {
		for _, e := range p.bookmarks.List() {
			out = append(out, e.Title)
		}
	})
	return out
}

func rawOf(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// details is what Sync details would show now.
func (p *profile) details() sync.Details {
	got := make(chan sync.Details, 1)
	p.sync.Details(func(d sync.Details) { got <- d })
	return <-got
}
