package downloads

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"projectb/engine"
	"projectb/fetch"
	"projectb/mockengine"
	"projectb/mocksystem"
)

// streamRig is a manager whose streams are fetched from a server of core/streams' test
// streams, through core/fetch on the mock engine's own fetch, with the mock engine's loop
// as the main thread.
type streamRig struct {
	t       *testing.T
	e       *mockengine.Engine
	m       *Manager
	fetch   *fetch.Manager
	server  *httptest.Server
	profile string
	dir     string

	mu    sync.Mutex
	asked []string
	// slow holds the pieces back, each one this much longer than the one before it in the
	// playlist, so they come in one at a time, in order.
	slow  time.Duration
	extra map[string]string
	live  map[engine.FetchID]context.CancelFunc
	asks  []engine.FetchRequest
}

func newStreamRig(t *testing.T) *streamRig {
	t.Helper()
	r := &streamRig{t: t, e: mockengine.New(), profile: t.TempDir(), dir: t.TempDir(), extra: map[string]string{}, live: map[engine.FetchID]context.CancelFunc{}}
	files := http.FileServer(http.Dir("../streams/testdata"))
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		r.mu.Lock()
		r.asked = append(r.asked, q.URL.Path)
		slow := r.slow
		body, written := r.extra[q.URL.Path]
		if name := filepath.Base(q.URL.Path); strings.HasPrefix(name, "piece") {
			slow *= time.Duration(name[5] - '0' + 1)
		}
		r.mu.Unlock()
		if written {
			w.Write([]byte(body))
			return
		}
		if slow > 0 && !strings.HasSuffix(q.URL.Path, ".m3u8") {
			time.Sleep(slow)
		}
		files.ServeHTTP(w, q)
	}))
	t.Cleanup(r.server.Close)
	go r.e.Run()
	r.fetch = fetch.New(r)
	r.m = New(r.e, mocksystem.New(func(f func()) { f() }), r.profile, r.dir)
	r.m.Streams(r.fetch, func(tab engine.TabID) string {
		if tab == 7 {
			return "work"
		}
		return ""
	})
	return r
}

// Fetch and CancelFetch are the engine's, for core/fetch. Main thread.
func (r *streamRig) Fetch(f engine.FetchRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	r.live[f.ID] = cancel
	r.asks = append(r.asks, f)
	go func() {
		got := mockengine.FetchFile(ctx, f)
		r.e.Post(func() {
			if _, ok := r.live[f.ID]; ok {
				delete(r.live, f.ID)
				r.fetch.Fetched(got)
			}
		})
	}()
}

func (r *streamRig) CancelFetch(id engine.FetchID) {
	if cancel := r.live[id]; cancel != nil {
		delete(r.live, id)
		cancel()
	}
}

// on runs f on the main thread and waits for it.
func (r *streamRig) on(f func()) {
	done := make(chan struct{})
	r.e.Post(func() { f(); close(done) })
	<-done
}

func (r *streamRig) row() Item {
	var it Item
	r.on(func() { it = r.m.List()[0] })
	return it
}

func (r *streamRig) until(what string, ok func(Item) bool) Item {
	r.t.Helper()
	for end := time.Now().Add(20 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		if it := r.row(); ok(it) {
			return it
		}
	}
	r.t.Fatalf("never %s: %+v", what, r.row())
	return Item{}
}

// download plays the engine downloading the playlist at path: asked where it goes, then
// done, with what the playlist holds at the place Go gave.
func (r *streamRig) download(id engine.DownloadID, path, name, mime string) (target string) {
	r.t.Helper()
	url := r.server.URL + path
	body, ok := r.extra[path]
	if !ok {
		b, err := os.ReadFile(filepath.Join("../streams/testdata", path))
		if err != nil {
			r.t.Fatal(err)
		}
		body = string(b)
	}
	r.on(func() {
		target = r.m.Target(engine.DownloadRequest{ID: id, Tab: 7, URL: url, Name: name, Mime: mime}).Path
		r.m.Updated(engine.Download{ID: id, Path: target, URL: url, Mime: mime, Total: int64(len(body)), Referrer: "https://video.test/watch"})
	})
	if err := os.WriteFile(target, []byte(body), 0o600); err != nil {
		r.t.Fatal(err)
	}
	r.on(func() {
		r.m.Updated(engine.Download{ID: id, Path: target, URL: url, Mime: mime, State: engine.DownloadDone, Received: int64(len(body)), Total: int64(len(body)), Referrer: "https://video.test/watch", Server: "127.0.0.1"})
	})
	return target
}

