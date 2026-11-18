# The engine interface

What a browser engine must give the Go core, and the smallest part of it that runs the first
app. The Go core knows nothing of Chromium's content layer: Chromium is one engine behind
this interface, and the mock (`core/mockengine`) is another.

**The rule:** the engine gives us page views; the window is ours. Go's window manager decides
what a window shows and where, and the engine places its views where it is told. An engine
that would own the window itself doesn't fit.

## Where it is

- **Go:** `core/engine/engine.go`. `Engine` is what the app asks of an engine, and `Listener` is
  what an engine tells the app. Every call both ways is on the main thread; a goroutine gets
  back onto it with `Engine.Post`.
- **C:** `engine/abi/pb_abi.h`. `PBEngineCalls` is the engine's table, `PBGoCalls` is Go's,
  and `PBEvent` lists the events. Everything the engine tells Go is one call,
  `hook(event, values)`. `core/abi` turns the events into `Listener` calls, and
  `core/cmd/lib` implements `Engine` on the table.
- **Only a Chromium engine has:** content's generated hooks (`pb_content_abi.h`, null in
  another engine), extensions (`pb_extensions_abi.h`, the `extensions` build tag) and
  `ProjectB.*` DevTools commands.

## Start-up

The engine owns the process and its run loop, and calls Go in this order:

1. `claim_profile(root, profile, user_data_dir, picker, urls)` gives back the folder this copy
   keeps its files in, or NULL when another copy has the profile, in which case quit.
2. `launch_switches(dir)` gives back the switches the profile's flags ask for. A flag marked
   `"engine": "chromium"` in `core/flags/registry.json` is Chromium's own switch; any other
   engine ignores it.
3. `timeline_start`, then `start(calls, blocked_hosts)`. Go opens the profile
   (`OpenProfile`), hands over the block rules (`SetBlockRules`), and opens its windows.
4. `run_posted` on the main thread, each time `post_wakeup` asks.
5. `stop` as the app ends.

## What an engine must implement

**Calls** (`Engine`): profile and policy (`OpenProfile`, `SetBlockRules`); tabs and
navigation; workspaces (`CreateTab(workspace)`, `SetFirstWorkspace`, `EraseWorkspace`);
windows and their layout (`CreateWindow`, `SetWindowLayout`, `RequestWindow`,
`SetWindowBounds`, `SetWindowMode`, `FocusTab`, `FocusShell`); the shell's, the picker's and our
pages' channels (`PageMessage`, `SendToPage`); menu and app commands; downloads, dialogs, find and page full screen;
pictures of the tabs' pages (`CaptureThumbnail`); and the names of our pages' addresses (`PageURL`, `OurPage`,
`ViewSourceURL`, `ThumbnailURL`).

**Events** (`Listener`): the adapter fires each one itself, from what the engine reports.
Events 1 to 59 are windows, pages of ours, menus, crashes, downloads, dialogs and permissions.
Events 60 to 79 are what a tab's page does: navigation, title, loading, context menu, scroll,
focus, crash, close and activate requests, keys, find, full screen and a new picture of it
(`Thumbnail`); 81, `AppLink`, is a press in a tab's page on one of the app's own links,
`project-b:`.

**Rules an engine must keep:**
- `NavigationStarting` is asked synchronously, before a tab's top-level navigation starts and
  on every redirect, including back and forward, reloads and POSTs. Its answer is one of
  three (`engine.Navigation`, `PBNavigation`): go on; block, and the tab shows it was blocked;
  or elsewhere, which drops the navigation with nothing shown, because the app has taken the
  address to another tab. Any other answer blocks it.
- `TabOpened` reaches Go before the new tab's first `NavigationStarting`.
- A navigation to one of the app's own links, `project-b:`, goes nowhere, wherever it starts,
  and is never asked of `NavigationStarting`. Only one a user's gesture started at the top of a
  tab's page, not by a redirect, is told as `AppLink`; a script's, a frame's or a redirect's
  is dropped unheard, so a page can't open one by itself ([sync.md](sync.md#the-link)).
- `Key` is asked synchronously, before a page sees the key and again if the page didn't use
  it, as a Windows virtual-key code with our modifier bits.
- The block rules apply to every request a page makes, not just its navigation: frames,
  images, scripts, fetches, workers and WebSockets. The hosts are fixed at launch today;
  updating them live is a later step, needed only if policy ever changes while the app runs.
- A failed navigation says why in `engine.Failure`'s words and gives the engine's own error
  name beside them, only to show. A crashed process gives how it ended in the words on
  `ProcessCrash.Status`. The engine's own vocabulary never crosses as something Go matches on.
- A tab of one workspace keeps that workspace's cookies and storage, and so does every tab it
  opens.
- `CaptureThumbnail(tab, maxWidth)` copies what the page shows now, scaled down to at most
  `maxWidth` device pixels across, never up, with its shape kept, and keeps it in the engine,
  one per tab, until the next or the tab closes; then `Thumbnail(tab, revision)` tells Go,
  the revision greater each time. A copy that fails, or a tab with nothing drawn yet, tells
  nothing; one kept is marked `tab.thumbnail` on the timeline with how long the copy and the
  JPEG took, its size and its bytes. The picture never crosses to Go: our pages load it at `ThumbnailURL(tab,
  revision)`, and no web page can. Chromium keeps a JPEG of about quality 70 and serves it at
  `chrome://thumb/<tab>?v=<revision>` in our pages' context, only the shell's policy letting
  it in (`img-src chrome://thumb`); the mock draws an SVG of the address and its dev server
  serves it at `/mock/thumb/<tab>`. A copy asked for just before the tab is hidden still
  completes on Android, as the window's one compositor keeps the page's surface until it is
  done; on a Mac a tab's view has a compositor of its own, which goes with it, so the copy can
  time out there. Go asks only while the profile's shell is a phone shell, 720 pixels wide at
  most ([shells.md](shells.md), `captureTab`).

**Hosting our pages:** the shell is a transparent web view over the tabs' views, and presses go
where `WindowLayout.At` says. It needs a JS-to-native call and a native-to-JS call
(`shell/src/bridge.js`), a strict CSP, and isolation from tab content. Settings and the picker
are pages hosted the same way.

## The minimal slice

Enough for the first app (one window, tabs, the shell, the gate):

- **Calls:** `OpenProfile`, `SetBlockRules`, `CreateTab`, `CloseTab`, `LoadURL`, `GoBack`,
  `GoForward`, `Reload`, `Stop`, `CreateWindow`, `SetWindowLayout`, `RequestWindow`,
  `FocusTab`, `FocusShell`, `SendToShell`, `SetMenu`, `AppCommand`, `Post`, `PageURL`,
  `OurPage`. Everything else can start as a no-op, and `extensions` and `content_hooks` stay
  null.
- **Events:** shell message, window changed, closed and focused, tab opened, menu chosen and
  menu list, plus navigation starting (with its answer), finished and failed, history, title,
  loading, tab focused, close requested and key.

## What is still Chromium-shaped

- `DevToolsCommand` and the test tools built on CDP and the `--pb-driver` driver
  (`tools/drive_first_app.py`, `check_basics.py`, `measure.py`).
- `Key`'s Windows virtual-key codes, which a non-Chromium engine has to translate into.
- `ProcessCrash.Type`, which carries the engine's own process names.
- Workspaces as fixed storage partitions that popups inherit, and `DuplicateTab` with
  history.
- On Android, the Java side and the one-compositor window.

The audit behind this is in `project-b-research/engines/engine-boundary-audit.md`.
