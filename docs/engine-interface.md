# The engine interface

What a browser engine must give the Go core, and the smallest part of it that runs the first
app. The Go core knows nothing of Chromium's content layer: Chromium is one engine behind
this interface, and the mock (`core/mockengine`) is another.

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
`SetWindowBounds`, `SetWindowMode`, `FocusTab`, `FocusShell`); the shell, settings and picker
channels; menu and app commands; downloads, dialogs, find and page full screen; and the
names of our pages' addresses (`PageURL`, `OurPage`, `ViewSourceURL`).

**Events** (`Listener`): the adapter fires each one itself, from what the engine reports.
Events 1 to 59 are windows, pages of ours, menus, crashes, downloads, dialogs and permissions.
Events 60 to 79 are what a tab's page does: navigation, title, loading, context menu, scroll,
focus, crash, close and activate requests, keys, find and full screen.

**Rules an engine must keep:**
- `NavigationStarting` is asked synchronously, before a tab's top-level navigation starts and
  on every redirect, including back and forward, reloads and POSTs. Its answer is one of
  three (`engine.Navigation`, `PBNavigation`): go on; block, and the tab shows it was blocked;
  or elsewhere, which drops the navigation with nothing shown, because the app has taken the
  address to another tab.
- `TabOpened` reaches Go before the new tab's first `NavigationStarting`.
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
- Events 1 to 59 are still decoded in `core/content/events.go`. They move into `core/abi`
  once the open branches adding to them have landed.

The audit behind this is in `project-b-research/engines/engine-boundary-audit.md`.
