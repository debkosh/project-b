package blocking_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"projectb/blocking"
	"projectb/blocking/blockingtest"
)

// rig is a lists manager on a folder of its own, its main thread a lock.
type rig struct {
	t       *testing.T
	m       *blocking.Manager
	rel     *blockingtest.Release
	mu      sync.Mutex
	changes int
}

var everyLists = blocking.Choices{Network: true, Safe: true}

func newRig(t *testing.T, dir string, rel *blockingtest.Release, languages ...string) *rig {
	t.Helper()
	return makeRig(t, dir, rel, true, languages)
}

// offline is a rig that checks the manifest on disk with rel's key, and downloads nothing.
func offline(t *testing.T, dir string, rel *blockingtest.Release, languages ...string) *rig {
	t.Helper()
	return makeRig(t, dir, rel, false, languages)
}

func makeRig(t *testing.T, dir string, rel *blockingtest.Release, online bool, languages []string) *rig {
	t.Helper()
	r := &rig{t: t, rel: rel}
	s := blocking.Setup{Dir: dir, Catalogue: blockingtest.Catalogue(), Languages: languages, Hosts: []string{"doubleclick.test", "tracker.test"},
		Post: func(f func()) { r.mu.Lock(); defer r.mu.Unlock(); f() }, Every: time.Hour}
	if rel != nil {
		s.Key = rel.Public
		if online {
			s.BaseURL = rel.URL()
		}
	}
	m, err := blocking.New(s)
	if err != nil {
		t.Fatal(err)
	}
	m.OnRulesChanged(func() { r.changes++ })
	r.m = m
	t.Cleanup(m.Stop)
	return r
}

// main runs f as the main thread.
func (r *rig) main(f func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f()
}

// until waits for ok; what it reads of the main thread's, it reads with main.
func (r *rig) until(what string, ok func() bool) {
	r.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if ok() {
			return
		}
		if time.Now().After(deadline) {
			r.t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (r *rig) blocks(url, source string) bool {
	return r.m.Network().Request(url, source, "script") == blocking.BlockIt
}

// listed is the Lists page's row of a list.
func (r *rig) listed(c blocking.Choices, id string) blocking.ShownList {
	var out blocking.ShownList
	r.main(func() {
		for _, g := range r.m.Shown(c).Groups {
			for _, l := range g.Lists {
				if l.ID == id {
					out = l
				}
			}
		}
	})
	return out
}

func publishFirst(rel *blockingtest.Release) {
	rel.Put("easylist", "||ads.test^\n")
	rel.Put("unbreak", "! nothing yet\n")
	rel.Put("easyprivacy", "||track.test^$third-party\n")
	rel.Put("de", "||werbung.test^\n")
	rel.Put("fr", "||pub.test^\n")
	rel.Put("cookies", "||cookies.test^\n")
	rel.Put("urlhaus", "||harm.test^\n")
	rel.Put("phishing-filter", "|https://steal.test/login|$document\n")
	rel.PutResources("resources", `[]`)
	rel.Publish()
}

// The manifest is taken only signed with the lists' key over its exact bytes, and only
// naming files as the release names them.
func TestTheManifestIsTakenOnlySigned(t *testing.T) {
	rel := blockingtest.New(t)
	publishFirst(rel)
	manifest, sig := rel.Manifest()
	if m, err := blocking.Verify(rel.Public, manifest, sig); err != nil || m.Lists["easylist"].SHA256 != blockingtest.Sum("||ads.test^\n") {
		t.Fatalf("signed: %v %+v", err, m)
	}
	damaged := slices.Clone(manifest)
	damaged[len(damaged)/2] ^= 1
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	otherSig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(other, manifest)))
	badName := []byte(strings.Replace(string(manifest), blockingtest.File("easylist", "||ads.test^\n", ".txt"), "../easylist", 1))
	badSig := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(rel.Key, badName)))
	for what, c := range map[string][2][]byte{
		"damaged":         {damaged, sig},
		"unsigned":        {manifest, nil},
		"not base64":      {manifest, []byte("signed!")},
		"another key":     {manifest, otherSig},
		"a file's folder": {badName, badSig},
	} {
		if _, err := blocking.Verify(rel.Public, c[0], c[1]); err == nil {
			t.Errorf("%s: taken", what)
		}
	}
	// The real key is the one compiled in.
	if len(blocking.PublicKey) != ed25519.PublicKeySize {
		t.Fatal("the compiled-in key")
	}
}

