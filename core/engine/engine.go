// Package engine is the contract between our app and a browser engine.
//
// App code only ever sees this package. The real implementation wraps Chromium's
// content layer (see ../../engine); the mock is pure Go (see ../mockengine).
//
// Threading: every Engine call and every Listener call happens on the main thread.
// A worker goroutine that finishes something uses Post to get back onto it.
//
// Start-up. An engine owns the process and its run loop, and calls the app's core in this
// order (core/cmd/lib, PBGoCalls in engine/abi/pb_abi.h):
//
//  1. claim_profile, with the command line's --root, --profile, --user-data-dir and
//     --picker: the core chooses the profile and answers with the folder this copy keeps
//     its files in, or none, when another copy has it and has been handed the addresses,
//     and this copy then quits.
//  2. launch_switches, with that folder: the switches the profile's flags ask for, which
//     the engine applies as it starts.
//  3. timeline_start, then start, handing over the engine's calls: the core opens the
//     profile (Engine.OpenProfile) and its first windows, or the picker's window.
//  4. run_posted, on the main thread, each time Engine.Post asks for a wakeup.
//  5. stop, as the app ends.
package engine

import (
	"errors"
	"strings"
	"time"
)

// TabID identifies a tab across the boundary. Content's own objects never cross it.
type TabID int64

// WindowID identifies a window across the boundary. The app makes them, 1 for the first.
type WindowID int64

// Rect is in window coordinates, CSS pixels.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Contains reports whether the point is inside the rectangle.
func (r Rect) Contains(x, y float64) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Mods are the modifier keys held with a key. The values cross to the engine as they are.
type Mods int

const (
	ModCtrl Mods = 1 << iota
	ModAlt       // Option on a Mac.
	ModShift
	ModCmd // The Command key on a Mac, the Windows key elsewhere.
)

// Key is one key press: its Windows virtual-key code, which every engine reports on
// every platform, and the modifiers held with it.
type Key struct {
	Code int  `json:"code"`
	Mods Mods `json:"mods"`
}

// KeyPhase is when the engine asks about a key.
type KeyPhase int

const (
	// KeyBeforePage: the key is about to go to a page. Saying yes keeps it from the page.
	KeyBeforePage KeyPhase = iota + 1
	// KeyUnhandled: the page had the key and didn't use it.
	KeyUnhandled
)

// Surface is which kind of page had the keyboard when a key was pressed: a tab's, ours
// among them, or the shell.
type Surface int

const (
	SurfacePage Surface = iota + 1
	SurfaceShell
)

// Page is which of our pages covers a window: a shell, or the picker in a copy that
// hasn't chosen its profile. Each is an app of its own, at an address of the engine's
// (Engine.PageURL): a shell's page is "shell/<id>". A native shell, drawn by the platform's own
// views and not a page, is "native/<id>": the engine makes that shell's view where it
// would load a page (docs/native-shell.md).
type Page string

// PagePicker is the picker's page.
const PagePicker Page = "picker"

// ShellPage is the page of the shell called id.
func ShellPage(id string) Page { return Page("shell/" + id) }

// NativeShellPage is the native shell called id.
func NativeShellPage(id string) Page { return Page("native/" + id) }

// Shell is the id of the shell the page is, web or native, or "" for a page that isn't a
// shell.
func (p Page) Shell() string {
	for _, prefix := range []string{"shell/", "native/"} {
		if id, ok := strings.CutPrefix(string(p), prefix); ok {
			return id
		}
	}
	return ""
}

// MenuItem is one entry of the native menu bar. A submenu has Items; a separator has
// none of the rest.
type MenuItem struct {
	Title     string `json:"title,omitempty"`
	Action    string `json:"action,omitempty"` // What choosing it runs, by id.
	Key       *Key   `json:"key,omitempty"`    // The key it shows; nil for none.
	Enabled   bool   `json:"enabled,omitempty"`
	Checked   bool   `json:"checked,omitempty"`
	Separator bool   `json:"separator,omitempty"`
	Role      string `json:"role,omitempty"` // For a submenu the OS fills: "services".
	// List stands in for the items of a list that changes, which the engine asks for,
	// by this name, each time its menu opens (Listener.MenuList).
	List  string     `json:"list,omitempty"`
	Items []MenuItem `json:"items,omitempty"`
}

// Menu is the whole menu bar. Role says which menu the OS knows as which: "app",
// "window" and "help".
type Menu []MenuItem

// EditCommand is a text-editing command for whichever page has the keyboard.
type EditCommand int

const (
	EditUndo EditCommand = iota + 1
	EditRedo
	EditCut
	EditCopy
	EditPaste
	EditPasteAndMatchStyle
	EditDelete
	EditSelectAll
)

