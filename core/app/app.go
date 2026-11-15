// Package app builds the components and wires them together. All hook subscriptions
// live in Start, so this one function answers "who listens to what".
package app

import (
	"encoding/json"
	"path/filepath"

	"projectb/commands"
	"projectb/crash"
	"projectb/downloads"
	"projectb/engine"
	"projectb/flags"
	"projectb/history"
	"projectb/policy"
	"projectb/prefs"
	"projectb/profiles"
	"projectb/relaunch"
	"projectb/settings"
	"projectb/shell"
	"projectb/shelldata"
	"projectb/shells"
	"projectb/suggest"
	"projectb/tabs"
	"projectb/timeline"
	"projectb/update"
	"projectb/window"
	"projectb/workspaces"
)

// Config is what the app needs to start.
type Config struct {
	ProfileDir string
	// Profiles is the list of profiles, and Profile this copy's: its id is "" for a folder
	// named outright, which isn't listed.
	Profiles     *profiles.Manager
	Profile      string
	BlockedHosts []string
	Update       update.Config
	// CrashReports is the folder macOS writes its crash reports to.
	CrashReports string
	// DownloadsDir is where downloads go: the user's Downloads folder in the real app, and
	// a Downloads folder in the profile when it is "".
	DownloadsDir string
	// System is what this platform says of itself in a crash report: the version, the
	// OS and the device, and on Android the app's own log.
	System func() []crash.Section
}

// menuRead is what the menu reads of the front window's active tab: which tab, whether it
// loads (Stop), has a page (View Source), and can go back and forward.
type menuRead struct {
	tab                          engine.TabID
	loading, page, back, forward bool
}

func readOf(t tabs.Tab) menuRead {
	return menuRead{t.ID, t.Loading, t.URL != "", t.CanGoBack, t.CanGoForward}
}

// refreshMenu sends the menu again if it changed, and keeps what it read of the active
// tab, so a tab's change that leaves that alone needn't build it.
func (a *App) refreshMenu() {
	a.menuRead = readOf(a.Tabs.Active())
	a.Commands.RefreshMenu()
}

// App holds the managers and is the engine's listener: each function only forwards.
type App struct {
	// menuRead is what the menu last read of the front window's active tab.
	menuRead menuRead
	Crash    *crash.Manager
	Tabs     *tabs.Manager
	History  *history.Manager
	Suggest  *suggest.Manager
	Policy   *policy.Manager
	Window   *window.Manager
	Prefs    *prefs.Manager
	Flags    *flags.Manager
	Update   *update.Manager
	Shell    *shell.Manager
	Commands *commands.Manager
	Settings *settings.Manager
	// Workspaces is the profile's list of workspaces, beside its prefs.
	Workspaces *workspaces.Manager
	// ShellData is what each shell saves for the whole profile, beside its prefs.
	ShellData *shelldata.Manager
	// Downloads is the profile's downloads.
	Downloads *downloads.Manager

	actions []*commands.Action
	// menus are the windows' open right-click menus.
	menus *contextMenus
	// lastFolder is where files were last chosen from for a page.
	lastFolder string
	// dialogs are the pages' dialogs waiting for an answer.
	dialogs *pageDialogs
	// finds are the tabs' open searches.
	finds *pageFinds
	// fulls are the windows whose page fills the screen.
	fulls *pageFulls
	// keyIn is which page had the keyboard at the last key press.
	keyIn engine.Surface
	// lastWindow is the id of the window made last; Go numbers them from 1.
	lastWindow engine.WindowID
	// madeForTab are the windows made for a tab opened in a window of its own, which close
	// with their last tab.
	madeForTab map[engine.WindowID]bool
	quit       func()
	profiles   *profiles.Manager
}

// starters are what a build tag adds to Start. A tagged file appends one when it loads,
// so nothing here names it; each builds its managers and wires them to the rest.
var starters []func(a *App, e engine.Engine, cfg Config)

