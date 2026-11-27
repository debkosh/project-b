package sparks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// embedded is a system spark's file as the app holds it.
func embedded(t *testing.T, id string) string {
	t.Helper()
	b, err := builtinFS.ReadFile("builtin/" + id + ".user.js")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The system sparks are the app's three, in order, each off on a new profile, each with a
// full header, outside the person's list.
func TestTheSystemSparksAreTheApps(t *testing.T) {
	m := New(t.TempDir())
	var names []string
	for _, s := range m.System() {
		names = append(names, s.Name)
		if s.On || !s.System || s.Error != "" {
			t.Fatalf("%+v", s)
		}
		src, ok := m.Source(s.ID)
		meta, _ := Parse(src)
		if !ok || src != embedded(t, s.ID) || meta.Description == "" || meta.Version == "" || meta.Namespace == "" {
			t.Fatalf("%s's source %v, header %+v", s.ID, ok, meta)
		}
	}
	if !slices.Equal(ids(m.System()), []string{"video-downloader", "screen-time-limit", "page-cleanup"}) ||
		!slices.Equal(names, []string{"Video downloader", "Screen time limit", "Page cleanup"}) {
		t.Fatalf("%v %v", ids(m.System()), names)
	}
	if len(m.List()) != 0 {
		t.Fatalf("the person's list %+v", m.List())
	}
	if m.FileName("video-downloader") != "Video downloader.user.js" {
		t.Fatal(m.FileName("video-downloader"))
	}
}

// An update: a profile saved by an older app, with a system spark on and even a file of
// that spark's id in its folder, runs the app's own code, and its switch is the profile's.
func TestASystemSparkRunsTheAppsCodeAfterAnUpdate(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sparks"), 0o700)
	old := script("@name Video downloader", "@version 1.0", "@match <all_urls>")
	os.WriteFile(filepath.Join(dir, "sparks", "video-downloader.user.js"), []byte(old), 0o600)
	os.WriteFile(filepath.Join(dir, "sparks", "sparks.json"),
		[]byte(`{"version":2,"sparks":[],"system":{"video-downloader":true,"page-cleanup":false,"gone":true}}`), 0o600)
	m := New(dir)
	scripts := m.Scripts("https://videos.example/watch", true)
	if len(scripts) != 1 || scripts[0].ID != "video-downloader" || scripts[0].Source != embedded(t, "video-downloader") {
		t.Fatalf("%+v", scripts)
	}
	if src, _ := m.Source("video-downloader"); src != embedded(t, "video-downloader") {
		t.Fatal("Source is the profile's file")
	}
	// It is matched by its own header: not on a plain http page.
	if got := m.Scripts("http://videos.example/watch", true); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
	if got := m.Matching("https://videos.example/", true); len(got) != 3 || !got[0].On || got[2].On {
		t.Fatalf("matching %+v", got)
	}
}

// A system spark's switch is kept in the profile and read back; the person's sparks come
// first wherever both are.
func TestASystemSparksSwitchLasts(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	own, _ := m.Add(script("@name Own", "@match <all_urls>"))
	m.SetOn("page-cleanup", true)
	if k := readKept(t, dir); !k.System["page-cleanup"] || k.System["video-downloader"] {
		t.Fatalf("%+v", k)
	}
	again := New(dir)
	if s := again.System(); s[2].ID != "page-cleanup" || !s[2].On || s[0].On || s[1].On {
		t.Fatalf("%+v", s)
	}
	if got := again.Scripts("https://a.example/", true); len(got) != 2 || got[0].ID != own.ID || got[1].ID != "page-cleanup" {
		t.Fatalf("%+v", got)
	}
	again.SetOn("page-cleanup", false)
	if got := New(dir).Scripts("https://a.example/", true); len(got) != 1 {
		t.Fatalf("off: %+v", got)
	}
}

// A system spark's source can't be changed, nor can it be deleted.
func TestASystemSparkIsNotEditedOrDeleted(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetOn("screen-time-limit", true)
	if err := m.Update("screen-time-limit", script("@name Mine", "@match <all_urls>")); err != errSystem {
		t.Fatal(err)
	}
	m.Remove("screen-time-limit")
	again := New(dir)
	if s := again.System(); len(s) != 3 || s[1].Name != "Screen time limit" || !s[1].On {
		t.Fatalf("%+v", s)
	}
	if src, _ := again.Source("screen-time-limit"); src != embedded(t, "screen-time-limit") {
		t.Fatal("its source changed")
	}
	if _, err := os.Stat(filepath.Join(dir, "sparks", "screen-time-limit.user.js")); !os.IsNotExist(err) {
		t.Fatal("a file was kept for it", err)
	}
}

// Duplicate keeps the person's own copy, on, named "<Name> copy" in its header too, and
// turns the system spark off, so a page gets only the copy.
func TestDuplicateMakesItYours(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetOn("video-downloader", true)
	c, err := m.Duplicate("video-downloader")
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "Video downloader copy" || !c.On || c.System {
		t.Fatalf("%+v", c)
	}
	src, _ := m.Source(c.ID)
	want := strings.Replace(embedded(t, "video-downloader"), "// @name         Video downloader\n", "// @name         Video downloader copy\n", 1)
	if src != want {
		t.Fatal("the copy's source isn't the system spark's with its name changed")
	}
	again := New(dir)
	if again.System()[0].On || len(again.List()) != 1 || again.List()[0].ID != c.ID {
		t.Fatalf("%+v %+v", again.System(), again.List())
	}
	if got := again.Scripts("https://a.example/", true); len(got) != 1 || got[0].ID != c.ID {
		t.Fatalf("%+v", got)
	}
	if err := again.Update(c.ID, script("@name Mine now", "@match <all_urls>")); err != nil {
		t.Fatal(err)
	}
	// A second copy takes the next free name.
	if c2, _ := again.Duplicate("video-downloader"); c2.Name != "Video downloader copy" {
		t.Fatalf("%+v", c2)
	}
	if c3, _ := again.Duplicate("video-downloader"); c3.Name != "Video downloader copy 2" {
		t.Fatalf("%+v", c3)
	}
	if _, err := again.Duplicate("nothing"); err == nil {
		t.Fatal("duplicated nothing")
	}
}

// An import of a spark with a system spark's name is added as the person's own, as
// "<Name> 2", with no question; the system spark's own file is already here.
func TestAnImportNamedAsASystemSparkIsRenamed(t *testing.T) {
	m := New(t.TempDir())
	found, skipped := Unpack([]Picked{
		{Name: "vd.user.js", Data: []byte(script("@name Video downloader", "@namespace https://github.com/sudiptadeb/project-b", "@match https://mine.example/*"))},
		{Name: "pc.user.js", Data: []byte(embedded(t, "page-cleanup"))},
	})
	im := m.Prepare(found, skipped)
	if len(im.Clashes()) != 0 {
		t.Fatalf("clashes %+v", im.Clashes())
	}
	got := m.Keep(im, nil)
	if !slices.Equal(got.Renamed, []Renamed{{From: "Video downloader", To: "Video downloader 2"}}) ||
		!slices.Equal(got.Unchanged, []string{"Page cleanup"}) || len(got.Added)+len(got.Replaced) != 0 {
		t.Fatalf("%+v", got)
	}
	l := m.List()
	if len(l) != 1 || l[0].Name != "Video downloader 2" || !l[0].On || l[0].Match[0] != "https://mine.example/*" {
		t.Fatalf("%+v", l)
	}
	if src, _ := m.Source("video-downloader"); src != embedded(t, "video-downloader") {
		t.Fatal("the system spark changed")
	}
}

// The system sparks don't count toward the 25, and an export of every spark has them.
func TestSystemSparksDontCount(t *testing.T) {
	m := New(t.TempDir())
	for i := range MaxSparks {
		if _, err := m.Add(script("@name S"+string(rune('a'+i)), "@match <all_urls>")); err != nil {
			t.Fatal(i, err)
		}
	}
	if _, err := m.Add(script("@name More", "@match <all_urls>")); err == nil {
		t.Fatal("a 26th was kept")
	}
	m.SetOn("page-cleanup", true)
	if len(m.System()) != 3 || len(m.Scripts("https://a.example/", true)) != MaxSparks+1 {
		t.Fatalf("%d scripts", len(m.Scripts("https://a.example/", true)))
	}
	if _, n := m.Zip(nil); n != MaxSparks+3 {
		t.Fatalf("zip of %d", n)
	}
	// A full list has no room for a copy.
	if _, err := m.Duplicate("page-cleanup"); err == nil || !m.System()[2].On {
		t.Fatal("duplicated with no room, or turned it off", err)
	}
}

// sparks.json from before system sparks, with the ids of the built-in sparks' copies it
// was given, still loads: the copies are the person's own, to edit and delete, and the
// list is written without them noted.
func TestAnOldBuiltinsListStillLoads(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sparks"), 0o700)
	copied := strings.Replace(embedded(t, "video-downloader"), "@version      1.3", "@version      1.0", 1)
	os.WriteFile(filepath.Join(dir, "sparks", "0000000a.user.js"), []byte(copied), 0o600)
	os.WriteFile(filepath.Join(dir, "sparks", "sparks.json"),
		[]byte(`{"version":2,"sparks":[{"id":"0000000a","on":true,"added":1,"modified":1}],"builtins":["0000000a"]}`), 0o600)
	m := New(dir)
	l := m.List()
	if len(l) != 1 || l[0].Name != "Video downloader" || !l[0].On || l[0].System {
		t.Fatalf("%+v", l)
	}
	if s := m.System(); len(s) != 3 || s[0].On {
		t.Fatalf("%+v", s)
	}
	if err := m.Update("0000000a", script("@name Video downloader", "@match <all_urls>")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "sparks", "sparks.json"))
	if strings.Contains(string(b), "builtins") {
		t.Fatalf("%s", b)
	}
	m.Remove("0000000a")
	if len(New(dir).List()) != 0 {
		t.Fatal("the copy wasn't deleted")
	}
}
