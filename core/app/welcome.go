package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"projectb/commands"
	"projectb/engine"
	"projectb/flags"
	"projectb/prefs"
	"projectb/search"
	"projectb/shell"
	"projectb/shells"
	"projectb/timeline"
	"projectb/tones"
	"projectb/workspaces"
)

// The welcome (docs/onboarding.md): a page of ours, chrome://welcome/, that the start which
// made the profile opens over its whole window, with none of the shell around it, until
// Start browsing. Show the welcome again, in Settings' Flags, marks it unfinished and closes
// the app, so the next start opens it as the first did: over the whole window, with the
// tabs that start brings back (On start, open) under it.
// Each choice it offers is a pref, set as it is made, so the page and the window take it
// at once. welcome.json in the profile says whether it was finished: a welcome left before
// the end opens again at the next start. The flag Show the welcome at each start opens it
// at every start while it is on. A folder named with --user-data-dir was made by no start,
// so it shows no welcome unless the flag asks.

// welcomePage is the welcome's name among our pages.
const welcomePage = "welcome"

type welcome struct {
	a    *App
	e    engine.Engine
	path string
	// whole is the welcome tab that has its window to itself, or 0. A shell draws nothing
	// around it while it is that window's active tab.
	whole engine.TabID
	// shown are the welcome tabs whose page has said it is ready, to send them changes.
	shown map[engine.TabID]bool
	// made are the workspaces How you work made, by name, and renamed says it gave Main
	// the first one's name, so a step taken again can undo either.
	made    map[string]string
	renamed bool
}

// welcomeFile is what welcome.json holds: whether the welcome was finished, and when.
type welcomeFile struct {
	Done bool      `json:"done"`
	At   time.Time `json:"at,omitzero"`
}

func newWelcome(a *App, e engine.Engine, dir string) *welcome {
	w := &welcome{a: a, e: e, path: filepath.Join(dir, "welcome.json"), shown: map[engine.TabID]bool{}, made: map[string]string{}}
	a.Prefs.OnChanged(func(p prefs.Prefs) { w.sendAll(map[string]any{"t": "prefs", "prefs": p}) })
	a.Prefs.OnSearchEngineChanged(func(string) { w.sendAll(map[string]any{"t": "prefs", "prefs": a.Prefs.All()}) })
	a.Prefs.OnThemeChanged(func(t string) { w.sendAll(map[string]any{"t": "theme", "theme": t}) })
	a.Window.OnColorChanged(func(c string) { w.sendAll(map[string]any{"t": "tones", "tones": tones.Of(c)}) })
	a.Update.OnAutoChanged(func(on bool) { w.sendAll(map[string]any{"t": "autoUpdate", "autoUpdate": on}) })
	// The whole window is the welcome's only while its tab is there.
	a.Tabs.OnListChanged(func() {
		if _, ok := a.Tabs.Get(w.whole); w.whole != 0 && !ok {
			w.setWhole(0)
		}
	})
	return w
}

// due says the welcome opens at this start: this start made the profile, one before it
// left the welcome unfinished, or the flag asks for it.
func (w *welcome) due(made bool) bool {
	if w.a.Flags.IsOn(flags.ShowWelcome) {
		return true
	}
	b, err := os.ReadFile(w.path)
	if os.IsNotExist(err) {
		return made
	}
	var f welcomeFile
	if err := json.Unmarshal(b, &f); err != nil {
		timeline.Errorf("welcome: %v", err)
	}
	return !f.Done
}

// openWhole opens the welcome in window win over all of it, as a first start does, and
// keeps that it isn't finished yet.
func (w *welcome) openWhole(win engine.WindowID) {
	if _, err := os.Stat(w.path); os.IsNotExist(err) {
		w.write(welcomeFile{})
	}
	timeline.Infof("welcome: shown over window %d, until it is finished; %s says when it is", win, w.path)
	w.setWhole(w.a.Tabs.OpenPage(win, welcomePage))
}

func (w *welcome) write(f welcomeFile) {
	b, _ := json.Marshal(f)
	if err := os.WriteFile(w.path, append(b, '\n'), 0o600); err != nil {
		timeline.Errorf("welcome: %v", err)
	}
}

