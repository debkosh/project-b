package session

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
	m.Close()
	path := filepath.Join(dir, "session.json")
	info, _ := os.Stat(path)
	os.Chtimes(path, info.ModTime().Add(-1e9*60), info.ModTime().Add(-1e9*60))
	m = New(dir)
	m.Save(s)
	m.Close()
	if again, _ := os.Stat(path); !again.ModTime().Before(info.ModTime()) {
		t.Fatal("the same session was written again")
	}
	if got := New(dir).Last(); !reflect.DeepEqual(got, s) {
		t.Fatalf("read back %+v", got)
	}
}

// BenchmarkSave is what a tab closing costs the main thread: encoding a session of 3
// windows of 20 tabs each and handing it to the writer.
func BenchmarkSave(b *testing.B) {
	m := New(b.TempDir())
	defer m.Close()
	var s Session
	for w := 0; w < 3; w++ {
		var win Window
		for i := 0; i < 20; i++ {
			win.Tabs = append(win.Tabs, Tab{URL: fmt.Sprintf("https://site%d.test/some/longer/path", i), Title: "A page with a title"})
		}
		s.Windows = append(s.Windows, win)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Windows[0].Active = i % 20
		m.Save(s)
	}
}

// A change is on disk within a second with no Close: the writer has no timer, so a crash
// loses at most the change being written.
func TestASaveReachesTheDiskWithoutClose(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	defer m.Close()
	m.Save(Session{Windows: []Window{{Tabs: []Tab{{URL: "https://a.test/"}}}}})
	m.Save(Session{Windows: []Window{{Tabs: []Tab{{URL: "https://b.test/"}}}}})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if got := New(dir).Last(); len(got.Windows) == 1 && got.Windows[0].Tabs[0].URL == "https://b.test/" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("not on disk after a second: %+v", New(dir).Last())
}