// A download that is a stream's playlist, by its type, becomes the stream's MP4 in the same
// row, under the playlist's name: the playlist itself never reaches the Downloads folder or
// the system, and the MP4 is handed on as a finished download is.
func TestAPlaylistDownloadBecomesAnMP4InItsRow(t *testing.T) {
	publishes = true
	defer func() { publishes = false }()
	r := newStreamRig(t)
	os.WriteFile(filepath.Join(r.dir, "film.mp4"), []byte("one already"), 0o644)
	var target string
	var first Item
	r.on(func() {
		target = r.m.Target(engine.DownloadRequest{ID: 3, Tab: 7, URL: r.server.URL + "/ts/index.m3u8", Name: "film.m3u8", Mime: "application/vnd.apple.mpegurl", SaveAs: true}).Path
		if !r.m.IsStream(3) {
			t.Error("not seen as a stream when it began")
		}
		r.m.Updated(engine.Download{ID: 3, Path: target, URL: r.server.URL + "/ts/index.m3u8", Total: 200})
		first = r.m.List()[0]
	})
	if strings.HasPrefix(target, r.dir) || !strings.HasPrefix(target, r.profile) {
		t.Fatalf("the playlist goes to %s", target)
	}
	if first.Name != "film.mp4" || first.State != "progress" || first.Stream != "hls" || first.Workspace != "work" {
		t.Fatalf("the row as the playlist arrives: %+v", first)
	}
	r.download(3, "/ts/index.m3u8", "film.m3u8", "application/vnd.apple.mpegurl")
	it := r.until("done", func(it Item) bool { return it.State == "done" })
	want := filepath.Join(r.dir, "film (1).mp4")
	info, err := os.Stat(want)
	if err != nil || it.Path != want || it.Name != "film (1).mp4" || it.Received != info.Size() || it.Total != info.Size() || it.Reason != "" {
		t.Fatalf("%+v, %v", it, err)
	}
	var n int
	r.on(func() { n = len(r.m.List()) })
	if _, err := os.Stat(target); err == nil || n != 1 {
		t.Fatalf("the playlist's file stayed: %v, rows %d", err == nil, n)
	}
	if left, _ := filepath.Glob(filepath.Join(r.dir, "*")); len(left) != 2 {
		t.Fatalf("the Downloads folder has %v", left)
	}
	// The system: the MP4 alone is handed on, and the row's notification ran as one
	// download's, never done before the MP4 was.
	if r.e.Published() != want {
		t.Fatalf("handed on %q", r.e.Published())
	}
	notices := r.e.Notices()
	for i, n := range notices {
		last := i == len(notices)-1
		if n.Name != "film.mp4" && n.Name != "film (1).mp4" || (n.State == engine.DownloadDone) != last {
			t.Fatalf("notification %d of %d: %+v", i, len(notices), n)
		}
	}
	if len(notices) < 4 {
		t.Fatalf("%d notifications", len(notices))
	}
	// Fetched as from the page it came from, in its tab's workspace, from where the
	// playlist's server is.
	for _, f := range r.asks {
		if f.Page != "https://video.test/watch" || f.Workspace != "work" || f.From != engine.SpaceLoopback {
			t.Fatalf("fetched as %+v", f)
		}
	}
	// Kept as a stream, for a later Retry.
	again := New(r.e, mocksystem.New(func(f func()) { f() }), r.profile, r.dir)
	again.Load()
	if l := again.List(); len(l) != 1 || l[0].Stream != "hls" || l[0].Workspace != "work" || l[0].Space != int(engine.SpaceLoopback) || l[0].State != "done" {
		t.Fatalf("kept %+v", l)
	}
}

// A playlist whose type and address don't say so is known by how it starts, once it is
// here; any other finished file is handed to the system as it was.
func TestAPlaylistIsKnownByHowItStarts(t *testing.T) {
	r := newStreamRig(t)
	r.extra["/watch"] = "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1.0,\nts/piece0.ts\n#EXT-X-ENDLIST\n"
	target := r.download(1, "/watch", "watch", "text/plain")
	if target != filepath.Join(r.dir, "watch") {
		t.Fatalf("it went to %s", target)
	}
	it := r.until("done", func(it Item) bool { return it.State == "done" })
	if it.Name != "watch.mp4" || it.Path != filepath.Join(r.dir, "watch.mp4") || it.Stream != "hls" {
		t.Fatalf("%+v", it)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("the playlist stayed in the Downloads folder")
	}
	if r.e.Published() != it.Path {
		t.Fatalf("handed on %q", r.e.Published())
	}
	r.extra["/notes.txt"] = "#EXTINF isn't how a playlist starts\n"
	notes := r.download(2, "/notes.txt", "notes.txt", "text/plain")
	if it := r.row(); it.State != "done" || it.Stream != "" || it.Path != notes || r.e.Published() != notes {
		t.Fatalf("a text file: %+v, handed on %q", it, r.e.Published())
	}
}

