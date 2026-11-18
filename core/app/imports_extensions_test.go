//go:build extensions

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"projectb/app"
	"projectb/engine"
	"projectb/flags"
	"projectb/profiles"
	"projectb/update"
)

// With the Extensions flag on, Chrome · Personal's extensions come over too: those its
// person installed from the Chrome Web Store are installed again by id, with no prompt,
// one at a time; the rest are left out, each with why. The mock store has the test
// extension and not uBlock Origin Lite, which fails. Brought over again, the test
// extension is installed already.
func TestExtensionsComeOverFromTheStore(t *testing.T) {
	dir := t.TempDir()
	flags.New(dir).Set(flags.Extensions, "on")
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	store := t.TempDir()
	os.MkdirAll(filepath.Join(store, "crx"), 0o755)
	pack(t, filepath.Join(store, "crx", testExtensionID+"_1.0.crx"), "1.0")
	r.e.SetExtensionStore(store)

	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	say(`{"t":"findBrowsers"}`)
	list := next(t, got, "browsers")["browsers"].([]any)
	if c := list[0].(map[string]any)["counts"].(map[string]any); c["extensions"] != 2.0 {
		t.Fatalf("Chrome · Personal counts %v; want 2 extensions", c)
	}

	type note struct{ Name, Why string }
	var brought struct {
		Extensions struct {
			Installed, Already []string
			Failed, Skipped    []note
		}
	}
	bring := func() {
		t.Helper()
		say(`{"t":"import","key":"chrome/Default","what":["extensions"]}`)
		b, _ := json.Marshal(next(t, got, "imported")["imported"])
		brought.Extensions.Installed, brought.Extensions.Already = nil, nil
		json.Unmarshal(b, &brought)
	}
	bring()
	x := brought.Extensions
	if len(x.Installed) != 1 || x.Installed[0] != "Project B test extension" {
		t.Errorf("installed %q", x.Installed)
	}
	if len(x.Failed) != 1 || x.Failed[0] != (note{"uBlock Origin Lite", "the store answered 404"}) {
		t.Errorf("failed %+v", x.Failed)
	}
	want := []note{{"my-extension", "loaded from a folder"}, {"Old Notes", "Manifest V2, which no longer runs"}, {"Other Store Helper", "not from the Chrome Web Store"}}
	if len(x.Skipped) != 3 || x.Skipped[0] != want[0] || x.Skipped[1] != want[1] || x.Skipped[2] != want[2] {
		t.Errorf("skipped %+v", x.Skipped)
	}
	asked := make(chan bool)
	r.e.Post(func() { asked <- len(r.a.Extensions().Installed()) == 1 && r.a.Extensions().Prompt() == nil })
	if !<-asked {
		t.Error("want the test extension installed, with no prompt left")
	}

	bring()
	if a := brought.Extensions.Already; len(a) != 1 || a[0] != "Project B test extension" || len(brought.Extensions.Installed) != 0 {
		t.Errorf("again: %+v", brought.Extensions)
	}
}
