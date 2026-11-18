package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"projectb/tones"
)

func TestTabLayoutSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.TabLayout() != TabsVertical {
		t.Fatalf("default = %q", m.TabLayout())
	}
	var got []string
	m.OnTabLayoutChanged(func(l string) { got = append(got, l) })
	m.SetTabLayout("sideways")
	m.SetTabLayout(TabsHorizontal)
	m.SetTabLayout(TabsHorizontal)
	if len(got) != 1 || got[0] != TabsHorizontal {
		t.Fatalf("changes = %v", got)
	}

	// The settings are written off the main thread; Close waits for that.
	m.Close()
	if New(dir).TabLayout() != TabsHorizontal {
		t.Fatal("layout not saved")
	}
}

func TestNoFrameColorIsPickedByDefaultAndOneSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.FrameColor() != tones.None {
		t.Fatalf("default = %q", m.FrameColor())
	}
	var got []string
	m.OnFrameColorChanged(func(c string) { got = append(got, c) })
	m.SetFrameColor("teal")
	m.SetFrameColor("#3470d0")
	m.SetFrameColor("#3470d0")
	if len(got) != 1 || got[0] != "#3470d0" {
		t.Fatalf("changes = %v", got)
	}
	m.Close()
	if New(dir).FrameColor() != "#3470d0" {
		t.Fatal("colour not saved")
	}
}

// A template's values go through the setters, so each listener hears as of a change by
// hand; a setting the values don't hold keeps its value.
func TestApplySetsWhatTheValuesHoldThroughTheSetters(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	m.SetTheme(ThemeDark)
	var heard []string
	m.OnTabLayoutChanged(func(l string) { heard = append(heard, l) })
	m.OnFrameColorChanged(func(c string) { heard = append(heard, c) })
	m.OnThemeChanged(func(th string) { heard = append(heard, th) })
	if err := m.Apply(json.RawMessage(`{"tabLayout":"horizontal","frameColor":"#5b7fd6"}`)); err != nil {
		t.Fatal(err)
	}
	if strings.Join(heard, " ") != "horizontal #5b7fd6" {
		t.Fatalf("heard %v", heard)
	}
	m.Close()
	if got := New(dir).All(); got != (Prefs{TabLayout: TabsHorizontal, FrameColor: "#5b7fd6", Theme: ThemeDark, Shell: "standard", AddressBarPosition: AddressBottom, SearchEngine: "duckduckgo", Start: StartContinue, BookmarksNewTab: true, BookmarksCommandBox: true, BlockAds: true}) {
		t.Fatalf("saved %+v", got)
	}
}

// The shell is a shell's id, standard by default; one the registry doesn't have keeps the
// default, as a value that isn't one does for every setting.
func TestTheShellIsAShellsIDAndSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.Shell() != "standard" {
		t.Fatalf("default = %q", m.Shell())
	}
	var got []string
	m.OnShellChanged(func(id string) { got = append(got, id) })
	m.SetShell("no-such-shell")
	m.SetShell("minimal")
	m.SetShell("minimal")
	m.Close()
	if len(got) != 1 || got[0] != "minimal" || New(dir).Shell() != "minimal" {
		t.Fatalf("changes %v, saved %q", got, New(dir).Shell())
	}
	if err := m.Apply(json.RawMessage(`{"shell":"standard"}`)); err != nil || m.Shell() != "standard" || len(got) != 2 {
		t.Fatalf("apply: %v, shell %q, changes %v", err, m.Shell(), got)
	}
	m.Close()
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"shell":"gone"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if New(dir).Shell() != "standard" {
		t.Fatalf("a shell that isn't one loads as %q", New(dir).Shell())
	}
}