// AppCommand is something only the OS can do for the app.
type AppCommand int

const (
	AppAbout AppCommand = iota + 1
	AppHide
	AppHideOthers
	AppShowAll
	AppBringAllToFront
	AppQuit // Ends the app, every window at once.
)

// EraseKind is what EraseWorkspace erases, as bits.
type EraseKind int

const (
	// EraseSiteData is the cookies, every kind of site storage and service workers.
	EraseSiteData EraseKind = 1 << iota
	// EraseCache is the HTTP cache, and the pages the engine kept ready ahead.
	EraseCache
	// EraseAll is everything a workspace keeps.
	EraseAll = EraseSiteData | EraseCache
)

// Erasure is what EraseWorkspace erases: the kinds of storage, stored since a time, the
// zero time for from the start, of the sites named, registrable domains, or of every site
// for none but those kept.
type Erasure struct {
	Since time.Time
	Kinds EraseKind
	Sites []string
	Keep  []string
}

// SiteData is what a workspace's storage holds for one host: the cookies set for it as
// their domain, and the bytes it keeps in every kind of site storage.
type SiteData struct {
	Host    string `json:"host"`
	Cookies int    `json:"cookies"`
	Bytes   int64  `json:"bytes"`
}

// Engine is what the app can ask a browser engine to do.
type Engine interface {
	// OpenProfile makes the profile's own storage, its cookies, site data and cache, in its
	// folder dir, where the engine keeps its files in a folder of its own. The app calls it
	// once, as this copy becomes the profile's, before it asks for any tab; a copy that
	// starts on the picker calls it in the copy it starts for the chosen profile.
	OpenProfile(dir string)
	// SetBlockRules says which hosts no request of a page may reach: the page itself, its
	// frames, images, scripts, fetches, workers and WebSockets. A tab's own navigation also
	// asks NavigationStarting, which decides what the tab shows. The app sets them once,
	// at start, before any tab.
	SetBlockRules(r BlockRules)
	// SetAdRules says which hosts the ad blocker keeps a page's requests from, and the
	// sites, by origin, where it is off. The app sets them at start, before any tab, and
	// again whenever they change; a page's requests follow the rules of when they start.
	SetAdRules(r AdRules)
	// CreateTab makes a hidden tab in a workspace, by the app's id for it: "" is the
	// profile's own storage, which a profile's first workspace, Main, has. A window's layout
	// shows it. A workspace keeps its own cookies and site storage, and a tab stays in its
	// workspace for its life, as does every tab it opens. An empty tab, one that loads
	// nothing yet, starts no renderer until it loads; one about to load takes the engine's
	// spare renderer, so its first page is quicker.
	CreateTab(workspace string, empty bool) TabID
	// CreateInternalTab makes a hidden tab of ours: one for our own pages, in their own
	// context, where the shells load, in no workspace, and that context's for its life. A
	// window's layout shows it, and LoadURL takes it to its page, chrome://<page>/.
	CreateInternalTab() TabID
	// SetFirstWorkspace says which workspace a tab the engine makes on its own goes in, one
	// a client of the debugging port or an extension asks for: the first in the profile's
	// list. The app says so at start, before any such tab, and whenever the first changes.
	SetFirstWorkspace(workspace string)
	// EraseWorkspace erases what the erasure says of a workspace's storage. Deleting a
	// workspace asks for all of it once the app has closed its tabs; Settings' Clear
	// browsing data asks with them open, and their pages stay. For "", the profile's own
	// storage, extensions keep their own.
	EraseWorkspace(workspace string, r Erasure)
	// ListSiteData counts what a workspace's storage holds, by host; SiteDataListed
	// follows with it.
	ListSiteData(workspace string)
	// SetQuitErasure says what to erase as the app quits, in each workspace named: a quit
	// runs it and waits for it, at most a few seconds, before the app ends. An erasure of
	// no kinds erases nothing. A desktop engine's: a phone's app has no quit of its own.
	SetQuitErasure(workspaces []string, r Erasure)
	CloseTab(tab TabID)
	LoadURL(tab TabID, url string)

	// GoBack, GoForward and Reload move within the tab's own session history. What
	// happens next arrives as ordinary navigation events.
	GoBack(tab TabID)
	GoForward(tab TabID)
	Reload(tab TabID)
	// ReloadBypassingCache reloads without the cache; Stop stops loading.
	ReloadBypassingCache(tab TabID)
	Stop(tab TabID)
	// Zoom sets the tab's page zoom as a factor, 1 for actual size; the engine keeps it
	// for the page's host, as browsers do. ZoomOf says what it is now.
	Zoom(tab TabID, factor float64)
	ZoomOf(tab TabID) float64
	// DuplicateTab makes a hidden copy of the tab, its history included, of its kind: in its
	// workspace, or in our pages' context for one of ours. The copy loads only once this has
	// returned, so its first navigation finds it known.
	DuplicateTab(tab TabID) TabID
	// Edit runs a text-editing command in whichever page has the keyboard.
	Edit(c EditCommand)
	// WriteClipboard puts plain text on the system clipboard.
	WriteClipboard(text string)
	// DownloadURL downloads url as tab's page would, through the Save panel when saveAs;
	// tab 0 downloads it as the browser's own.
	DownloadURL(tab TabID, url string, saveAs bool)
	// DownloadCommand acts on a running download.
	DownloadCommand(id DownloadID, c DownloadCommand)
	// OpenFile opens a file as the OS would; RevealFile shows it in the file manager.
	OpenFile(path string)
	RevealFile(path string)
	// ShowDownload shows how a download stands where the system shows such things: on a
	// phone, its notification, with its progress while it runs and, once done, a tap that
	// opens the file. id is Go's, the same for the download's whole life. An engine with
	// nowhere to show it does nothing.
	ShowDownload(id int, d Download)
	// AnswerDialog answers a page's dialog: OK or not, and a prompt's text.
	AnswerDialog(id DialogID, ok bool, text string)
	// AnswerSignIn answers a site's or a proxy's sign-in: the username and password, or
	// no to cancel it.
	AnswerSignIn(id DialogID, ok bool, username, password string)
	// AnswerPasskey says what the person did at a passkey request's step
	// (docs/passkeys.md): the action, cancel, pin, account, phone, key, bluetoothOn,
	// bluetoothSettings or retry, and its text, a PIN or an account's index. An engine
	// with no passkey requests does nothing.
	AnswerPasskey(id DialogID, action, text string)
	// Find searches a tab's page for text: a new search, or the next match forward or
	// back. FindReply answers with request. StopFinding ends it, keeping the match
	// selected.
	Find(tab TabID, request int, text string, forward, next bool)
	StopFinding(tab TabID)
	// Print prints a tab's page through the system's print panel, over its window on a
	// Mac and Android's print sheet on a phone, which have their own preview.
	Print(tab TabID)
	// CaptureThumbnail copies what a tab's page shows now, scaled down to at most maxWidth
	// device pixels across, never up, and keeps it in the engine as the tab's picture
	// until the next or the tab closes. Thumbnail follows once it is kept; a copy that
	// fails says nothing. ThumbnailURL is the address our pages load that picture at,
	// changing with each revision; it only names it.
	CaptureThumbnail(tab TabID, maxWidth int)
	// AskDefaultBrowser asks the system to make the app the default browser, which asks
	// the person itself: macOS's question, Android's sheet. DefaultBrowserChanged follows
	// with how it ended. IsDefaultBrowser says whether the app is it now.
	AskDefaultBrowser()
	IsDefaultBrowser() bool
	// AskNotifications asks a phone's system to let the app show its notifications, as a
	// download's, which asks the person; NotificationsAnswered follows. NotificationsAllowed
	// says whether it may now. A desktop's are the app's own to show: allowed, and asking
	// does nothing.
	AskNotifications()
	NotificationsAllowed() bool
	// ExitReason is why the app's process with this id, one from an earlier run, ended, as
	// a phone's system keeps it: "crash", "native crash" or "not responding" for a crash,
	// or one of Android's own reasons, such as "low memory" or "user requested"; "" when
	// the system doesn't say, as a desktop's doesn't.
	ExitReason(pid int) string
	// ChooseFolder asks the person for a folder, in the system's panel on the window, as
	// Import asks for another browser's; FolderChosen follows with it, or "" for none. A
	// phone has no such panel and answers nothing.
	ChooseFolder(w WindowID)
	// OpenFullDiskAccess opens the Mac's System Settings at Privacy & Security's Full Disk
	// Access, where the person can let the app read what macOS keeps behind it, as Safari's
	// files. Nothing says when they have: reading again does. A phone has no such thing.
	OpenFullDiskAccess()
	ThumbnailURL(tab TabID, revision int64) string
	// ExitPageFullScreen takes a tab's page out of full screen, as its own
	// exitFullscreen() would; PageFullScreen follows.
	ExitPageFullScreen(tab TabID)
	// AnswerPermission answers a page's PermissionRequested, one status for each of its
	// kinds, in their order.
	AnswerPermission(id PermissionID, statuses []PermissionStatus)

	// CreateWindow makes window w, with its page, and puts it on screen with its first
	// layout, a little down and right of window near, or where a first window goes when
	// near is 0. It tells WindowChanged once this call has returned.
	CreateWindow(w, near WindowID, page Page, l WindowLayout)
	// SetWindowPage loads another page in window w in place of the one covering it, which
	// is gone with all it had. The new page starts as a new window's does. The tabs stay
	// where the window's layout last put them.
	SetWindowPage(w WindowID, page Page)

	// RequestWindow asks a native window to close, minimise, zoom, go in or out of full
	// screen, or come to the front. What follows arrives as WindowClosed or
	// WindowChanged.
	RequestWindow(w WindowID, r WindowRequest)
	// SetWindowBounds puts a window's frame on screen at r, y down from the top of the
	// main screen. What follows arrives as WindowChanged.
	SetWindowBounds(w WindowID, r Rect)
	// SetWindowMode puts a window into a mode from whatever it is in now, where the
	// requests above toggle. What follows arrives as WindowChanged.
	SetWindowMode(w WindowID, m WindowMode)

	// SetWindowLayout is everything about how a window behaves, from the window
	// manager, whenever any of it changes: which tabs it shows, and where.
	SetWindowLayout(w WindowID, l WindowLayout)
	// FocusTab gives keyboard focus to a visible tab's page; FocusShell gives it to a
	// window's shell page, which then places it inside itself.
	FocusTab(tab TabID)
	FocusShell(w WindowID)

	// SendToShell delivers one message to a window's shell page.
	SendToShell(w WindowID, msg []byte)
	// SetShellBack says whether the phone's back key goes to the shell first, while it
	// has something open that the key closes: the engine then sends {"t":"backKey"} on
	// the shell's channel, as if the shell had, instead of going back in the page.
	SetShellBack(w WindowID, on bool)
	// ShareFile offers the file at path to the system's share sheet, anchored on a
	// rectangle in the window's shell coordinates.
	ShareFile(w WindowID, path string, anchor Rect)
	// ShareText offers text, such as a link, to the system's share sheet: Android's chooser.
	// Nothing on the Mac, whose menus don't offer it.
	ShareText(w WindowID, text string)
	// SaveImage fetches the image at url as tab's page would, from its cache when it has
	// it, scaled down to at most maxSize pixels on its longer side (0 keeps its size), and
	// writes it at path as a PNG; ImageSaved follows with path.
	SaveImage(tab TabID, url, path string, maxSize int)
	// CopyImage puts the image file at path on the system's clipboard: Android's. Nothing
	// on the Mac, whose menus don't offer it.
	CopyImage(path string)
	// PublishFile takes a file Go put in the downloads folder itself, finished, such as a
	// saved crash report, as a finished download of the engine's is taken: on a phone it
	// is copied into the shared Downloads, and opened from there; on the Mac, where the
	// folder is the user's own Downloads, nothing is to be done.
	PublishFile(path string)

	// SendToPage delivers one message to the page of ours a tab shows.
	SendToPage(tab TabID, msg []byte)

	// SetMenu replaces the native menu bar, where the platform has one.
	SetMenu(m Menu)
	AppCommand(c AppCommand)
	// SetDockColor draws the app's Dock icon with a small dot in color, "#rrggbb", the
	// profile's, so each running profile's copy tells itself apart there; tones.None, no
	// colour picked, draws no dot.
	SetDockColor(color string)

	// Post runs fn on the main thread, later.
	Post(fn func())

	// PageURL is the address one of our pages has in this engine, and OurPage the page an
	// address is, when it is one of ours. ViewSourceURL is the address that shows a
	// page's source. Each only names an address; nothing loads.
	PageURL(p Page) string
	OurPage(url string) (Page, bool)
	ViewSourceURL(url string) string
}

