package sparks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// script is a spark's source with a header of the given lines.
func script(lines ...string) string {
	return "// ==UserScript==\n// " + strings.Join(lines, "\n// ") + "\n// ==/UserScript==\n\nconsole.log('hi');\n"
}

func TestParseReadsTheHeader(t *testing.T) {
	src := "'use strict';\n" + `// ==UserScript==
// @name          My Script
// @namespace     https://example.com
// @description   Does   something
// @match         https://example.com/*
// @match         *://*.example.org/*
// @exclude-match https://example.com/admin/*
// @run-at        document-start
// @grant         none
// @noframes
//@version	1.0
// ==/UserScript==
document.body.remove();
`
	m, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	want := Meta{
		Name:         "My Script",
		Description:  "Does   something",
		Namespace:    "https://example.com",
		Version:      "1.0",
		Match:        []string{"https://example.com/*", "*://*.example.org/*"},
		ExcludeMatch: []string{"https://example.com/admin/*"},
		RunAt:        DocumentStart,
		NoFrames:     true,
	}
	if !equalMeta(m, want) {
		t.Fatalf("got %+v, want %+v", m, want)
	}
}

func TestParseDefaults(t *testing.T) {
	m, err := Parse(strings.ReplaceAll(script("@name\tTabbed", "@match <all_urls>"), "\n", "\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Tabbed" || m.RunAt != DocumentEnd || m.NoFrames || m.Description != "" || len(m.ExcludeMatch) != 0 {
		t.Fatalf("%+v", m)
	}
}

func TestParseTakesTheFirstName(t *testing.T) {
	m, err := Parse(script("@name First", "@name Second", "@match <all_urls>"))
	if err != nil || m.Name != "First" {
		t.Fatalf("%+v %v", m, err)
	}
}

func TestParseRefusesWithASentence(t *testing.T) {
	big := script("@name Big", "@match <all_urls>") + strings.Repeat("x", MaxSource)
	for _, c := range []struct {
		name, src, want string
	}{
		{"no header", "console.log(1)", "no ==UserScript== header"},
		{"unclosed header", "// ==UserScript==\n// @name A\n// @match <all_urls>\n", "no ==UserScript== header"},
		{"header in a string", "var s = '// ==UserScript==';\n// ==/UserScript==", "no ==UserScript== header"},
		{"no name", script("@match <all_urls>"), "no @name"},
		{"empty name", script("@name", "@match <all_urls>"), "no @name"},
		{"no match", script("@name A"), "no @match"},
		{"bad match", script("@name A", "@match example.com"), `@match pattern "example.com"`},
		{"bad exclude", script("@name A", "@match <all_urls>", "@exclude-match ftp://x/*"), `@exclude-match pattern "ftp://x/*"`},
		{"long name", script("@name "+strings.Repeat("n", 51), "@match <all_urls>"), "longer than 50"},
		{"long description", script("@name A", "@description "+strings.Repeat("d", 251), "@match <all_urls>"), "longer than 250"},
		{"too big", big, "larger than 140 KB"},
		{"bad run-at", script("@name A", "@match <all_urls>", "@run-at document-idle"), `@run-at value "document-idle"`},
	} {
		_, err := Parse(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
			continue
		}
		if msg := err.Error(); !regexp.MustCompile(`^[A-Z].*\.$`).MatchString(msg) {
			t.Errorf("%s: %q is not a sentence", c.name, msg)
		}
	}
}

func TestParseTakesTheLimitsExactly(t *testing.T) {
	src := script("@name "+strings.Repeat("é", 50), "@description "+strings.Repeat("d", 250), "@match <all_urls>")
	src += strings.Repeat("x", MaxSource-len(src))
	if _, err := Parse(src); err != nil {
		t.Fatal(err)
	}
}

func equalMeta(a, b Meta) bool {
	return a.Name == b.Name && a.Description == b.Description && slices.Equal(a.Match, b.Match) &&
		slices.Equal(a.ExcludeMatch, b.ExcludeMatch) && a.RunAt == b.RunAt && a.NoFrames == b.NoFrames &&
		a.Namespace == b.Namespace && a.Version == b.Version
}

func ids(list []Spark) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.ID)
	}
	return out
}

