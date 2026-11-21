package commands

import "slices"

// ShellItem is one item of a shell's own menu: an action by its id, which the shell runs
// with the run message, or a list's item, "<list>:<key>"; a separator; or a submenu.
type ShellItem struct {
	Title     string      `json:"title,omitempty"`
	Action    string      `json:"action,omitempty"`
	Enabled   bool        `json:"enabled,omitempty"`
	Checked   bool        `json:"checked,omitempty"`
	Separator bool        `json:"separator,omitempty"`
	Items     []ShellItem `json:"items,omitempty"`
}

// ShellMenu is the menu bar's tree as the shell's own menu shows it, as it is now: only the
// entries for this window and its profile (Node.Scope), on a phone without the desktop's,
// and of the shells' own actions only the shell's, whose id is shell. The menu bar's menus
// are its submenus, but the program's own, whose items come first; a submenu left with one
// item is that item, and lists are their items. An entry marked ShellTop leaves its submenu
// for the top level, just after the menu it was in.
func (m *Manager) ShellMenu(shell string, phone bool) []ShellItem {
	var out []ShellItem
	for _, n := range m.tree {
		var top []ShellItem
		items := m.shellItems(n.Items, n.Scope, shell, phone, &top)
		if n.Role == "app" {
			out = append(out, items...)
			out = append(out, top...)
			if len(items)+len(top) > 0 {
				out = append(out, ShellItem{Separator: true})
			}
			continue
		}
		if sub, ok := shellSub(n, items); ok {
			out = append(out, sub)
		}
		out = append(out, top...)
	}
	return tidy(out)
}

// shellItems is what nodes show in the shell's menu, each lifted to its top level added to
// top instead.
func (m *Manager) shellItems(nodes []Node, scope, shell string, phone bool, top *[]ShellItem) []ShellItem {
	var out []ShellItem
	for _, n := range nodes {
		dest := &out
		if n.Shell == ShellTop {
			dest = top
		}
		s := scope
		if n.Scope != "" {
			s = n.Scope
		}
		if (n.Only != "" && n.Only != m.platform) || s == ScopeApp || s == ScopeKeys || (phone && n.Desktop) {
			continue
		}
		switch {
		case n.Separator:
			*dest = append(*dest, ShellItem{Separator: true})
		case n.List != "":
			for _, it := range m.ListItems(n.List) {
				*dest = append(*dest, ShellItem{Title: it.Title, Action: it.Action, Enabled: true, Checked: it.Checked})
			}
		case n.Action != "":
			a, ok := m.actions[n.Action]
			if !ok || m.hidden(n) || (a.Shell != "" && a.Shell != shell) {
				continue
			}
			title := a.Title
			if a.Label != nil {
				title = a.Label()
			}
			*dest = append(*dest, ShellItem{Title: title, Action: a.ID, Enabled: a.Enabled == nil || a.Enabled(), Checked: a.Checked != nil && a.Checked()})
		default:
			if sub, ok := shellSub(n, m.shellItems(n.Items, s, shell, phone, top)); ok {
				*dest = append(*dest, sub)
			}
		}
	}
	return tidy(out)
}

// shellSub is submenu n holding items, or what it holds alone when that is one action,
// whose title says what it does; none when it holds nothing. A list's item needs its
// submenu's title, as Main does under New Tab in Workspace.
func shellSub(n Node, items []ShellItem) (ShellItem, bool) {
	switch {
	case len(items) == 0:
		return ShellItem{}, false
	case len(items) == 1 && !slices.ContainsFunc(n.Items, func(c Node) bool { return c.List != "" }):
		return items[0], true
	}
	return ShellItem{Title: n.Title, Items: items}, true
}

// tidy drops separators at either end and doubled ones, which entries left out leave.
func tidy(items []ShellItem) []ShellItem {
	var out []ShellItem
	for _, it := range items {
		if it.Separator && (len(out) == 0 || out[len(out)-1].Separator) {
			continue
		}
		out = append(out, it)
	}
	if len(out) > 0 && out[len(out)-1].Separator {
		out = out[:len(out)-1]
	}
	return out
}

// BarMenu is the menu bar's tree for a shell that draws the menu bar itself, off the Mac
// (Linux, Windows; ui/MenuBar.vue): every menu this platform's tree has, whatever its
// scope, with its submenus, lists as their items, and of the shells' own actions only
// shell's. These desktops have no app menu: its items go where their apps keep them, Quit
// at the end of File, Settings at the end of Edit, and the rest in Help, About last.
func (m *Manager) BarMenu(shell string) []ShellItem {
	var out []ShellItem
	var app []ShellItem
	for _, n := range m.tree {
		if n.Only != "" && n.Only != m.platform {
			continue
		}
		items := m.barItems(n.Items, shell)
		if n.Role == "app" {
			app = items
			continue
		}
		if len(items) > 0 {
			out = append(out, ShellItem{Title: n.Title, Items: items})
		}
	}
	add := func(title string, items ...ShellItem) {
		i := slices.IndexFunc(out, func(s ShellItem) bool { return s.Title == title })
		if i < 0 {
			out = append(out, ShellItem{Title: title})
			i = len(out) - 1
		}
		out[i].Items = tidy(append(append(out[i].Items, ShellItem{Separator: true}), items...))
	}
	var help, about []ShellItem
	for _, it := range app {
		switch it.Action {
		case "":
		case "app.quit":
			add("File", it)
		case "app.settings":
			add("Edit", it)
		case "app.about":
			about = append(about, it)
		default:
			help = append(help, it)
		}
	}
	if len(help)+len(about) > 0 {
		add("Help", append(append(help, ShellItem{Separator: true}), about...)...)
	}
	return out
}

// barItems is what nodes show in the menu bar a shell draws: BarMenu's.
func (m *Manager) barItems(nodes []Node, shell string) []ShellItem {
	var out []ShellItem
	for _, n := range nodes {
		if n.Only != "" && n.Only != m.platform {
			continue
		}
		switch {
		case n.Separator:
			out = append(out, ShellItem{Separator: true})
		case n.List != "":
			for _, it := range m.ListItems(n.List) {
				out = append(out, ShellItem{Title: it.Title, Action: it.Action, Enabled: true, Checked: it.Checked})
			}
		case n.Action != "":
			a, ok := m.actions[n.Action]
			if !ok || m.hidden(n) || (a.Shell != "" && a.Shell != shell) {
				continue
			}
			title := a.Title
			if a.Label != nil {
				title = a.Label()
			}
			out = append(out, ShellItem{Title: title, Action: a.ID, Enabled: a.Enabled == nil || a.Enabled(), Checked: a.Checked != nil && a.Checked()})
		default:
			if items := m.barItems(n.Items, shell); len(items) > 0 {
				out = append(out, ShellItem{Title: n.Title, Items: items})
			}
		}
	}
	return tidy(out)
}