// again marks the welcome unfinished, for the next start to open it, as Show the welcome
// again does before it closes the app.
func (w *welcome) again() {
	timeline.Infof("welcome: marked unfinished, for the next start")
	w.write(welcomeFile{})
}

func (w *welcome) setWhole(tab engine.TabID) {
	if tab == w.whole {
		return
	}
	w.whole = tab
	w.a.Shell.Send(map[string]any{"t": "welcome", "welcome": tab})
}

// shellPart tells every shell which tab is the welcome over its whole window.
func (w *welcome) shellPart() shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["welcome"] = w.whole },
		Handle:   func(engine.WindowID, string, []byte) bool { return false },
	}
}

func (w *welcome) send(tab engine.TabID, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		timeline.Errorf("welcome: %v", err)
		return
	}
	w.e.SendToPage(tab, b)
}

func (w *welcome) sendAll(v any) {
	for tab := range w.shown {
		if _, ok := w.a.Tabs.Get(tab); !ok {
			delete(w.shown, tab)
			continue
		}
		w.send(tab, v)
	}
}

// welcomeMessage is what the page sends: t, a pref's name and value, a switch's state, and
// the workspaces ticked.
type welcomeMessage struct {
	T          string                 `json:"t"`
	Name       string                 `json:"name"`
	Value      string                 `json:"value"`
	On         bool                   `json:"on"`
	Workspaces []workspaces.Workspace `json:"workspaces"`
}

// welcomeKeymaps are the keymaps How you work offers: Project B's own, which are Chrome's,
// and Firefox's.
var welcomeKeymaps = []string{commands.DefaultTemplate, "firefox"}

func (w *welcome) handle(tab engine.TabID, msg []byte) {
	var m welcomeMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		timeline.Warnf("welcome: bad message: %v", err)
		return
	}
	switch m.T {
	case "timeline":
		timeline.FromPage(welcomePage, msg)
	case "ready":
		w.shown[tab] = true
		w.send(tab, w.snapshot(tab))
	case "setPref":
		if !w.a.Prefs.Set(m.Name, m.Value) {
			timeline.Warnf("welcome: no pref %q", m.Name)
		}
	case "setSearchEngine":
		w.a.Prefs.SetSearchEngine(m.Value)
	case "setAutoUpdate":
		w.a.Update.SetAuto(m.On)
	case "useKeymap":
		if slices.Contains(welcomeKeymaps, m.Value) {
			w.a.Commands.UseTemplate(m.Value)
			w.sendAll(map[string]any{"t": "keymap", "keymap": m.Value})
		}
	case "setWorkspaces":
		w.starters(m.Workspaces)
		w.sendAll(map[string]any{"t": "workspaces", "workspaces": w.a.Workspaces.List()})
	case "makeDefault":
		timeline.Infof("welcome: asking the system to make this the default browser")
		w.e.AskDefaultBrowser()
	case "askNotifications":
		w.e.AskNotifications()
	case "done":
		w.finish(tab)
	default:
		if !w.a.imports.handle(m.T, msg) {
			timeline.Warnf("welcome: the page sent %q, which it has no use for", m.T)
		}
	}
}

// DefaultBrowserChanged tells the welcome pages and the settings page how asking to be the
// default browser ended.
func (a *App) DefaultBrowserChanged(is bool) {
	timeline.Infof("welcome: the default browser is this one: %v", is)
	m := map[string]any{"t": "isDefault", "isDefault": is}
	a.welcome.sendAll(m)
	a.Settings.Send(m)
}

// NotificationsAnswered tells the welcome pages whether the app may show its notifications.
func (a *App) NotificationsAnswered(allowed bool) {
	timeline.Infof("welcome: notifications allowed: %v", allowed)
	a.welcome.sendAll(map[string]any{"t": "notifications", "notifications": allowed})
}

// welcomeShell is a way to work the style step offers: a shell, and for Standard where
// its tabs go.
type welcomeShell struct {
	ID        string `json:"id"`
	Shell     string `json:"shell"`
	TabLayout string `json:"tabLayout,omitempty"`
	Name      string `json:"name"`
}

