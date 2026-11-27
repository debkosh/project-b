package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// sources is a stand-in for the lists' hosts: path to body, changed between runs.
type sources struct {
	mu   sync.Mutex
	body map[string]string
}

func (s *sources) set(path, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body[path] = body
}

func (s *sources) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	b, ok := s.body[r.URL.Path]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, b)
}

type rig struct {
	t       *testing.T
	src     *sources
	srv     *httptest.Server
	cat     *Catalog
	release string // the release's assets as the workflow leaves them
	pub     ed25519.PublicKey
	priv    ed25519.PrivateKey
	runs    int
	clock   time.Time
}

func newRig(t *testing.T) *rig {
	src := &sources{body: map[string]string{}}
	srv := httptest.NewTLSServer(src)
	t.Cleanup(srv.Close)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	u := srv.URL
	cat := &Catalog{
		Groups: []Group{{ID: "ads", Title: "Ads", Blocker: "network", Shown: true}},
		Lists: []List{
			{ID: "a", Title: "A", Group: "ads", On: true, Licence: "MIT", Home: u,
				Sources: []Source{{URL: u + "/a.txt", Format: "standard"}}},
			{ID: "b", Title: "B", Group: "ads", On: true, Licence: "MIT", Home: u,
				Sources: []Source{{URL: u + "/b.hosts", Format: "hosts"}}},
			{ID: "c", Title: "C", Group: "ads", On: true, Licence: "MIT", Home: u,
				Sources: []Source{{URL: u + "/c1.txt", Format: "standard"}, {URL: u + "/c2.txt", Format: "standard"}}},
		},
	}
	if err := cat.validate(); err != nil {
		t.Fatal(err)
	}
	src.set("/a.txt", bigList("a", 0))
	src.set("/b.hosts", "# a hosts file\n0.0.0.0 ads.example\n127.0.0.1 localhost\ntracker.example # one\n")
	src.set("/c1.txt", bigList("c1", 0))
	src.set("/c2.txt", "##.c2\n")
	src.set("/resources.json", `[{"name":"noop.js","aliases":[],"kind":{"mime":"application/javascript"},"content":"KGZ1bmN0aW9uKCl7fSkoKTs="}]`)
	return &rig{t: t, src: src, srv: srv, cat: cat, release: t.TempDir(), pub: pub, priv: priv,
		clock: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}
}

// bigList is a list of a few thousand rules; version v changes a few of them.
func bigList(name string, v int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "! Title: %s\n! Version: %d\n", name, v)
	for i := 0; i < 3000; i++ {
		if i%1000 == 7 {
			fmt.Fprintf(&b, "||changed-%d-%d.example^\n", i, v)
			continue
		}
		fmt.Fprintf(&b, "||host-%s-%d.example^$third-party\n", name, i)
	}
	return b.String()
}

// publish runs one round into a new out folder, then does what the workflow does: uploads it
// and deletes what -keep doesn't name.
func (r *rig) publish() (changed bool, out string, m *Manifest) {
	r.t.Helper()
	r.runs++
	r.clock = r.clock.Add(time.Hour)
	out = filepath.Join(r.t.TempDir(), "out")
	keep := filepath.Join(r.t.TempDir(), "keep.txt")
	own := r.t.TempDir()
	if err := os.WriteFile(filepath.Join(own, "pb-test.js"), []byte("document.title;\n"), 0o644); err != nil {
		r.t.Fatal(err)
	}
	changed, err := run(context.Background(), config{
		catalog: r.cat, out: out, keep: keep, prev: dirStore(r.release), workers: 4,
		fetch:        &fetcher{client: r.srv.Client(), maxSize: 1 << 20, retryWait: time.Millisecond},
		resourcesURL: r.srv.URL + "/resources.json", ownResources: own, key: r.priv,
		now: func() time.Time { return r.clock },
	})
	if err != nil {
		r.t.Fatal(err)
	}
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil {
			r.t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(r.release, e.Name()), b, 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	kb, err := os.ReadFile(keep)
	if err != nil {
		r.t.Fatal(err)
	}
	kept := map[string]bool{}
	for _, n := range strings.Fields(string(kb)) {
		kept[n] = true
	}
	if !kept[manifestName] {
		r.t.Fatalf("keep doesn't name %s", manifestName)
	}
	all, _ := os.ReadDir(r.release)
	for _, e := range all {
		if !kept[e.Name()] {
			os.Remove(filepath.Join(r.release, e.Name()))
		}
	}
	for n := range kept {
		if _, err := os.Stat(filepath.Join(r.release, n)); err != nil {
			r.t.Fatalf("keep names %s, which the release doesn't have", n)
		}
	}
	return changed, out, r.manifest()
}

// manifest reads the release's manifest, checking its signature with the public key.
func (r *rig) manifest() *Manifest {
	r.t.Helper()
	mb, err := os.ReadFile(filepath.Join(r.release, manifestName))
	if err != nil {
		r.t.Fatal(err)
	}
	sb, err := os.ReadFile(filepath.Join(r.release, signatureName))
	if err != nil {
		r.t.Fatal(err)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sb)))
	if err != nil {
		r.t.Fatal(err)
	}
	if !ed25519.Verify(r.pub, mb, sig) {
		r.t.Fatal("the manifest's signature doesn't verify")
	}
	tampered := append([]byte{}, mb...)
	tampered[len(tampered)/2] ^= 1
	if ed25519.Verify(r.pub, tampered, sig) {
		r.t.Fatal("a changed manifest verified")
	}
	var m Manifest
	if err := json.Unmarshal(mb, &m); err != nil {
		r.t.Fatal(err)
	}
	return &m
}