// A first start has no lists: the built-in hosts block third-party requests until the ads
// group's lists come, then give way to them; the lists, the resources and the engines are
// kept in the folder, and a start after loads the engines with no download.
func TestTheFirstStartUsesTheBuiltInHostsUntilTheListsCome(t *testing.T) {
	dir := t.TempDir()
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, dir, rel, "de-DE")
	r.m.Load(everyLists)
	if !r.blocks("https://ads.doubleclick.test/x.js", "https://news.test/") || r.blocks("https://doubleclick.test/x.js", "https://doubleclick.test/") {
		t.Fatal("the built-in hosts")
	}
	if r.blocks("https://ads.test/x.js", "https://news.test/") {
		t.Fatal("a list's rule before any list came")
	}
	if got := r.listed(everyLists, "easylist"); got.Downloaded || !got.On {
		t.Fatalf("before the download: %+v", got)
	}
	r.m.Start()
	r.until("the ads group", func() bool { return r.blocks("https://ads.test/x.js", "https://news.test/") })
	r.until("the regional list", func() bool { return r.blocks("https://werbung.test/x.js", "https://news.test/") })
	r.until("the built-in hosts gone", func() bool { return !r.blocks("https://ads.doubleclick.test/x.js", "https://news.test/") })
	if r.blocks("https://pub.test/x.js", "https://news.test/") || r.blocks("https://cookies.test/x.js", "https://news.test/") {
		t.Fatal("a list that is off by default blocks")
	}
	r.until("Settings' sizes", func() bool { return r.listed(everyLists, "easylist").Size == int64(len("||ads.test^\n")) })
	asked := rel.Asked()
	for _, f := range []string{"manifest.json", "manifest.json.sig", blockingtest.File("easylist", "||ads.test^\n", ".txt"), blockingtest.File("resources", "[]", ".json")} {
		if !slices.Contains(asked, f) {
			t.Errorf("never asked for %s: %v", f, asked)
		}
	}
	if slices.Contains(asked, blockingtest.File("cookies", "||cookies.test^\n", ".txt")) || slices.Contains(asked, blockingtest.File("fr", "||pub.test^\n", ".txt")) {
		t.Errorf("fetched a list that is off: %v", asked)
	}
	for _, f := range []string{"manifest.json", "manifest.json.sig", "texts/easylist-" + blockingtest.Sum("||ads.test^\n") + ".txt", "resources/resources-" + blockingtest.Sum("[]") + ".json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Error(err)
		}
	}
	engines, _ := filepath.Glob(filepath.Join(dir, "engines", "*.engine"))
	if len(engines) != 5 { // ads, trackers, regional; urlhaus, phishing-filter
		t.Fatalf("engines %v", engines)
	}
	r.m.Stop()

	// The next start: the engines load from disk before any download.
	r2 := offline(t, dir, rel, "de-DE")
	r2.m.Load(everyLists)
	if !r2.blocks("https://ads.test/x.js", "https://news.test/") || r2.blocks("https://ads.doubleclick.test/x.js", "https://news.test/") {
		t.Fatal("the next start")
	}
	if _, _, ok := r2.m.Dangerous("https://harm.test/"); !ok {
		t.Fatal("the next start's Safe Browsing")
	}
}

// Safe Browsing asks a page's own address: a host rule and a whole address's rule with
// $document match, and the match names its list and group; off, nothing is asked.
func TestSafeBrowsingNamesTheListThatMatched(t *testing.T) {
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, t.TempDir(), rel)
	r.m.Load(everyLists)
	r.m.Start()
	r.until("the safe lists", func() bool { _, _, ok := r.m.Dangerous("https://steal.test/login"); return ok })
	if l, g, ok := r.m.Dangerous("https://steal.test/login"); !ok || l.Title != "Phishing URL Blocklist" || g.ID != "phishing" {
		t.Fatalf("phishing: %v %v %v", l, g, ok)
	}
	if l, g, ok := r.m.Dangerous("https://www.harm.test/a/b"); !ok || l.ID != "urlhaus" || g.ID != "malware" {
		t.Fatalf("malware: %v %v %v", l, g, ok)
	}
	if _, _, ok := r.m.Dangerous("https://steal.test/"); ok {
		t.Fatal("another page of the phishing host")
	}
	// The network blocker has none of the safe lists' rules.
	if r.blocks("https://harm.test/x.js", "https://news.test/") {
		t.Fatal("a safe list's rule in the network blocker")
	}
	r.main(func() { r.m.SetChoices(blocking.Choices{Network: true}) })
	r.until("Safe Browsing off", func() bool { _, _, ok := r.m.Dangerous("https://harm.test/"); return !ok })
}

