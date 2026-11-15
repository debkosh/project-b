package app

import (
	"encoding/json"
	"fmt"
	"strconv"

	"projectb/commands"
	"projectb/engine"
	"projectb/prefs"
	"projectb/session"
	"projectb/settings"
	"projectb/tabs"
	"projectb/window"
)

// The profile's windows and tabs, kept as they change and opened again at the next start,
// and the tabs and windows the person closed, to bring back (docs/daily-driver.md).

// restore opens the windows and tabs the last run kept, the first in window first, and
// says whether it did: at every start when the On start setting says continue, and after
// a crash whatever it says. Each window's active tab loads now; the others once first
// shown.
func (a *App) restore(e engine.Engine, first engine.WindowID) bool {
	last := a.Session.Last()
	// What was closed in the last run comes back into the windows there are now.
	a.closed = last.Closed
	for i := range a.closed {
		a.closed[i].In = 0
	}
	if a.Crash.Last() == "" && a.Prefs.Start() != prefs.StartContinue || len(last.Windows) == 0 {
		return false
	}
	for i, sw := range last.Windows {
		w := first
		if i > 0 {
			w = a.openWindow()
		}
		a.openSessionWindow(e, w, sw)
	}
	// The window that was in front comes to the front again.
	e.Post(func() { a.Window.Request(first, engine.WindowFront) })
	return true
}