// Start builds everything, on the main thread. The caller then gives the returned App
// to the engine as its listener.
func Start(e engine.Engine, cfg Config) *App {
	e.OpenProfile(cfg.ProfileDir)
	a := &App{profiles: cfg.Profiles, madeForTab: map[engine.WindowID]bool{}}
	a.Crash = crash.New(cfg.ProfileDir, cfg.CrashReports)
	a.Crash.System = cfg.System
	a.Policy = policy.New(cfg.BlockedHosts)
	a.History = history.New(cfg.ProfileDir, e.Post)
	a.Suggest = suggest.New(a.History)
	a.Workspaces = workspaces.New(cfg.ProfileDir)
	a.ShellData = shelldata.New(cfg.ProfileDir)
	if cfg.DownloadsDir == "" {
		cfg.DownloadsDir = filepath.Join(cfg.ProfileDir, "Downloads")
	}
	a.Downloads = downloads.New(e, cfg.ProfileDir, cfg.DownloadsDir)
	a.Downloads.Load()
	a.Tabs = tabs.New(e, a.Workspaces.First)
	a.Prefs = prefs.New(cfg.ProfileDir)
	a.Flags = flags.New(cfg.ProfileDir)
	a.Window = window.New(e, shells.Page(a.Prefs.Shell()), a.Prefs.FrameColor(), themeOf(a.Prefs.Theme()), a.Tabs.Front)
	a.Tabs.OnFrontChanged(a.Window.HandleFrontChanged)
	a.Tabs.OnMoved(a.Window.HandleTabMoved)
	first := a.openWindow()
	a.quit = func() { e.AppCommand(engine.AppQuit) }
	a.Update = update.New(cfg.Update, e.Post, a.quit)
	a.Commands = commands.New(e, cfg.ProfileDir, func() bool { return a.Window.FullScreen(a.Tabs.Front()) })
	a.Commands.RegisterList(a.profileList(cfg))
	a.Commands.RegisterList(a.workspaceList())
	a.Commands.RegisterList(a.shellList())
	a.Shell = shell.New(e, a.Tabs, a.Suggest, a.Window, a.Prefs, a.Update, a.Crash, a.Commands.Run, a.Commands.ShownKeys, a.Commands.Platform())
	a.Shell.AddPart(a.workspaceShellPart(e))
	a.Shell.AddPart(a.shellDataPart())
	a.menus = newContextMenus(a, e)
	a.Shell.AddPart(a.downloadsShellPart())
	a.dialogs = &pageDialogs{a: a, e: e, list: []pageDialog{}}
	a.Shell.AddPart(a.dialogs.shellPart())
	a.finds = newPageFinds(a, e)
	a.Shell.AddPart(a.finds.shellPart())
	a.fulls = newPageFulls(a, e)
	a.Shell.AddPart(a.menus.shellPart())
	// Every action a shell can run by name, for a shell that lists them, as Glide's
	// Command Bar does.
	a.Shell.AddPart(shell.Part{
		Snapshot: func(into map[string]any) { into["actions"] = a.Commands.Titles() },
		Handle:   func(engine.WindowID, string, []byte) bool { return false },
	})
	a.Settings = settings.New(e, a.Window, a.Prefs, a.Flags, a.Update, a.Commands)
	a.Settings.OnRelaunch(func() { relaunch.Self(); a.quit() })
	a.Settings.AddPart(a.workspaceSettingsPart(e))
	a.registerActions(e)

	a.Tabs.AddGate(a.Policy.IsAllowedURL)
	a.Tabs.OnNavigationFinished(func(t tabs.Tab) { a.History.AddVisit(t.URL, t.Title) })
	a.Tabs.OnTitleChanged(func(t tabs.Tab) { a.History.UpdateTitle(t.URL, t.Title) })
	a.Tabs.OnChanged(a.Shell.TabChanged)
	a.Tabs.OnListChanged(a.Shell.TabsChanged)
	// A window may have no tabs: closing its last one leaves it open. It closes with its
	// last tab only when that tab moved to another window, as what it held went there, or
	// when the window was made for a tab of its own.
	a.Tabs.OnEmpty(func(w engine.WindowID, moved bool) {
		if moved || a.madeForTab[w] {
			a.Window.Request(w, engine.WindowClose)
		}
	})
	a.Window.OnControlsChanged(a.Shell.WindowControlsChanged)
	a.Window.OnControlsChanged(a.Settings.WindowControlsChanged)
	a.Window.OnColorChanged(a.Shell.WindowColorChanged)
	a.Window.OnColorChanged(a.Settings.WindowColorChanged)
	a.Prefs.OnFrameColorChanged(a.Window.SetColor)
	// The profile's colour is its frame colour: on its Dock icon, and in the list.
	e.SetDockColor(a.Prefs.FrameColor())
	a.Prefs.OnFrameColorChanged(e.SetDockColor)
	if cfg.Profile != "" {
		a.Prefs.OnFrameColorChanged(func(c string) {
			if err := cfg.Profiles.SetColor(cfg.Profile, c); err != nil {
				timeline.Errorf("profiles: %v", err)
			}
		})
		a.Settings.AddPart(a.profileSettingsPart(cfg.Profiles, cfg.Profile))
	}
	if shells.OnPhone {
		a.Settings.AddPart(a.phoneShellPart())
	}
	// A change to the workspaces reaches every shell at once, for the dots, and every
	// settings page; the Tab menu reads the list as it opens. The engine hears which is
	// first, at start and whenever that changes, for the tabs it makes on its own.
	firstWorkspace := a.Workspaces.First()
	e.SetFirstWorkspace(firstWorkspace)
	a.Workspaces.OnChanged(func() {
		m := map[string]any{"t": "workspaces", "workspaces": a.workspacesShown()}
		a.Shell.Send(m)
		a.Settings.Send(m)
		if now := a.Workspaces.First(); now != firstWorkspace {
			firstWorkspace = now
			e.SetFirstWorkspace(now)
		}
	})
	a.Prefs.OnThemeChanged(func(t string) { a.Window.SetTheme(themeOf(t)) })
	a.Prefs.OnThemeChanged(a.Shell.ThemeChanged)
	a.Prefs.OnChanged(a.Shell.PrefsChanged)
	a.Prefs.OnThemeChanged(a.Settings.ThemeChanged)
	// Typed words go to the profile's search engine, from the start and after a change.
	a.Tabs.SetSearch(a.Prefs.SearchEngine().For)
	a.Prefs.OnSearchEngineChanged(func(string) { a.Tabs.SetSearch(a.Prefs.SearchEngine().For) })
	a.Prefs.OnSearchEngineChanged(a.Shell.SearchEngineChanged)
	a.Prefs.OnSearchEngineChanged(a.Settings.SearchEngineChanged)
	a.Suggest.SetSources(a.suggestSources())
	a.Shell.AddPart(a.suggestShellPart())
	// Another shell loads in every window at once. The old one's regions go, as after a
	// crash, and the tabs stay where they were until the new one reports.
	a.Prefs.OnShellChanged(func(id string) {
		a.Shell.ShellChanged()
		a.Window.SetPage(shells.Page(id))
	})
	a.Prefs.OnTabLayoutChanged(a.Shell.TabLayoutChanged)
	a.Prefs.OnTabLayoutChanged(a.Settings.TabLayoutChanged)
	a.Prefs.OnAddressBarPositionChanged(a.Shell.AddressBarPositionChanged)
	a.Prefs.OnAddressBarPositionChanged(a.Settings.AddressBarPositionChanged)
	a.Update.OnAutoChanged(a.Settings.AutoUpdateChanged)
	a.Update.OnChanged(a.Shell.UpdateChanged)
	a.Update.OnChanged(a.Settings.UpdateChanged)
	a.Commands.OnChanged(a.Settings.KeyboardChanged)
	a.Flags.OnChanged(a.Settings.FlagsChanged)
	a.Commands.OnChanged(a.Shell.KeysChanged)
	// What the menu shows enabled or checked follows the window in front: its active tab
	// and full screen, the layout, and the shell. Of a tab's changes, only those to the
	// front window's active tab that Stop, View Source, Back and Forward read can change
	// it: building the menu was most of what Go did for each of a navigation's events.
	a.Tabs.OnChanged(func(t tabs.Tab) {
		if t.ID != a.Tabs.ActiveID() {
			return
		}
		if readOf(t) != a.menuRead {
			a.refreshMenu()
		}
	})
	a.Tabs.OnListChanged(a.refreshMenu)
	a.Prefs.OnTabLayoutChanged(func(string) { a.refreshMenu() })
	a.Prefs.OnShellChanged(func(string) { a.refreshMenu() })
	// Keyboard settings lists only the shell in use's own actions.
	a.Prefs.OnShellChanged(func(string) { a.Settings.KeyboardChanged() })
	a.Window.OnControlsChanged(func(engine.WindowID, engine.Rect, float64) { a.refreshMenu() })
	a.Commands.Load()
	for _, start := range starters {
		start(a, e, cfg)
	}

	a.History.Load(nil)
	if !a.startsEmpty() {
		a.Tabs.Create(first)
	}
	a.Update.Start()
	return a
}

