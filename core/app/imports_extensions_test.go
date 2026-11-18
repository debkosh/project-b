//go:build extensions

package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	dir := t.TempDir()
	flags.New(dir).Set(flags.Extensions, "on")
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, BrowserHome: home, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	store := t.TempDir()
	os.MkdirAll(filepath.Join(store, "crx"), 0o755)
	pack(t, filepath.Join(store, "crx", testExtensionID+"_1.0.crx"), "1.0")
	r.e.SetExtensionStore(store)

	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	p := &importPage{t: t, dir: dir, r: r, got: got, say: func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }}
	p.say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	p.say(`{"t":"findBrowsers"}`)
	personal := profilesOf(browserIn(t, next(t, got, "browsers")["browsers"].([]any), "chrome"))[0]
	if c := personal["counts"].(map[string]any); c["extensions"] != 2.0 {
		t.Fatalf("Chrome · Personal counts %v; want 2 extensions", c)
	}
	key := personal["key"].(string)

	type note struct{ Name, Why string }
	var x struct {
		Installed, Already []string
		Failed, Skipped    []note
	}
	json.Unmarshal(p.bring(key, "extensions")["extensions"], &x)
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

	x.Installed, x.Already = nil, nil
	json.Unmarshal(p.bring(key, "extensions")["extensions"], &x)
	if len(x.Already) != 1 || x.Already[0] != "Project B test extension" || len(x.Installed) != 0 {
		t.Errorf("again: %+v", x)
	}
}

// An import going on can be cancelled, which drops the installs it asked for that haven't
// started; asked for again, as Retry does, it starts over and the first one's late answers
// are dropped; and a step with nothing new for long enough ends as failed. The mock store
// holds its installs here, as a store that doesn't answer.
func TestAnImportCanBeCancelledRetriedAndStalls(t *testing.T) {
	home, _ := filepath.Abs("../../browserdata/testdata/home")
	dir := t.TempDir()
	flags.New(dir).Set(flags.Extensions, "on")
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
	hold := func(on bool) { r.e.Post(func() { r.e.HoldStoreInstalls(on) }) }
	say(`{"t":"ready"}`)
	next(t, got, "snapshot")
	say(`{"t":"findBrowsers"}`)
	key := profilesOf(browserIn(t, next(t, got, "browsers")["browsers"].([]any), "chrome"))[0]["key"].(string)
	importExtensions := `{"t":"import","key":"` + key + `","what":["extensions"]}`
	// ended is how the run ended, and the extensions step's last word.
	ended := func() (string, map[string]any) {
		t.Helper()
		var last map[string]any
		deadline := time.After(5 * time.Second)
		for {
			var m map[string]any
			select {
			case b := <-got:
				json.Unmarshal(b, &m)
			case <-deadline:
				t.Fatal("the import didn't end")
			}
			switch m["t"] {
			case "importStep":
				last = m
			case "imported":
				return m["how"].(string), last
			}
		}
	}

	hold(true)
	say(importExtensions)
	for s := next(t, got, "importStep"); s["state"] != "doing" || s["result"] == nil; s = next(t, got, "importStep") {
	}
	say(`{"t":"cancelImport","key":"` + key + `"}`)
	if how, _ := ended(); how != "cancelled" {
		t.Fatalf("cancelled: %s", how)
	}
	hold(false)

	restore := app.SetStallAfter(200 * time.Millisecond)
	hold(true)
	say(importExtensions)
	how, last := ended()
	restore()
	if how != "failed" || last["error"] != "Stopped: nothing arrived for 30 seconds" {
		t.Fatalf("stalled: %s, %v", how, last)
	}
	hold(false)

	hold(true)
	say(importExtensions)
	next(t, got, "importStep")
	say(importExtensions)
	time.Sleep(100 * time.Millisecond)
	hold(false)
	how, last = ended()
	b, _ := json.Marshal(last["result"])
	if how != "done" || !strings.Contains(string(b), "Project B test extension") {
		t.Fatalf("retried: %s, %s", how, b)
	}
	select {
	case m := <-got:
		var x map[string]any
		json.Unmarshal(m, &x)
		if x["t"] == "imported" || x["t"] == "importStep" {
			t.Fatalf("a late answer from a run that ended: %s", m)
		}
	case <-time.After(300 * time.Millisecond):
	}
}
