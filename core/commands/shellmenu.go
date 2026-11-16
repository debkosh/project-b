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
// item is that item, and lists are their items.
func (m *Manager) ShellMenu(shell string, phone bool) []ShellItem {
	var out []ShellItem
	for _, n := range m.tree {
		items := m.shellItems(n.Items, n.Scope, shell, phone)
		if n.Role == "app" {
			out = append(out, items...)
			if len(items) > 0 {
				out = append(out, ShellItem{Separator: true})
			}
			continue
		}
		if sub, ok := m.shellSub(n, n.Scope, shell, phone); ok {
			out = append(out, sub)
		}
	}
	return tidy(out)
}

func (m *Manager) shellItems(nodes []Node, scope, shell string, phone bool) []ShellItem {
	var out []ShellItem
	for _, n := range nodes {
		s := scope
		if n.Scope != "" {
			s = n.Scope
		}
		if (n.Only != "" && n.Only != m.platform) || s == ScopeApp || s == ScopeKeys || (phone && n.Desktop) {
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
			if sub, ok := m.shellSub(n, s, shell, phone); ok {
				out = append(out, sub)
			}
		}
	}
	return tidy(out)
}

// shellSub is a submenu with what it holds, or what it holds alone when that is one action,
// whose title says what it does; none when it holds nothing. A list's item needs its
// submenu's title, as Main does under New Tab in Workspace.
func (m *Manager) shellSub(n Node, scope, shell string, phone bool) (ShellItem, bool) {
	items := m.shellItems(n.Items, scope, shell, phone)
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