// profileList is the Profiles menu's list: every profile, with a tick on this one.
// Choosing one starts a copy of the app on it, which hands over to the copy already
// running on it, if any.
func (a *App) profileList(cfg Config) commands.List {
	return commands.List{
		ID: "profiles",
		Items: func() []commands.ListItem {
			list, err := cfg.Profiles.List()
			if err != nil {
				timeline.Errorf("profiles: %v", err)
			}
			var items []commands.ListItem
			for _, p := range list {
				items = append(items, commands.ListItem{Key: p.ID, Title: p.Name, Checked: p.ID == cfg.Profile})
			}
			return items
		},
		Choose: func(id string) { relaunch.Copy("--root="+cfg.Profiles.Root(), "--profile="+id) },
	}
}

// phoneShellPart is a phone's choice of shell, in General: a phone has no Profile
// section or View menu, the desktop's two places for it. The shells it offers are a
// phone's own (shells.Listed).
func (a *App) phoneShellPart() settings.Part {
	view := func() map[string]any {
		listed := []map[string]string{}
		for _, s := range shells.Listed() {
			listed = append(listed, map[string]string{"id": s.ID, "name": s.Name})
		}
		return map[string]any{"shell": a.Prefs.Shell(), "shells": listed}
	}
	a.Prefs.OnShellChanged(func(string) { a.Settings.Send(map[string]any{"t": "phoneShells", "phoneShells": view()}) })
	return settings.Part{
		Snapshot: func(into map[string]any) { into["phoneShells"] = view() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			if t != "setShell" {
				return false
			}
			var c struct {
				Text string `json:"text"`
			}
			json.Unmarshal(msg, &c)
			a.Prefs.SetShell(c.Text)
			return true
		},
	}
}

