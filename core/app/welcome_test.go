package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"projectb/app"
	"projectb/engine"
	"projectb/profiles"
	"projectb/update"
)

func startMade(t *testing.T, dir string, made bool) *rig {
	t.Helper()
	return startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: made, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
}

func welcomeDone(t *testing.T, dir string) (bool, bool) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "welcome.json"))
	if os.IsNotExist(err) {
		return false, false
	}
	var f struct{ Done bool }
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return true, f.Done
}

// The start that made the profile opens the welcome over the whole window: the shell hears
// which tab has it, and the page that it is the whole window's. A choice is a pref at once,
// so the page hears its new tones. Finishing keeps that it was finished, closes the tab and
// gives the window back its shell, with the tab a start makes; the next start opens none.
func TestTheStartThatMadeTheProfileOpensTheWelcomeOverTheWholeWindow(t *testing.T) {
	dir := t.TempDir()
	r := startMade(t, dir, true)
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	if tab == 0 || engine.TabID(snap["active"].(float64)) != tab || len(snap["tabs"].([]any)) != 1 {
		t.Fatalf("the snapshot: welcome %v, active %v, tabs %v", snap["welcome"], snap["active"], snap["tabs"])
	}
	if there, done := welcomeDone(t, dir); !there || done {
		t.Fatalf("welcome.json there %v, done %v, while it shows", there, done)
	}
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	say(`{"t":"ready"}`)
	if page := next(t, got, "snapshot"); page["whole"] != true || len(page["layouts"].([]any)) != 3 {
		t.Fatalf("the page's snapshot: %v", page)
	}
	say(`{"t":"setPref","name":"frameColor","value":"#5b7fd6"}`)
	if m := next(t, got, "tones"); m["tones"].(map[string]any)["light"].(map[string]any)["--frame"] != "#5b7fd6" {
		t.Fatalf("tones = %v", m["tones"])
	}
	if l := r.e.Layout(1); l.FrameColor != "#5b7fd6" {
		t.Fatalf("the window paints %s", l.FrameColor)
	}
	say(`{"t":"done"}`)
	r.waitFor(func(m map[string]any) bool { return m["t"] == "welcome" && m["welcome"] == float64(0) })
	tabs := r.waitFor(func(m map[string]any) bool {
		if m["t"] != "tabs" {
			return false
		}
		list := m["tabs"].([]any)
		return len(list) == 1 && list[0].(map[string]any)["page"] == nil
	})
	if len(tabs["tabs"].([]any)) != 1 {
		t.Fatalf("tabs after: %v", tabs["tabs"])
	}
	if _, done := welcomeDone(t, dir); !done {
		t.Fatal("welcome.json doesn't say it was finished")
	}
	r.stop()

	again := startMade(t, dir, false)
	defer again.stop()
	if snap := again.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); snap["welcome"] != float64(0) {
		t.Fatalf("a start after it was finished shows it: %v", snap["welcome"])
	}
}

// A welcome left before its end opens again at the next start; a folder no start made
// shows none, unless the flag asks for it at each start.
func TestTheWelcomeOpensAgainUnfinishedOrByTheFlag(t *testing.T) {
	dir := t.TempDir()
	plain := startMade(t, dir, false)
	if snap := plain.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); snap["welcome"] != float64(0) {
		t.Fatalf("a folder no start made shows the welcome")
	}
	plain.stop()

	os.WriteFile(filepath.Join(dir, "welcome.json"), []byte(`{"done":false}`), 0o600)
	left := startMade(t, dir, false)
	if snap := left.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); snap["welcome"] == float64(0) {
		t.Fatal("a welcome left unfinished doesn't open again")
	}
	left.stop()

	os.WriteFile(filepath.Join(dir, "welcome.json"), []byte(`{"done":true}`), 0o600)
	os.WriteFile(filepath.Join(dir, "flags.json"), []byte(`{"showWelcome":"on"}`), 0o600)
	flagged := startMade(t, dir, false)
	defer flagged.stop()
	if snap := flagged.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); snap["welcome"] == float64(0) {
		t.Fatal("the flag doesn't open the welcome")
	}
}

// Show the welcome again, in Settings' Flags, marks it unfinished and closes the app; the
// next start opens it over the whole window.
func TestShowTheWelcomeAgainClosesTheApp(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "welcome.json"), []byte(`{"done":true}`), 0o600)
	r := startMade(t, dir, false)
	settingsTab, _ := r.openPage("app.settings", "settings")
	r.e.Post(func() { r.a.PageMessage(settingsTab, "settings", []byte(`{"t":"welcomeAgain"}`)) })
	deadline := time.Now().Add(5 * time.Second)
	for !r.e.Quit() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !r.e.Quit() {
		t.Fatal("the app didn't close")
	}
	if _, done := welcomeDone(t, dir); done {
		t.Fatal("welcome.json still says it was finished")
	}
	r.stop()

	again := startMade(t, dir, false)
	defer again.stop()
	if snap := again.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); snap["welcome"] == float64(0) {
		t.Fatal("the next start doesn't open the welcome")
	}
}

