package app_test

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"projectb/sites"
	"projectb/sparks"
)

// Settings' Sparks, by the messages the page sends (docs/action-coverage.md): a spark added,
// updated, turned on, its source shown, and removed; and an import that asked about a clash,
// cancelled, keeping nothing.
func TestSettingsSparksMessages(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, _ := r.openSettings()
	list := func() (out []sparks.Spark) {
		r.main(func() { out = r.a.Sparks.List() })
		return out
	}
	say := func(m map[string]any) {
		b, _ := json.Marshal(m)
		r.settings(string(b))
	}
	saved := func() {
		t.Helper()
		if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkSaved" }); m["problem"] != "" {
			t.Fatalf("saving said %v", m["problem"])
		}
	}

	say(map[string]any{"t": "sparkAdd", "source": sparkSource("Mine", "https://a.test/*")})
	saved()
	all := list()
	if len(all) != 1 || all[0].Name != "Mine" {
		t.Fatalf("after sparkAdd: %+v", all)
	}
	id := all[0].ID

	say(map[string]any{"t": "sparkUpdate", "id": id, "source": sparkSource("Mine", "https://b.test/*")})
	saved()
	if all := list(); len(all) != 1 || len(all[0].Match) != 1 || all[0].Match[0] != "https://b.test/*" {
		t.Fatalf("after sparkUpdate: %+v", all)
	}

	on := !list()[0].On
	say(map[string]any{"t": "sparkSetOn", "id": id, "on": on})
	r.e.Settle()
	if all := list(); all[0].On != on {
		t.Fatalf("after sparkSetOn %v: %+v", on, all)
	}

	say(map[string]any{"t": "sparkSource", "id": id})
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkSource" }); m["id"] != id || m["source"] != sparkSource("Mine", "https://b.test/*") {
		t.Fatalf("sparkSource said %v", m)
	}

	// An import of one the profile has asks first; cancelled, it keeps nothing.
	say(map[string]any{"t": "sparkImport", "files": []map[string]string{
		{"name": "mine.user.js", "data": base64.StdEncoding.EncodeToString([]byte(sparkSource("Mine", "https://c.test/*")))},
	}})
	r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkClashes" })
	say(map[string]any{"t": "sparkImportCancel"})
	r.e.Settle()
	if all := list(); len(all) != 1 || all[0].Match[0] != "https://b.test/*" {
		t.Fatalf("after sparkImportCancel: %+v", all)
	}

	say(map[string]any{"t": "sparkRemove", "id": id})
	r.e.Settle()
	if all := list(); len(all) != 0 {
		t.Fatalf("after sparkRemove: %+v", all)
	}
}

// The system sparks, by the same messages: settings hears them apart from the person's;
// one turns on and off, its source shows, an edit is refused, and Duplicate keeps a copy
// as the person's own, on, with the system one turned off; the page then runs the copy.
func TestSettingsSystemSparksMessages(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, snap := r.openSettings()
	say := func(m map[string]any) {
		b, _ := json.Marshal(m)
		r.settings(string(b))
	}
	system := func(m map[string]any) []any { l, _ := m["systemSparks"].([]any); return l }
	if s := system(snap)[0].(map[string]any); s["id"] != "video-downloader" || s["name"] != "Video downloader" || s["system"] != true || s["on"] != false {
		t.Fatalf("system sparks %v", system(snap))
	}

	say(map[string]any{"t": "sparkSetOn", "id": "video-downloader", "on": true})
	m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparks" })
	if !system(m)[0].(map[string]any)["on"].(bool) {
		t.Fatalf("after sparkSetOn: %v", m)
	}
	var scripts []string
	r.main(func() {
		for _, s := range r.a.Sparks.Scripts("https://videos.test/", true) {
			scripts = append(scripts, s.ID)
		}
	})
	if len(scripts) != 1 || scripts[0] != "video-downloader" {
		t.Fatalf("scripts %v", scripts)
	}
	// The site's own switch for sparks holds for a system spark too.
	var onSite, offSite int
	r.main(func() {
		onSite = len(r.a.SparksFor(0, "https://videos.test/", true))
		r.a.Sites.Set("https://videos.test", sites.Sparks, sites.Off)
		offSite = len(r.a.SparksFor(0, "https://videos.test/", true))
		r.a.Sites.Set("https://videos.test", sites.Sparks, sites.On)
	})
	if onSite != 1 || offSite != 0 {
		t.Fatalf("on the site %d, off for it %d", onSite, offSite)
	}

	say(map[string]any{"t": "sparkSource", "id": "video-downloader"})
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkSource" }); !strings.Contains(m["source"].(string), "@name         Video downloader\n") ||
		m["fileName"] != "Video downloader.user.js" {
		t.Fatalf("sparkSource said %v", m["fileName"])
	}

	say(map[string]any{"t": "sparkUpdate", "id": "video-downloader", "source": sparkSource("Mine", "https://a.test/*")})
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkSaved" }); m["problem"] == "" {
		t.Fatal("an edit of a system spark was taken")
	}

	say(map[string]any{"t": "sparkDuplicate", "id": "video-downloader"})
	if m := r.waitOn(got, func(m map[string]any) bool { return m["t"] == "sparkSaved" }); m["problem"] != "" {
		t.Fatalf("sparkDuplicate said %v", m["problem"])
	}
	var mine []sparks.Spark
	var vd sparks.Spark
	r.main(func() {
		mine, vd = r.a.Sparks.List(), r.a.Sparks.System()[0]
		scripts = nil
		for _, s := range r.a.Sparks.Scripts("https://videos.test/", true) {
			scripts = append(scripts, s.ID)
		}
	})
	if len(mine) != 1 || mine[0].Name != "Video downloader copy" || !mine[0].On || vd.On {
		t.Fatalf("after sparkDuplicate: %+v, %+v", mine, vd)
	}
	if len(scripts) != 1 || scripts[0] != mine[0].ID {
		t.Fatalf("scripts after sparkDuplicate %v", scripts)
	}
}