// The address bar is at the foot by default, moves to the top and back, and a position
// that isn't one keeps the default.
func TestTheAddressBarPosition(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	var got []string
	m.OnAddressBarPositionChanged(func(at string) { got = append(got, at) })
	m.SetAddressBarPosition("middle")
	m.SetAddressBarPosition(AddressTop)
	m.SetAddressBarPosition(AddressTop)
	m.Close()
	if m.AddressBarPosition() != AddressTop || len(got) != 1 || New(dir).AddressBarPosition() != AddressTop {
		t.Fatalf("changes %v, saved %q", got, New(dir).AddressBarPosition())
	}
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"addressBarPosition":"side"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if New(dir).AddressBarPosition() != AddressBottom {
		t.Fatalf("a position that isn't one loads as %q", New(dir).AddressBarPosition())
	}
}

// Bookmarks show on the new tab page and in the command box by default, not beside the tabs
// nor in a bar. Each turns on and off by name, as a shell's setPref does, the bar's folder
// is a folder's id or the top, and they survive a restart.
func TestWhereBookmarksShow(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if p := m.All(); !p.BookmarksNewTab || p.BookmarksSidebar || !p.BookmarksCommandBox || p.BookmarksBar || p.BookmarksBarFolder != "" {
		t.Fatalf("defaults %+v", p)
	}
	changes := 0
	m.OnChanged(func(Prefs) { changes++ })
	for _, set := range [][2]string{{"bookmarksSidebar", "true"}, {"bookmarksNewTab", "false"}, {"bookmarksBar", "true"},
		{"bookmarksBar", "yes"}, {"bookmarksBarFolder", "0123456789abcdef"}, {"bookmarksBarFolder", "../x"}} {
		if !m.Set(set[0], set[1]) {
			t.Fatalf("%s is no pref", set[0])
		}
	}
	m.Close()
	p := New(dir).All()
	if changes != 4 || p.BookmarksNewTab || !p.BookmarksSidebar || !p.BookmarksCommandBox || !p.BookmarksBar || p.BookmarksBarFolder != "0123456789abcdef" {
		t.Fatalf("%d changes, saved %+v", changes, p)
	}
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"bookmarksBarFolder":"nope"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if p := New(dir).All(); p.BookmarksBarFolder != "" || !p.BookmarksNewTab {
		t.Fatalf("a folder that isn't an id loads as %+v", p)
	}
}

// The ad blocker is on every site by default; off is kept, and a file from before it was a
// pref has it on.
func TestBlockAds(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if !m.All().BlockAds {
		t.Fatal("the ad blocker is off by default")
	}
	if !m.Set("blockAds", "false") || m.All().BlockAds {
		t.Fatalf("blockAds false: %+v", m.All())
	}
	m.Set("blockAds", "no")
	m.Close()
	if New(dir).All().BlockAds {
		t.Fatal("off wasn't kept")
	}
	if err := os.WriteFile(filepath.Join(dir, "prefs.json"), []byte(`{"theme":"dark"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !New(dir).All().BlockAds {
		t.Fatal("a file without it turns it off")
	}
}

// The search engine is one of the list's, DuckDuckGo by default, and survives a restart.
func TestTheSearchEngineIsOneOfTheList(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if m.SearchEngine().ID != "duckduckgo" {
		t.Fatalf("default = %+v", m.SearchEngine())
	}
	var heard []string
	m.OnSearchEngineChanged(func(id string) { heard = append(heard, id) })
	m.SetSearchEngine("nobody")
	m.SetSearchEngine("google")
	m.SetSearchEngine("google")
	if strings.Join(heard, " ") != "google" {
		t.Fatalf("heard %v", heard)
	}
	m.Close()
	if got := New(dir).SearchEngine(); got.Name != "Google" || got.For("a b") != "https://www.google.com/search?q=a+b" {
		t.Fatalf("after a restart: %+v", got)
	}
}

// A change is on disk at once, written by the writer as soon as it hears of it, not only
// when Close waits for it: an app killed without Stop, as Android's often is, keeps it.
func TestAChangeReachesTheDiskWithoutClose(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	defer m.Close()
	m.SetTheme(ThemeDark)
	deadline := time.Now().Add(time.Second)
	for New(dir).Theme() != ThemeDark {
		if time.Now().After(deadline) {
			t.Fatal("the change wasn't on disk a second later")
		}
		time.Sleep(time.Millisecond)
	}
}