// --skip-welcome notes the welcome finished at a start that would open it, as Skip setup
// does, so the next start, without the switch, opens none either.
func TestSkipWelcomeNotesItFinished(t *testing.T) {
	dir := t.TempDir()
	r := startWith(t, app.Config{ProfileDir: dir, Profiles: profiles.New(t.TempDir()), Made: true, SkipWelcome: true, Update: update.Config{StateDir: filepath.Join(dir, "updates")}})
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	if snap["welcome"] != float64(0) || len(snap["tabs"].([]any)) != 1 {
		t.Fatalf("with --skip-welcome: welcome %v, tabs %v", snap["welcome"], snap["tabs"])
	}
	if _, done := welcomeDone(t, dir); !done {
		t.Fatal("welcome.json doesn't say it was finished")
	}
	r.stop()
	again := startMade(t, dir, false)
	defer again.stop()
	if snap := again.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" }); snap["welcome"] != float64(0) {
		t.Fatal("the next start opens it")
	}
}

// How you work: the workspaces ticked are made, the first in Main's place, and taken again
// the step undoes what is no longer ticked; the keys it offers are Project B's and
// Firefox's, each with its own, and choosing one is the keymap's template.
func TestHowYouWorkMakesTheWorkspacesAndChoosesTheKeys(t *testing.T) {
	dir := t.TempDir()
	r := startMade(t, dir, true)
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	say(`{"t":"ready"}`)
	page := next(t, got, "snapshot")
	keymaps := page["keymaps"].(map[string]any)
	if page["keymap"] != "project-b" || keymaps["project-b"].(map[string]any)["downloads.show"] != "Shift+Cmd+J" || keymaps["firefox"].(map[string]any)["downloads.show"] != "Cmd+J" {
		t.Fatalf("keymap %v, keymaps' downloads %v / %v", page["keymap"], keymaps["project-b"].(map[string]any)["downloads.show"], keymaps["firefox"].(map[string]any)["downloads.show"])
	}

	names := func(m map[string]any) []string {
		var out []string
		for _, w := range m["workspaces"].([]any) {
			out = append(out, w.(map[string]any)["name"].(string))
		}
		return out
	}
	say(`{"t":"setWorkspaces","workspaces":[{"name":"Personal","color":"#2e9d5b"},{"name":"Work","color":"#3470d0"},{"name":"Travel","color":"#8a4fd0"}]}`)
	if n := names(next(t, got, "workspaces")); len(n) != 3 || n[0] != "Personal" || n[1] != "Work" || n[2] != "Travel" {
		t.Fatalf("workspaces %q", n)
	}
	if first := r.a.Workspaces.List()[0]; first.ID != "" || first.Color != "#2e9d5b" {
		t.Fatalf("Main is %+v; want it named Personal, green", first)
	}
	say(`{"t":"setWorkspaces","workspaces":[{"name":"Work","color":"#3470d0"}]}`)
	if n := names(next(t, got, "workspaces")); len(n) != 1 || n[0] != "Work" {
		t.Fatalf("taken again with Work alone: %q", n)
	}
	say(`{"t":"setWorkspaces","workspaces":[]}`)
	if n := names(next(t, got, "workspaces")); len(n) != 1 || n[0] != "Main" {
		t.Fatalf("with none ticked: %q", n)
	}

	say(`{"t":"useKeymap","value":"firefox"}`)
	if m := next(t, got, "keymap"); m["keymap"] != "firefox" {
		t.Fatalf("keymap %v", m)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "keymap.json"))
	var k struct{ Follows string }
	json.Unmarshal(b, &k)
	if k.Follows != "firefox" {
		t.Fatalf("keymap.json follows %q", k.Follows)
	}
	say(`{"t":"useKeymap","value":"arshell"}`)
	say(`{"t":"useKeymap","value":"project-b"}`)
	if m := next(t, got, "keymap"); m["keymap"] != "project-b" {
		t.Fatalf("a keymap the step doesn't offer was taken: %v", m)
	}
}

// A profile's first start gives it the built-in sparks, off. The welcome's page hears them,
// for the phone's Extras for any page, and a switch there turns one on.
func TestTheWelcomeOffersTheBuiltInSparks(t *testing.T) {
	dir := t.TempDir()
	r := startMade(t, dir, true)
	defer r.stop()
	snap := r.waitFor(func(m map[string]any) bool { return m["t"] == "snapshot" })
	tab := engine.TabID(snap["welcome"].(float64))
	got, stop := r.e.PageFeed(tab).Subscribe()
	defer stop()
	say := func(msg string) { r.e.Post(func() { r.a.PageMessage(tab, "welcome", []byte(msg)) }) }
	say(`{"t":"ready"}`)
	sparks := next(t, got, "snapshot")["sparks"].([]any)
	var names []string
	for _, s := range sparks {
		s := s.(map[string]any)
		if s["on"] != false || s["description"] == "" {
			t.Fatalf("offered %v", s)
		}
		names = append(names, s["name"].(string))
	}
	if len(names) != 3 || names[0] != "Video downloader" || names[1] != "Screen time limit" || names[2] != "Page cleanup" {
		t.Fatalf("offered %v", names)
	}
	id := sparks[1].(map[string]any)["id"].(string)
	say(`{"t":"setSparkOn","id":"` + id + `","on":true}`)
	if m := next(t, got, "sparks"); !m["sparks"].([]any)[1].(map[string]any)["on"].(bool) {
		t.Fatalf("after the switch %v", m["sparks"])
	}
	var on bool
	r.main(func() { on = r.a.Sparks.List()[1].On })
	if !on {
		t.Fatal("Screen time limit isn't on")
	}
}
