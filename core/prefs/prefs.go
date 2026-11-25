// Package prefs owns the profile's settings: where the tabs go, the frame colour, the
// theme and the shell. They live in one small JSON file in the profile, read when the app starts and
// written when one changes. Automatic updates are the app's, not a profile's
// (core/update).
package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"projectb/search"
	"projectb/shells"
	"projectb/timeline"
	"projectb/tones"
	"regexp"
	"slices"
)

// Tab layouts.
const (
	TabsVertical   = "vertical"
	TabsHorizontal = "horizontal"
)

// Where a phone shell puts its address bar: at the foot, by the thumb, or on top.
const (
	AddressBottom = "bottom"
	AddressTop    = "top"
)

// What a start opens: the windows and tabs as they were, or a window opened fresh, as the
// profile's shell opens one.
const (
	StartContinue = "continue"
	StartFresh    = "fresh"
)

// What OneShell keeps at every size: the phone shell, or the wide one.
const (
	OneShellPhone = "phone"
	OneShellWide  = "wide"
)

// Themes: the shell and the window follow the system's light or dark, or stay one.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Prefs is every setting, as the file holds them.
type Prefs struct {
	TabLayout  string `json:"tabLayout"`
	FrameColor string `json:"frameColor"`
	Theme      string `json:"theme"`
	// Shell is the id of the shell every window loads; on Android, a wide window's.
	Shell string `json:"shell"`
	// PhoneShell is the id of the shell a narrow window loads on Android.
	PhoneShell string `json:"phoneShell"`
	// OneShell keeps one of the two at every size on Android: OneShellPhone or
	// OneShellWide; "" lets the window's size choose. docs/large-screens.md.
	OneShell string `json:"oneShell,omitempty"`
	// AddressBarPosition is AddressBottom or AddressTop. Only the Mobile shell reads it.
	AddressBarPosition string `json:"addressBarPosition"`
	// SearchEngine is the id of the engine typed words are searched with (package search).
	SearchEngine string `json:"searchEngine"`
	// Start is what a start opens: StartContinue or StartFresh.
	Start string `json:"start"`
	// Where bookmarks show, each on or off: on the new tab page, beside the tabs (the
	// sidebar, or the Mobile shell's switcher) and in the command box (the Mobile shell's
	// address, while it is empty). docs/bookmark-folders.md.
	BookmarksNewTab     bool `json:"bookmarksNewTab"`
	BookmarksSidebar    bool `json:"bookmarksSidebar"`
	BookmarksCommandBox bool `json:"bookmarksCommandBox"`
	// BookmarksBar puts a bar of bookmarks over every page, BookmarksBarFolder's: a
	// folder's id, "" for the top. A shell shows the top when the folder is gone.
	BookmarksBar       bool   `json:"bookmarksBar"`
	BookmarksBarFolder string `json:"bookmarksBarFolder"`
	// BlockAds is the ad blocker on every site: off, it keeps no page's requests from
	// anything, whatever a site's own switch says, and the site panel has no switch.
	// docs/site-features.md.
	BlockAds bool `json:"blockAds"`
	// AskWhereToSave sends every download through the Save panel, as Save Link As does,
	// rather than straight to the downloads folder. docs/daily-driver.md.
	AskWhereToSave bool `json:"askWhereToSave"`
	// ShowMenuBar keeps the shell's menu bar over the window, off the Mac (Linux, Windows);
	// off, Alt alone shows it for a moment.
	ShowMenuBar bool `json:"showMenuBar"`
	// ClearCookiesOnQuit erases every workspace's cookies and site storage as the app
	// quits, but the sites' in KeepCookies, registrable domains, sorted. A desktop's: a
	// phone's app has no quit of its own. docs/settings.md.
	ClearCookiesOnQuit bool     `json:"clearCookiesOnQuit"`
	KeepCookies        []string `json:"keepCookies"`
	// DownloadFolder is the folder downloads are saved in, a whole path, "" for the
	// Downloads folder. A desktop's. docs/settings.md.
	DownloadFolder string `json:"downloadFolder"`
	// UpdateNoticed is the release whose one-time notice the person has seen, so the shell
	// shows a ready release's notice once in a profile, not at every start. docs/updates.md.
	UpdateNoticed string `json:"updateNoticed,omitempty"`
}