// A live stream and a protected one fail in their row, each saying why; a protected one
// has no Retry.
func TestAStreamThatCantBeSavedSaysWhy(t *testing.T) {
	r := newStreamRig(t)
	r.extra["/live.m3u8"] = "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1.0,\nts/piece0.ts\n"
	r.download(1, "/live.m3u8", "live.m3u8", "application/x-mpegurl")
	if it := r.until("failed", func(it Item) bool { return it.State == "failed" }); it.Reason != "live" || it.NoRetry || it.Name != "live.mp4" {
		t.Fatalf("%+v", it)
	}
	r.extra["/locked.m3u8"] = "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://k\",KEYFORMAT=\"com.apple.streamingkeydelivery\"\n#EXTINF:1.0,\nts/piece0.ts\n#EXT-X-ENDLIST\n"
	r.download(2, "/locked.m3u8", "locked.m3u8", "application/x-mpegurl")
	if it := r.until("failed", func(it Item) bool { return it.State == "failed" }); it.Reason != "protected" || !it.NoRetry {
		t.Fatalf("%+v", it)
	}
	// Retry does nothing to one it can't help.
	r.on(func() { r.m.Act(2, "retry") })
	if it := r.row(); it.State != "failed" || it.Reason != "protected" {
		t.Fatalf("a protected stream tried again: %+v", it)
	}
	if left, _ := filepath.Glob(filepath.Join(r.dir, "*")); len(left) != 0 {
		t.Fatalf("left %v", left)
	}
	if last := r.e.Notices()[len(r.e.Notices())-1]; last.State != engine.DownloadFailed {
		t.Fatalf("the last notification: %+v", last)
	}
}

// Pause, Resume, Cancel and Retry work on a stream's row as on any download's.
func TestAStreamsRowPausesResumesCancelsAndRetries(t *testing.T) {
	r := newStreamRig(t)
	r.slow = 120 * time.Millisecond
	r.download(1, "/ts/index.m3u8", "index.m3u8", "application/vnd.apple.mpegurl")
	it := r.until("under way", func(it Item) bool { return it.Received > 0 })
	if it.Total < it.Received || it.State != "progress" {
		t.Fatalf("under way: %+v", it)
	}
	r.on(func() { r.m.Act(it.ID, "pause") })
	time.Sleep(300 * time.Millisecond)
	paused := r.row()
	time.Sleep(300 * time.Millisecond)
	if still := r.row(); !paused.Paused || paused.State != "progress" || still.Received != paused.Received {
		t.Fatalf("paused: %+v then %+v", paused, still)
	}
	if n := r.e.Notices(); !n[len(n)-1].Paused {
		t.Fatalf("its notification isn't paused: %+v", n[len(n)-1])
	}
	r.on(func() { r.m.Act(it.ID, "resume") })
	r.until("going on", func(it Item) bool { return !it.Paused && it.Received > paused.Received })
	r.on(func() { r.m.Act(it.ID, "cancel") })
	if got := r.row(); got.State != "cancelled" {
		t.Fatalf("cancelled: %+v", got)
	}
	time.Sleep(300 * time.Millisecond)
	if left, _ := filepath.Glob(filepath.Join(r.dir, "*")); len(left) != 0 || r.row().State != "cancelled" {
		t.Fatalf("cancelled, it left %v, %+v", left, r.row())
	}
	// Retry starts it over from its playlist's address, in the same row, under the same name.
	r.mu.Lock()
	r.slow, r.asked = 0, nil
	r.mu.Unlock()
	r.on(func() { r.m.Act(it.ID, "retry") })
	done := r.until("done after Retry", func(it Item) bool { return it.State == "done" })
	if done.ID != it.ID || done.Name != "index.mp4" || done.Path != filepath.Join(r.dir, "index.mp4") {
		t.Fatalf("%+v", done)
	}
	r.mu.Lock()
	asked := strings.Join(r.asked, " ")
	r.mu.Unlock()
	if !strings.HasPrefix(asked, "/ts/index.m3u8 ") {
		t.Fatalf("Retry asked for %s", asked)
	}
	// And a running one: Retry throws away what it has and starts over.
	r.mu.Lock()
	r.slow = 120 * time.Millisecond
	r.mu.Unlock()
	r.download(2, "/fmp4/index.m3u8", "clip.m3u8", "application/vnd.apple.mpegurl")
	second := r.until("under way", func(it Item) bool { return it.Received > 0 })
	r.on(func() { r.m.Act(second.ID, "retry") })
	if got := r.until("done", func(it Item) bool { return it.State == "done" }); got.ID != second.ID || got.Name != "clip.mp4" {
		t.Fatalf("%+v", got)
	}
	// Each job's folder goes a moment after its end, on the job's own goroutine.
	var left []string
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if left, _ = filepath.Glob(filepath.Join(r.profile, "streams", "job-*")); len(left) == 0 {
			break
		}
	}
	if len(left) != 0 {
		t.Fatalf("the jobs left %v", left)
	}
}

