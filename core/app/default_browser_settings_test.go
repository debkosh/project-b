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
