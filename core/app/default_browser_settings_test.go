package app_test

import "testing"

// The settings page's status line: the snapshot says whether the app is the default
// browser, and Make default browser asks the system, whose answer reaches the page; the
// next page opens knowing it.
func TestSettingsMakesTheAppTheDefaultBrowser(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, snap := r.openSettings()
	if snap["isDefault"] != false {
		t.Fatalf("the snapshot's isDefault is %v", snap["isDefault"])
	}
	r.settings(`{"t":"makeDefault"}`)
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "isDefault" && m["isDefault"] == true })
	r.settings(`{"t":"ready"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "snapshot" }); m["isDefault"] != true {
		t.Fatalf("a snapshot after: isDefault is %v", m["isDefault"])
	}
}

// Make default browser pressed twice, and the system's question closed unanswered: the system
// is asked once, and the page hears no, the app still not the default browser, for each press.
func TestSettingsDefaultBrowserQuestionDismissed(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.sys.Dismiss(true)
	got, _ := r.openSettings()
	r.settings(`{"t":"makeDefault"}`)
	r.settings(`{"t":"makeDefault"}`)
	for range 2 {
		r.waitOn(got, func(m map[string]any) bool { return m["t"] == "isDefault" && m["isDefault"] == false })
	}
	if n := r.sys.Asked("defaultBrowser"); n != 1 || r.sys.IsDefaultBrowser() {
		t.Fatalf("the system was asked %d times, and the app is the default: %v", n, r.sys.IsDefaultBrowser())
	}
}

// The default browser set in the OS's own settings, outside the app, as Linux's desktops
// have the person do: the window coming to the front asks the system again, and the open
// settings page hears it; another focus, with nothing changed, sends nothing more.
// Where the system reads the default browser at start only, as Linux's Flatpak does, a window
// coming to the front doesn't ask again: the page keeps what it was told.
func TestFocusDoesntRecheckWhereTheSystemCant(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.sys.SetDesktop("gnome")
	r.sys.SetDefaultBrowserStale(true)
	got, _ := r.openSettings()
	r.sys.SetDefaultBrowser(true)
	r.e.Fire(1, 0, "focus", "")
	r.settings(`{"t":"ready"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "isDefault" || m["t"] == "snapshot" }); m["t"] != "snapshot" {
		t.Fatalf("a focus asked again where the system can't see a change: %v", m)
	}
}

func TestFocusRechecksTheDefaultBrowser(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.sys.SetDesktop("gnome")
	got, snap := r.openSettings()
	if snap["isDefault"] != false || snap["desktop"] != "gnome" {
		t.Fatalf("the snapshot: isDefault %v, desktop %v", snap["isDefault"], snap["desktop"])
	}
	r.sys.SetDefaultBrowser(true)
	r.e.Fire(1, 0, "focus", "")
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "isDefault" && m["isDefault"] == true })
	r.e.Fire(1, 0, "focus", "")
	r.settings(`{"t":"ready"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "isDefault" || m["t"] == "snapshot" }); m["t"] != "snapshot" {
		t.Fatalf("a focus with nothing changed sent %v", m)
	}
}