// A DASH playlist's download, by its type, its address or how it starts, becomes its
// stream's MP4 in the same way.
func TestADASHPlaylistDownloadBecomesAnMP4(t *testing.T) {
	r := newStreamRig(t)
	r.download(1, "/dash/template/manifest.mpd", "manifest.mpd", "application/dash+xml")
	it := r.until("done", func(it Item) bool { return it.State != "progress" })
	if it.State != "done" || it.Name != "manifest.mp4" || it.Stream != "dash" || it.Path != filepath.Join(r.dir, "manifest.mp4") {
		t.Fatalf("%+v", it)
	}
	// One with nothing in its type or its address to say so, known by its root element once
	// it is here, and taken out of the Downloads folder again.
	b, _ := os.ReadFile("../streams/testdata/dash/single/base.mpd")
	r.extra["/dash/single/film"] = string(b)
	target := r.download(2, "/dash/single/film", "film", "text/xml")
	if target != filepath.Join(r.dir, "film") {
		t.Fatalf("it went to %s", target)
	}
	if it := r.until("done", func(it Item) bool { return it.State != "progress" }); it.State != "done" || it.Name != "film.mp4" || it.Stream != "dash" {
		t.Fatalf("%+v", it)
	}
	// Retry, after a restart's worth of state: from its address, as DASH.
	var id int
	r.on(func() { id = r.m.List()[0].ID; r.m.Act(id, "retry") })
	if it := r.until("done again", func(it Item) bool { return it.State == "done" && it.Name == "film (1).mp4" }); it.Stream != "dash" {
		t.Fatalf("%+v", it)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("the playlist stayed in the Downloads folder")
	}
	// A live one says so.
	r.extra["/live.mpd"] = `<?xml version="1.0"?><MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="dynamic"><Period id="0"/></MPD>`
	r.download(3, "/live.mpd", "live.mpd", "application/dash+xml")
	if it := r.until("failed", func(it Item) bool { return it.State == "failed" }); it.Reason != "live" {
		t.Fatalf("%+v", it)
	}
}

// A stream whose playlist's name says nothing is named after better text, with the MP4's
// ending whatever the text ends in, and keeps the playlist's own name beside it.
func TestAStreamIsNamedAfterItsPage(t *testing.T) {
	r := newStreamRig(t)
	r.on(func() {
		r.m.NameBy(func(q engine.DownloadRequest) Naming {
			return Naming{Mode: NamesAuto, Linked: LinkedTitle(q.URL), Title: "A page of films"}
		})
	})
	r.download(1, "/ts/index.m3u8", "index.m3u8", "application/vnd.apple.mpegurl")
	if it := r.until("done", func(it Item) bool { return it.State != "progress" }); it.State != "done" || it.Name != "A page of films.mp4" || it.Original != "index.m3u8" || it.Path != filepath.Join(r.dir, "A page of films.mp4") {
		t.Fatalf("%+v", it)
	}
	r.on(func() {
		r.m.NameBy(func(q engine.DownloadRequest) Naming {
			return Naming{Mode: NamesAuto, Linked: "Big Buck Bunny.m3u8", Title: "A page of films"}
		})
	})
	r.download(2, "/fmp4/index.m3u8", "index.m3u8", "application/vnd.apple.mpegurl")
	if it := r.until("done", func(it Item) bool { return it.State != "progress" }); it.State != "done" || it.Name != "Big Buck Bunny.m3u8.mp4" || it.Original != "index.m3u8" {
		t.Fatalf("%+v", it)
	}
	// One whose name says something keeps it, with the MP4's ending.
	r.extra["/ts/holiday-in-goa.m3u8"] = "#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1.0,\npiece0.ts\n#EXT-X-ENDLIST\n"
	r.download(3, "/ts/holiday-in-goa.m3u8", "holiday-in-goa.m3u8", "application/vnd.apple.mpegurl")
	if it := r.until("done", func(it Item) bool { return it.State != "progress" }); it.Name != "holiday-in-goa.mp4" || it.Original != "" {
		t.Fatalf("%+v", it)
	}
}
