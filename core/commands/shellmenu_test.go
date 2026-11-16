package commands

import (
	"encoding/json"
	"testing"

	"projectb/mockengine"
)

// The shell's menu is the menu bar's tree for this window and its profile: what is the whole
// program's or made for a key stays out, a phone leaves the desktop's out, only the shell's
// own actions of the shells' show, a submenu of one action is that action, a list keeps its
// submenu, the app menu's items lead, an entry marked for the top level follows its menu
// there, and no line is left at an end or doubled.
func TestTheShellsMenu(t *testing.T) {
	m := New(mockengine.New(), t.TempDir(), func() bool { return false })
	for _, a := range []Action{
		{ID: "app.about"}, {ID: "app.settings", Title: "Settings…"}, {ID: "app.quit"},
		{ID: "tab.new", Title: "New Tab"}, {ID: "window.new", Title: "New Window"}, {ID: "tab.close", Title: "Close Tab", Enabled: func() bool { return false }},
		{ID: "page.print", Title: "Print…"},
		{ID: "edit.copy"}, {ID: "find.open", Title: "Find…"}, {ID: "find.next"},
		{ID: "view.tabsSide", Title: "Tabs on Side", Checked: func() bool { return true }},
		{ID: "glide.ownTest", Title: "Glide's", Shell: "glide"}, {ID: "terminal.ownTest", Title: "Zoom Pane", Shell: "terminal"},
	} {
		m.Register(a)
	}
	m.RegisterList(List{ID: "shells", Items: func() []ListItem { return []ListItem{{Key: "terminal", Title: "Terminal", Checked: true}} }})
	m.Load()
	m.tree = []Node{
		{Title: "Project B", Role: "app", Items: []Node{{Action: "app.about", Scope: ScopeApp}, {Separator: true}, {Action: "app.settings"}, {Separator: true}, {Action: "app.quit", Scope: ScopeApp}}},
		{Title: "File", Items: []Node{{Action: "tab.new"}, {Action: "window.new", Desktop: true}, {Separator: true}, {Separator: true}, {Action: "tab.close"}, {Separator: true}, {Action: "page.print", Shell: ShellTop}}},
		{Title: "Edit", Items: []Node{{Action: "edit.copy", Scope: ScopeKeys}, {Separator: true}, {Title: "Find", Items: []Node{{Action: "find.open"}, {Action: "find.next", Scope: ScopeKeys}}}}},
		{Title: "View", Items: []Node{{Action: "view.tabsSide", Desktop: true}, {Title: "Shell", Items: []Node{{List: "shells"}}}, {Action: "glide.ownTest"}, {Action: "terminal.ownTest"}}},
		{Title: "Profiles", Scope: ScopeApp, Items: []Node{{Action: "tab.new"}}},
	}
	got := func(phone bool) string {
		b, _ := json.Marshal(m.ShellMenu("terminal", phone))
		return string(b)
	}
	desktop := `[{"title":"Settings…","action":"app.settings","enabled":true},{"separator":true},` +
		`{"title":"File","items":[{"title":"New Tab","action":"tab.new","enabled":true},{"title":"New Window","action":"window.new","enabled":true},{"separator":true},{"title":"Close Tab","action":"tab.close"}]},` +
		`{"title":"Print…","action":"page.print","enabled":true},` +
		`{"title":"Find…","action":"find.open","enabled":true},` +
		`{"title":"View","items":[{"title":"Tabs on Side","action":"view.tabsSide","enabled":true,"checked":true},{"title":"Shell","items":[{"title":"Terminal","action":"shells:terminal","enabled":true,"checked":true}]},{"title":"Zoom Pane","action":"terminal.ownTest","enabled":true}]}]`
	if g := got(false); g != desktop {
		t.Fatalf("on a desktop the shell's menu is\n%s\nnot\n%s", g, desktop)
	}
	phone := `[{"title":"Settings…","action":"app.settings","enabled":true},{"separator":true},` +
		`{"title":"File","items":[{"title":"New Tab","action":"tab.new","enabled":true},{"separator":true},{"title":"Close Tab","action":"tab.close"}]},` +
		`{"title":"Print…","action":"page.print","enabled":true},` +
		`{"title":"Find…","action":"find.open","enabled":true},` +
		`{"title":"View","items":[{"title":"Shell","items":[{"title":"Terminal","action":"shells:terminal","enabled":true,"checked":true}]},{"title":"Zoom Pane","action":"terminal.ownTest","enabled":true}]}]`
	if g := got(true); g != phone {
		t.Fatalf("on a phone the shell's menu is\n%s\nnot\n%s", g, phone)
	}
}
