package sparks

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// zipOf is a zip of the named files.
func zipOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		fw, err := w.Create(files[i])
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(files[i+1]))
	}
	w.Close()
	return buf.Bytes()
}

func names(found []Found) []string {
	var out []string
	for _, f := range found {
		out = append(out, f.meta.Name)
	}
	return out
}

func TestUnpackTakesFilesAndZips(t *testing.T) {
	a := script("@name A", "@match <all_urls>")
	b := script("@name B", "@match <all_urls>")
	c := script("@name C", "@match <all_urls>")
	found, skipped := Unpack([]Picked{
		{Name: "a.user.js", Data: []byte(a)},
		{Name: "Sparks.ZIP", Data: zipOf(t,
			"b.user.js", b,
			"deep/c.user.js", c,
			"__MACOSX/._b.user.js", "\x00\x05junk",
			".hidden/x.user.js", a,
			"README.md", "# hi",
			"broken.user.js", "no header",
		)},
		{Name: "notes.txt", Data: []byte("hello")},
		{Name: "bad.zip", Data: []byte("not a zip")},
	})
	if got := names(found); !slices.Equal(got, []string{"A", "B", "C"}) {
		t.Fatalf("found %v", got)
	}
	want := []Skipped{
		{File: "Sparks.ZIP › broken.user.js", Reason: "The script has no ==UserScript== header."},
		{File: "notes.txt", Reason: "The script has no ==UserScript== header."},
		{File: "bad.zip", Reason: "The file is not a zip that can be read."},
	}
	if !slices.Equal(skipped, want) {
		t.Fatalf("skipped %+v", skipped)
	}
}

func TestUnpackSkipsWhatIsTooLarge(t *testing.T) {
	big := script("@name Big", "@match <all_urls>") + strings.Repeat("x", MaxSource)
	_, skipped := Unpack([]Picked{
		{Name: "s.zip", Data: zipOf(t, "big.user.js", big)},
		{Name: "huge.js", Data: make([]byte, MaxPicked+1)},
		{Name: "huge.zip", Size: MaxPicked + 1},
	})
	if len(skipped) != 3 || skipped[2].Reason != skipped[1].Reason || skipped[0].Reason != "The script is larger than 140 KB." || skipped[1].Reason != "The file is larger than 8 MB." {
		t.Fatalf("%+v", skipped)
	}
}

func TestImportAddsNewSparks(t *testing.T) {
	m := New(t.TempDir())
	changes := 0
	m.OnChanged(func() { changes++ })
	found, skipped := Unpack([]Picked{
		{Name: "a.user.js", Data: []byte(script("@name A", "@match <all_urls>"))},
		{Name: "b.user.js", Data: []byte(script("@name B", "@match <all_urls>"))},
	})
	im := m.Prepare(found, skipped)
	if len(im.Clashes()) != 0 {
		t.Fatalf("clashes %+v", im.Clashes())
	}
	got := m.Keep(im, nil)
	if !slices.Equal(got.Added, []string{"A", "B"}) || len(got.Replaced)+len(got.Renamed)+len(got.Skipped)+len(got.Unchanged) != 0 {
		t.Fatalf("%+v", got)
	}
	if changes != 1 {
		t.Fatalf("told of %d changes, want 1", changes)
	}
	again := New(m.dir[:len(m.dir)-len("/sparks")])
	if l := again.List(); len(l) != 2 || !l[0].On || !l[1].On {
		t.Fatalf("kept %+v", l)
	}
}

