package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"projectb/engine"
	"projectb/mockengine"
)

// through runs each fetch on the mock engine's own fetch, at once, as the engine would
// later: the rules it keeps are the engine's (engine/browser/fetch.cc).
type through struct {
	m     *Manager
	asked []engine.FetchRequest
	// lie makes the engine say a fetch went through whatever it was: the manager's own
	// rules are then all that hold.
	lie bool
}

func (t *through) Fetch(f engine.FetchRequest) {
	t.asked = append(t.asked, f)
	if t.lie {
		os.WriteFile(f.Path, []byte("x"), 0o600)
		t.m.Fetched(engine.Fetched{ID: f.ID, Outcome: engine.FetchOK, Space: engine.SpaceOf(hostOf(f.URL)), URL: f.URL})
		return
	}
	t.m.Fetched(mockengine.FetchFile(context.Background(), f))
}

func (t *through) CancelFetch(engine.FetchID) {}

func hostOf(u string) string {
	r, _ := http.NewRequest("GET", u, nil)
	return r.URL.Hostname()
}

func setup(t *testing.T) (*Manager, *through, *httptest.Server, string) {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/piece.ts":
			w.Header().Set("Content-Type", "video/mp2t")
			w.Header().Set("Content-Disposition", `attachment; filename="one.ts"`)
			if r.Header.Get("Range") == "bytes=2-4" {
				w.WriteHeader(http.StatusPartialContent)
				w.Write([]byte("ece"))
				return
			}
			w.Write([]byte("piece from " + r.Header.Get("Referer")))
		case "/to-file":
			http.Redirect(w, r, "file:///etc/hosts", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(s.Close)
	th := &through{}
	m := New(th)
	th.m = m
	return m, th, s, t.TempDir()
}

func fetch(m *Manager, r engine.FetchRequest) engine.Fetched {
	var got engine.Fetched
	m.Fetch(r, func(f engine.Fetched) { got = f })
	return got
}

// A piece is fetched into its file, as from the page that plays it, and its end says where
// its server is, its type and the name the server gave it.
func TestAFetchWritesItsFileAndSaysWhereItCameFrom(t *testing.T) {
	m, th, s, dir := setup(t)
	path := filepath.Join(dir, "1")
	got := fetch(m, engine.FetchRequest{Workspace: "w1", Page: "https://video.test/watch", URL: s.URL + "/piece.ts", Path: path, From: engine.SpaceLoopback})
	b, _ := os.ReadFile(path)
	if got.Outcome != engine.FetchOK || got.Status != 200 || got.Space != engine.SpaceLoopback || got.Mime != "video/mp2t" || got.Name != "one.ts" || string(b) != "piece from https://video.test/watch" {
		t.Fatalf("%+v, file %q", got, b)
	}
	if a := th.asked[0]; a.Workspace != "w1" || a.Page != "https://video.test/watch" || a.ID != got.ID {
		t.Fatalf("asked %+v", a)
	}
	ranged := fetch(m, engine.FetchRequest{URL: s.URL + "/piece.ts", Path: path, Range: "2-4"})
	if b, _ := os.ReadFile(path); ranged.Outcome != engine.FetchOK || string(b) != "ece" {
		t.Fatalf("a range: %+v, file %q", ranged, b)
	}
	if missing := fetch(m, engine.FetchRequest{URL: s.URL + "/none", Path: filepath.Join(dir, "2")}); missing.Outcome != engine.FetchServer || missing.Status != 404 {
		t.Fatalf("a missing piece: %+v", missing)
	}
}

// Only http and https are fetched: any other address is refused before the engine hears of
// it, and so is one a server redirects to.
func TestOnlyHTTPAndHTTPSAreFetched(t *testing.T) {
	m, th, s, dir := setup(t)
	for _, u := range []string{"file:///etc/hosts", "data:text/plain,x", "ftp://files.test/a.ts", "blob:https://a.test/1", "chrome://settings/", "javascript:1", "//no-scheme.test/a", ""} {
		path := filepath.Join(dir, "f")
		if got := fetch(m, engine.FetchRequest{URL: u, Path: path}); got.Outcome != engine.FetchRefused {
			t.Errorf("%q: %+v", u, got)
		}
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%q left a file", u)
		}
	}
	if len(th.asked) != 0 {
		t.Fatalf("the engine was asked for %+v", th.asked)
	}
	path := filepath.Join(dir, "r")
	if got := fetch(m, engine.FetchRequest{URL: s.URL + "/to-file", Path: path}); got.Outcome != engine.FetchRefused {
		t.Fatalf("a redirect to a file: %+v", got)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the redirect left a file")
	}
}