// BlockRules are what the app's policy blocks, compiled for the engine to refuse
// requests by.
type BlockRules struct {
	// Hosts are blocked with every host under them, each lower case.
	Hosts []string
}

// AdRules are the ad blocker's: the hosts it keeps requests from, and any under them, and
// the origins of the pages it lets reach them.
type AdRules struct {
	Hosts []string
	Off   []string
}

// Script is one of the person's scripts for a page: its id, its source, and when it runs,
// "document-start" or "document-end".
type Script struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	RunAt  string `json:"runAt"`
}

// BlankURL is the empty page every engine has.
const BlankURL = "about:blank"

// Listener is what an engine tells the app. There is exactly one.
type Listener interface {
	// ShellMessage is one message from a window's shell page.
	ShellMessage(w WindowID, msg []byte)

	// NavigationStarting fires at the start and on each redirect, whatever started it,
	// and its answer says what becomes of it. nav is the engine's id for the navigation,
	// larger for one started later; its finish or failure carries the same.
	NavigationStarting(tab TabID, nav int64, url string) Navigation
	NavigationFinished(tab TabID, nav int64, url string)
	// AppLink says a press in a tab's own page went to one of the app's own links,
	// project-b:, which no page loads. The engine drops every navigation to one, and tells
	// of none started without a user gesture, in a frame or by a redirect.
	AppLink(tab TabID, url string)
	// NavigationFailed says why, as one of the Failure words, and code is the engine's
	// own name for it, only to show.
	NavigationFailed(tab TabID, nav int64, url string, why Failure, code string)
	// HistoryChanged says whether the tab can currently go back and forward.
	HistoryChanged(tab TabID, canGoBack, canGoForward bool)
	TitleChanged(tab TabID, title string)
	LoadingChanged(tab TabID, loading bool)
	// LoadProgressChanged says how far the page's load has got, 0 to 1.
	LoadProgressChanged(tab TabID, progress float64)
	// ContextMenu says a long press or a right click in a tab's page was on a link, an
	// image or a selection, which a shell may offer to act on.
	ContextMenu(tab TabID, m ContextMenu)
	// ScrollDirectionChanged says the page began to scroll down, or back up: a phone
	// shell hides its bars while the page scrolls down.
	ScrollDirectionChanged(tab TabID, down bool)
	PageCrashed(tab TabID)
	// FaviconChanged carries the page's icon as a data: URL, or "" when the page has
	// none the engine could fetch.
	FaviconChanged(tab TabID, dataURL string)

	// TabOpened says a page opened a new tab (target=_blank, window.open). The engine
	// has already created it, hidden, in workspace, its opener's; the app decides whether
	// to show it. A tab a client of the debugging port asks for is in the first
	// workspace, and may be in a window of its own, newWindow; a page's never is, so its
	// tab goes in its window.
	TabOpened(opener, tab TabID, url, workspace string, foreground, newWindow bool)

	// TabFocused says the keyboard went into a visible tab's page: a click, Tab from the
	// shell, or FocusTab.
	TabFocused(tab TabID)

	// CloseRequested says something asked to close a tab: its page called
	// window.close(), or a DevTools client closed its target. ActivateRequested asks
	// for a tab to be shown, as a DevTools client's Target.activateTarget does. The app
	// decides; the engine has done nothing yet.
	CloseRequested(tab TabID)
	ActivateRequested(tab TabID)

	// Key asks about one key press in any of our pages, at either phase. Returning true
	// means the app used it.
	Key(k Key, phase KeyPhase, in Surface) bool
	// ModsChanged says which modifiers are held now, whenever that changes.
	ModsChanged(m Mods)
	// MenuChosen says the user chose a menu item, by its action id.
	MenuChosen(action string)
	// MenuList is the items of the menu list name (MenuItem.List) as they are now. The
	// engine asks each time a menu holding it opens.
	MenuList(name string) Menu
	// OSBindings carries the keys the OS lets people give our menu items, by item
	// title, as macOS App Shortcuts does. It fires after start and whenever they change.
	OSBindings(byTitle map[string]Key)

	// PageMessage is one message from the page of ours a tab shows, page being its name.
	PageMessage(tab TabID, page Page, msg []byte)
	// PickerMessage is one message from the picker's page, in a copy that hasn't chosen
	// its profile.
	PickerMessage(w WindowID, msg []byte)

	// WindowClosed says a window closed, asked by the app or by its own close button.
	// Its shell page is gone with it; its tabs are still there.
	WindowClosed(w WindowID)
	// WindowFocused says a window became the one in front, the one keys go to.
	WindowFocused(w WindowID)

	// WindowChanged says what a native window is now. It fires after start, on
	// resize and on entering or leaving full screen.
	WindowChanged(w WindowID, s WindowState)

	// ShellCrashed says a window's shell page's renderer died. The engine reloads it, and
	// the new shell reports its layout again.
	ShellCrashed(w WindowID)

	// ProcessCrashed says a child process crashed.
	ProcessCrashed(p ProcessCrash)

	// DevToolsCommand is one of our own DevTools commands, ProjectB.*, from a client of
	// the debugging port, with its params as JSON. The result is the reply's result,
	// as JSON. ErrUnknownCommand leaves the reply to content, which says it has no such
	// method.
	DevToolsCommand(method string, params []byte) (result any, err error)

	// DownloadTarget asks where a download goes, before any of it is saved. A target
	// with no path refuses it.
	DownloadTarget(r DownloadRequest) DownloadTarget
	// DownloadUpdated says what a download is now: on each change once it has a path.
	DownloadUpdated(d Download)

	// FileChooser asks whether a page may open the file panel, and where it opens. The
	// panel is the system's own.
	FileChooser(r FileChooserRequest) FileChooserAnswer
	// FilesChosen says the user chose count files in the panel, from folder.
	FilesChosen(tab TabID, count int, folder string)

	// DialogOpened says a page asks the user something and waits for AnswerDialog.
	// DialogClosed says the engine gave up on one, as when its tab navigated or closed.
	DialogOpened(d Dialog)
	DialogClosed(id DialogID)
	// PasskeyStep says what a passkey request, a dialog of kind passkey, asks of the
	// person now (docs/passkeys.md): the step's name, key, pin, touch, account, phone,
	// phoneBusy or error, and what it carries, a JSON object.
	PasskeyStep(id DialogID, step, carries string)

	// FindReply says how a search in a tab's page went: the matches so far, which one is
	// active, from 1, and whether the count is final.
	FindReply(tab TabID, request, matches, active int, final bool)

	// PageFullScreen says a tab's page asked to fill the screen, as a video's full screen
	// button does, or to stop. IsPageFullScreen is the engine asking whether it does now.
	PageFullScreen(tab TabID, on bool)
	IsPageFullScreen(tab TabID) bool
	// Thumbnail says the engine kept a new picture of a tab's page, as CaptureThumbnail
	// asked: revision is greater for each newer one, for ThumbnailURL.
	Thumbnail(tab TabID, revision int64)
	// DefaultBrowserChanged is how AskDefaultBrowser ended: whether the app is the default
	// browser now.
	DefaultBrowserChanged(is bool)
	// NotificationsAnswered is how AskNotifications ended: whether the app may show them.
	NotificationsAnswered(allowed bool)
	// FolderChosen is how ChooseFolder ended: the folder, or "" for none.
	FolderChosen(w WindowID, path string)
	// ImageSaved is how SaveImage ended: whether the image is now a PNG at path.
	ImageSaved(path string, ok bool)
	// DownloadNotificationCommand is a phone's notification of a download, by the id
	// ShowDownload gave it, asking to cancel, pause or resume it.
	DownloadNotificationCommand(id int, c DownloadCommand)
	// SiteDataListed is what ListSiteData found in a workspace's storage, by host.
	SiteDataListed(workspace string, hosts []SiteData)

	// PermissionRequested is a page asking to use what kinds name. Every kind a page can
	// ask for comes here, those the app answers without asking the person too. The app
	// answers with AnswerPermission, now or once the person has.
	PermissionRequested(r PermissionRequest)
	// PermissionStatusOf is a kind's state for origin, as a page reads it, in tab's page or
	// 0 for a page in no tab: a worker's, or one of our own pages'.
	PermissionStatusOf(tab TabID, origin, kind string) PermissionStatus

	// ConnectionChanged says how the page a tab shows reached it: over a secure
	// connection, an insecure one and why, or neither, as for one of our pages.
	ConnectionChanged(tab TabID, c Connection)

	// CertificateError asks about a certificate error on a request of tab's, 0 for none.
	// True lets the request through; false refuses it.
	CertificateError(tab TabID, e CertificateError) bool

	// SparksFor is the scripts a frame of tab's runs in the page it is about to show at
	// url, each in a world of its own: mainFrame says it is the tab's own.
	SparksFor(tab TabID, url string, mainFrame bool) []Script
	// AdBlocked says the ad blocker kept one request of tab's page from its host.
	AdBlocked(tab TabID)
	// PrintRequested asks whether a frame of tab's page may print itself, as its
	// window.print() asks. True prints it, as Print does.
	PrintRequested(tab TabID) bool

	// NotificationShown is a page showing a notification. True shows it on the system.
	NotificationShown(n Notification) bool
	// NotificationClicked says the person clicked a notification the system showed; the
	// page has had its click.
	NotificationClicked(id string)
}

