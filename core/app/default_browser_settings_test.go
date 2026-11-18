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