// shellList is View, Shell: every shell the user can choose, with a tick on the one in use.
// Choosing one makes it the profile's, and every window loads it.
func (a *App) shellList() commands.List {
	return commands.List{
		ID: "shells",
		Items: func() []commands.ListItem {
			var items []commands.ListItem
			for _, s := range shells.Listed() {
				items = append(items, commands.ListItem{Key: s.ID, Title: s.Name, Checked: s.ID == a.Prefs.Shell()})
			}
			return items
		},
		Choose: a.Prefs.SetShell,
	}
}

// openWindow makes a window with no tabs, a little down and right of the window in
// front, and puts it in front.
func (a *App) openWindow() engine.WindowID {
	near := a.Tabs.Front()
	a.lastWindow++
	w := a.lastWindow
	a.Window.Open(w, near)
	a.Tabs.AddWindow(w)
	return w
}

// startsEmpty says whether a window opens with no tabs, at the app's start and for New
// Window: when the profile's shell takes New Tab, it shows its own way to a tab there.
func (a *App) startsEmpty() bool { return shells.Takes(a.Prefs.Shell(), "tab.new") }

// NewWindow opens a window with one tab, in the first workspace, and returns both: a new
// tab, or one loading u when it isn't "".
func (a *App) NewWindow(u string) (engine.WindowID, engine.TabID) {
	w := a.openWindow()
	if u == "" {
		return w, a.Tabs.Create(w)
	}
	// The new window is in front, where OpenIn opens.
	return w, a.Tabs.OpenIn(u, true)
}

// NewWindowWith opens a window holding a tab moved out of its window, with no tab of its
// own, and returns it. A tab that is its window's only one has a window already: it stays,
// and that window is the one returned.
func (a *App) NewWindowWith(tab engine.TabID) engine.WindowID {
	if w := a.Tabs.WindowOf(tab); len(a.Tabs.AllIn(w)) == 1 {
		return w
	}
	w := a.openWindow()
	a.Tabs.Move(tab, w, 0)
	return w
}

// Stop flushes what's pending and says the run ended cleanly. Call it on the way out.
func (a *App) Stop() {
	a.Update.Stop()
	a.History.Close()
	a.Prefs.Close()
	a.Crash.Stop()
}

func (a *App) ShellMessage(w engine.WindowID, msg []byte) { a.Shell.Handle(w, msg) }

func (a *App) NavigationStarting(tab engine.TabID, nav int64, url string) bool {
	return a.Tabs.HandleNavigationStarting(tab, nav, url)
}

func (a *App) NavigationFinished(tab engine.TabID, nav int64, url string) {
	a.Tabs.HandleNavigationFinished(tab, nav, url)
}

func (a *App) NavigationFailed(tab engine.TabID, nav int64, url string, why engine.Failure, code string) {
	a.Tabs.HandleNavigationFailed(tab, nav, url, why, code)
}

func (a *App) HistoryChanged(tab engine.TabID, canGoBack, canGoForward bool) {
	a.Tabs.HandleHistoryChanged(tab, canGoBack, canGoForward)
}

func (a *App) TitleChanged(tab engine.TabID, title string) { a.Tabs.HandleTitleChanged(tab, title) }

func (a *App) LoadingChanged(tab engine.TabID, loading bool) {
	a.Tabs.HandleLoadingChanged(tab, loading)
}

func (a *App) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	a.menus.show(tab, m)
}