// Connection is how a tab's page reached it. Reason says why an insecure one is:
// "not-encrypted" (plain http), "certificate" (its certificate has an error) or
// "mixed-content" (a secure page that loaded something over plain http).
type Connection struct {
	State  ConnectionState
	Reason string
}

// ConnectionState matches PBConnection.
type ConnectionState int

const (
	ConnectionNone ConnectionState = iota
	ConnectionSecure
	ConnectionInsecure
)

// CertificateError is what is wrong with a site's certificate: the address asked for,
// why ("expired", "untrusted", "wrong-host", "revoked" or "invalid"), whether the request
// is for a tab's own page, and whether the person may pass it: not for a site that
// demands a valid certificate.
type CertificateError struct {
	URL       string
	Reason    string
	MainFrame bool
	Passable  bool
}

// Notification is one a page shows: the engine's id for it, the origin showing it, the
// page's address, "" for a service worker's, and what it says.
type Notification struct {
	ID     string
	Origin string
	Page   string
	Title  string
	Body   string
}

// PermissionID is the engine's number for a permission request, for this run only.
type PermissionID uint32

// PermissionRequest is a page's request: its tab, 0 for none, the origin asking, the kinds
// it asks for, and whether a press or a key led to it. A kind is named as the Permissions
// API names it ("camera", "microphone", "notifications", "clipboard-write",
// "geolocation"...), "unknown" for one the engine has no name for.
type PermissionRequest struct {
	ID      PermissionID
	Tab     TabID
	Origin  string
	Kinds   []string
	Gesture bool
}

