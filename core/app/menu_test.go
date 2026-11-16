package app_test

import (
	"slices"
	"testing"
)

// A shell that opens its menu asks Go for it, and gets the menu bar's menus for its window:
// Settings first, the page's menus, and nothing of the whole program's, such as Quit.
func TestAShellGetsTheMenuBarsMenus(t *testing.T) {
	r := start(t, t.TempDir())
	defer r.stop()
	r.shell(`{"t":"menu"}`)
	m := r.waitFor(func(m map[string]any) bool { return m["t"] == "menu" })
	menu, _ := m["menu"].([]any)
	var titles []string
	var walk func([]any)
	walk = func(items []any) {
		for _, x := range items {
			it := x.(map[string]any)
			if title, ok := it["title"].(string); ok {
				titles = append(titles, title)
			}
			if sub, ok := it["items"].([]any); ok {
				walk(sub)
			}
		}
	}
	walk(menu)
	if len(menu) == 0 || menu[0].(map[string]any)["action"] != "app.settings" {
		t.Fatalf("the menu starts %v", menu)
	}
	for _, want := range []string{"File", "View", "History", "Bookmarks", "Tab", "New Tab", "Reload This Page", "Downloads"} {
		if !slices.Contains(titles, want) {
			t.Errorf("no %q in %v", want, titles)
		}
	}
	for _, out := range []string{"Quit Project B", "About Project B", "Copy", "Paste", "Minimize", "Select Next Tab"} {
		if slices.Contains(titles, out) {
			t.Errorf("%q is in the shell's menu: %v", out, titles)
		}
	}
}
