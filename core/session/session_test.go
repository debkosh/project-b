package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"projectb/engine"
)

func TestKeepsTheSessionAcrossManagersAndWritesOnlyChanges(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	if len(m.Last().Windows) != 0 {
		t.Fatalf("a new profile has %+v", m.Last())
	}
	s := Session{Windows: []Window{{Bounds: engine.Rect{X: 10, Y: 20, W: 800, H: 600}, Active: 1,
		Tabs:   []Tab{{URL: "https://a.test/", Title: "A", Workspace: ""}, {URL: "https://b.test/", Workspace: "w1", Group: "g1"}},
		Groups: []Group{{ID: "g1", Name: "Work", Color: "#3470d0"}}}}}
	m.Save(s)
	path := filepath.Join(dir, "session.json")
	info, _ := os.Stat(path)
	os.Chtimes(path, info.ModTime().Add(-1e9*60), info.ModTime().Add(-1e9*60))
	m.Save(s)
	if again, _ := os.Stat(path); !again.ModTime().Before(info.ModTime()) {
		t.Fatal("the same session was written again")
	}
	if got := New(dir).Last(); !reflect.DeepEqual(got, s) {
		t.Fatalf("read back %+v", got)
	}
}