// PermissionStatus is a permission's state. Matches PBPermissionStatus.
type PermissionStatus int

const (
	// PermissionAsk is undecided: a request may ask. A page that asked and got it back
	// was neither granted nor denied, as when a prompt is closed.
	PermissionAsk PermissionStatus = iota
	PermissionGranted
	PermissionDenied
)

// DialogID is the engine's number for a page's dialog.
type DialogID int64

// Dialog is what a page asks: its kind, alert, confirm, prompt, beforeunload (leave the
// page), signin (a site's or a proxy's request for a username and password) or passkey
// (a passkey request, whose steps follow as PasskeySteps), its words (a sign-in's realm,
// a passkey request's site), a prompt's text to start from, the origin of the frame asking (or
// of the site or proxy), and whether a leave-page question is for a reload, or a sign-in
// is a proxy's.
type Dialog struct {
	ID          DialogID
	Tab         TabID
	Kind        string
	Message     string
	DefaultText string
	Origin      string
	Reload      bool
}

// FileChooserMode is what a page's file panel picks.
type FileChooserMode int

const (
	FileChooserOpen FileChooserMode = iota
	FileChooserOpenMultiple
	FileChooserFolder
	FileChooserSave
)

// FileChooserRequest is a page asking for files: its tab, what it picks, and the types
// it accepts, such as ".pdf" or "image/*", none for any.
type FileChooserRequest struct {
	Tab    TabID
	Mode   FileChooserMode
	Accept []string
}

