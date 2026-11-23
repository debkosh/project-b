package sites

import (
	"reflect"
	"testing"

	"github.com/sudiptadeb/oskit/media"
)

// A site's camera, allowed, still needs the OS's own permission: the OS is asked then, and its
// yes gives the page the device.
func TestTheOSIsAskedWhenAnAllowedSiteFirstNeedsTheCamera(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	os := media.NewFake()
	m.UseOS(os, "android")
	m.Set("https://meet.test", Camera, Allow)
	m.Set("https://meet.test", Microphone, Allow)
	ask(m, 1, 1, "https://meet.test", true, Camera, Microphone)
	if got[1] != nil || !reflect.DeepEqual(os.Asked(), [][]string{{Camera, Microphone}}) {
		t.Fatalf("answered %v, the OS asked %v; want it asked first", got[1], os.Asked())
	}
	os.Answer(Camera, Microphone)
	want(t, got, 1, G, G)
	// Allowed now, the OS isn't asked again.
	ask(m, 2, 1, "https://meet.test", true, Camera)
	want(t, got, 2, G)
	if len(os.Asked()) != 1 {
		t.Fatalf("the OS asked again: %v", os.Asked())
	}
}

// The OS's no is a prompt of its own on the tab, the page's request waiting: Settings opens
// the OS's settings and keeps it, and back in front with the OS now allowing, the page gets the
// device.
func TestTheOSsNoIsAPromptThatSettingsCanTurnIntoAYes(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	os := media.NewFake()
	m.UseOS(os, "macos")
	m.Set("https://meet.test", Camera, Allow)
	ask(m, 1, 1, "https://meet.test", true, Camera)
	os.Answer()
	p := m.Prompts()
	if len(p) != 1 || p[0].OS != "macos" || !reflect.DeepEqual(p[0].Kinds, []string{Camera}) || got[1] != nil {
		t.Fatalf("prompts %+v, answered %v", p, got[1])
	}
	m.Answer(p[0].ID, Settings)
	if !reflect.DeepEqual(os.Opened(), []string{Camera}) || len(m.Prompts()) != 1 {
		t.Fatalf("opened %v, prompts %v", os.Opened(), m.Prompts())
	}
	m.Returned()
	if len(m.Prompts()) != 1 {
		t.Fatal("back with the OS still saying no, the prompt went")
	}
	os.Set(Camera, true)
	m.Returned()
	want(t, got, 1, G)
	if len(m.Prompts()) != 0 {
		t.Fatal("the prompt stayed")
	}
	if m.sites["https://meet.test"][Camera] != Allow {
		t.Fatal("the site's own setting changed")
	}
}

// Not now denies the page, and the OS's no is answered with no prompt on that tab's page until
// it leaves it; the site's setting stays.
func TestNotNowDeniesAndKeepsQuietUntilTheTabLeavesThePage(t *testing.T) {
	m, tabs, got := setup(t, t.TempDir())
	os := media.NewFake()
	m.UseOS(os, "android")
	m.Set("https://meet.test", Camera, Allow)
	ask(m, 1, 1, "https://meet.test", true, Camera)
	os.Answer()
	m.Answer(m.Prompts()[0].ID, Dismiss)
	want(t, got, 1, D)
	ask(m, 2, 1, "https://meet.test", true, Camera)
	want(t, got, 2, D)
	if len(m.Prompts()) != 0 || len(os.Asked()) != 1 {
		t.Fatalf("prompts %v, the OS asked %v", m.Prompts(), os.Asked())
	}
	tabs[1] = "https://meet.test/other"
	m.TabChanged(1, tabs[1])
	ask(m, 3, 1, "https://meet.test", true, Camera)
	os.Answer()
	if p := m.Prompts(); len(p) != 1 || p[0].OS == "" {
		t.Fatalf("on another page the OS's no is a prompt again: %v", p)
	}
	if m.sites["https://meet.test"][Camera] != Allow {
		t.Fatal("the site's own setting changed")
	}
}

// A tab that leaves the page or closes while the OS's prompt waits answers the page denied.
func TestTheOSsPromptGoesWithItsPage(t *testing.T) {
	m, tabs, got := setup(t, t.TempDir())
	os := media.NewFake()
	m.UseOS(os, "android")
	m.Set("https://meet.test", Microphone, Allow)
	ask(m, 1, 1, "https://meet.test", true, Microphone)
	os.Answer()
	tabs[1] = "https://elsewhere.test/"
	m.TabChanged(1, tabs[1])
	want(t, got, 1, D)
	if len(m.Prompts()) != 0 {
		t.Fatal("the prompt stayed")
	}
}

// A site the person blocked, or one still asking, never reaches the OS.
func TestOnlyAGrantedKindIsCheckedWithTheOS(t *testing.T) {
	m, _, got := setup(t, t.TempDir())
	os := media.NewFake()
	m.UseOS(os, "android")
	m.Set("https://meet.test", Camera, Block)
	ask(m, 1, 1, "https://meet.test", true, Camera)
	want(t, got, 1, D)
	if len(os.Asked()) != 0 {
		t.Fatalf("the OS was asked for a blocked site: %v", os.Asked())
	}
}
