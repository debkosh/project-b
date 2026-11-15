# Several windows, and the tabs each one shows

A design note, 2026-11-13, for the second step of [profiles.md](profiles.md), with the
owner's decisions. Built for macOS; Android keeps its one window. The survey of the code
behind it is `project-b-research/windows-survey.md`.

## Today

- Go makes the windows (`CreateWindow`), each with its own shell page, and keeps each
  one's tabs, its settings tab among them while it has one, and active tab, and the window in front, which the engine
  reports (`WindowFocused`). Commands and the menu follow the window in front. File, New
  Window (Cmd+N) opens one. A window's close button closes it (`WindowClosed`), and so does
  Close Tab in a window with no tabs: closing its last tab leaves it open
  ([below](#a-window-with-no-tabs)). Closing the last one quits, and Quit (`AppQuit`) ends
  every window at once. A second start's URLs open in the window used last. The Window menu
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

- A new window (Cmd+N) starts with one new tab, and opens a little down and right of the
  window in front. With a shell that takes New Tab (`"takes"` in the registry, as the
  standard shell does for its box), it starts with no tabs, and so does the app's first
  window: the shell shows its own way to a tab there (the owner, 2026-11-16: "yes on open
  no tab page created").
- A window may have no tabs (2026-11-14, [below](#a-window-with-no-tabs)): closing its last
  tab leaves it open, and Close Tab then closes it.
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

## A window with no tabs

2026-11-14, the owner: "closing the window on last tab closure seems like something we
copied from chrome without actual need for it right.. the shell might decide what to do
when no tabs aare there and then another close actiually closes". He decided the rest the
same day: a window whose last tab moves out still closes, New Window and the app's start
keep their one new tab, and a workspace delete leaves a window with none.

- **A window may have no tabs.** Closing its last tab leaves it open, with no tabs and no
  active tab: its shell hears `tabs: []` and `active: 0`, and shows no page. Only closing
  the window closes it. A window with tabs always has an active one, so a tab that comes
  into a window with none is its active tab, even one opened behind.
- **Two kinds of window still close with their last tab** (`Tabs.OnEmpty` in `App.Start`):
  - one made for a tab opened in a window of its own, `TabOpened` with `newWindow`, which Go
    remembers per window (`App.madeForTab`). Only a client of the debugging port asks for
    one (`Target.createTarget` with `newWindow`): a page's `window.open` and `target=_blank`
    open a tab in the page's own window;
  - one whose last tab moved to another window: dragged out, `ProjectB.moveTab`, an
    extension's move. What it held went elsewhere.
- **Close Tab,** Cmd+W, the menu, or a shell's `closeTab` naming no tab, closes the active
  tab, and in a window with no tabs the window. So two Cmd+W on a window's only tab close
  the window. It stays enabled in a window with no tabs.
- **The actions that need a tab are disabled with none:** Back, Forward, Reload, Force
  Reload, Stop, the zooms, View Source, Focus Next and Previous Pane, Duplicate, Move Tab to
  New Window, and choosing a tab (next, previous, recent, Tab 1 to 8, Last Tab). New Tab,
  Open Location, New Window, Close Window, Settings and the shells' own actions stay as
  they are.
- **The address still works:** `navigate` in a window with no tabs opens a tab there, in the
  workspace the message names or the first in the list, at the address.
- **New Window and the app's start open with one new tab,** as before.
- **Its title** is the app's own, Project B, as a new window's is before its first tab: the
  engine titles a window after its focused tab, and with none shown, after the app
  (`Engine::UpdateTitle`). The Window menu and Cmd+` list it so.
- **Each shell draws an empty state of its own** ([shells.md](shells.md#what-every-shell-does),
  rule 18): a quiet "No tabs" and New Tab with its key where the page goes, as its own, in
  the shell's own type and colours; Glide's is a card where the page's card goes. Most give
  the address the keyboard. Mobile doesn't draw one yet: on a phone the back key is to go
  back in the page, else close the tab and show the one used before it, down to no tabs,
  and only then close the app.
- **Extensions** see a window with no tabs as one whose `tabs` are `[]`, with no active tab,
  and its last tab closing as `tabs.onRemoved` with `isWindowClosing` false
  ([extensions.md](extensions.md)). Chrome never has a normal window without a tab, so an
  extension may assume one has.
- **Deleting a workspace** closes its tabs, and a window that held only them stays open
  with none ([workspaces.md](workspaces.md)).

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
  Glide draws them ([glide.md](glide.md)); the standard shell's are its own next step.

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