// Defaults are the settings before the user changes any.
func Defaults() Prefs {
	return Prefs{TabLayout: TabsVertical, FrameColor: tones.Brand, Theme: ThemeSystem, Shell: shells.Default(), PhoneShell: shells.DefaultPhone(), AddressBarPosition: AddressBottom, SearchEngine: search.Default, Start: StartContinue,
		BookmarksNewTab: true, BookmarksCommandBox: true, BlockAds: true, KeepCookies: []string{}}
}

func isTheme(t string) bool { return t == ThemeSystem || t == ThemeLight || t == ThemeDark }

// A frame colour is "#rrggbb", as a colour input gives it.
var frameColor = regexp.MustCompile(`^(#[0-9a-f]{6})?$`)

// A version, as a release manifest names one: what UpdateNoticed may hold.
var version = regexp.MustCompile(`^[A-Za-z0-9._+-]{1,64}$`)

// A bookmark folder's id is 16 hex digits (core/bookmarks), "" being the top.
var folderID = regexp.MustCompile(`^([0-9a-f]{16})?$`)

// Manager owns the settings.
type Manager struct {
	path         string
	prefs        Prefs
	onLayout     []func(string)
	onFrameColor []func(string)
	onTheme      []func(string)
	onShell      []func()
	onAddress    []func(string)
	onSearch     []func(string)
	onChanged    []func(Prefs)
	// writes hands the settings, as JSON, to the writer, which writes them off the main
	// thread as history's does. At most one copy waits: a newer one takes its place. Nil
	// until the first change, so a manager made only to read starts nothing.
	writes chan []byte
	done   chan struct{}
}

// New returns a manager with the settings saved under dir, or the defaults. A setting the
// file doesn't hold, or holds a value for that isn't one, keeps its default. The file is a
// few bytes, so it is read right here.
func New(dir string) *Manager {
	m := &Manager{path: filepath.Join(dir, "prefs.json"), prefs: Defaults()}
	if b, err := os.ReadFile(m.path); err == nil {
		s := Defaults()
		if json.Unmarshal(b, &s) == nil {
			if s.TabLayout != TabsVertical && s.TabLayout != TabsHorizontal {
				s.TabLayout = TabsVertical
			}
			if !frameColor.MatchString(s.FrameColor) {
				s.FrameColor = Defaults().FrameColor
			}
			if !isTheme(s.Theme) {
				s.Theme = ThemeSystem
			}
			if s.AddressBarPosition != AddressBottom && s.AddressBarPosition != AddressTop {
				s.AddressBarPosition = AddressBottom
			}
			if s.Start != StartContinue && s.Start != StartFresh {
				s.Start = StartContinue
			}
			if _, ok := search.Get(s.SearchEngine); !ok {
				s.SearchEngine = search.Default
			}
			if !shells.Fits(s.Shell) {
				s.Shell = Defaults().Shell
			}
			if !shells.FitsPhone(s.PhoneShell) {
				s.PhoneShell = Defaults().PhoneShell
			}
			if s.OneShell != OneShellPhone && s.OneShell != OneShellWide {
				s.OneShell = ""
			}
			if !folderID.MatchString(s.BookmarksBarFolder) {
				s.BookmarksBarFolder = ""
			}
			s.KeepCookies = keptSites(s.KeepCookies)
			if !isFolder(s.DownloadFolder) {
				s.DownloadFolder = ""
			}
			m.prefs = s
		}
	}
	return m
}

// All returns a copy of every setting.
func (m *Manager) All() Prefs { return m.prefs }

// TabLayout says where the tabs go: TabsVertical or TabsHorizontal.
func (m *Manager) TabLayout() string { return m.prefs.TabLayout }

// OnTabLayoutChanged subscribes to the tab layout changing.
func (m *Manager) OnTabLayoutChanged(f func(string)) { m.onLayout = append(m.onLayout, f) }

// SetTabLayout changes where the tabs go. Anything but the two layouts is ignored.
func (m *Manager) SetTabLayout(layout string) {
	if layout != TabsVertical && layout != TabsHorizontal || layout == m.prefs.TabLayout {
		return
	}
	m.prefs.TabLayout = layout
	m.save()
	for _, f := range m.onLayout {
		f(layout)
	}
	m.changed()
}

// FrameColor is the colour the window's frame is painted, "#rrggbb", or tones.None for
// none picked, which paints it grey. The shell's other tones derive from it.
func (m *Manager) FrameColor() string { return m.prefs.FrameColor }

