package sites

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"projectb/engine"
)

type fakeTabs map[engine.TabID]string

func (f fakeTabs) URLOf(tab engine.TabID) (string, bool) { u, ok := f[tab]; return u, ok }

type answers map[engine.PermissionID][]engine.PermissionStatus

const (
	G = engine.PermissionGranted
	D = engine.PermissionDenied
	A = engine.PermissionAsk
)

func setup(t *testing.T, dir string) (*Manager, fakeTabs, answers) {
	t.Helper()
	tabs := fakeTabs{1: "https://meet.test/room", 2: "https://news.test/"}
	got := answers{}
	m := New(dir, tabs, func(id engine.PermissionID, s []engine.PermissionStatus) { got[id] = s })
	return m, tabs, got
}

func ask(m *Manager, id engine.PermissionID, tab engine.TabID, origin string, gesture bool, kinds ...string) {
	m.Request(engine.PermissionRequest{ID: id, Tab: tab, Origin: origin, Kinds: kinds, Gesture: gesture})
}

func want(t *testing.T, got answers, id engine.PermissionID, s ...engine.PermissionStatus) {
	t.Helper()
	if !reflect.DeepEqual(got[id], s) {
		t.Fatalf("request %d answered %v, want %v", id, got[id], s)
	}
}

func TestAPromptAllowedIsKeptForTheSiteAcrossARestart(t *testing.T) {
	dir := t.TempDir()
	m, _, got := setup(t, dir)
	ask(m, 1, 1, "https://meet.test", true, Camera, Microphone)
	p := m.Prompts()
	if len(p) != 1 || p[0].Tab != 1 || !reflect.DeepEqual(p[0].Kinds, []string{Camera, Microphone}) || !p[0].Once {
		t.Fatalf("prompts %+v", p)
	}
	if got[1] != nil {
		t.Fatal("answered before the person did")
	}
	m.Answer(p[0].ID, Allow)
	want(t, got, 1, G, G)
	if len(m.Prompts()) != 0 {
		t.Fatal("the prompt stayed")
	}
	again, _, got2 := setup(t, dir)
	ask(again, 2, 2, "https://meet.test", false, Camera)
	want(t, got2, 2, G)
	if s := again.Status(0, "https://meet.test", Microphone); s != G {
		t.Fatalf("status %v", s)
	}
	if len(again.Prompts()) != 0 {
		t.Fatal("a decided kind prompted")
	}
}

func TestBlockIsKeptAndDismissKeepsNothing(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", true, Camera)
	m.Answer(m.Prompts()[0].ID, Block)
	want(t, got, 1, D)
	ask(m, 2, 1, "https://meet.test", true, Camera)
	want(t, got, 2, D)

	ask(m, 3, 2, "https://news.test", true, Notifications)
	p := m.Prompts()[0]
	if p.Once {
		t.Fatal("notifications offered Allow this time")
	}
	m.Answer(p.ID, Once) // not offered: ignored
	if got[3] != nil {
		t.Fatal("Allow this time answered a notifications prompt")
	}
	m.Answer(p.ID, Dismiss)
	want(t, got, 3, A)
	if s := m.Status(2, "https://news.test", Notifications); s != A {
		t.Fatalf("dismiss kept %v", s)
	}
	if len(m.Sites()) != 1 {
		t.Fatalf("sites %+v", m.Sites())
	}
}

func TestAllowThisTimeLastsUntilTheTabLeavesTheOrigin(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", true, Camera)
	m.Answer(m.Prompts()[0].ID, Once)
	want(t, got, 1, G)
	ask(m, 2, 1, "https://meet.test", true, Camera)
	want(t, got, 2, G)
	if s := m.Status(2, "https://meet.test", Camera); s != A {
		t.Fatalf("another tab got this time's grant: %v", s)
	}
	m.TabChanged(1, "https://meet.test/other")
	if s := m.Status(1, "https://meet.test", Camera); s != G {
		t.Fatalf("a page of the same origin lost it: %v", s)
	}
	m.TabChanged(1, "https://elsewhere.test/")
	if s := m.Status(1, "https://meet.test", Camera); s != A {
		t.Fatalf("kept after leaving the origin: %v", s)
	}
	if len(m.Sites()) != 0 {
		t.Fatal("this time's grant was kept")
	}
}