func readList(t *testing.T, dir string) []listed {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "sparks", "sparks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var l []listed
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestAMissingFolderIsNoSparks(t *testing.T) {
	m := New(filepath.Join(t.TempDir(), "nowhere"))
	if len(m.List()) != 0 {
		t.Fatal(m.List())
	}
	if m.Matching("https://example.com/", true) != nil {
		t.Fatal("matched")
	}
}

func TestSparksLastAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	changes := 0
	m.OnChanged(func() { changes++ })

	a, err := m.Add(script("@name Alpha", "@match https://a.com/*"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := m.Add(script("@name Beta", "@description Second", "@match https://b.com/*", "@run-at document-start"))
	c, _ := m.Add(script("@name Gamma", "@match https://c.com/*"))
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(a.ID) || a.ID == b.ID || b.ID == c.ID || a.ID == c.ID {
		t.Fatalf("ids %q %q %q", a.ID, b.ID, c.ID)
	}
	if !a.On || a.Name != "Alpha" || a.RunAt != DocumentEnd || a.Error != "" {
		t.Fatalf("%+v", a)
	}
	if _, err := m.Add("nothing here"); err == nil {
		t.Fatal("added a source without a header")
	}
	if changes != 3 {
		t.Fatalf("%d changes", changes)
	}

	newB := script("@name Beta Two", "@match https://b.org/*")
	if err := m.Update(b.ID, newB); err != nil {
		t.Fatal(err)
	}
	if err := m.Update(b.ID, "broken"); err == nil {
		t.Fatal("updated to a broken source")
	}
	if err := m.Update("00000000", newB); err == nil {
		t.Fatal("updated a spark that isn't there")
	}
	m.SetOn(a.ID, false)
	m.SetOn(a.ID, false)
	m.Remove(c.ID)
	m.Remove(c.ID)
	if changes != 6 {
		t.Fatalf("%d changes", changes)
	}
	if _, err := os.Stat(filepath.Join(dir, "sparks", c.ID+".user.js")); !os.IsNotExist(err) {
		t.Fatal("removed spark's file kept")
	}
	if _, ok := m.Source(c.ID); ok {
		t.Fatal("removed spark has a source")
	}
	if l := readList(t, dir); !slices.Equal(l, []listed{{a.ID, false}, {b.ID, true}}) {
		t.Fatalf("sparks.json %+v", l)
	}

	m = New(dir)
	got := m.List()
	if !slices.Equal(ids(got), []string{a.ID, b.ID}) {
		t.Fatalf("order %v", ids(got))
	}
	if got[0].On || got[0].Name != "Alpha" || !got[1].On || got[1].Name != "Beta Two" || got[1].Description != "" {
		t.Fatalf("%+v", got)
	}
	if src, ok := m.Source(b.ID); !ok || src != newB {
		t.Fatalf("source %q %v", src, ok)
	}
	if m.FileName(b.ID) != "Beta Two.user.js" {
		t.Fatal(m.FileName(b.ID))
	}
}

func TestNoMoreThan25(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	for i := range MaxSparks {
		if _, err := m.Add(script("@name S", "@match <all_urls>")); err != nil {
			t.Fatal(i, err)
		}
	}
	_, err := m.Add(script("@name S", "@match <all_urls>"))
	if err == nil || err.Error() != "You can keep up to 25 sparks." {
		t.Fatalf("%v", err)
	}
	if n := len(New(dir).List()); n != MaxSparks {
		t.Fatalf("%d kept", n)
	}
	m.Remove(m.List()[3].ID)
	if _, err := m.Add(script("@name S", "@match <all_urls>")); err != nil {
		t.Fatal(err)
	}
}

func TestABrokenFileStaysListedAndNeverMatches(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	a, _ := m.Add(script("@name Alpha", "@match <all_urls>"))
	b, _ := m.Add(script("@name Beta", "@match <all_urls>"))
	c, _ := m.Add(script("@name Gamma", "@match <all_urls>"))
	broken := "// just some code\n"
	if err := os.WriteFile(filepath.Join(dir, "sparks", a.ID+".user.js"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "sparks", c.ID+".user.js")); err != nil {
		t.Fatal(err)
	}

	m = New(dir)
	got := m.List()
	if !slices.Equal(ids(got), []string{a.ID, b.ID, c.ID}) {
		t.Fatalf("%v", ids(got))
	}
	if got[0].Error == "" || got[0].Name != "" || !got[0].On || got[2].Error == "" || got[1].Error != "" {
		t.Fatalf("%+v", got)
	}
	if !slices.Equal(ids(m.Matching("https://example.com/", true)), []string{b.ID}) {
		t.Fatal("a broken spark matched")
	}
	if src, ok := m.Source(a.ID); !ok || src != broken {
		t.Fatalf("broken source %q %v", src, ok)
	}
	if _, ok := m.Source(c.ID); ok {
		t.Fatal("a missing file has a source")
	}
	if m.FileName(a.ID) != "spark.user.js" {
		t.Fatal(m.FileName(a.ID))
	}

	if err := m.Update(a.ID, script("@name Mended", "@match <all_urls>")); err != nil {
		t.Fatal(err)
	}
	if l := m.List(); l[0].Error != "" || l[0].Name != "Mended" {
		t.Fatalf("%+v", l[0])
	}
	if len(m.Matching("https://example.com/", true)) != 2 {
		t.Fatal("mended spark doesn't match")
	}
}

func TestAListThatDoesntReadIsNoSparks(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "sparks"), 0o700)
	os.WriteFile(filepath.Join(dir, "sparks", "sparks.json"), []byte("{not json"), 0o600)
	if len(New(dir).List()) != 0 {
		t.Fatal("listed")
	}
}