func TestImportSettlesClashes(t *testing.T) {
	m := New(t.TempDir())
	keep := func(src string) string {
		s, err := m.Add(src)
		if err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	same := keep(script("@name Same", "@match <all_urls>"))
	rep := keep(script("@name Rep", "@version 1", "@match <all_urls>"))
	ren := keep(script("@name Ren", "@match <all_urls>"))
	ign := keep(script("@name Ign", "@match <all_urls>"))
	other := keep(script("@name Rep", "@namespace elsewhere", "@match <all_urls>"))
	m.SetOn(rep, false)

	found, skipped := Unpack([]Picked{{Name: "all.zip", Data: zipOf(t,
		"same.user.js", script("@name Same", "@match <all_urls>"),
		"rep.user.js", script("@name Rep", "@version 2", "@match https://new.example/*"),
		"ren.user.js", script("@name Ren", "@match https://new.example/*"),
		"ign.user.js", script("@name Ign", "@match https://new.example/*"),
		"twice.user.js", script("@name Ren", "@match https://twice.example/*"),
	)}})
	im := m.Prepare(found, skipped)
	clashes := im.Clashes()
	if len(clashes) != 3 || clashes[0].Name != "Rep" || clashes[0].Version != "2" || clashes[0].KeptVersion != "1" ||
		clashes[1].RenameTo != "Ren 2" || clashes[2].Name != "Ign" {
		t.Fatalf("clashes %+v", clashes)
	}
	got := m.Keep(im, map[int]Choice{clashes[0].Index: Replace, clashes[1].Index: Rename})
	if !slices.Equal(got.Replaced, []string{"Rep"}) || !slices.Equal(got.Renamed, []Renamed{{From: "Ren", To: "Ren 2"}}) ||
		len(got.Added) != 0 || !slices.Equal(got.Ignored, []string{"Ign"}) || !slices.Equal(got.Unchanged, []string{"Same"}) {
		t.Fatalf("%+v", got)
	}
	if !slices.Equal(got.Skipped, []Skipped{{File: "all.zip › twice.user.js", Name: "Ren", Reason: "in the import twice, the first one used"}}) {
		t.Fatalf("skipped %+v", got.Skipped)
	}

	list := m.List()
	if got := ids(list)[:5]; !slices.Equal(got, []string{same, rep, ren, ign, other}) {
		t.Fatalf("order %v", got)
	}
	// Replaced in place: its id, and off as it was.
	if r := list[1]; r.Name != "Rep" || r.On || r.Match[0] != "https://new.example/*" {
		t.Fatalf("replaced %+v", r)
	}
	if r := list[4]; r.Match[0] != "<all_urls>" {
		t.Fatalf("the other namespace's changed: %+v", r)
	}
	if r := list[5]; r.Name != "Ren 2" || !r.On {
		t.Fatalf("renamed %+v", r)
	}
	if src, _ := m.Source(list[5].ID); !strings.Contains(src, "// @name Ren 2\n") {
		t.Fatalf("renamed source %q", src)
	}
	if r := list[3]; r.Match[0] != "<all_urls>" {
		t.Fatalf("ignored one changed: %+v", r)
	}
}

func TestImportStopsAtTheLimit(t *testing.T) {
	m := New(t.TempDir())
	var files []string
	for i := 0; i < MaxSparks+1; i++ {
		files = append(files, string(rune('a'+i))+".user.js", script("@name "+string(rune('A'+i)), "@match <all_urls>"))
	}
	found, skipped := Unpack([]Picked{{Name: "many.zip", Data: zipOf(t, files...)}})
	got := m.Keep(m.Prepare(found, skipped), nil)
	if len(got.Added) != MaxSparks || len(got.Skipped) != 1 || got.Skipped[0].Reason != "You can keep up to 25 sparks." {
		t.Fatalf("%d added, skipped %+v", len(got.Added), got.Skipped)
	}
}

func TestRenamedKeepsTheLine(t *testing.T) {
	src := "// ==UserScript==\r\n//   @name\t  Old Name  \r\n// @name Second\r\n// ==/UserScript==\r\n// @name After\r\n"
	want := "// ==UserScript==\r\n//   @name\t  New  \r\n// @name Second\r\n// ==/UserScript==\r\n// @name After\r\n"
	if got := renamed(src, "New"); got != want {
		t.Fatalf("got %q", got)
	}
	long := Meta{Name: strings.Repeat("é", MaxName)}
	if got := freeName(long, map[string]bool{}); len([]rune(got)) != MaxName || !strings.HasSuffix(got, " 2") {
		t.Fatalf("got %q", got)
	}
}

func TestZipReadsBack(t *testing.T) {
	m := New(t.TempDir())
	a, _ := m.Add(script("@name Twin", "@match <all_urls>"))
	m.Add(script("@name twin", "@namespace x", "@match <all_urls>"))
	c, _ := m.Add(script("@name A/B", "@match <all_urls>"))

	data, n := m.Zip(nil)
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || n != 3 {
		t.Fatal(n, err)
	}
	var files []string
	for _, f := range r.File {
		files = append(files, f.Name)
	}
	if !slices.Equal(files, []string{"Twin.user.js", "twin (2).user.js", "A-B.user.js"}) {
		t.Fatalf("files %q", files)
	}
	found, skipped := Unpack([]Picked{{Name: "sparks.zip", Data: data}})
	if len(found) != 3 || len(skipped) != 0 {
		t.Fatalf("read back %v %+v", names(found), skipped)
	}
	if im := New(t.TempDir()).Prepare(found, nil); len(im.Clashes()) != 0 {
		t.Fatal(im.Clashes())
	}
	if im := m.Prepare(found, nil); len(im.found) != 0 || len(im.unchanged) != 3 {
		t.Fatalf("an export imported again is skipped: %+v", im)
	}

	data, n = m.Zip([]string{c.ID, a.ID})
	r, _ = zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if n != 2 || len(r.File) != 2 || r.File[0].Name != "Twin.user.js" {
		t.Fatalf("chosen: %d", n)
	}
}

// The owner's own sparks (testdata/site-sparks, as he wrote them) import from a zip a Mac
// made, and run where their headers say.
func TestHisSparksImportFromAMacZip(t *testing.T) {
	files, err := filepath.Glob("testdata/site-sparks/*.user.js")
	if err != nil || len(files) != 3 {
		t.Fatal(files, err)
	}
	var entries []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(f)
		entries = append(entries, "SiteSparks/"+base, string(b), "__MACOSX/SiteSparks/._"+base, "\x00\x05\x16\x07junk")
	}
	found, skipped := Unpack([]Picked{{Name: "SiteSparks.zip", Data: zipOf(t, entries...)}})
	if len(skipped) != 0 {
		t.Fatalf("skipped %+v", skipped)
	}
	m := New(t.TempDir())
	got := m.Keep(m.Prepare(found, nil), nil)
	if !slices.Equal(got.Added, []string{"Limit Screen Time", "Page Cleanup", "dhd"}) {
		t.Fatalf("%+v", got)
	}
	run := func(url string) []string {
		var out []string
		for _, s := range m.Scripts(url, true) {
			out = append(out, m.find(s.ID).meta.Name)
		}
		return out
	}
	if got := run("https://video.example/watch"); len(got) != 3 {
		t.Fatalf("on https: %v", got)
	}
	if got := run("http://plain.example/"); !slices.Equal(got, []string{"Limit Screen Time", "Page Cleanup"}) {
		t.Fatalf("on http: %v", got)
	}
}