// A list that changed comes as its patch from the text there, and the group swaps to the
// new rules, which the engine hears; a later check asks with the ETag and takes nothing.
func TestAChangedListComesAsAPatchAndSwaps(t *testing.T) {
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, t.TempDir(), rel)
	r.m.Load(everyLists)
	r.m.Start()
	r.until("the first lists", func() bool { return r.blocks("https://ads.test/x.js", "https://news.test/") })
	rel.Asked()
	var before int
	r.main(func() { before = r.changes })

	old := "||ads.test^\n"
	next := old + "||more-ads.test^\n"
	rel.Put("easylist", next)
	rel.Publish()
	r.m.Start() // as the hourly check does
	r.until("the patch", func() bool { return r.blocks("https://more-ads.test/x.js", "https://news.test/") })
	asked := rel.Asked()
	if !slices.Contains(asked, blockingtest.Patch("easylist", old, next)) || slices.Contains(asked, blockingtest.File("easylist", next, ".txt")) {
		t.Fatalf("asked %v", asked)
	}
	r.main(func() {
		if r.changes <= before {
			t.Error("the engine wasn't told the rules changed")
		}
	})

	r.m.Start()
	r.until("the next check", func() bool { return len(rel.Asked()) > 0 })
	time.Sleep(20 * time.Millisecond)
	if asked := rel.Asked(); len(asked) != 0 {
		t.Fatalf("after a 304: %v", asked)
	}
}

// The ETag is kept on disk: a start after asks with it.
func TestTheETagOutlivesAStart(t *testing.T) {
	dir := t.TempDir()
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, dir, rel)
	r.m.Load(everyLists)
	r.m.Start()
	r.until("the lists", func() bool { return r.blocks("https://ads.test/x.js", "https://news.test/") })
	r.m.Stop()
	rel.Asked()
	r2 := newRig(t, dir, rel)
	r2.m.Load(everyLists)
	r2.m.Start()
	var asked []string
	r2.until("the check", func() bool { asked = append(asked, rel.Asked()...); return len(asked) > 0 })
	if asked[0] != "manifest.json 304" || len(asked) != 1 {
		t.Fatalf("asked %v", asked)
	}
}

// A patch that doesn't make the manifest's text falls back to the whole file; a whole
// file that isn't the manifest's text is thrown away, and the list keeps the text it had.
func TestABadPatchFallsBackAndADamagedFileIsThrownAway(t *testing.T) {
	dir := t.TempDir()
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, dir, rel)
	r.m.Load(everyLists)
	r.m.Start()
	r.until("the first lists", func() bool { return r.blocks("https://ads.test/x.js", "https://news.test/") })

	old, next := "||ads.test^\n", "||ads.test^\n||second.test^\n"
	rel.Put("easylist", next)
	rel.Publish()
	rel.Set(blockingtest.Patch("easylist", old, next), []byte("not a patch"))
	rel.Asked()
	r.m.Start()
	r.until("the whole file", func() bool { return r.blocks("https://second.test/x.js", "https://news.test/") })
	if asked := rel.Asked(); !slices.Contains(asked, blockingtest.File("easylist", next, ".txt")) {
		t.Fatalf("asked %v", asked)
	}

	third := next + "||third.test^\n"
	rel.Put("easylist", third)
	rel.Publish()
	rel.Set(blockingtest.Patch("easylist", next, third), []byte("not a patch"))
	rel.Set(blockingtest.File("easylist", third, ".txt"), []byte("not zstd"))
	rel.Put("easyprivacy", "||track.test^$third-party\n||track2.test^\n")
	rel.Publish()
	rel.Set(blockingtest.Patch("easylist", next, third), []byte("not a patch"))
	rel.Set(blockingtest.File("easylist", third, ".txt"), []byte("not zstd"))
	r.m.Start()
	r.until("the other list", func() bool { return r.blocks("https://track2.test/x.js", "https://news.test/") })
	if r.blocks("https://third.test/x.js", "https://news.test/") || !r.blocks("https://second.test/x.js", "https://news.test/") {
		t.Fatal("a damaged file was taken, or the text before it lost")
	}
	if _, err := os.Stat(filepath.Join(dir, "texts", "easylist-"+blockingtest.Sum(third)+".txt")); err == nil {
		t.Fatal("a damaged text was kept")
	}
	// A manifest that isn't signed is refused, and the one before stays.
	manifest, _ := rel.Manifest()
	rel.SetManifest(manifest, []byte("bad"))
	r.m.Start()
	time.Sleep(50 * time.Millisecond)
	if !r.blocks("https://track2.test/x.js", "https://news.test/") {
		t.Fatal("an unsigned manifest dropped the lists")
	}
}

