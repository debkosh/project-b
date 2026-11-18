package app_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Settings' Sync: the page has the status, a deep link's address is only proposed, and a
// connect with a wrong address is answered with why. A folder named outright keeps its own
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
	if m["address"] != "https://memd.example/sync/t0k" || m["host"] != "memd.example" || m["needsToken"] != false || m["error"] != nil {
		t.Fatalf("proposal %v", m)
	}
	r.settings(`{"t":"syncPropose","address":"http://memd.example/sync"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncProposal" }); m["error"] == nil {
		t.Fatalf("an http address was proposed: %v", m)
	}
	r.settings(`{"t":"syncConnect","address":"https://memd.example/sync","passphrase":"p"}`)
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "syncConnected" }); m["error"] == nil {
		t.Fatalf("connected with no token: %v", m)
	}
	for _, f := range []string{"profile.key", "folder.key", "secrets"} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s before anything was sealed", f)
		}
	}
}