func (a *App) ScrollDirectionChanged(tab engine.TabID, down bool) {
	a.Shell.ScrollDirectionChanged(tab, down)
}

func (a *App) LoadProgressChanged(tab engine.TabID, progress float64) {
	a.Tabs.HandleLoadProgressChanged(tab, progress)
}

func (a *App) PageCrashed(tab engine.TabID) { a.Tabs.HandlePageCrashed(tab) }

func (a *App) FaviconChanged(tab engine.TabID, dataURL string) {
	a.Tabs.HandleFaviconChanged(tab, dataURL)
}

func (a *App) TabOpened(opener, tab engine.TabID, url, workspace string, foreground, newWindow bool) {
	// A tab nothing opened goes in the window in front, which a new one is. A window made
	// for the tab closes with its last tab.
	if newWindow {
		a.madeForTab[a.openWindow()] = true
	}
	a.Tabs.HandleTabOpened(opener, tab, url, workspace, foreground)
}

func (a *App) TabFocused(tab engine.TabID) { a.Tabs.HandleTabFocused(tab) }

func (a *App) CloseRequested(tab engine.TabID) { a.Tabs.Close(tab) }

// ActivateRequested shows a tab in its window, and brings the window to the front.
func (a *App) ActivateRequested(tab engine.TabID) {
	a.Tabs.Activate(tab)
	a.Window.Request(a.Tabs.WindowOf(tab), engine.WindowFront)
}

// Handover is the request of a copy started on this profile while this one runs: the
// window used last comes to the front, and each URL opens in a new tab there.
func (a *App) Handover(urls []string) {
	timeline.Infof("a copy started on this profile handed over %d URLs", len(urls))
	a.Window.Request(a.Tabs.Front(), engine.WindowFront)
	for _, u := range urls {
		a.Tabs.Open(u)
	}
}

// themeOf is the window's theme for a theme pref.
func themeOf(t string) engine.Theme {
	switch t {
	case prefs.ThemeLight:
		return engine.ThemeLight
	case prefs.ThemeDark:
		return engine.ThemeDark
	}
	return engine.ThemeSystem
}

// Key hands a key press to the commands manager, which decides what it runs.
func (a *App) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	a.keyIn = in
	// Escape leaves a page's full screen before the page sees it.
	if phase == engine.KeyBeforePage && a.fulls.key(k) {
		return true
	}
	// The key press the window put on the timeline has reached us.
	timeline.End("input.key")
	return a.Commands.Key(k, phase)
}

func (a *App) ModsChanged(m engine.Mods) { a.Commands.ModsChanged(m) }

func (a *App) MenuChosen(action string) { a.Commands.Run(action) }

func (a *App) MenuList(name string) engine.Menu { return a.Commands.ListItems(name) }

func (a *App) OSBindings(byTitle map[string]engine.Key) { a.Commands.OSBindings(byTitle) }

func (a *App) SettingsMessage(w engine.WindowID, msg []byte) { a.Settings.Handle(w, msg) }

// PickerMessage is the picker's, which a copy with a profile never shows.
func (a *App) PickerMessage(engine.WindowID, []byte) {}

func (a *App) WindowChanged(w engine.WindowID, s engine.WindowState) {
	a.Window.HandleWindowChanged(w, s)
	a.fulls.windowChanged(w, s)
}

func (a *App) ShellCrashed(w engine.WindowID) {
	a.Window.HandleShellCrashed(w)
	a.Shell.HandleShellCrashed(w)
}

// WindowClosed forgets the window and closes its tabs. Closing the last window quits.
func (a *App) WindowClosed(w engine.WindowID) {
	a.Settings.WindowClosed(w)
	a.Shell.WindowClosed(w)
	a.Window.Close(w)
	a.Tabs.RemoveWindow(w)
	delete(a.madeForTab, w)
	if len(a.Tabs.Windows()) == 0 {
		a.quit()
		return
	}
	a.refreshMenu()
}

// WindowFocused puts the window in front: commands and the menu follow it.
func (a *App) WindowFocused(w engine.WindowID) {
	a.Tabs.Focus(w)
	a.refreshMenu()
}

func (a *App) ProcessCrashed(p engine.ProcessCrash) { a.Crash.ProcessCrashed(p) }

// DownloadTarget refuses a download from an address policy blocks; the list says where
// the rest go.
func (a *App) DownloadTarget(r engine.DownloadRequest) engine.DownloadTarget {
	if !a.Policy.IsAllowedURL(r.URL) {
		return engine.DownloadTarget{}
	}
	return a.Downloads.Target(r)
}

func (a *App) DownloadUpdated(d engine.Download) { a.Downloads.Updated(d) }