// An engine another version of the library compiled doesn't load at the start: its group
// is off, the built-in hosts stand in, and after the start-up wait it is compiled again
// from the texts there.
func TestAnEngineOfAnotherVersionIsCompiledAgain(t *testing.T) {
	dir := t.TempDir()
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, dir, rel)
	r.m.Load(everyLists)
	r.m.Start()
	r.until("the lists", func() bool { return r.blocks("https://ads.test/x.js", "https://news.test/") })
	r.m.Stop()

	was := blocking.PlainVersion
	blocking.PlainVersion = "plain/2\n"
	t.Cleanup(func() { blocking.PlainVersion = was })
	r2 := offline(t, dir, rel)
	r2.m.Load(everyLists)
	if r2.blocks("https://ads.test/x.js", "https://news.test/") || !r2.blocks("https://ads.doubleclick.test/x.js", "https://news.test/") {
		t.Fatal("an engine of another version loaded")
	}
	r2.m.Start()
	r2.until("compiled again", func() bool { return r2.blocks("https://ads.test/x.js", "https://news.test/") })
	r2.until("the built-in hosts gone", func() bool { return !r2.blocks("https://ads.doubleclick.test/x.js", "https://news.test/") })
}

// Turning a list on fetches and compiles it at once, before the start-up wait; turning
// every ads list off takes the built-in hosts with them; Block ads off unloads every
// network group.
func TestTurningAListOnFetchesItAtOnce(t *testing.T) {
	rel := blockingtest.New(t)
	publishFirst(rel)
	r := newRig(t, t.TempDir(), rel)
	r.m.Load(everyLists)
	c := blocking.Choices{Network: true, Safe: true}
	r.main(func() {
		c.Lists = r.m.WithList(c.Lists, "cookies", true)
		r.m.SetChoices(c)
	})
	r.until("the annoyance", func() bool { return r.blocks("https://cookies.test/x.js", "https://news.test/") })
	if asked := rel.Asked(); slices.Contains(asked, blockingtest.File("easylist", "||ads.test^\n", ".txt")) == false {
		t.Fatalf("asked %v", asked)
	}
	r.main(func() {
		c.Lists = r.m.WithGroup(c.Lists, "ads", false)
		r.m.SetChoices(c)
	})
	r.until("the ads group off", func() bool { return !r.blocks("https://ads.test/x.js", "https://news.test/") })
	if r.blocks("https://ads.doubleclick.test/x.js", "https://news.test/") {
		t.Fatal("the built-in hosts with the ads group off")
	}
	r.main(func() { r.m.SetChoices(blocking.Choices{Safe: true, Lists: c.Lists}) })
	r.until("Block ads off", func() bool { return !r.blocks("https://cookies.test/x.js", "https://news.test/") })
}