// whole decodes an entry's whole file and checks it against the manifest.
func (r *rig) whole(e *Entry) []byte {
	r.t.Helper()
	z, err := os.ReadFile(filepath.Join(r.release, e.File))
	if err != nil {
		r.t.Fatal(err)
	}
	d, _ := zstd.NewReader(nil, zstd.WithDecoderMaxMemory(appLimit(e.Size)))
	defer d.Close()
	text, err := d.DecodeAll(z, nil)
	if err != nil {
		r.t.Fatal(err)
	}
	checkEntry(r.t, e, text)
	return text
}

// appLimit is the decoder's memory limit as the app should set it: the text's size, but never
// below zstd's smallest window (1 KB), which even a tiny list's frame declares.
func appLimit(size int64) uint64 { return uint64(max(size, 64<<10)) }

func checkEntry(t *testing.T, e *Entry, text []byte) {
	t.Helper()
	sum := sha256.Sum256(text)
	if hex.EncodeToString(sum[:]) != e.SHA256 || int64(len(text)) != e.Size {
		t.Fatalf("%s: the text doesn't match the manifest's sha256 and size", e.File)
	}
}

// applyPatch is what the app does: the old text as the raw dictionary.
func applyPatch(t *testing.T, patch, old []byte, size int64) []byte {
	t.Helper()
	d, err := zstd.NewReader(nil, zstd.WithDecoderDictRaw(0, old), zstd.WithDecoderMaxMemory(appLimit(size)))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	out, err := d.DecodeAll(patch, nil)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPublishing(t *testing.T) {
	r := newRig(t)
	changed, out, m := r.publish()
	if !changed || m.Format != 1 || len(m.Lists) != 3 || m.Resources["resources"] == nil {
		t.Fatalf("first run: changed %v, %d lists, resources %v", changed, len(m.Lists), m.Resources != nil)
	}
	if got := string(r.whole(m.Lists["b"])); !strings.Contains(got, "||ads.example^\n||tracker.example^\n") || strings.Contains(got, "localhost") {
		t.Fatalf("the hosts list became %q", got)
	}
	if got := string(r.whole(m.Lists["c"])); !strings.HasSuffix(got, "$third-party\n\n##.c2\n") {
		t.Fatalf("c's sources weren't joined with a newline: ...%q", got[len(got)-40:])
	}
	var res []map[string]any
	if err := json.Unmarshal(r.whole(m.Resources["resources"]), &res); err != nil || len(res) != 2 || res[1]["name"] != "pb-test.js" {
		t.Fatalf("resources: %v %v", res, err)
	}
	if !strings.HasPrefix(m.Resources["resources"].File, "resources-") || !strings.HasSuffix(m.Resources["resources"].File, ".json.zst") {
		t.Fatalf("resources file %s", m.Resources["resources"].File)
	}
	lic, _ := os.ReadFile(filepath.Join(out, licencesName))
	if !strings.Contains(string(lic), "C (c)\n  Licence: MIT\n") {
		t.Fatalf("LICENSES.txt:\n%s", lic)
	}
	first := r.whole(m.Lists["a"])
	firstB, firstC := m.Lists["b"], m.Lists["c"]

	// a changes; b doesn't; one of c's sources shrinks to a stub.
	r.src.set("/a.txt", bigList("a", 1))
	r.src.set("/c1.txt", "! nothing\n")
	changed, out, m = r.publish()
	if !changed {
		t.Fatal("second run: nothing changed")
	}
	names := dir(t, out)
	a := m.Lists["a"]
	if len(a.Patches) != 1 || len(a.History) != 1 || a.Changed != r.clock.Format(time.RFC3339) {
		t.Fatalf("a: %+v", a)
	}
	patch := a.Patches[a.History[0]]
	if !names[a.File] || !names[patch] || !strings.HasPrefix(patch, "a-"+a.History[0][:16]+"-"+a.SHA256[:16]) {
		t.Fatalf("a's new files: %v", names)
	}
	pb, _ := os.ReadFile(filepath.Join(out, patch))
	second := applyPatch(t, pb, first, a.Size)
	checkEntry(t, a, second)
	if string(second) != bigList("a", 1) || string(r.whole(a)) != bigList("a", 1) {
		t.Fatal("a's patch or whole file isn't the new text")
	}
	wz, _ := os.ReadFile(filepath.Join(out, a.File))
	if len(pb)*4 > len(wz) {
		t.Fatalf("a's patch is %d bytes, its whole file %d: no smaller", len(pb), len(wz))
	}
	if m.Lists["b"].SHA256 != firstB.SHA256 || m.Lists["b"].File != firstB.File {
		t.Fatal("b changed")
	}
	if m.Lists["c"].SHA256 != firstC.SHA256 {
		t.Fatal("c took a source that shrank by more than half")
	}
	for n := range names {
		if strings.HasPrefix(n, "b-") || strings.HasPrefix(n, "c-") || strings.HasPrefix(n, "resources-") {
			t.Fatalf("an unchanged list got a new file: %s", n)
		}
	}

	// Nothing changes: nothing new, and the manifest stays as it was.
	before, _ := os.ReadFile(filepath.Join(r.release, manifestName))
	changed, out, _ = r.publish()
	after, _ := os.ReadFile(filepath.Join(r.release, manifestName))
	if changed || len(dir(t, out)) != 0 || string(before) != string(after) {
		t.Fatalf("a run with nothing new: changed %v, files %v", changed, dir(t, out))
	}

	// Ten more versions of a: at most eight patches, each from a version an app may have.
	texts := map[string][]byte{a.SHA256: second, a.History[0]: first}
	for v := 2; v <= 11; v++ {
		r.src.set("/a.txt", bigList("a", v))
		_, _, m = r.publish()
		a = m.Lists["a"]
		texts[a.SHA256] = []byte(bigList("a", v))
	}
	if len(a.History) != maxHistory || len(a.Patches) != maxHistory {
		t.Fatalf("a has %d versions and %d patches, not %d", len(a.History), len(a.Patches), maxHistory)
	}
	for _, h := range a.History {
		pb, err := os.ReadFile(filepath.Join(r.release, a.Patches[h]))
		if err != nil {
			t.Fatal(err)
		}
		checkEntry(t, a, applyPatch(t, pb, texts[h], a.Size))
	}
	if texts[a.History[0]] == nil || string(texts[a.History[0]]) != bigList("a", 10) {
		t.Fatal("history isn't newest first")
	}
	// The whole file, the eight patches and the archive of earlier versions: no more.
	n := 0
	for name := range dir(t, r.release) {
		if strings.HasPrefix(name, "a-") {
			n++
		}
	}
	if n != 1+maxHistory+1 {
		t.Fatalf("the release keeps %d files for a", n)
	}
}

func TestFirstFailureLeavesTheListOut(t *testing.T) {
	r := newRig(t)
	r.src.set("/b.hosts", "<!DOCTYPE html><html>sign in</html>")
	delete(r.src.body, "/c2.txt")
	_, _, m := r.publish()
	if _, ok := m.Lists["b"]; ok {
		t.Fatal("a web page became list b")
	}
	if _, ok := m.Lists["c"]; ok {
		t.Fatal("c was published without one of its sources")
	}
	if _, ok := m.Lists["a"]; !ok {
		t.Fatal("a is missing")
	}
}

func TestHostsToRules(t *testing.T) {
	got := string(hostsToRules([]byte("# c\n! c\n0.0.0.0 A.Example\n::1 localhost\n127.0.0.1 localhost.localdomain\nbare.example\n1.2.3.4 x.example y.example\n0.0.0.0\nbad/host.example\n"), "u"))
	want := "! Made from the hosts file u, each host as ||host^\n||a.example^\n||bare.example^\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

// The catalogue in the repo is one this program takes.
func TestRepoCatalog(t *testing.T) {
	c, err := loadCatalog("../../core/blocking/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	order := []string{"ads", "trackers", "regional", "annoyances", "extreme", "malware", "phishing", "test"}
	for i, g := range c.Groups {
		if i >= len(order) || g.ID != order[i] {
			t.Fatalf("group %d is %s", i, g.ID)
		}
	}
	for _, l := range c.Lists {
		if l.Group == "test" && l.On {
			t.Fatalf("test list %s is on", l.ID)
		}
	}
}

func dir(t *testing.T, path string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}