func (w *welcome) snapshot(tab engine.TabID) map[string]any {
	s := map[string]any{
		"t":             "snapshot",
		"whole":         tab == w.whole,
		"phone":         shells.OnPhone,
		"theme":         w.a.Prefs.Theme(),
		"tones":         tones.Of(w.a.Window.Color()),
		"prefs":         w.a.Prefs.All(),
		"searchEngines": search.All(),
		"autoUpdate":    w.a.Update.Auto(),
		"isDefault":     w.e.IsDefaultBrowser(),
		"notifications": w.e.NotificationsAllowed(),
		"layouts": []welcomeShell{
			{ID: "side", Shell: "standard", TabLayout: "vertical", Name: "Tabs on the side"},
			{ID: "top", Shell: "standard", TabLayout: "horizontal", Name: "Tabs on top"},
			{ID: "glide", Shell: "glide", Name: "Glide"},
		},
	}
	keymaps := map[string]map[string]string{}
	for _, k := range welcomeKeymaps {
		keymaps[k] = commands.TemplateKeys(k, w.a.Commands.Platform())
	}
	s["keymaps"] = keymaps
	s["keymap"] = w.a.Commands.Template()
	s["platform"] = w.a.Commands.Platform()
	s["workspaces"] = w.a.Workspaces.List()
	w.a.imports.snapshot(s)
	return s
}

// starters makes the workspaces How you work has ticked, in its order. The first takes
// Main's place, renamed and recoloured, while Main is as a new profile has it; the rest
// are made. Taken again, the step undoes what it did that is no longer ticked: a workspace
// it made goes, with nothing in it yet, and Main gets its name back when none is ticked.
func (w *welcome) starters(ticked []workspaces.Workspace) {
	ws := w.a.Workspaces
	list := ws.List()
	fresh := len(list) == 1 && list[0].ID == "" && list[0].Name == "Main"
	rest := ticked
	takesMain := len(ticked) > 0 && (fresh || w.renamed)
	if takesMain {
		rest = ticked[1:]
	}
	for name, id := range w.made {
		if !slices.ContainsFunc(rest, func(t workspaces.Workspace) bool { return t.Name == name }) {
			if err := w.a.deleteWorkspace(w.e, id); err != nil {
				timeline.Warnf("welcome: %v", err)
			}
			delete(w.made, name)
		}
	}
	switch {
	case takesMain:
		w.name("", ticked[0])
		w.renamed = true
	case w.renamed:
		w.name("", workspaces.Workspace{Name: "Main", Color: "#3470d0"})
		w.renamed = false
	}
	for _, t := range rest {
		if _, ok := w.made[t.Name]; ok || slices.ContainsFunc(ws.List(), func(x workspaces.Workspace) bool { return x.Name == t.Name }) {
			continue
		}
		made, err := ws.Create(t.Name, t.Color)
		if err != nil {
			timeline.Warnf("welcome: workspace %q: %v", t.Name, err)
			continue
		}
		w.made[t.Name] = made.ID
	}
	var names []string
	for _, x := range ws.List() {
		names = append(names, x.Name)
	}
	timeline.Infof("welcome: the workspaces are %q", names)
}

// name gives the workspace id t's name and colour.
func (w *welcome) name(id string, t workspaces.Workspace) {
	if err := w.a.Workspaces.SetName(id, t.Name); err != nil {
		timeline.Warnf("welcome: workspace %q: %v", t.Name, err)
	}
	if err := w.a.Workspaces.SetColor(id, t.Color); err != nil {
		timeline.Warnf("welcome: workspace %q: %v", t.Name, err)
	}
}

// skip keeps that the welcome was finished without showing it, as --skip-welcome asks: what
// its Skip setup does.
func (w *welcome) skip() {
	w.write(welcomeFile{Done: true, At: time.Now().UTC()})
	timeline.Infof("welcome: skipped, as --skip-welcome asks; noted as finished in %s", w.path)
}

// finish keeps that the welcome was finished and closes its tab. The window it had to
// itself gets its shell back, with a new tab where the shell opens one.
func (w *welcome) finish(tab engine.TabID) {
	w.write(welcomeFile{Done: true, At: time.Now().UTC()})
	timeline.Infof("welcome: finished, noted in %s", w.path)
	win := w.a.Tabs.WindowOf(tab)
	if tab == w.whole {
		w.setWhole(0)
	}
	w.a.Tabs.Close(tab)
	if win != 0 && len(w.a.Tabs.AllIn(win)) == 0 && !w.a.startsEmpty() {
		w.a.Tabs.Create(win)
	}
}
