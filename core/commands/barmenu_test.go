package commands

import (
	"slices"
	"testing"

	"projectb/mockengine"
)

// Off the Mac the menu bar the shell draws has every menu, and the app menu's items where
// Linux apps keep them: Quit ends File, Settings ends Edit, About ends Help.
func TestBarMenuOffTheMac(t *testing.T) {
	m := New(mockengine.New(), t.TempDir(), func() bool { return false })
	m.platform = Linux
	for _, id := range []string{"app.about", "app.settings", "app.checkUpdate", "app.quit", "app.hide", "tab.new", "edit.copy", "edit.selectAll", "window.minimize", "profile.picker", "downloads.show"} {
		m.Register(Action{ID: id, Title: id, Run: func() {}})
	}
	m.Load()
	bar := m.BarMenu("pbshell")
	var titles []string
	for _, it := range bar {
		titles = append(titles, it.Title)
	}
	for _, want := range []string{"File", "Edit", "Profiles", "Window", "Help"} {
		if !slices.Contains(titles, want) {
			t.Errorf("no %s menu: %v", want, titles)
		}
	}
	if slices.Contains(titles, "Project B") {
		t.Errorf("an app menu off the Mac: %v", titles)
	}
	last := func(title string) string {
		i := slices.IndexFunc(bar, func(s ShellItem) bool { return s.Title == title })
		if i < 0 || len(bar[i].Items) == 0 {
			return ""
		}
		return bar[i].Items[len(bar[i].Items)-1].Action
	}
	for menu, want := range map[string]string{"File": "app.quit", "Edit": "app.settings", "Help": "app.about"} {
		if got := last(menu); got != want {
			t.Errorf("%s ends with %q, want %q", menu, got, want)
		}
	}
	var walk func([]ShellItem)
	walk = func(items []ShellItem) {
		for _, it := range items {
			if it.Action == "app.hide" {
				t.Errorf("the Mac's Hide off the Mac")
			}
			walk(it.Items)
		}
	}
	walk(bar)
}