func TestANotificationsRequestWithoutAPressGetsNoPrompt(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", false, Notifications)
	want(t, got, 1, A)
	if len(m.Prompts()) != 0 {
		t.Fatal("a page put up a prompt on its own")
	}
	// Camera may ask without a press, as Meet does on load.
	ask(m, 2, 1, "https://meet.test", false, Camera)
	if len(m.Prompts()) != 1 {
		t.Fatal("camera without a press got no prompt")
	}
}

func TestClipboardWriteIsGrantedAndOtherKindsDeniedWithoutAsking(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", true, ClipboardWrite, "geolocation")
	want(t, got, 1, G, D)
	if len(m.Prompts()) != 0 {
		t.Fatal("prompted")
	}
}

func TestARequestWaitsBehindItsTabsPromptAndAskOnlyForWhatIsUndecided(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", true, Camera)
	ask(m, 2, 1, "https://meet.test", true, Camera, Microphone)
	ask(m, 3, 2, "https://news.test", true, Notifications)
	if n := len(m.Prompts()); n != 2 {
		t.Fatalf("%d prompts, want one per tab", n)
	}
	m.Answer(m.Prompts()[0].ID, Allow)
	want(t, got, 1, G)
	p := m.Prompts()
	if len(p) != 2 || p[1].Tab != 1 || !reflect.DeepEqual(p[1].Kinds, []string{Microphone}) {
		t.Fatalf("next prompt %+v", p)
	}
	m.Answer(p[1].ID, Block)
	want(t, got, 2, G, D)
}

func TestAPromptGoesWhenItsTabNavigatesOrClosesAndALateAnswerIsIgnored(t *testing.T) {
	m, tabs, got := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", true, Camera)
	ask(m, 2, 1, "https://meet.test", true, Microphone)
	id := m.Prompts()[0].ID
	m.TabChanged(1, "https://meet.test/room#chat") // the same document
	if len(m.Prompts()) != 1 {
		t.Fatal("a fragment change dropped the prompt")
	}
	m.TabChanged(1, "https://meet.test/other")
	want(t, got, 1, A)
	want(t, got, 2, A)
	if len(m.Prompts()) != 0 {
		t.Fatal("the prompt stayed after the tab navigated")
	}
	m.Answer(id, Allow)
	if len(m.Sites()) != 0 {
		t.Fatal("a late answer was kept")
	}

	tabs[2] = "https://news.test/"
	ask(m, 3, 2, "https://news.test", true, Notifications)
	m.TabClosed(2)
	want(t, got, 3, A)
}

func TestForgetMakesASiteAskAgain(t *testing.T) {
	m, _, _ := setup(t, t.TempDir())
	ask(m, 1, 1, "https://meet.test", true, Camera, Microphone)
	m.Answer(m.Prompts()[0].ID, Allow)
	m.Forget("https://meet.test", Camera)
	if m.Status(0, "https://meet.test", Camera) != A || m.Status(0, "https://meet.test", Microphone) != G {
		t.Fatal("forget removed the wrong kind")
	}
	m.Forget("https://meet.test", "")
	if len(m.Sites()) != 0 {
		t.Fatalf("sites %+v", m.Sites())
	}
}

func TestOriginIsTheEnginesForm(t *testing.T) {
	for u, o := range map[string]string{
		"https://Meet.test:443/a": "https://meet.test",
		"http://localhost:8080/x": "http://localhost:8080",
		"about:blank":             "",
	} {
		if got := Origin(u); got != o {
			t.Errorf("%s: %q, want %q", u, got, o)
		}
	}
}

