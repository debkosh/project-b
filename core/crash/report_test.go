package crash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"projectb/engine"
)

// A report holds what the platform says, the last run's crash, the processes that
// crashed and the timelines' ends, in one file in the profile's reports folder.
func TestAReportHoldsTheLogs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "crash.log"), []byte("pid 1\npanic: boom\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "timeline.log"), []byte("mark app.ready\n"), 0o600)
	m := New(dir, t.TempDir(), noReason)
	m.System = func() []Section { return []Section{{"Version", "Project B 0.2.61"}} }
	m.ProcessCrashed(engine.ProcessCrash{Type: "renderer", Status: "killed", PID: 7, URL: "https://duckduckgo.com/"})
	path, err := m.WriteReport()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	for _, want := range []string{"== Version\nProject B 0.2.61", "panic: boom", "renderer killed", "duckduckgo.com", "mark app.ready", "== The last run's timeline, its end\n(none)"} {
		if !strings.Contains(s, want) {
			t.Fatalf("no %q in\n%s", want, s)
		}
	}
	if filepath.Dir(path) != filepath.Join(dir, "reports") {
		t.Fatalf("written to %s", path)
	}
}