// A list's default is the catalogue's, or for a regional list whether the app speaks its
// language; the person's changes are kept only where they differ; a group is on while any
// of its lists is, off turns them all off, on brings back its defaults, or every list where
// none is on by default; the test group is never shown.
func TestTheChoicesAndTheirDefaults(t *testing.T) {
	r := newRig(t, t.TempDir(), nil, "pt-BR", "de_DE.UTF-8")
	m := r.m
	cat := m.Catalogue()
	de, _ := cat.List("de")
	fr, _ := cat.List("fr")
	if !m.On(blocking.Choices{}, de) || m.On(blocking.Choices{}, fr) {
		t.Fatal("the regional defaults")
	}
	lists := m.WithList(nil, "fr", true)
	lists = m.WithList(lists, "easylist", false)
	lists = m.WithList(lists, "fr", false)
	if len(lists) != 1 || lists["easylist"] != false {
		t.Fatalf("kept %v", lists)
	}
	lists = m.WithGroup(lists, "ads", false)
	if len(lists) != 2 || lists["easylist"] || lists["unbreak"] {
		t.Fatalf("ads off: %v", lists)
	}
	lists = m.WithGroup(lists, "ads", true)
	if len(lists) != 0 {
		t.Fatalf("ads on again: %v", lists)
	}
	if got := m.WithGroup(nil, "annoyances", true); !got["cookies"] {
		t.Fatalf("annoyances on: %v", got)
	}
	if got := m.WithGroup(m.WithList(nil, "fr", true), "regional", true); len(got) != 0 {
		t.Fatalf("regional on: %v", got)
	}

	r.m.Load(blocking.Choices{Network: true, Safe: true})
	var shown blocking.Shown
	c := blocking.Choices{Network: true, Safe: true, Lists: map[string]bool{"easylist": false}}
	r.main(func() { shown = m.Shown(c) })
	var ids []string
	for _, g := range shown.Groups {
		ids = append(ids, g.ID)
	}
	if !slices.Equal(ids, []string{"ads", "trackers", "regional", "annoyances", "malware", "phishing"}) || shown.Downloaded {
		t.Fatalf("groups %v, downloaded %v", ids, shown.Downloaded)
	}
	ads, regional, annoyances := shown.Groups[0], shown.Groups[2], shown.Groups[3]
	if !ads.On || ads.OnCount != 1 || ads.Total != 2 || ads.Lists[0].On || ads.Lists[0].Home != "https://easylist.test/" {
		t.Fatalf("ads %+v", ads)
	}
	if !regional.On || regional.OnCount != 1 || annoyances.On || annoyances.OnCount != 0 || annoyances.Total != 1 {
		t.Fatalf("regional %+v, annoyances %+v", regional, annoyances)
	}
}

// Plain reads the rules the tests use as adblock-rust does: a host rule holds for a page's
// own address, a third-party one doesn't, and a whole address only with $document.
func TestPlainReadsTheTestsRules(t *testing.T) {
	lib := blocking.Plain{}
	b := lib.New()
	engine, _ := lib.Compile([][]byte{[]byte("||ads.test^$third-party\n||harm.test^\n|https://steal.test/login|$document\n|https://cdn.test/x.js|\n! a comment\n")})
	if !b.Load("test", engine) {
		t.Fatal("load")
	}
	for _, c := range []struct {
		url, source, kind string
		want              int
	}{
		{"https://ads.test/a.js", "https://site.test/", "script", blocking.BlockIt},
		{"https://ads.test/a.js", "https://www.ads.test/", "script", blocking.Allow},
		{"https://harm.test/", "https://harm.test/", "document", blocking.BlockIt},
		{"https://steal.test/login", "https://steal.test/login", "document", blocking.BlockIt},
		{"https://steal.test/", "https://steal.test/", "document", blocking.Allow},
		{"https://cdn.test/x.js", "https://cdn.test/x.js", "document", blocking.Allow},
		{"https://cdn.test/x.js", "https://site.test/", "script", blocking.BlockIt},
	} {
		if got := b.Request(c.url, c.source, c.kind); got != c.want {
			t.Errorf("%s from %s as %s: %d", c.url, c.source, c.kind, got)
		}
	}
	if b.Load("test", []byte("plain/0\n")) {
		t.Fatal("another version's engine loaded")
	}
}

// The catalogue the app carries reads, and its test group's lists go to the blocker their
// ids say.
func TestTheEmbeddedCatalogueReads(t *testing.T) {
	c, err := blocking.Embedded()
	if err != nil {
		t.Fatal(err)
	}
	g, ok := c.Group("test")
	if !ok || g.Shown {
		t.Fatalf("the test group: %+v", g)
	}
	for id, want := range map[string]string{"pb-test": blocking.Network, "pb-test-malware": blocking.Safe, "pb-test-phishing": blocking.Safe, "easylist": blocking.Network, "urlhaus": blocking.Safe} {
		l, ok := c.List(id)
		if !ok || c.BlockerOf(l) != want {
			t.Errorf("%s: %v in %s", id, ok, c.BlockerOf(l))
		}
	}
	for id, want := range map[string]string{"pb-test-phishing": "phishing", "phishing-filter": "phishing", "pb-test-malware": "malware", "urlhaus": "malware"} {
		if l, _ := c.List(id); c.KindOf(l) != want {
			t.Errorf("%s warns of %s", id, c.KindOf(l))
		}
	}
	if l, _ := c.List("pb-test"); blocking.Default(l, []string{"en"}) {
		t.Fatal("the test list is on by default")
	}
}