func TestSettingsSetsAnAskableKindForASite(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	m.Set("https://meet.test", Camera, Block)
	m.Set("https://meet.test", ClipboardWrite, Block) // not askable: ignored
	m.Set("https://meet.test/x", Camera, Allow)       // not an origin: ignored
	ask(m, 1, 1, "https://meet.test", true, Camera, ClipboardWrite)
	want(t, got, 1, D, G)
	m.Set("https://meet.test", Camera, "")
	if m.Status(0, "https://meet.test", Camera) != A || len(m.Sites()) != 0 {
		t.Fatalf("reset kept %+v", m.Sites())
	}
	if s := Settable(); len(s) != 6 || s[3].Kind != Ads || s[4].Kind != Links || s[5].Kind != Sparks || s[0].Kind != Camera || s[0].Title != "Camera" {
		t.Fatalf("settable %+v", s)
	}
}

func TestOurOwnKindsKeepOnlyWhatIsntTheirDefault(t *testing.T) {
	m, _, _ := setup(t, t.TempDir())
	o := "https://news.test"
	if m.Value(o, Ads) != On || m.Value(o, Links) != Allow || m.Value(o, Sparks) != On {
		t.Fatalf("defaults %q %q", m.Value(o, Ads), m.Value(o, Links))
	}
	m.Set(o, Ads, Off)
	m.Set(o, Links, Block)
	m.Set(o, Sparks, Off)
	m.Set(o, Links, "sideways")    // not a choice: ignored
	m.Set(o, "spark:ab12cd34", On) // no such kind: ignored
	if m.Value(o, Ads) != Off || m.Value(o, Links) != Block || m.Value(o, Sparks) != Off {
		t.Fatalf("set %+v", m.Sites())
	}
	got := m.SettingsOf(o)
	if got[3].Value != Off || got[4].Value != Block || got[0].Value != "" {
		t.Fatalf("settings of %+v", got)
	}
	m.Set(o, Ads, On)
	m.Set(o, Links, Allow)
	m.Set(o, Sparks, On)
	if len(m.Sites()) != 0 {
		t.Fatalf("the defaults are kept: %+v", m.Sites())
	}
}

// A site is one registrable domain, over http or https: subdomains are one site, two
// domains under a public suffix are two, and the blank page is none.
func TestSameSiteIsOneRegistrableDomain(t *testing.T) {
	for _, c := range []struct {
		a, b string
		same bool
	}{
		{"https://narto-drama.com/x", "https://www.narto-drama.com/y", true},
		{"http://news.example.co.uk/", "https://shop.example.co.uk/", true},
		{"https://example.co.uk/", "https://other.co.uk/", false},
		{"https://a.github.io/", "https://b.github.io/", false},
		{"http://localhost:8765/", "http://localhost:9000/", true},
		{"http://localhost:8765/", "http://127.0.0.1:8765/", false},
		{"https://shop.test/", "about:blank", false},
		{"https://shop.test/", "", false},
	} {
		if got := SameSite(c.a, c.b); got != c.same {
			t.Errorf("%s and %s: %v, want %v", c.a, c.b, got, c.same)
		}
	}
}

// Links to other sites is Allow or Block; a site kept at New tab, which it no longer
// offers, reads as Allow.
func TestLinksIsAllowOrBlock(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sites.json"), []byte(`{"https://shop.test":{"links":"newtab"}}`), 0o600)
	m, _, _ := setup(t, dir)
	if v := m.Value("https://shop.test", Links); v != Allow {
		t.Fatalf("a kept New tab reads %q", v)
	}
	m.Set("https://shop.test", Links, "newtab")
	if v := m.Value("https://shop.test", Links); v != Allow {
		t.Fatalf("New tab set reads %q", v)
	}
}

// Each spark's own switch for a site, which one switch for all of them replaced, goes when
// the settings are read, and a site with nothing else goes with it.
func TestEachSparksOwnSwitchGoes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sites.json"), []byte(`{"https://a.test":{"spark:ab12cd34":"off"},"https://b.test":{"spark:ab12cd34":"off","ads":"off"}}`), 0o600)
	m, _, _ := setup(t, dir)
	if s := m.Sites(); len(s) != 1 || s[0].Origin != "https://b.test" || len(s[0].Settings) != 1 || m.Value("https://b.test", Sparks) != On {
		t.Fatalf("%+v", s)
	}
}
