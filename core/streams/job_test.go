package streams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"projectb/streams/mp4check"
)

// rig is a job's world in a test: a server of testdata's streams, the main thread as a
// line of posted work, and the fetches made through core/fetch on the mock engine's own
// fetch, which keeps the engine's rules.
type rig struct {
	t      *testing.T
	server *httptest.Server
	posted chan func()
	fetch  *fetch.Manager
	dir    string

	mu sync.Mutex
	// asked are the paths the server was asked for, in order, and from the Referer of each.
	asked []string
	from  map[string]string
	// slow holds each piece back this long; fail answers a path with 503 this many times.
	slow time.Duration
	fail map[string]int
	// extra are playlists written for a test, by path.
	extra map[string]string

	fetches map[engine.FetchID]context.CancelFunc
	wanted  []engine.FetchRequest
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{t: t, posted: make(chan func(), 1024), dir: t.TempDir(), from: map[string]string{}, fail: map[string]int{},
		extra: map[string]string{}, fetches: map[engine.FetchID]context.CancelFunc{}}
	files := http.FileServer(http.Dir("testdata"))
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		r.mu.Lock()
		r.asked = append(r.asked, q.URL.RequestURI())
		r.from[q.URL.Path] = q.Header.Get("Referer")
		slow := r.slow
		failing := r.fail[q.URL.Path] > 0
		if failing {
			r.fail[q.URL.Path]--
		}
		body, written := r.extra[q.URL.Path]
		r.mu.Unlock()
		if failing {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		if written {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Write([]byte(body))
			return
		}
		if slow > 0 && !strings.HasSuffix(q.URL.Path, ".m3u8") {
			time.Sleep(slow)
		}
		files.ServeHTTP(w, q)
	}))
	t.Cleanup(r.server.Close)
	r.fetch = fetch.New(r)
	return r
}

// Fetch and CancelFetch are the engine's, for core/fetch.
func (r *rig) Fetch(f engine.FetchRequest) {
	ctx, cancel := context.WithCancel(context.Background())
	r.fetches[f.ID] = cancel
	r.wanted = append(r.wanted, f)
	go func() {
		got := mockengine.FetchFile(ctx, f)
		r.posted <- func() {
			if _, ok := r.fetches[f.ID]; ok {
				delete(r.fetches, f.ID)
				r.fetch.Fetched(got)
			}
		}
	}()
}

func (r *rig) CancelFetch(id engine.FetchID) {
	if cancel := r.fetches[id]; cancel != nil {
		delete(r.fetches, id)
		cancel()
	}
}

func (r *rig) requests() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.asked...)
}

// run does the main thread's posted work until done says so.
func (r *rig) run(done func() bool) {
	r.t.Helper()
	limit := time.After(30 * time.Second)
	for !done() {
		select {
		case f := <-r.posted:
			f()
		case <-limit:
			r.t.Fatal("the job didn't get there in 30 seconds")
		}
	}
}

// idle does posted work until none has come for a while.
func (r *rig) idle(quiet time.Duration) {
	for {
		select {
		case f := <-r.posted:
			f()
		case <-time.After(quiet):
			return
		}
	}
}

// saved is how a job ended.
type saved struct {
	job      *Job
	out      string
	done     bool
	err      error
	progress []Progress
}

// start starts saving the stream at path on the rig's server, its playlist already in hand
// as a download brings it, unless fetched says the job fetches it.
func (r *rig) start(path string, space engine.AddressSpace, fetched bool) *saved {
	r.t.Helper()
	s := &saved{out: filepath.Join(r.dir, "out.mp4")}
	src := Source{URL: r.server.URL + path, Page: "https://video.test/watch?v=1", Workspace: "w1", Space: space}
	if !fetched {
		body, ok := r.extra[path]
		if !ok {
			b, err := os.ReadFile(filepath.Join("testdata", path))
			if err != nil {
				r.t.Fatal(err)
			}
			body = string(b)
		}
		src.Body = []byte(body)
	}
	s.job = Start(Config{Source: src, Fetcher: r.fetch, Post: func(f func()) { r.posted <- f },
		Dir: filepath.Join(r.dir, "work"), Out: s.out,
		Changed: func(p Progress) { s.progress = append(s.progress, p) },
		Done:    func(err error) { s.done, s.err = true, err }})
	return s
}