// openSessionWindow puts a kept window's tabs and groups in window w, where it was on
// screen. Its active tab loads now; the others once first shown.
func (a *App) openSessionWindow(e engine.Engine, w engine.WindowID, sw session.Window) {
	if b := sw.Bounds; b.W > 0 && b.H > 0 {
		// Once the app has started and listens to the engine.
		e.Post(func() { e.SetWindowBounds(w, b) })
	}
	ids := make([]engine.TabID, 0, len(sw.Tabs))
	for j, st := range sw.Tabs {
		ids = append(ids, a.restoreTab(w, st, j == sw.Active))
	}
	if len(ids) == 0 {
		a.Tabs.Create(w)
		return
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

// restoreTab opens a kept tab at the end of window w, in its workspace, or the first when
// that one is gone; it loads now when load.
func (a *App) restoreTab(w engine.WindowID, st session.Tab, load bool) engine.TabID {
	ws := st.Workspace
	if !a.knownWorkspace(ws) {
		ws = a.Workspaces.First()
	}
	if st.URL == "" {
		return a.Tabs.Add(w, ws, false)
	}
	return a.Tabs.Restore(w, ws, st.URL, st.Title, load)
}

func (a *App) knownWorkspace(id string) bool {
	for _, ws := range a.Workspaces.List() {
		if ws.ID == id {
			return true
		}
	}
	return false
}

// keepSession writes the profile's windows and tabs whenever they change: a tab's
// address, title or place, a group, a window's frame; and keeps what the person closes.
func (a *App) keepSession(e engine.Engine) {
	// A change comes in a burst (a close moves the active tab and changes the list): one
	// save after the burst, in the main loop's next turn.
	pending := false
	later := func() {
		if pending {
			return
		}
		pending = true
		e.Post(func() {
			pending = false
			a.saveSession()
		})
	}
	a.Tabs.OnListChanged(later)
	a.Tabs.OnChanged(func(tabs.Tab) { later() })
	a.Window.OnEvent(func(ev window.Event) {
		if ev.Kind == window.BoundsChanged {
			later()
		}
	})
	a.Tabs.OnClosedAt(func(t tabs.Tab, w engine.WindowID, index int) {
		if a.quitting || t.Page != "" {
			return
		}
		st := sessionTab(t)
		a.keepClosed(session.Closed{Tab: &st, In: int64(w), Index: index})
	})
}

// windowClosing keeps a window the person closed, before its tabs go with it. One that
// closes because the app quits is the session's, not closed.
func (a *App) windowClosing(w engine.WindowID) {
	if a.quitting || len(a.Tabs.Windows()) <= 1 {
		return
	}
	if sw := a.sessionWindow(w); len(sw.Tabs) > 0 {
		a.keepClosed(session.Closed{Window: &sw})
	}
}

func (a *App) keepClosed(c session.Closed) {
	a.closed = append([]session.Closed{c}, a.closed...)
	if len(a.closed) > session.ClosedLimit {
		a.closed = a.closed[:session.ClosedLimit]
	}
	a.saveSession()
	a.refreshMenu()
}

// reopen brings back the closed tab or window at i, newest first: a tab into its window at
// its place, or the window in front when its own is gone, active and loading; a window
// where it was.
func (a *App) reopen(e engine.Engine, i int) {
	if i < 0 || i >= len(a.closed) {
		return
	}
	c := a.closed[i]
	a.closed = append(a.closed[:i:i], a.closed[i+1:]...)
	if c.Window != nil {
		a.openSessionWindow(e, a.openWindow(), *c.Window)
		a.saveSession()
		a.refreshMenu()
		return
	}
	w := engine.WindowID(c.In)
	if !a.windowOpen(w) {
		w = a.Tabs.Front()
	}
	if w == 0 {
		w = a.openWindow()
	}
	id := a.restoreTab(w, *c.Tab, true)
	a.Tabs.Reorder(id, c.Index)
	a.Tabs.Activate(id)
	a.saveSession()
	a.refreshMenu()
}

func (a *App) windowOpen(w engine.WindowID) bool {
	for _, x := range a.Tabs.Windows() {
		if x == w {
			return true
		}
	}
	return false
}

// closedList is the History menu's Recently Closed: each closed tab by its title, each
// window by its number of tabs, newest first.
func (a *App) closedList(e engine.Engine) commands.List {
	return commands.List{
		ID: "recentlyClosed",
		Items: func() []commands.ListItem {
			var items []commands.ListItem
			for i, c := range a.closed {
				title := ""
				if c.Window != nil {
					title = fmt.Sprintf("%d tabs", len(c.Window.Tabs))
					if len(c.Window.Tabs) == 1 {
						title = "1 tab"
					}
				} else {
					title = c.Tab.Title
					if title == "" {
						title = c.Tab.URL
					}
				}
				items = append(items, commands.ListItem{Key: strconv.Itoa(i), Title: title})
			}
			return items
		},
		Choose: func(key string) {
			if i, err := strconv.Atoi(key); err == nil {
				a.reopen(e, i)
			}
		},
	}
}

// startSettingsPart gives the settings page the On start setting and takes its changes.
func (a *App) startSettingsPart() settings.Part {
	return settings.Part{
		Snapshot: func(into map[string]any) { into["start"] = a.Prefs.Start() },
		Handle: func(_ engine.TabID, t string, msg []byte) bool {
			if t != "setStart" {
				return false
			}
			var c struct {
				Text string `json:"text"`
			}
			json.Unmarshal(msg, &c)
			a.Prefs.SetStart(c.Text)
			a.Settings.Send(map[string]any{"t": "start", "start": a.Prefs.Start()})
			return true
		},
	}
}

// saveSession keeps the windows as they are now, and what was closed. Closing the last
// window keeps what was there before, for the next start, and so does quitting.
func (a *App) saveSession() {
	windows := a.Tabs.Windows()
	if a.quitting || len(windows) == 0 {
		return
	}
	s := session.Session{Closed: a.closed}
	for _, w := range windows {
		s.Windows = append(s.Windows, a.sessionWindow(w))
	}
	a.Session.Save(s)
}

// sessionWindow is window w as the session keeps it.
func (a *App) sessionWindow(w engine.WindowID) session.Window {
	sw := session.Window{Bounds: a.Window.Bounds(w), Tabs: []session.Tab{}}
	active := a.Tabs.ActiveIn(w)
	for _, t := range a.Tabs.AllIn(w) {
		// Our own pages, such as settings, aren't kept.
		if t.Page != "" {
			continue
		}
		if t.ID == active {
			sw.Active = len(sw.Tabs)
		}
		sw.Tabs = append(sw.Tabs, sessionTab(t))
	}
	for _, g := range a.Tabs.GroupsIn(w) {
		sw.Groups = append(sw.Groups, session.Group{ID: g.ID, Name: g.Name, Color: g.Color, Collapsed: g.Collapsed})
	}
	return sw
}

func sessionTab(t tabs.Tab) session.Tab {
	return session.Tab{URL: t.URL, Title: t.Title, Workspace: t.Workspace, Group: t.Group}
}