// OnFrameColorChanged subscribes to the frame colour changing.
func (m *Manager) OnFrameColorChanged(f func(string)) {
	m.onFrameColor = append(m.onFrameColor, f)
}

// SetFrameColor changes the frame colour. Anything but "#rrggbb" or tones.None is ignored.
func (m *Manager) SetFrameColor(color string) {
	if !frameColor.MatchString(color) || color == m.prefs.FrameColor {
		return
	}
	m.prefs.FrameColor = color
	m.save()
	for _, f := range m.onFrameColor {
		f(color)
	}
	m.changed()
}

// Theme is ThemeSystem, ThemeLight or ThemeDark.
func (m *Manager) Theme() string { return m.prefs.Theme }

// OnThemeChanged subscribes to the theme changing.
func (m *Manager) OnThemeChanged(f func(string)) { m.onTheme = append(m.onTheme, f) }

// SetTheme changes the theme. Anything but the three themes is ignored.
func (m *Manager) SetTheme(theme string) {
	if !isTheme(theme) || theme == m.prefs.Theme {
		return
	}
	m.prefs.Theme = theme
	m.save()
	for _, f := range m.onTheme {
		f(theme)
	}
	m.changed()
}

// Shell is the id of the shell every window loads; on Android, a wide window's.
func (m *Manager) Shell() string { return m.prefs.Shell }

// PhoneShell is the id of the shell a narrow window loads on Android.
func (m *Manager) PhoneShell() string { return m.prefs.PhoneShell }

// OneShell is OneShellPhone or OneShellWide when one shell stays at every size, else "".
func (m *Manager) OneShell() string { return m.prefs.OneShell }

// OnShellChanged subscribes to any of the three changing.
func (m *Manager) OnShellChanged(f func()) { m.onShell = append(m.onShell, f) }

// SetShell changes the shell. Anything but the id of a shell this device loads is ignored.
func (m *Manager) SetShell(id string) {
	if shells.Fits(id) {
		m.setShell(&m.prefs.Shell, id)
	}
}

// SetPhoneShell changes a narrow window's shell. Anything but a phone shell's id is
// ignored.
func (m *Manager) SetPhoneShell(id string) {
	if shells.FitsPhone(id) {
		m.setShell(&m.prefs.PhoneShell, id)
	}
}

// SetOneShell keeps the phone shell or the wide one at every size, or with "" lets the
// size choose again. Anything else is ignored.
func (m *Manager) SetOneShell(one string) {
	if one == "" || one == OneShellPhone || one == OneShellWide {
		m.setShell(&m.prefs.OneShell, one)
	}
}

func (m *Manager) setShell(pref *string, v string) {
	if *pref == v {
		return
	}
	*pref = v
	m.save()
	for _, f := range m.onShell {
		f()
	}
	m.changed()
}

// SearchEngine is the engine typed words are searched with.
func (m *Manager) SearchEngine() search.Engine {
	e, _ := search.Get(m.prefs.SearchEngine)
	return e
}

// OnSearchEngineChanged subscribes to another engine being chosen, by its id.
func (m *Manager) OnSearchEngineChanged(f func(string)) { m.onSearch = append(m.onSearch, f) }

// SetSearchEngine chooses the engine by its id. An id that isn't one is ignored.
func (m *Manager) SetSearchEngine(id string) {
	if _, ok := search.Get(id); !ok || id == m.prefs.SearchEngine {
		return
	}
	m.prefs.SearchEngine = id
	m.save()
	for _, f := range m.onSearch {
		f(id)
	}
	m.changed()
}

// AddressBarPosition is where a phone shell puts its address bar: AddressBottom or
// AddressTop.
func (m *Manager) AddressBarPosition() string { return m.prefs.AddressBarPosition }

// OnAddressBarPositionChanged subscribes to the address bar moving.
func (m *Manager) OnAddressBarPositionChanged(f func(string)) {
	m.onAddress = append(m.onAddress, f)
}

// SetAddressBarPosition moves the address bar. Anything but the two positions is ignored.
func (m *Manager) SetAddressBarPosition(at string) {
	if at != AddressBottom && at != AddressTop || at == m.prefs.AddressBarPosition {
		return
	}
	m.prefs.AddressBarPosition = at
	m.save()
	for _, f := range m.onAddress {
		f(at)
	}
	m.changed()
}

// Start is what a start opens: StartContinue or StartFresh.
func (m *Manager) Start() string { return m.prefs.Start }

