package app_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"projectb/sparks"
)

// Settings' Sparks, by the messages the page sends (docs/action-coverage.md): a spark added,
// updated, turned on, its source shown, and removed; and an import that asked about a clash,
// cancelled, keeping nothing.
func TestSettingsSparksMessages(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	got, _ := r.openSettings()
	r.main(func() {
		for _, s := range r.a.Sparks.Builtins() {
			r.a.Sparks.Remove(s.ID)
		}
	})
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