// A request made from the internet, as a piece named by a playlist from a public address is,
// may not reach this computer or its network: the engine's network refuses it, and the
// manager refuses it again should the engine ever let one through. From this computer or
// its network, and for the person's own request, it goes.
func TestAPrivateAddressFromAPublicPlaylistIsRefused(t *testing.T) {
	m, th, s, dir := setup(t)
	path := filepath.Join(dir, "p")
	if got := fetch(m, engine.FetchRequest{Page: "https://video.test/watch", URL: s.URL + "/piece.ts", Path: path, From: engine.SpacePublic}); got.Outcome != engine.FetchRefused {
		t.Fatalf("this computer, from the internet: %+v", got)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the refused piece left a file")
	}
	for _, from := range []engine.AddressSpace{engine.SpaceAny, engine.SpaceLoopback, engine.SpaceLocal} {
		if got := fetch(m, engine.FetchRequest{URL: s.URL + "/piece.ts", Path: path, From: from}); got.Outcome != engine.FetchOK {
			t.Fatalf("from %d: %+v", from, got)
		}
	}
	th.lie = true
	for _, u := range []string{s.URL + "/piece.ts", "http://192.168.1.1/key", "http://10.0.0.8/a.ts", "http://[::1]/a.ts", "http://169.254.169.254/latest", "http://100.64.0.1/a"} {
		os.Remove(path)
		if got := fetch(m, engine.FetchRequest{URL: u, Path: path, From: engine.SpacePublic}); got.Outcome != engine.FetchRefused {
			t.Errorf("%s let through by the engine: %+v", u, got)
		}
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s left a file", u)
		}
	}
	if got := fetch(m, engine.FetchRequest{URL: "http://203.0.113.9/a.ts", Path: path, From: engine.SpacePublic}); got.Outcome != engine.FetchOK {
		t.Fatalf("a public piece from a public playlist: %+v", got)
	}
}

// A cancelled fetch is stopped at the engine, its file goes, and its end is told to no one.
func TestACancelledFetchTellsNothing(t *testing.T) {
	var cancelled []engine.FetchID
	e := &held{cancel: func(id engine.FetchID) { cancelled = append(cancelled, id) }}
	m := New(e)
	path := filepath.Join(t.TempDir(), "c")
	os.WriteFile(path, []byte("half"), 0o600)
	told := false
	cancel := m.Fetch(engine.FetchRequest{URL: "https://cdn.test/1.ts", Path: path}, func(engine.Fetched) { told = true })
	cancel()
	m.Fetched(engine.Fetched{ID: e.asked[0].ID, Outcome: engine.FetchOK})
	if _, err := os.Stat(path); err == nil || told || len(cancelled) != 1 || cancelled[0] != e.asked[0].ID {
		t.Fatalf("told %v, cancelled %v, file left %v", told, cancelled, err == nil)
	}
}

type held struct {
	asked  []engine.FetchRequest
	cancel func(engine.FetchID)
}

func (h *held) Fetch(f engine.FetchRequest)   { h.asked = append(h.asked, f) }
func (h *held) CancelFetch(id engine.FetchID) { h.cancel(id) }