// FileChooserAnswer says whether the panel opens, and in which folder, "" for the
// system's choice.
type FileChooserAnswer struct {
	Allow  bool   `json:"allow"`
	Folder string `json:"folder"`
}

// DownloadID is the engine's number for a download, for this run only.
type DownloadID uint32

// DownloadRequest is a download about to start: its tab, 0 for none, its address, the
// file name the engine suggests from its headers, its type, and whether it asked for the
// Save panel, as Save Link As does.
type DownloadRequest struct {
	ID     DownloadID
	Tab    TabID
	URL    string
	Name   string
	Mime   string
	SaveAs bool
}

// DownloadTarget is where a download goes, and whether the Save panel shows first,
// starting there.
type DownloadTarget struct {
	Path   string `json:"path"`
	Prompt bool   `json:"prompt"`
}

// DownloadState is where a download is.
type DownloadState int

const (
	DownloadProgress DownloadState = iota
	DownloadDone
	DownloadCancelled
	DownloadFailed
)

// Download is a download as the engine reports it. Total is -1 when unknown.
type Download struct {
	ID       DownloadID
	Path     string
	URL      string
	Mime     string
	State    DownloadState
	Received int64
	Total    int64
	Paused   bool
	// Reason is why a failed one failed: "connection" when its connection failed or
	// dropped, "server" when the server refused it, "disk" when there was no room for
	// it, or "" for any other reason.
	Reason string
}