// SetStart sets what a start opens. Anything but the two is ignored.
func (m *Manager) SetStart(start string) {
	if start != StartContinue && start != StartFresh || start == m.prefs.Start {
		return
	}
	m.prefs.Start = start
	m.save()
	m.changed()
}

// SetBookmarks turns where bookmarks show on or off: at is "newTab", "sidebar",
// "commandBox" or "bar". Anything else is ignored.
func (m *Manager) SetBookmarks(at string, on bool) {
	var p *bool
	switch at {
	case "newTab":
		p = &m.prefs.BookmarksNewTab
	case "sidebar":
		p = &m.prefs.BookmarksSidebar
	case "commandBox":
		p = &m.prefs.BookmarksCommandBox
	case "bar":
		p = &m.prefs.BookmarksBar
	}
	if p == nil || *p == on {
		return
	}
	*p = on
	m.save()
	m.changed()
}

// SetBookmarksBarFolder sets the folder the bookmarks bar shows: its id, "" for the top.
// Anything that isn't a folder's id is ignored.
func (m *Manager) SetBookmarksBarFolder(id string) {
	if !folderID.MatchString(id) || id == m.prefs.BookmarksBarFolder {
		return
	}
	m.prefs.BookmarksBarFolder = id
	m.save()
	m.changed()
}

// SetBlockAds turns the ad blocker on or off on every site.
func (m *Manager) SetBlockAds(on bool) { m.setSwitch(&m.prefs.BlockAds, on) }

// SetShowMenuBar turns the menu bar always shown on or off.
func (m *Manager) SetShowMenuBar(on bool) { m.setSwitch(&m.prefs.ShowMenuBar, on) }

// SetAskWhereToSave turns on or off asking where each download goes.
func (m *Manager) SetAskWhereToSave(on bool) { m.setSwitch(&m.prefs.AskWhereToSave, on) }

// SetDownloadFolder saves downloads in a folder, a whole path, from now on, or in the
// Downloads folder for "".
func (m *Manager) SetDownloadFolder(dir string) {
	if !isFolder(dir) || dir == m.prefs.DownloadFolder {
		return
	}
	m.prefs.DownloadFolder = dir
	m.save()
	m.changed()
}

// isFolder reports whether a download folder is one: "" or a whole path.
func isFolder(dir string) bool { return dir == "" || filepath.IsAbs(dir) }

// SetUpdateNoticed records that the person has seen the notice of a ready release, by its
// version. Anything that isn't a version is ignored.
func (m *Manager) SetUpdateNoticed(v string) {
	if !version.MatchString(v) || v == m.prefs.UpdateNoticed {
		return
	}
	m.prefs.UpdateNoticed = v
	m.save()
	m.changed()
}

// SetClearCookiesOnQuit turns on or off erasing cookies and site storage as the app quits.
func (m *Manager) SetClearCookiesOnQuit(on bool) { m.setSwitch(&m.prefs.ClearCookiesOnQuit, on) }

// KeepCookies keeps a site's cookies and storage as the app quits, or stops keeping them.
func (m *Manager) KeepCookies(site string, keep bool) {
	list := slices.DeleteFunc(slices.Clone(m.prefs.KeepCookies), func(s string) bool { return s == site })
	if keep && site != "" {
		list = append(list, site)
	}
	m.setKeepCookies(list)
}

func (m *Manager) setKeepCookies(list []string) {
	list = keptSites(list)
	if slices.Equal(list, m.prefs.KeepCookies) {
		return
	}
	m.prefs.KeepCookies = list
	m.save()
	m.changed()
}

// keptSites is a list of kept sites as the file keeps it: each once, sorted, none empty.
func keptSites(list []string) []string {
	out := slices.DeleteFunc(slices.Clone(list), func(s string) bool { return s == "" })
	slices.Sort(out)
	if out == nil {
		return []string{}
	}
	return slices.Compact(out)
}

// setSwitch sets a setting that is on or off, and says so when it changed.
func (m *Manager) setSwitch(p *bool, on bool) {
	if on == *p {
		return
	}
	*p = on
	m.save()
	m.changed()
}

// OnChanged subscribes to any setting changing, with them all as they are now.
func (m *Manager) OnChanged(f func(Prefs)) { m.onChanged = append(m.onChanged, f) }

// changed tells OnChanged's subscribers, after a setting's own have heard.
func (m *Manager) changed() {
	for _, f := range m.onChanged {
		f(m.prefs)
	}
}

