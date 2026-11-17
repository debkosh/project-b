package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"projectb/engine"
	"projectb/flags"
	"projectb/prefs"
	"projectb/search"
	"projectb/shell"
	"projectb/shells"
	"projectb/timeline"
	"projectb/tones"
)

// The welcome (docs/onboarding.md): a page of ours, chrome://welcome/, that the start which
// made the profile opens over its whole window, with none of the shell around it, until
// Start browsing. Settings' Run setup again opens it later in a tab, as any page of ours.
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
}

// welcomeFile is what welcome.json holds: whether the welcome was finished, and when.
type welcomeFile struct {
	Done bool      `json:"done"`
	At   time.Time `json:"at,omitzero"`
}

func newWelcome(a *App, e engine.Engine, dir string) *welcome {
	w := &welcome{a: a, e: e, path: filepath.Join(dir, "welcome.json"), shown: map[engine.TabID]bool{}}
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

// openInTab opens the welcome in a tab of window win, as Run setup again does.
func (w *welcome) openInTab(win engine.WindowID) { w.a.Tabs.OpenPage(win, welcomePage) }

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

// welcomeMessage is what the page sends: t, a pref's name and value, and a switch's state.
type welcomeMessage struct {
	T     string `json:"t"`
	Name  string `json:"name"`
	Value string `json:"value"`
	On    bool   `json:"on"`
}

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
	case "done":
		w.finish(tab)
	default:
		timeline.Warnf("welcome: the page sent %q, which it has no use for", m.T)
	}
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
	return map[string]any{
		"t":             "snapshot",
		"whole":         tab == w.whole,
		"phone":         shells.OnPhone,
		"theme":         w.a.Prefs.Theme(),
		"tones":         tones.Of(w.a.Window.Color()),
		"prefs":         w.a.Prefs.All(),
		"searchEngines": search.All(),
		"autoUpdate":    w.a.Update.Auto(),
		"layouts": []welcomeShell{
			{ID: "side", Shell: "standard", TabLayout: "vertical", Name: "Tabs on the side"},
			{ID: "top", Shell: "standard", TabLayout: "horizontal", Name: "Tabs on top"},
			{ID: "glide", Shell: "glide", Name: "Glide"},
		},
	}
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