// DownloadCommand is what can be done to a running download.
type DownloadCommand int

const (
	DownloadCancel DownloadCommand = iota + 1
	DownloadPause
	DownloadResume
	DownloadRemove
)

// Failure is why a navigation failed, in words any engine can say. A shell may match on
// them; the engine's own name for the error comes beside them, only to show.
type Failure string

const (
	FailNotFound    Failure = "not-found"    // The host's name doesn't resolve.
	FailOffline     Failure = "offline"      // There is no network.
	FailRefused     Failure = "refused"      // The host refused, reset or closed the connection.
	FailUnreachable Failure = "unreachable"  // No route to the host's address.
	FailTimeout     Failure = "timeout"      // The connection or the response took too long.
	FailTLS         Failure = "tls"          // The certificate or the TLS handshake failed.
	FailProxy       Failure = "proxy"        // The proxy failed.
	FailRedirects   Failure = "redirects"    // Too many redirects.
	FailBadResponse Failure = "bad-response" // An empty, invalid or undecodable response.
	FailBlocked     Failure = "blocked"      // The page's own rules or a port browsers refuse. The app's own block is the gate's.
	FailFile        Failure = "file"         // A local file is missing or can't be read.
	FailOther       Failure = "other"
)

// Navigation is what becomes of a navigation a tab is about to make.
type Navigation int