// save saves the stream at path and waits for its end.
func (r *rig) save(path string, space engine.AddressSpace) *saved {
	r.t.Helper()
	s := r.start(path, space, false)
	r.run(func() bool { return s.done })
	return s
}

func expected(t *testing.T, name string) *mp4check.File {
	t.Helper()
	b, err := os.ReadFile("testdata/expect.json")
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]*mp4check.File{}
	if err := json.Unmarshal(b, &all); err != nil {
		t.Fatal(err)
	}
	return all[name]
}

func near(a, b, by int64) bool { return a-b <= by && b-a <= by }

// same holds the MP4 at path to what ffmpeg made of the same stream: a regular MP4 with
// its index first, the same tracks, and in each every frame, in order, whole, shown when
// ffmpeg's is and for as long, and so the same length.
func same(t *testing.T, path string, want *mp4check.File) *mp4check.File {
	t.Helper()
	return sameTracks(t, path, want, false)
}

// sameTracks is same, each track held to its own start when apart says so.
func sameTracks(t *testing.T, path string, want *mp4check.File, apart bool) *mp4check.File {
	t.Helper()
	got, err := mp4check.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.MoovFirst || got.Fragmented {
		t.Errorf("index first %v, fragmented %v: not a regular MP4 with its index at the front", got.MoovFirst, got.Fragmented)
	}
	if len(got.Tracks) != len(want.Tracks) {
		t.Fatalf("%d tracks, want %d", len(got.Tracks), len(want.Tracks))
	}
	// ffmpeg lets a track start a little after nothing where its stream did; the app starts
	// the earliest at nothing. Every frame is held to the same shift.
	shift := int64(1 << 62)
	for i := range want.Tracks {
		for _, f := range want.Tracks[i].Frames {
			shift = min(shift, f.At)
		}
	}
	var end int64
	for i, w := range want.Tracks {
		g := got.Tracks[i]
		if g.Kind != w.Kind || g.Codec != w.Codec || len(g.Frames) != len(w.Frames) {
			t.Fatalf("track %d is %s %s with %d frames, want %s %s with %d", i, g.Kind, g.Codec, len(g.Frames), w.Kind, w.Codec, len(w.Frames))
		}
		var own int64
		if apart && len(g.Frames) > 0 {
			own = g.Frames[0].At - (w.Frames[0].At - shift)
		}
		for n, f := range w.Frames {
			h := g.Frames[n]
			if h.Sum != f.Sum || h.Sync != f.Sync || !near(h.At-own, f.At-shift, 1500) || !near(h.For, f.For, 1500) {
				t.Fatalf("%s frame %d is %+v, want %+v (less %d)", w.Kind, n, h, f, shift)
			}
		}
		end = max(end, g.End())
	}
	if !near(got.Micros, end, 2000) {
		t.Errorf("the MP4 says it is %d long, and its frames end at %d", got.Micros, end)
	}
	return got
}