// Set sets one setting by its name in the file, through its own setter, so a value that
// setter refuses changes nothing. It says whether name is a setting.
func (m *Manager) Set(name, value string) bool {
	switch name {
	case "tabLayout":
		m.SetTabLayout(value)
	case "frameColor":
		m.SetFrameColor(value)
	case "theme":
		m.SetTheme(value)
	case "shell":
		m.SetShell(value)
	case "phoneShell":
		m.SetPhoneShell(value)
	case "oneShell":
		m.SetOneShell(value)
	case "addressBarPosition":
		m.SetAddressBarPosition(value)
	case "bookmarksNewTab", "bookmarksSidebar", "bookmarksCommandBox", "bookmarksBar":
		if value != "true" && value != "false" {
			return true
		}
		m.SetBookmarks(bookmarkPlaces[name], value == "true")
	case "bookmarksBarFolder":
		m.SetBookmarksBarFolder(value)
	case "blockAds":
		if value == "true" || value == "false" {
			m.SetBlockAds(value == "true")
		}
	case "askWhereToSave":
		if value == "true" || value == "false" {
			m.SetAskWhereToSave(value == "true")
		}
	case "showMenuBar":
		if value == "true" || value == "false" {
			m.SetShowMenuBar(value == "true")
		}
	case "downloadFolder":
		m.SetDownloadFolder(value)
	case "clearCookiesOnQuit":
		if value == "true" || value == "false" {
			m.SetClearCookiesOnQuit(value == "true")
		}
	case "updateNoticed":
		m.SetUpdateNoticed(value)
	default:
		return false
	}
	return true
}

// bookmarkPlaces is SetBookmarks' name for each pref.
var bookmarkPlaces = map[string]string{"bookmarksNewTab": "newTab", "bookmarksSidebar": "sidebar", "bookmarksCommandBox": "commandBox", "bookmarksBar": "bar"}

// Apply sets each setting values holds, in the form the file has them, through its own
// setter, so every listener hears of it as of a change by hand. A setting values doesn't
// hold keeps its value. A profile reset to a template takes the template's values so.
func (m *Manager) Apply(values json.RawMessage) error {
	p := m.prefs
	if err := json.Unmarshal(values, &p); err != nil {
		return err
	}
	m.SetTabLayout(p.TabLayout)
	m.SetFrameColor(p.FrameColor)
	m.SetTheme(p.Theme)
	m.SetShell(p.Shell)
	m.SetPhoneShell(p.PhoneShell)
	m.SetOneShell(p.OneShell)
	m.SetAddressBarPosition(p.AddressBarPosition)
	m.SetSearchEngine(p.SearchEngine)
	m.SetStart(p.Start)
	m.SetBookmarks("newTab", p.BookmarksNewTab)
	m.SetBookmarks("sidebar", p.BookmarksSidebar)
	m.SetBookmarks("commandBox", p.BookmarksCommandBox)
	m.SetBookmarks("bar", p.BookmarksBar)
	m.SetBookmarksBarFolder(p.BookmarksBarFolder)
	m.SetBlockAds(p.BlockAds)
	m.SetAskWhereToSave(p.AskWhereToSave)
	m.SetClearCookiesOnQuit(p.ClearCookiesOnQuit)
	m.setKeepCookies(p.KeepCookies)
	m.SetDownloadFolder(p.DownloadFolder)
	return nil
}

// save hands the settings to the writer, starting it on the first change.
func (m *Manager) save() {
	b, _ := json.Marshal(m.prefs)
	if m.writes == nil {
		m.writes = make(chan []byte, 1)
		m.done = make(chan struct{})
		go m.writer(m.writes, m.done)
	}
	select {
	case <-m.writes:
	default:
	}
	m.writes <- b
}

// Close writes what is waiting and stops the writer. A change after it starts another.
func (m *Manager) Close() {
	if m.writes == nil {
		return
	}
	close(m.writes)
	<-m.done
	m.writes = nil
}

func (m *Manager) writer(writes <-chan []byte, done chan<- struct{}) {
	defer close(done)
	for b := range writes {
		m.write(b)
	}
}

func (m *Manager) write(b []byte) {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		timeline.Errorf("prefs: %v", err)
		return
	}
	// A new file renamed over the old, so no reader and no crash sees half of one.
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		timeline.Errorf("prefs: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		timeline.Errorf("prefs: %v", err)
	}
}
