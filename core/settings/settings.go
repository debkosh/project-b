// Package settings is the contract with the settings page: one of our pages, which a tab
// of ours shows, one in a window at most (docs/windows.md). Like the shell's contract, it
// sends a snapshot when the page starts and events when things change, and turns the
// user's changes into calls on the managers. Every message names the tab whose page it is
// to or from. It owns nothing else.
package settings

import (
	"encoding/json"

	"projectb/commands"
	"projectb/engine"
	"projectb/flags"
	"projectb/prefs"
	"projectb/search"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/tones"
	"projectb/update"
	"projectb/webfeatures"
	"projectb/window"
)

// Page is the settings page's name among our pages: chrome://settings/.
const Page = "settings"

// command is what the settings page sends us.
type command struct {
	T       string `json:"t"`
	Text    string `json:"text"`
	ID      string `json:"id"`
	Index   int    `json:"index"`
	On      bool   `json:"on"`
	Reserve string `json:"reserve"`
}

// Manager connects the settings page to the managers.
type Manager struct {
	eng      engine.Engine
	tabs     *tabs.Manager
	window   *window.Manager
	prefs    *prefs.Manager
	flags    *flags.Manager
	update   *update.Manager
	commands *commands.Manager
	// pages holds each settings tab with what it was asked to show and whether its page
	// has said it's ready.
	pages map[engine.TabID]*page
	// recorder is the tab whose page is recording a key, or 0.
	recorder engine.TabID
	parts    []Part
	// relaunch starts the app again, for the flags to take effect.
	relaunch func()
}

// Part is what another manager adds to the settings page's contract: its state in the
// snapshot, and commands of its own, from the page in tab. Handle says whether the
// command was one of its.
type Part struct {
	Snapshot func(into map[string]any)
	Handle   func(tab engine.TabID, t string, msg []byte) bool
}

// AddPart adds a part.
func (m *Manager) AddPart(p Part) { m.parts = append(m.parts, p) }

// page is one settings tab's page.
type page struct {
	// section is the section a caller asked the page to show, or "" for its first.
	section string
	// Whether the page has said it's ready. Until then nothing reaches it: its script
	// isn't there to take a message, and its snapshot has it all.
	heard bool
	// then are the messages OpenSection has for the page after its snapshot.
	then []any
}

// New returns a manager.
func New(eng engine.Engine, t *tabs.Manager, w *window.Manager, p *prefs.Manager, f *flags.Manager, u *update.Manager, c *commands.Manager) *Manager {
	return &Manager{eng: eng, tabs: t, window: w, prefs: p, flags: f, update: u, commands: c, pages: map[engine.TabID]*page{}}
}

// OnRelaunch names what the Flags section's Relaunch does.
func (m *Manager) OnRelaunch(f func()) { m.relaunch = f }

// Open shows window w's settings tab, opening one when it has none, and returns it.
func (m *Manager) Open(w engine.WindowID) engine.TabID {
	if m.tabs.PageIn(w, Page) == 0 {
		timeline.Begin("settings.open")
		timeline.Mark("settings.open.go")
		defer timeline.Mark("settings.open.done")
	}
	return m.tabs.OpenPage(w, Page)
}

// OpenSection shows window w's settings tab at one of its sections, by id, and then tells
// its page the messages then: now, or after its snapshot when the page is still to start.
func (m *Manager) OpenSection(w engine.WindowID, section string, then ...any) {
	tab := m.Open(w)
	p := m.page(tab)
	p.section = section
	m.send(tab, map[string]any{"t": "section", "section": section})
	if !p.heard {
		p.then = append(p.then, then...)
		return
	}
	for _, v := range then {
		m.send(tab, v)
	}
}

// page is what the manager keeps for a settings tab, made the first time it is needed.
func (m *Manager) page(tab engine.TabID) *page {
	if m.pages[tab] == nil {
		m.pages[tab] = &page{}
	}
	return m.pages[tab]
}

// TabClosed forgets a settings tab that closed, and the key its page was recording.
func (m *Manager) TabClosed(t tabs.Tab) {
	delete(m.pages, t.ID)
	if t.ID == m.recorder {
		m.recorder = 0
		m.commands.SetRecording(false)
	}
}