// MPEG-TS pieces become one MP4 with every frame of picture and sound, in order.
func TestTSPiecesBecomeOneMP4(t *testing.T) {
	r := newRig(t)
	s := r.save("/ts/index.m3u8", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	got := same(t, s.out, expected(t, "ts"))
	if !near(got.Micros, 4_000_000, 60_000) {
		t.Errorf("4 seconds of stream made %d", got.Micros)
	}
	// Progress counts pieces, and each is fetched as from the page, in its workspace, from
	// where the playlist's server is.
	var bytes int64
	for i := range 4 {
		info, _ := os.Stat(fmt.Sprintf("testdata/ts/piece%d.ts", i))
		bytes += info.Size()
	}
	if last := s.progress[len(s.progress)-1]; last.Pieces != 4 || last.Total != 4 || last.Bytes != bytes {
		t.Errorf("progress ended %+v", last)
	}
	for i, p := range s.progress {
		if i > 0 && p.Pieces != s.progress[i-1].Pieces+1 {
			t.Errorf("progress %+v", s.progress)
		}
	}
	if got := strings.Join(r.requests(), " "); got != "/ts/piece0.ts /ts/piece1.ts /ts/piece2.ts /ts/piece3.ts" && len(r.requests()) != 4 {
		t.Errorf("asked for %s", got)
	}
	for _, f := range r.wanted {
		if f.Page != "https://video.test/watch?v=1" || f.Workspace != "w1" || f.From != engine.SpaceLoopback {
			t.Errorf("fetched as %+v", f)
		}
	}
	if r.from["/ts/piece2.ts"] != "https://video.test/watch?v=1" {
		t.Errorf("a piece came from %q", r.from["/ts/piece2.ts"])
	}
	// The job's folder goes a moment after its end, on the job's own goroutine.
	var left []string
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		if left, _ = filepath.Glob(filepath.Join(r.dir, "*")); len(left) == 1 {
			break
		}
	}
	if len(left) != 1 || left[0] != s.out {
		t.Errorf("the job left %v", left)
	}
}

