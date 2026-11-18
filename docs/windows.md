# Several windows, and the tabs each one shows

A design note, 2026-11-13, for the second step of [profiles.md](profiles.md), with the
owner's decisions. Built for macOS; Android keeps its one window. The survey of the code
behind it is `project-b-research/windows-survey.md`.

## Today

- Go makes the windows (`CreateWindow`), each with its own shell page, and keeps each
  one's tabs, its settings tab among them while it has one, and active tab, and the window in front, which the engine
  reports (`WindowFocused`). Commands and the menu follow the window in front. File, New
  Window (Cmd+N) opens one. A window's close button closes it (`WindowClosed`), and so does
  Close Tab in a window with no tabs: closing its last tab leaves it open, until a window
  always has a tab ([below](#every-window-has-a-tab-decided-2026-11-18)). Closing the last
  one quits, and Quit (`AppQuit`) ends every window at once. A second start's URLs open in the window used last. The Window menu
  lists the windows by title and Cmd+` cycles them: AppKit does both by itself, and the
  window it brings forward reports itself to Go as any key window does.
- The window's layout lists the tabs it shows, each with a rectangle; the shell lists one,
  the active tab. The engine places each listed tab's view, tells a tab that joins or
  leaves the list that it is shown or hidden, and says when a click moves the keyboard
  into a visible page. `FocusTab` gives a tab's page the keyboard.
- Every call about a window and every event from one carries Go's id for it, 1 for the
  first: the layout, a window request, a message to or from the shell or settings, a
  change of size. The shell is still "page 0" to the hooks, so a key doesn't say which
  window it came from; keys go to the window in front.
- Extensions see one window, id 1, and "the current window" is the window in front.
  DevTools works with the real windows: `Target.createTarget` with `newWindow` opens one,
  showing a target brings its window to the front, and `Browser`'s window calls answer
  from and act on the target's window.
- The engine reports each window's frame on screen and whether it is minimised, zoomed or
  in full screen with the rest of its state (`WindowChanged`), and puts a window's frame
  where Go asks (`SetWindowBounds`), and puts it into normal, minimised, zoomed or full
  screen from whatever it is in now (`SetWindowMode`; the window requests still toggle,
  for the menu and the buttons). Go's window manager is where another manager reads the
  windows, each with its id, frame, state and whether it is in front (`window.Manager.List`,
  `Get`), sets a frame or a state (`SetBounds`, `SetState`), and hears windows open and
  close, the window in front change, tabs move between windows, and a frame change
  (`OnEvent`). `App.NewWindow` takes the first tab's URL and returns the new window and
  its tab; `App.NewWindowWith` returns the window the tab is in. A minimised window's tabs are
  told they are hidden.
- Go moves a tab to a place in another window's list, or into a new window made for it,
  which then holds only that tab (`tabs.Manager.Move`, `App.NewWindowWith`). The tab keeps
  its id and its page goes on running: the engine moves its view when the other window's
  layout lists it, and tells it nothing hidden. The window it arrives in makes it active;
  the one it left goes on as if it had closed there, but closes if it was its last tab, as
  what it held went elsewhere.
  Both shells hear it in their tab lists. Tab, Move Tab to New Window moves the active tab
  of the window in front; the driver and DevTools ask through `ProjectB.moveTab`.

## What changes

**A window has an id.** Go makes the ids, 1 for the first. Every call about a window and
every event from one carries it: the layout, a window request, a message to or from the
shell, settings, a window's change of size.

**A window is a shell page, a settings page over it when open, and the tabs it shows.** Go
keeps the windows, and in each an ordered list of tabs. A tab keeps its id for its life,
whichever window it is in.

**The layout lists the visible tabs.** The shell reports, with its regions, a list of
rectangles, each with a tab. That list takes the place of the one page rectangle and of
`ShowTab`. The engine places each listed tab's view at its rectangle and tells a tab it is
shown or hidden as it enters or leaves the list. The shells we have list one tab.

**Focus stays its own call.** `FocusPage` becomes `FocusTab(tab)` and `FocusShell` takes a
window. When a click moves the keyboard into a visible page, the engine says which tab.

**Go's one active tab becomes three things:** each window's visible tabs, each window's
focused tab, and the window in front. A command acts on the focused tab of the window in
front.

## Behaviour, as decided

- A new window (Cmd+N) starts with one new tab, whatever the shell, and opens a little down
  and right of the window in front.
- Every window has a tab (2026-11-18, [below](#every-window-has-a-tab-decided-2026-11-18)):
  its last tab closing closes it.
- Closing a profile's last window quits its process.
- Addresses on the app's command line open as tabs of its first window at the start, in
  place of the blank one, whatever the shell; ChromeDriver starts the app with one and looks
  for a page. A second start on the same profile hands them to the first copy, as before.
- Quit closes every window at once, with no question.
- A link from another app, or from a second start of the app, opens as a new tab in the
  window used last.
- Settings is a tab of the window, one per window ([below](#our-own-pages-are-tabs-decided-2026-11-14)).
- A window a page opens (`window.open`) stays a tab, as today.
- Ctrl+Tab stays within a window.
- A tab in a minimised window is told it is hidden.
- A shell's own data, when shells have any, is kept per window.

### Our own pages are tabs (decided 2026-11-14)

The owner's words: "no we ahould move the settings also to be similar to other internals
pages... clean simple rules that you mentioned... address bar can be changed to a title
with option to close it so no typing allowed any links in internals pages can naviagte to
ingernal pages or new tab for extenal pages". Settings is the first such page; history,
bookmarks and downloads follow it.

- **An internal tab is a tab Go marks as ours.** It runs in our pages' own context, not the
  profile's, and is in no workspace. It remembers the workspace of the tab it was opened
  from: New Tab from it, and a page it opens, go there (the first workspace if that one is
  gone). Its kind is fixed for life: an internal tab never becomes a web tab, and a web tab
  never becomes an internal one, so no tab swaps its WebContents between the two contexts.
- **One per page per window.** Opening settings (Cmd+, the menu, a shell's Settings
  button) in a window that has a settings tab switches to it; otherwise a new one opens
  beside the active tab and becomes active.
- **It closes as any tab closes:** Close Tab, Cmd+W, its tab's close. Nothing about it is
  special to focus, the keyboard or closing; settings no longer covers the window.
- **No typing.** For an internal tab a shell shows no address field: in its place the page's
  title (which the page may make "Downloads › In progress") and a way to close the tab.
- **One navigation rule:** a link in an internal page to another of our pages navigates
  that tab; anything else opens a new tab beside it, in the workspace the internal tab
  remembers. One check in our pages' context does it, for every way a navigation can
  start there: a link, a link dragged onto the page, an extension, DevTools or ChromeDriver
  (so a WebDriver navigation of an internal tab yields a new tab). A web tab asked to load
  one of our addresses opens or switches to the internal tab instead. The check is Go's
  gate, which every navigation in a tab passes: it lets a navigation go on, blocks it, or
  drops it with nothing shown, the tab staying as it was, when it takes the address to
  another tab (`tabs.Manager.HandleNavigationStarting`).
- **Moving to another window keeps a tab's kind,** which its address says. An internal tab
  is never duplicated (Duplicate Tab is off for one) nor pinned, and one moved to a window
  that has a tab of its page shows that tab and stays where it was, so a window keeps one
  of each.
- **To a shell an internal tab is a tab:** drawn, switched, closed and laid out as any
  other. The shell's state says which tabs are ours, and the contract holds a shell to
  showing a title, not a text field, where the address goes for one.
- **Extensions don't see internal tabs:** they live in another context, as Chrome keeps
  its own pages out of reach of extensions.

Known gaps, as built:
- A tab the engine makes by itself for one of our addresses, such as DevTools'
  `Target.createTarget` on `chrome://settings`, leaves a blank web tab beside the settings
  tab, as the tab is made before Go sees the address.
- An extension's button pressed while a settings tab is active tells the extension
  nothing, as it doesn't see that tab.

## Every window has a tab (decided 2026-11-18)

This replaces the window with no tabs, decided on 14 and 16 November. The owner, 2026-11-18:
"closing last tab can kill the browser since the user clearly sees the single tab there", and
of New Tab, "this should be shells decision... for our standard lets go with non renderer new
tab and cmd t new tab action shows box on top". Windows went without tabs to save the
renderer every new tab started, 16 to 18 MiB each, measured on 0.2.85
(`project-b-research/newtab-model/measurement.md`). A new tab that loads nothing starts none,
so an empty window has no job left. Until this is built the app behaves as the other docs
say.

- **A window always has a tab, and an active one.** The app's start and New Window open one
  new tab, whatever the shell; the addresses on the command line, or a restored session,
  take its place, as before.
- **A window closes when its last tab goes,** however it goes: Close Tab, a tab's close, a
  move to another window, a workspace delete, an extension. So Cmd+W on a window's only tab
  closes the window, and closing a profile's last window quits it, as before.
- **On a phone,** closing the last tab closes the app, and the next start opens on a new tab.
  The back key works as now down to the last tab: on a tab with no history it closes the
  tab, so on the last one it closes the app.
- **A new tab loads nothing.** The engine makes every tab with no renderer (content's
  `kNoRendererProcess`) and Go loads nothing into a new tab, so no process starts until it has
  an address. Its address is empty and its title New Tab. The shell draws its new tab page
  where the page goes, since a tab with nothing loaded shows the shell under it. A shell
  that has none draws a backdrop in the page's colour there.
- **New Tab is the shell's to decide.** Without a shell of its own, Go's New Tab opens a new
  tab. A shell that takes it (`"takes": ["tab.new"]`) does what suits it. What a shell does
  on its new tab page, such as Enter with nothing typed, is the shell's too.
- **Standard:** New Tab (Cmd+T, the menu, the shell's buttons) switches to the window's new
  tab in the workspace shown, opening one when there is none, and shows the box on it. On
  the new tab already, it shows the box there. One rule: go to the new tab, then show the
  box. The box no longer opens over the page that was showing.
- **A new tab may later hold an address:** the shell's own (a search page), the user's
  choice, or an extension's new tab page, asking the user first as Chrome does. None of
  these is built; drawn by the shell is the only kind for now. A page of the shell as the
  new tab doesn't fit, since one of our tabs stays ours for life
  ([above](#our-own-pages-are-tabs-decided-2026-11-14)).
- **Extensions** see Chrome's model: a window always has a tab, and its last tab closing
  closes it, with `tabs.onRemoved` saying `isWindowClosing`.
- **What goes once it's built:** Go's rule for a window with no active tab, and the
  exceptions in `Tabs.OnEmpty` (`App.madeForTab`); the actions switched off with no tab;
  `navigate` opening a tab in a window with none; the empty start of a shell that takes New
  Tab; the blank page a new tab loads, with its workspace workaround in `Engine::LoadURL`;
  each shell's No tabs (shells rule 18); the back key's stop at No tabs; and the caveat in
  [extensions.md](extensions.md).

## Pinned tabs

The owner approved the design on 2026-11-14, with closing a pinned tab keeping the pin,
and built in its simplest form; what goes further is listed at the end.

- **A pin is a saved place in a workspace,** kept by Go for the profile in its own file,
  `pins.json` beside `workspaces.json` (`core/pins`): `[{id, workspace, url, title,
  favicon}]`, in the order pinned, a new pin after the others of its workspace, with the
  title and icon its tab had then. Pins survive a restart; tabs don't, so after one a pin
  has no tab until it is opened.
- **A tab open for a pin carries its id,** `pin`, and a window has one tab for a pin at most
  (`tabs.Tab.Pin`). Go keeps a window's pinned tabs first in its order, in the pins' order,
  so a shell that draws nothing for pins still lists them first. A pinned tab is in no tab
  group: pinning takes it out of its own.
- **Pinning** (`pinTab {id}`, or Pin Tab in the Tab menu, `tab.pin`, with no default key)
  makes a pin of a web tab's address, at the end of its workspace's pins, bound to the tab.
  A tab of ours can't be pinned, nor a tab with no address yet. Other windows show the pin
  with no tab.
- **Unpin** (`unpin {pin}`, or the same menu item, which says Unpin Tab on a pinned tab)
  removes the pin. Each tab open for it stays, as an ordinary tab first after the pinned
  ones.
- **Opening a pin** (`openPin {pin}`) shows this window's tab for it, or opens one bound to
  it, in the pin's workspace and at its address, in its place among the pinned tabs.
- **Closing a pinned tab** closes the tab and keeps the pin, which then shows with no tab,
  as after a restart. Only Unpin removes a pin.
- **Deleting a workspace** deletes its pins, as it closes its tabs.
- **Moving tabs:** a pinned tab moved into another window stays bound there, among its
  pinned tabs; if that window already has a tab for the pin, the move shows that tab
  instead and leaves the moved one where it was, as for our pages' tabs. `moveTab` keeps an
  unpinned tab after the pinned ones, and a pinned tab where it is.
- **Shells** hear `pins` in the snapshot and again on any change, and each tab's `pin`
  ([shells.md](shells.md#the-messages)). No contract rule holds a shell to drawing them:
  Glide draws them ([glide.md](glide.md)). The standard shell lists them first, in the
  sidebar or the strip, with a rule after them: a pin with its tab here as that tab, one
  without muted, a press opening its tab and its × unpinning it. Its tab menu has Pin Tab,
  off on a tab of ours or one with nothing in it, and Unpin Tab on a pinned tab, which
  offers no group.

Later steps, not built:
- Reordering pins, by `moveTab` on a pinned tab or a drag in a shell's list of them; pins
  stay in the order they were pinned.
- A pin that follows its tab: its title and icon are its tab's when it was pinned, and a
  pinned tab that goes elsewhere stays bound, the pin keeping its address.
- Extensions see `pinned: false` on every tab; `chrome.tabs` could report and set it from
  the pins.
- A contract rule (rules 40 to 49 are kept for pinned tabs), once the shells draw them.

## Order of building

Each commit builds and leaves a working app.

1. Done. A window id on every window call and event, with one window.
2. Done. The list of visible tabs in the layout, with one entry; `ShowTab` goes.
3. Done. A second window: making one, closing one, the window in front, Cmd+N, commands
   and menus following it, quit apart from close, the handover into the window used last.
4. Done. What macOS expects of an app with several windows: the Window menu's list and
   Cmd+`.
5. Done. Moving a tab to another window.
6. Done. Extensions' `chrome.windows` and DevTools' window calls with real windows.

## Seeing the regions

The Show window regions flag (`--pb-show-regions`) has each native window draw a 1px
border around every rectangle the window manager tracks, from the layout it last
applied and its own state, over everything else:

| Kind | Border |
|---|---|
| The window buttons | purple `#a855f7` |
| Drag areas | blue `#2563eb`, dashed |
| Shell areas | red `#dc2626` |
| Each shown tab's page, a settings tab's too | green `#16a34a`, labelled `tab <id>` |
| The frame band's inner edge | grey `#9ca3af`, dashed |

It only draws: it takes no presses and claims no part of the title bar, so the window
drags and clicks as it does without it. It is macOS's `PBRegionOutlineView` in
`engine/browser/window_mac.mm`. The controls inside a drag bar are shell areas, so they
are red inside the bar's blue.

The mock draws the same outlines in its window page (`core/cmd/dev/window.html`), from the
window state the mock engine sends it, the flag among it: the drag areas dashed 4 and 3,
the frame band 2 and 2, each shown tab's id in 10px at its page's top left. As in the real
app the flag takes effect at launch: the dev server hands the mock engine the switches of
the profile's saved flags as it starts, so after turning it on in settings, start
`go run ./cmd/dev` again.
`shell/e2e/outlines.cjs` checks an outline of the right kind on each tracked rectangle,
with one tab and with the settings tab shown, and none with the flag off.
