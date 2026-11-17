package app_test

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"projectb/timeline"
)

// A start says where the profile keeps its files, and a workspace made later says where
// the engine keeps its sites: content's folder for a partition named by the workspace's
// id, the first 6 bytes of its SHA-256 in capitals, in chromium/Storage/ext/workspace/.
func TestTheLifecycleNamesTheFolders(t *testing.T) {
	_, from := timeline.Since(^uint64(0) >> 1)
	dir := t.TempDir()
	r := start(t, dir)
	defer r.stop()
	list := func(m map[string]any) []any { l, _ := m["workspaces"].([]any); return l }
	r.shell(`{"t":"createWorkspace","text":"Work","color":"#5b7fd6"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "workspaces" && len(list(m)) == 2 })
	work := list(m)[1].(map[string]any)["id"].(string)
	r.e.Settle()

	var lines []string
	entries, _ := timeline.Since(from)
	for _, e := range entries {
		if e.Kind == timeline.KindInfo && strings.Contains(e.Text, dir) {
			lines = append(lines, e.Text)
		}
	}
	got := strings.Join(lines, "\n")
	sum := sha256.Sum256([]byte(work))
	for _, want := range []string{
		"profile: the engine's storage in " + filepath.Join(dir, "chromium"),
		"in " + filepath.Join(dir, "prefs.json"),
		"workspaces: 1, listed in " + filepath.Join(dir, "workspaces.json"),
		"workspace the first: its sites in " + filepath.Join(dir, "chromium"),
		"workspace " + work + ": made, its sites in " + filepath.Join(dir, "chromium", "Storage", "ext", "workspace", fmt.Sprintf("%X", sum[:6])),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no line with %q in:\n%s", want, got)
		}
	}
}