// Handle takes one command from the settings page in tab.
func (m *Manager) Handle(tab engine.TabID, msg []byte) {
	var c command
	if err := json.Unmarshal(msg, &c); err != nil {
		timeline.Warnf("settings: bad message: %v", err)
		return
	}
	switch c.T {
	case "ready":
		p := m.page(tab)
		p.heard = true
		snapshot := map[string]any{
			"t":                  "snapshot",
			"windowColor":        m.window.Color(),
			"tones":              tones.Of(m.window.Color()),
			"tabLayout":          m.prefs.TabLayout(),
			"theme":              m.prefs.Theme(),
			"addressBarPosition": m.prefs.AddressBarPosition(),
			"searchEngine":       m.prefs.SearchEngine().ID,
			"searchEngines":      search.All(),
			"autoUpdate":         m.update.Auto(),
			"update":             m.update.State(),
			"running":            m.update.Running(),
			"keyboard":           m.commands.View(),
			"flags":              m.flags.View(),
			"webFeatures":        webfeatures.View(),
		}
		snapshot["section"] = p.section
		for _, part := range m.parts {
			part.Snapshot(snapshot)
		}
		m.send(tab, snapshot)
		for _, v := range p.then {
			m.send(tab, v)
		}
		p.then = nil
		timeline.Mark("settings.ready")
	case "timeline":
		timeline.FromPage("settings", msg)
	case "setTabLayout":
		m.prefs.SetTabLayout(c.Text)
	case "setTheme":
		m.prefs.SetTheme(c.Text)
	case "setAddressBarPosition":
		m.prefs.SetAddressBarPosition(c.Text)
	case "setSearchEngine":
		m.prefs.SetSearchEngine(c.Text)
	case "setAutoUpdate":
		m.update.SetAuto(c.On)
	case "setFrameColor":
		m.prefs.SetFrameColor(c.Text)
	case "checkUpdate":
		m.update.CheckNow()
	case "retryUpdate":
		m.update.Retry()
	case "cancelUpdate":
		m.update.Cancel()
	case "useTemplate":
		m.commands.UseTemplate(c.Text)
	case "addKey":
		if err := m.commands.AddBinding(c.ID, c.Text, c.Reserve); err != nil {
			timeline.Errorf("settings: %v", err)
		}
	case "removeKey":
		m.commands.RemoveBinding(c.ID, c.Index)
	case "setReserve":
		m.commands.SetReserve(c.ID, c.Index, c.Reserve)
	case "setHidden":
		m.commands.SetHidden(c.ID, c.On)
	case "resetAction":
		m.commands.ResetAction(c.ID)
	case "dismissMissing":
		m.commands.DismissMissing()
	case "recording":
		m.recorder = 0
		if c.On {
			m.recorder = tab
		}
		m.commands.SetRecording(c.On)
	case "setFlag":
		m.flags.Set(c.ID, c.Text)
	case "relaunch":
		// With a release ready, relaunching is applying it, as the shell's own button
		// does: on a phone that hands it to Android's installer, which a plain relaunch
		// never would.
		if m.update.State().Phase == update.Ready {
			m.update.Relaunch()
		} else if m.relaunch != nil {
			m.relaunch()
		}
	default:
		for _, p := range m.parts {
			if p.Handle(tab, c.T, msg) {
				return
			}
		}
		timeline.Warnf("settings: unknown command %q", c.T)
	}
}

// SearchEngineChanged tells the settings pages the engine chosen, by its id.
func (m *Manager) SearchEngineChanged(id string) {
	m.sendAll(map[string]any{"t": "searchEngine", "searchEngine": id})
}

// TabLayoutChanged, ThemeChanged, AutoUpdateChanged, UpdateChanged and WindowColorChanged
// tell every settings page what the managers now say.
func (m *Manager) TabLayoutChanged(layout string) {
	m.sendAll(map[string]any{"t": "tabLayout", "tabLayout": layout})
}

func (m *Manager) AddressBarPositionChanged(at string) {
	m.sendAll(map[string]any{"t": "addressBarPosition", "addressBarPosition": at})
}

func (m *Manager) ThemeChanged(theme string) {
	m.sendAll(map[string]any{"t": "theme", "theme": theme})
}

func (m *Manager) AutoUpdateChanged(on bool) {
	m.sendAll(map[string]any{"t": "autoUpdate", "autoUpdate": on})
}

func (m *Manager) UpdateChanged(s update.State) {
	m.sendAll(map[string]any{"t": "update", "update": s})
}

func (m *Manager) WindowColorChanged(color string) {
	m.sendAll(map[string]any{"t": "windowColor", "windowColor": color, "tones": tones.Of(color)})
}

// KeyboardChanged sends the keys again after any change to them.
func (m *Manager) KeyboardChanged() {
	m.sendAll(map[string]any{"t": "keyboard", "keyboard": m.commands.View()})
}

// FlagsChanged sends the flags again after one is saved.
func (m *Manager) FlagsChanged() {
	m.sendAll(map[string]any{"t": "flags", "flags": m.flags.View()})
}

// Send sends one message to every settings page, for a part.
func (m *Manager) Send(v any) { m.sendAll(v) }

func (m *Manager) sendAll(v any) {
	for tab := range m.pages {
		m.send(tab, v)
	}
}

// send sends to the settings page in tab, once it has said it's ready.
func (m *Manager) send(tab engine.TabID, v any) {
	if p := m.pages[tab]; p == nil || !p.heard {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		timeline.Errorf("settings: %v", err)
		return
	}
	m.eng.SendToPage(tab, b)
}
