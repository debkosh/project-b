package app

import (
	"projectb/engine"
	"projectb/session"
	"projectb/tabs"
	"projectb/window"
)

// The profile's windows and tabs, kept as they change and opened again when the last run
// crashed (docs/daily-driver.md).

// restore opens the windows and tabs the last run kept, when it crashed, the first in
// window first, and says whether it did. Each window's active tab loads now; the others
// once first shown.
func (a *App) restore(e engine.Engine, first engine.WindowID) bool {
	last := a.Session.Last()
	if a.Crash.Last() == "" || len(last.Windows) == 0 {
		return false
	}
	known := map[string]bool{}
	for _, ws := range a.Workspaces.List() {
		known[ws.ID] = true
	}
	for i, sw := range last.Windows {
		w := first
		if i > 0 {
			w = a.openWindow()
		}
		if b := sw.Bounds; b.W > 0 && b.H > 0 {
			// Once the app has started and listens to the engine.
			e.Post(func() { e.SetWindowBounds(w, b) })
		}
		ids := make([]engine.TabID, 0, len(sw.Tabs))
		for j, st := range sw.Tabs {
			ws := st.Workspace
			if !known[ws] {
				ws = a.Workspaces.First()
			}
			if st.URL == "" {
				ids = append(ids, a.Tabs.Add(w, ws, false))
			} else {
				ids = append(ids, a.Tabs.Restore(w, ws, st.URL, st.Title, j == sw.Active))
			}
		}
		if len(ids) == 0 {
			a.Tabs.Create(w)
			continue
		}
		for _, g := range sw.Groups {
			var members []engine.TabID
			for j, st := range sw.Tabs {
				if st.Group == g.ID {
					members = append(members, ids[j])
				}
			}
			if len(members) == 0 {
				continue
			}
			id := a.Tabs.CreateGroup(members, g.Name, g.Color)
			if g.Collapsed {
				collapsed := true
				a.Tabs.EditGroup(id, nil, nil, &collapsed)
			}
		}
		a.Tabs.Activate(ids[min(max(sw.Active, 0), len(ids)-1)])
	}
	// The window that was in front comes to the front again.
	e.Post(func() { a.Window.Request(first, engine.WindowFront) })
	return true
}

// keepSession writes the profile's windows and tabs whenever they change: a tab's
// address, title or place, a group, a window's frame.
func (a *App) keepSession() {
	a.Tabs.OnListChanged(a.saveSession)
	a.Tabs.OnChanged(func(tabs.Tab) { a.saveSession() })
	a.Window.OnEvent(func(ev window.Event) {
		if ev.Kind == window.BoundsChanged {
			a.saveSession()
		}
	})
}

// saveSession keeps the windows as they are now. Closing the last window keeps what was
// there before, for the next start.
func (a *App) saveSession() {
	windows := a.Tabs.Windows()
	if len(windows) == 0 {
		return
	}
	var s session.Session
	for _, w := range windows {
		sw := session.Window{Bounds: a.Window.Bounds(w), Tabs: []session.Tab{}}
		active := a.Tabs.ActiveIn(w)
		for i, t := range a.Tabs.AllIn(w) {
			if t.ID == active {
				sw.Active = i
			}
			sw.Tabs = append(sw.Tabs, session.Tab{URL: t.URL, Title: t.Title, Workspace: t.Workspace, Group: t.Group})
		}
		for _, g := range a.Tabs.GroupsIn(w) {
			sw.Groups = append(sw.Groups, session.Group{ID: g.ID, Name: g.Name, Color: g.Color, Collapsed: g.Collapsed})
		}
		s.Windows = append(s.Windows, sw)
	}
	a.Session.Save(s)
}