// MP4 fragments with a header piece (EXT-X-MAP) become the same.
func TestMP4FragmentsBecomeOneMP4(t *testing.T) {
	r := newRig(t)
	s := r.save("/fmp4/index.m3u8", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	same(t, s.out, expected(t, "fmp4"))
	init := 0
	for _, a := range r.requests() {
		if a == "/fmp4/init.mp4" {
			init++
		}
	}
	if init != 1 {
		t.Errorf("the header piece was fetched %d times", init)
	}
}

// Pieces locked with AES-128 are unlocked with the key the playlist names, each key fetched
// once: the IV the playlist gives, or with none the piece's sequence number.
func TestAES128PiecesAreUnlocked(t *testing.T) {
	r := newRig(t)
	s := r.save("/aes/index.m3u8", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	same(t, s.out, expected(t, "ts"))
	keys := map[string]int{}
	for _, a := range r.requests() {
		if strings.HasSuffix(a, ".key") {
			keys[a]++
		}
	}
	if len(keys) != 2 || keys["/aes/one.key"] != 1 || keys["/aes/two.key"] != 1 {
		t.Errorf("keys fetched: %v", keys)
	}
}

// A master playlist is saved at its best quality, the largest picture and then the greatest
// bandwidth, and its separate sound is fetched and joined to the picture.
func TestAMasterPlaylistIsSavedAtItsBestWithItsSound(t *testing.T) {
	r := newRig(t)
	s := r.start("/split/master.m3u8", engine.SpaceLoopback, true)
	r.run(func() bool { return s.done })
	if s.err != nil {
		t.Fatal(s.err)
	}
	// ffmpeg, given the two playlists as two inputs, starts each at nothing. The picture's
	// own header says it starts 80 ms in, its reordering delay, as ffmpeg's HLS writer has
	// it, and a player of the stream shows it so: the MP4 keeps that.
	got := sameTracks(t, s.out, expected(t, "split"), true)
	if got.Tracks[0].Kind != "vide" || got.Tracks[1].Kind != "soun" {
		t.Errorf("tracks %s, %s", got.Tracks[0].Kind, got.Tracks[1].Kind)
	}
	if v, a := got.Tracks[0].Frames[0].At, got.Tracks[1].Frames[0].At; a != 0 || !near(v, 80_000, 1500) {
		t.Errorf("the sound starts at %d and the picture at %d", a, v)
	}
	asked := strings.Join(r.requests(), " ")
	if !strings.HasPrefix(asked, "/split/master.m3u8 ") || !strings.Contains(asked, " /split/video.m3u8 ") || !strings.Contains(asked, " /split/audio.m3u8 ") {
		t.Errorf("asked for %s", asked)
	}
	if strings.Contains(asked, "small") || strings.Contains(asked, "lesser") {
		t.Errorf("a lesser quality was fetched: %s", asked)
	}
	// Picture and sound lie side by side in the file, not one after the other.
	if s.progress[len(s.progress)-1].Total != 3+4 {
		t.Errorf("progress %+v", s.progress[len(s.progress)-1])
	}
}

// The best quality is the highest resolution, then the highest bandwidth, whatever the order.
func TestTheBestQuality(t *testing.T) {
	for _, c := range []struct{ list, best string }{
		{"#EXT-X-STREAM-INF:BANDWIDTH=900,RESOLUTION=640x360\nlow.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=500,RESOLUTION=1920x1080\nhd.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=700,RESOLUTION=1280x720\nmid.m3u8\n", "hd.m3u8"},
		{"#EXT-X-STREAM-INF:BANDWIDTH=500,RESOLUTION=1920x1080\na.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=800,RESOLUTION=1920x1080\nb.m3u8\n", "b.m3u8"},
		{"#EXT-X-STREAM-INF:BANDWIDTH=300\na.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=800\nb.m3u8\n#EXT-X-I-FRAME-STREAM-INF:BANDWIDTH=9000,RESOLUTION=3840x2160,URI=\"i.m3u8\"\n", "b.m3u8"},
	} {
		plan, err := ReadHLS([]byte("#EXTM3U\n"+c.list), "https://cdn.test/v/master.m3u8?token=1")
		if err != nil || len(plan.Next) != 1 || plan.Next[0] != "https://cdn.test/v/"+c.best {
			t.Errorf("%q: %+v, %v", c.list, plan, err)
		}
	}
}

const head = "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n"

// A live playlist and a protected one are refused, in the words the download's row shows,
// before any piece is fetched.
func TestLiveAndProtectedStreamsAreRefused(t *testing.T) {
	piece := "#EXTINF:1.0,\npiece0.ts\n"
	for name, c := range map[string]struct {
		list   string
		want   error
		reason string
	}{
		"live":      {head + piece, ErrLive, "live"},
		"sample":    {head + "#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"one.key\"\n" + piece + "#EXT-X-ENDLIST\n", ErrProtected, "protected"},
		"fairplay":  {head + "#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"skd://key\",KEYFORMAT=\"com.apple.streamingkeydelivery\",KEYFORMATVERSIONS=\"1\"\n" + piece + "#EXT-X-ENDLIST\n", ErrProtected, "protected"},
		"widevine":  {head + "#EXT-X-KEY:METHOD=SAMPLE-AES-CTR,URI=\"data:text/plain;base64,AAAA\",KEYFORMAT=\"urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed\"\n" + piece + "#EXT-X-ENDLIST\n", ErrProtected, "protected"},
		"playready": {head + "#EXT-X-KEY:METHOD=AES-128,URI=\"one.key\",KEYFORMAT=\"com.microsoft.playready\"\n" + piece + "#EXT-X-ENDLIST\n", ErrProtected, "protected"},
		"later":     {head + piece + "#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"one.key\"\n" + piece + "#EXT-X-ENDLIST\n", ErrProtected, "protected"},
		// Live and protected says protected: it will never be saved.
		"both":    {head + "#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"one.key\"\n" + piece, ErrProtected, "protected"},
		"session": {"#EXTM3U\n#EXT-X-SESSION-KEY:METHOD=SAMPLE-AES,URI=\"skd://key\",KEYFORMAT=\"com.apple.streamingkeydelivery\"\n#EXT-X-STREAM-INF:BANDWIDTH=300\nindex.m3u8\n", ErrProtected, "protected"},
	} {
		r := newRig(t)
		r.extra["/ts/"+name+".m3u8"] = c.list
		s := r.save("/ts/"+name+".m3u8", engine.SpaceLoopback)
		// A live one may be tried again, once it has ended; a protected one never.
		reason, again := Reason(s.err)
		if !errors.Is(s.err, c.want) || reason != c.reason || again != (c.want == ErrLive) {
			t.Errorf("%s: %v, said %q, again %v", name, s.err, reason, again)
		}
		if asked := r.requests(); len(asked) != 0 {
			t.Errorf("%s: fetched %v", name, asked)
		}
		if _, err := os.Stat(s.out); err == nil {
			t.Errorf("%s left a file", name)
		}
	}
	// A file that isn't a playlist, whatever its name said.
	r := newRig(t)
	r.extra["/ts/page.m3u8"] = "<!doctype html><title>Not found</title>"
	s := r.save("/ts/page.m3u8", engine.SpaceLoopback)
	if reason, again := Reason(s.err); !errors.Is(s.err, ErrFormat) || reason != "format" || again {
		t.Errorf("a page named .m3u8: %v", s.err)
	}
	// One whose best quality's own playlist is live or protected: refused once that is read.
	r = newRig(t)
	r.extra["/ts/master.m3u8"] = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=300\nlive.m3u8\n"
	r.extra["/ts/live.m3u8"] = head + piece
	if s := r.save("/ts/master.m3u8", engine.SpaceLoopback); !errors.Is(s.err, ErrLive) {
		t.Errorf("a master of a live playlist: %v", s.err)
	}
	if asked := strings.Join(r.requests(), " "); asked != "/ts/live.m3u8" {
		t.Errorf("fetched %s", asked)
	}
}

// The fetch's rules. A playlist from a public address may not fetch a piece, a key or
// another playlist from this computer or its network, and only http and https are fetched.
func TestAPlaylistFromTheInternetMayNotReachThisComputer(t *testing.T) {
	// The server is on this computer; the playlist says it came from the internet.
	for _, path := range []string{"/ts/index.m3u8", "/aes/index.m3u8", "/fmp4/index.m3u8", "/split/master.m3u8"} {
		r := newRig(t)
		s := r.save(path, engine.SpacePublic)
		reason, _ := Reason(s.err)
		if !errors.Is(s.err, ErrRefused) || reason != "refused" {
			t.Errorf("%s: %v", path, s.err)
		}
		if asked := r.requests(); len(asked) != 0 {
			t.Errorf("%s: this computer was asked for %v", path, asked)
		}
		if _, err := os.Stat(s.out); err == nil {
			t.Errorf("%s left a file", path)
		}
		for _, f := range r.wanted {
			if f.From != engine.SpacePublic {
				t.Errorf("%s: asked from %d", path, f.From)
			}
		}
	}
	// A router's address, in a playlist from the internet: the engine refuses it at the
	// connection; here core/fetch's own rule is what the mock engine's fetch shares.
	r := newRig(t)
	r.extra["/ts/router.m3u8"] = head + "#EXTINF:1.0,\nhttp://192.168.1.1/cgi-bin/reboot\n#EXT-X-ENDLIST\n"
	if s := r.save("/ts/router.m3u8", engine.SpacePublic); !errors.Is(s.err, ErrRefused) {
		t.Errorf("a router's address: %v", s.err)
	}
}

func TestOnlyHTTPAndHTTPSPiecesAreFetched(t *testing.T) {
	for name, list := range map[string]string{
		"file":  head + "#EXTINF:1.0,\nfile:///etc/passwd\n#EXT-X-ENDLIST\n",
		"key":   head + "#EXT-X-KEY:METHOD=AES-128,URI=\"file:///etc/passwd\"\n#EXTINF:1.0,\npiece0.ts\n#EXT-X-ENDLIST\n",
		"data":  head + "#EXTINF:1.0,\ndata:video/mp2t;base64,AAAA\n#EXT-X-ENDLIST\n",
		"ftp":   head + "#EXTINF:1.0,\nftp://files.test/piece0.ts\n#EXT-X-ENDLIST\n",
		"map":   head + "#EXT-X-MAP:URI=\"chrome://settings/\"\n#EXTINF:1.0,\npiece0.ts\n#EXT-X-ENDLIST\n",
		"media": "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=300\nfile:///tmp/index.m3u8\n",
	} {
		r := newRig(t)
		r.extra["/ts/"+name+".m3u8"] = list
		s := r.save("/ts/"+name+".m3u8", engine.SpaceLoopback)
		if !errors.Is(s.err, ErrRefused) {
			t.Errorf("%s: %v", name, s.err)
		}
		for _, f := range r.wanted {
			if !strings.HasPrefix(f.URL, "http://") {
				t.Errorf("%s: the engine was asked for %s", name, f.URL)
			}
		}
	}
}

// Pause stops the fetching where it is, Resume goes on, and the MP4 is whole.
func TestPauseAndResume(t *testing.T) {
	r := newRig(t)
	r.slow = 80 * time.Millisecond
	s := r.start("/split/master.m3u8", engine.SpaceLoopback, false)
	r.run(func() bool { return len(s.progress) > 1 })
	s.job.Pause()
	if !s.job.Paused() {
		t.Fatal("not paused")
	}
	r.idle(300 * time.Millisecond)
	at, asked := s.job.Progress(), len(r.requests())
	if s.done || at.Pieces >= at.Total {
		t.Fatalf("paused, it went on to %+v, done %v", at, s.done)
	}
	r.idle(300 * time.Millisecond)
	if again := len(r.requests()); again != asked || s.job.Progress() != at {
		t.Fatalf("paused, it fetched on: %d then %d requests, %+v then %+v", asked, again, at, s.job.Progress())
	}
	s.job.Resume()
	r.run(func() bool { return s.done })
	if s.err != nil {
		t.Fatal(s.err)
	}
	sameTracks(t, s.out, expected(t, "split"), true)
}

// Cancel stops it, removes what it made, and tells nothing more.
func TestCancel(t *testing.T) {
	r := newRig(t)
	r.slow = 80 * time.Millisecond
	s := r.start("/ts/index.m3u8", engine.SpaceLoopback, false)
	r.run(func() bool { return len(s.progress) > 1 })
	s.job.Cancel()
	r.idle(400 * time.Millisecond)
	if s.done {
		t.Fatalf("cancelled, it said it ended: %v", s.err)
	}
	if left, _ := filepath.Glob(filepath.Join(r.dir, "*")); len(left) != 0 {
		t.Fatalf("cancelled, it left %v", left)
	}
	// Cancelled before it has read its playlist, too.
	early := r.start("/split/master.m3u8", engine.SpaceLoopback, true)
	early.job.Cancel()
	r.idle(300 * time.Millisecond)
	if left, _ := filepath.Glob(filepath.Join(r.dir, "*")); early.done || len(left) != 0 {
		t.Fatalf("cancelled at once: done %v, left %v", early.done, left)
	}
}

// A piece that fails is asked for again; one that keeps failing fails the job, saying why.
func TestAFailingPieceIsAskedForAgain(t *testing.T) {
	r := newRig(t)
	r.fail["/ts/piece1.ts"] = 2
	s := r.save("/ts/index.m3u8", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	same(t, s.out, expected(t, "ts"))

	r = newRig(t)
	r.fail["/ts/piece1.ts"] = 3
	s = r.save("/ts/index.m3u8", engine.SpaceLoopback)
	var failed *FetchError
	if reason, again := Reason(s.err); !errors.As(s.err, &failed) || reason != "server" || !again {
		t.Fatalf("%v", s.err)
	}
	if _, err := os.Stat(s.out); err == nil {
		t.Fatal("a failed job left a file")
	}
	r.idle(200 * time.Millisecond)
	if left, _ := filepath.Glob(filepath.Join(r.dir, "*")); len(left) != 0 {
		t.Fatalf("failed, it left %v", left)
	}
}

// A discontinuity starts the clock over, and may change the picture's size: the pieces
// after it go on where the ones before ended, with a sample entry of their own.
func TestADiscontinuityGoesOnWhereTheLastRunEnded(t *testing.T) {
	r := newRig(t)
	r.extra["/ts/joined.m3u8"] = head + "#EXTINF:1.0,\npiece0.ts\n#EXTINF:1.0,\npiece1.ts\n#EXT-X-DISCONTINUITY\n#EXTINF:1.0,\n../small/piece0.ts\n#EXTINF:1.0,\n../small/piece1.ts\n#EXT-X-ENDLIST\n"
	s := r.save("/ts/joined.m3u8", engine.SpaceLoopback)
	if s.err != nil {
		t.Fatal(s.err)
	}
	got, err := mp4check.Read(s.out)
	if err != nil {
		t.Fatal(err)
	}
	first, second := expected(t, "ts"), expected(t, "small")
	for i, track := range got.Tracks {
		a, b := first.Tracks[i].Frames, second.Tracks[i].Frames
		// The first two pieces hold half the picture's frames, and about half the sound's.
		half := len(track.Frames) - len(b)
		if half < len(a)/2-4 || half > len(a)/2+4 || track.Kind == "vide" && half != len(a)/2 {
			t.Fatalf("%s has %d frames, want about %d and %d", track.Kind, len(track.Frames), len(a)/2, len(b))
		}
		for n, f := range track.Frames {
			want := b[max(n-half, 0)].Sum
			if n < half {
				want = a[n].Sum
			}
			if f.Sum != want {
				t.Fatalf("%s frame %d isn't the stream's", track.Kind, n)
			}
		}
		// The second run starts where the first ended, 2 seconds in, and nothing is shown
		// twice or out of order by more than the frames' own reordering.
		if at := track.Frames[half].At; !near(at, 2_000_000, 80_000) {
			t.Errorf("%s: the second run starts at %d", track.Kind, at)
		}
		if !near(track.End(), 4_000_000, 80_000) {
			t.Errorf("%s ends at %d", track.Kind, track.End())
		}
	}
	if !near(got.Micros, 4_000_000, 80_000) {
		t.Errorf("it is %d long", got.Micros)
	}
}

func TestWhatIsAPlaylist(t *testing.T) {
	for _, c := range []struct {
		mime, address string
		want          Kind
	}{
		{"application/vnd.apple.mpegurl", "https://a.test/v", HLS},
		{"Application/X-MPEGURL; charset=utf-8", "https://a.test/v", HLS},
		{"text/plain", "https://a.test/v/index.m3u8?token=1#pb-download", HLS},
		{"application/octet-stream", "https://a.test/v/INDEX.M3U8", HLS},
		{"video/mp4", "https://a.test/v.mp4", None},
		{"text/plain", "https://a.test/m3u8", None},
		{"", "https://a.test/list.m3u", None},
	} {
		if got := KindOf(c.mime, c.address); got != c.want {
			t.Errorf("%q %q: %q", c.mime, c.address, got)
		}
	}
	if Sniff([]byte("#EXTM3U\n#EXT-X-VERSION:3")) != HLS || Sniff([]byte("\xef\xbb\xbf#EXTM3U\n")) != HLS || Sniff([]byte("<html>")) != None || Sniff(nil) != None {
		t.Error("sniffing")
	}
	for in, want := range map[string]string{"index.m3u8": "index.mp4", "My Film.m3u8": "My Film.mp4", "playlist": "playlist.mp4", "": "video.mp4", "a.b.m3u8": "a.b.mp4"} {
		if got := NameOf(in); got != want {
			t.Errorf("%q is named %q", in, got)
		}
	}
}
