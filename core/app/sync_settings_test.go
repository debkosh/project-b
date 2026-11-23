package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Settings' Sync: the page has the status, a deep link's address is only proposed, and a
// connect with a wrong address is answered with why, and the problem's name; Suggest one gets
// five words. A folder named outright keeps its own
// key, and nothing is sealed until something is.
func TestSettingsSync(t *testing.T) {
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	got, snapshot := r.openSettings()
	if s, ok := snapshot["sync"].(map[string]any); !ok || s["on"] != false {
		t.Fatalf("the snapshot's sync: %v", snapshot["sync"])
	}

	r.settings(`{"t":"syncPropose","link":"project-b://sync?address=https%3A%2F%2Fmemd.example%2Fsync%2Ft0k"}`)
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncProposal" })
	if m["address"] != "https://memd.example/sync/t0k" || m["host"] != "memd.example" || m["token"] != "" || m["error"] != nil {
		t.Fatalf("proposal %v", m)
	}
	r.settings(`{"t":"syncPropose","address":"http://memd.example/sync"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncProposal" }); m["error"] == nil || m["problem"] != "address" {
		t.Fatalf("an http address was proposed: %v", m)
	}
	// A link may carry the token, which goes back to the page for its Token field.
	r.settings(`{"t":"syncPropose","link":"project-b://sync?address=https%3A%2F%2Fmemd.example%2Fsync&token=t0k"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncProposal" }); m["address"] != "https://memd.example/sync" || m["token"] != "t0k" || m["error"] != nil {
		t.Fatalf("a link with a token proposed: %v", m)
	}
	r.settings(`{"t":"syncSuggest"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncSuggested" }); len(strings.Fields(m["passphrase"].(string))) != 5 {
		t.Fatalf("suggested %v", m)
	}
	// Sync details answers with sync off too: nothing to show, and never null where the page
	// expects a list.
	r.settings(`{"t":"syncDetails"}`)
	m = r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncDetails" })
	if d, ok := m["details"].(map[string]any); !ok || d["host"] != "" || d["running"] != nil || len(d["runs"].([]any)) != 0 || len(d["kinds"].([]any)) != 0 || len(d["waiting"].([]any)) != 0 {
		t.Fatalf("the details with sync off: %v", m)
	}
	for _, f := range []string{"profile.key", "folder.key", "secrets"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s before anything was sealed", f)
		}
	}
}

// A project-b://sync link from another app, as the Mac and Android hand one over, opens
// Settings at Sync and tells the page the link after its snapshot; Go only reads it. Another
// project-b: link does nothing, and a web link still opens a tab.
func TestASyncLinkOpensSettingsAtSync(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	tabsBefore := len(r.tabsNow(1))
	r.e.Post(func() { r.a.Handover([]string{"project-b://elsewhere?address=x"}) })
	if r.settingsIn(1) != 0 || len(r.tabsNow(1)) != tabsBefore {
		t.Fatal("another project-b: link opened something")
	}
	link := "project-b://sync?address=https%3A%2F%2Fmemd.example%2Fsync%2Ft0k"
	r.e.Post(func() { r.a.Handover([]string{link}) })
	got, snapshot := r.openSettings()
	if snapshot["section"] != "sync" {
		t.Fatalf("settings opened at %v", snapshot["section"])
	}
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncLink" }); m["link"] != link {
		t.Fatalf("the page was told %v", m)
	}
	// With the page open, a second link reaches it at once; an http address is answered with
	// its error and the address it had.
	r.e.Post(func() { r.a.Handover([]string{"project-b://sync?address=http%3A%2F%2Fmemd.example%2Fsync"}) })
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncLink" })
	r.settings(`{"t":"syncPropose","link":"project-b://sync?address=http%3A%2F%2Fmemd.example%2Fsync"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncProposal" }); m["problem"] != "address" || m["address"] != "http://memd.example/sync" {
		t.Fatalf("an http link's proposal: %v", m)
	}
	if s := snapshot["sync"].(map[string]any); s["on"] != false {
		t.Fatalf("sync turned on: %v", s)
	}
}

// A page's link to project-b://sync, pressed, opens Settings at Sync as one from another app
// does; the page's own script going there, as a press-less navigation, opens nothing, and the
// page stays where it was.
func TestAPressedSyncLinkInAPage(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.visit("https://memd.example/")
	link := "project-b://sync?address=https%3A%2F%2Fmemd.example%2Fsync%2Ft0k"
	tab := r.activeID()
	r.e.Fire(1, tab, "navigate", link)
	r.e.Settle()
	if r.settingsIn(1) != 0 {
		t.Fatal("a script's navigation opened settings")
	}
	if u := r.urls(); len(u) != 1 || u[0] != "https://memd.example/" {
		t.Fatalf("the tabs went to %v", u)
	}
	r.e.Fire(1, tab, "link", link)
	got, snapshot := r.openSettings()
	if snapshot["section"] != "sync" {
		t.Fatalf("settings opened at %v", snapshot["section"])
	}
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncLink" }); m["link"] != link {
		t.Fatalf("the page was told %v", m)
	}
}