const (
	// NavigationProceed lets it go on.
	NavigationProceed Navigation = iota
	// NavigationBlock stops it, and the tab shows that it was blocked.
	NavigationBlock
	// NavigationElsewhere stops it, and the tab stays as it was: the app has taken the
	// address to another tab.
	NavigationElsewhere
)

// ErrUnknownCommand is DevToolsCommand's answer for a method it doesn't have.
var ErrUnknownCommand = errors.New("unknown command")

// Area is what a press at a point does.
type Area int

const (
	AreaPage  Area = iota // No region holds the point: it goes to the visible tab's page under it.
	AreaShell             // It goes to the shell.
	AreaDrag              // It moves the window.
)

// Region is one rectangle and what a press inside it does.
type Region struct {
	Rect Rect
	Area Area
}

// VisibleTab is a tab whose page a window shows, and where.
type VisibleTab struct {
	Tab  TabID `json:"tab"`
	Rect Rect  `json:"rect"`
}

// WindowLayout is everything the engine needs to make a window behave.
type WindowLayout struct {
	ViewW, ViewH float64      // The window size the shell measured; an engine places the pages only when it still has that size.
	Tabs         []VisibleTab // The tabs whose pages show, and where. Every other tab is hidden.
	Regions      []Region     // In order. The first that holds a point decides.
	FrameColor   string       // "#rrggbb", painted under the shell and the page.
	FrameDark    string       // In its place while the window is dark: the same, unless no colour is picked (tones.None).
	PageColor    string       // "#rrggbb", our own pages' background, the tones' --bar: shown where a page of ours has yet to draw.
	PageDark     string       // In its place while the window is dark.
	FrameWidth   float64      // The band drawn around the window's edge, over both.
	Theme        Theme        // Light or dark for the window's own parts, or the system's.
}

// Theme is how the window's own parts, its buttons and title bar, are drawn.
type Theme int

const (
	ThemeSystem Theme = iota // Whatever the system is.
	ThemeLight
	ThemeDark
)

// At is the one hit test: what a press at x, y does.
func (l WindowLayout) At(x, y float64) Area {
	for _, r := range l.Regions {
		if r.Rect.Contains(x, y) {
			return r.Area
		}
	}
	return AreaPage
}

// ProcessCrash is a child process that died when it shouldn't have: a renderer, the GPU
// process or a utility process. The engine starts another when one is needed.
type ProcessCrash struct {
	Type     string // The kind of process, as the engine names it.
	Status   string // How it ended: "crashed", "was killed", "ran out of memory", "failed to launch", "was evicted for memory" or "exited abnormally".
	ExitCode int
	PID      int
	URL      string // The page a renderer showed, or "".
}

// WindowState is what the engine reports about its native window.
type WindowState struct {
	W, H         float64 // The content area, CSS pixels.
	Controls     Rect    // The native window buttons; empty when there are none.
	CornerRadius float64 // 0 when the corners are square.
	FullScreen   bool
	Bounds       Rect // The window's frame on screen, points, y down from the top of the main screen.
	Minimised    bool
	Zoomed       bool
}

// WindowRequest is what the window is asked to do. The first four are what pressing that
// window button does, so zoom and full screen go in or out. WindowFront brings the window
// to the front and makes the app the active one.
type WindowRequest int

const (
	WindowClose WindowRequest = iota + 1
	WindowMinimise
	WindowZoom
	WindowFullScreen
	WindowFront
)

// WindowMode is how a window sits on screen.
type WindowMode int

const (
	ModeNormal WindowMode = iota
	ModeMinimised
	ModeZoomed
	ModeFullScreen
)

// ContextMenu is what a long press or a right click in a page was on, at X, Y in the
// page: a link's address and words, an image's address (Media "image") or another
// medium's, and the selected text.
type ContextMenu struct {
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Link      string  `json:"link"`
	LinkText  string  `json:"linkText"`
	Src       string  `json:"src"`
	Media     string  `json:"media"`
	Selection string  `json:"selection"`
	// Editable says the press was in a field.
	Editable bool `json:"editable"`
}