func TestOddIdsInTheListAreLeftOut(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	a, _ := m.Add(script("@name Alpha", "@match <all_urls>"))
	l := []listed{{"../../x", true}, {a.ID, true}, {a.ID, false}, {"ABCDEF12", true}}
	b, _ := json.Marshal(l)
	os.WriteFile(filepath.Join(dir, "sparks", "sparks.json"), b, 0o600)
	got := New(dir).List()
	if len(got) != 1 || got[0].ID != a.ID || !got[0].On {
		t.Fatalf("%+v", got)
	}
}

func TestFileNamesAreSafe(t *testing.T) {
	m := New(t.TempDir())
	for name, want := range map[string]string{
		"Plain":               "Plain.user.js",
		`a/b\c:d*e?f"g<h>i|j`: "a-b-c-d-e-f-g-h-i-j.user.js",
		"...dots...":          "dots.user.js",
		"ünïcødé ok":          "ünïcødé ok.user.js",
		"***":                 "spark.user.js",
	} {
		s, err := m.Add(script("@name "+name, "@match <all_urls>"))
		if err != nil {
			t.Fatal(err)
		}
		if got := m.FileName(s.ID); got != want {
			t.Errorf("%q: %q, want %q", name, got, want)
		}
	}
	if m.FileName("ffffffff") != "spark.user.js" {
		t.Fatal(m.FileName("ffffffff"))
	}
}

func TestMatchingAndScripts(t *testing.T) {
	m := New(t.TempDir())
	all, _ := m.Add(script("@name All", "@match *://*/*", "@exclude-match https://example.com/private/*"))
	top, _ := m.Add(script("@name Top", "@match https://*.example.com/*", "@noframes", "@run-at document-start"))
	off, _ := m.Add(script("@name Off", "@match https://example.com/*"))
	other, _ := m.Add(script("@name Other", "@match https://other.net/*"))
	m.SetOn(off.ID, false)

	for _, c := range []struct {
		url  string
		main bool
		want []string
	}{
		{"https://example.com/", true, []string{all.ID, top.ID, off.ID}},
		{"https://example.com/", false, []string{all.ID, off.ID}},
		{"https://www.example.com/a", true, []string{all.ID, top.ID}},
		{"https://example.com/private/x", true, []string{top.ID, off.ID}},
		{"https://other.net/", false, []string{all.ID, other.ID}},
		{"file:///home/a.html", true, nil},
		{"not an address", true, nil},
	} {
		if got := ids(m.Matching(c.url, c.main)); !slices.Equal(got, c.want) {
			t.Errorf("Matching(%q, %v) = %v, want %v", c.url, c.main, got, c.want)
		}
	}

	scripts := m.Scripts("https://example.com/", true)
	if len(scripts) != 2 || scripts[0].ID != all.ID || scripts[1].ID != top.ID {
		t.Fatalf("%+v", scripts)
	}
	if scripts[1].RunAt != DocumentStart || scripts[0].RunAt != DocumentEnd {
		t.Fatalf("%+v", scripts)
	}
	if src, _ := m.Source(top.ID); scripts[1].Source != src {
		t.Fatal("source")
	}

	if got := m.Scripts("https://example.com/", false); len(got) != 1 || got[0].ID != all.ID {
		t.Fatalf("%+v", got)
	}
}
