# To do

What is decided but not built, and what is waiting for a discussion. Last updated 2026-11-16.
Take an item off when it lands or is dropped.

## For discussion with the owner first

- **A start that gives the UI thread back.** On a cold start the shell's request for its page
  waits about 90 ms on the browser's UI thread: first for Go's synchronous start (the window,
  about 35 to 40 ms, its show, the menus and managers, until about 140 ms), then for macOS's
  launch step, about 50 ms with no Chromium task. Serving the request takes under 1 ms. Making
  the shell first and starting its load before the window (branch `shell-first` on origin)
  moved the load from about 52 to 11 ms but the first paint not at all, interleaved 2x5 on
  2026-11-15 (warm 247 main, 253 branch; fresh 259, 243). Serving chrome://shell's files off
  the UI thread gains nothing: they are read on the ThreadPool already. What could gain is
  making Go's start asynchronous, so the request is served while Go builds its managers and
  window: a change to how Go and the engine start, with at most those 90 ms to win.
- **Dragging tabs** between workspaces (Spaces), onto groups, and in a sidebar: not designed
  yet. The Mobile shell's switcher drags a card onto another within one workspace only.
- **URL policy on every request.** Each request is allowed or blocked by policy. The options so
  far: a proxy in the browser process, or rules in content's network layer, before each
  request is sent. Not discussed yet; nothing to build until it is.
- **Menus and shortcuts, what is left.** Built for macOS ([menus-shortcuts.md](menus-shortcuts.md)).
  Not discussed yet: the six points from the guidelines at its end. Decided, not built:
  catching keys at the window (the late keys after clicking a suggestion), the menu editor,
  the in-app menu for Windows and Linux. Ctrl+Cmd+F works through the driver; the VM test
  should confirm it with AppKit's own dispatch.
- **The certificate error page.** Mock and two ways to wire it (in the tab, or painted by the
  shell) in `project-b-research/morning-2026-11-10/cert-error-mock-notes.md`.
- **Certificate revocation.** Recommendation: Go fetches Google's CRLSet, with a copy in the
  bundle. `revocation.md` in the same folder.
- **Browser contexts for automation** (what Playwright needs). `browser-contexts.md` in the same
  folder.
- **A new tab page and a crash page.** A crashed page shows the frame colour today.
- **Content's own `chrome://` pages** (such as `chrome://gpu`) pass the gate but don't load,
  because the resource pack lacks their files.
- **Back after the first page in a new tab** returns to `about:blank`.
- **What the first timelines show** ([logging.md](logging.md), baselines in `tools/baseline/`, medians of 10
  starts of the development build, 2026-11-12). Nothing is changed yet; each is for a decision.
  - *Settings is laid over the window before it has drawn anything.* From the click: over the window at 3 ms
    on the Mac and 7 ms on the Android emulator; its first paint at 71 ms and 117 ms; its first frame on screen
    at 132 ms and 163 ms. On Android the window's first frame with the empty overlay is on screen at 45 ms, so
    the phone shows a blank page for about 120 ms: the flash. The Mac covers the window for as long.
  - *A first frame takes long to reach the screen after it is painted:* 64 ms on the Mac for the shell at
    start-up and 62 ms for settings, about four frames; 118 ms and 45 ms on the emulator.
  - *Mac start-up is 296 ms to the shell on screen.* The larger steps: 27 ms before `main`, 41 ms to content
    ready, 39 ms to the window, 53 ms from the window shown to the first tab's commit, and the 64 ms above.
  - *Input on this Mac is not late:* a real click reaches the window 1 to 2 ms after the OS made it and the
    shell 1 ms later. One real Cmd+T took 22 ms from the window to Go, against 0.5 ms through the driver;
    one sample, worth a look. The macOS VM's lag (clicks landing 1 to 3 s late) hasn't been read this way
    yet: `tools/timeline.py` on the VM's `timeline.log` will say which side it is on.
  - *Android start-up is 989 ms on the emulator:* 276 ms from Go started to the shell's commit, and 202 ms
    from the shell's first state to its first paint.
  - *Go sends to the shell before its script listens:* each start logs "__shellReceive is not defined" a few
    times on stderr. The snapshot that follows `ready` makes up for it.
- **Slowness, fixed at the cause, never masked:** a stale picture after a resize, and a brief
  flash at launch. The rejected masking attempts are on branch `parked-window`.
- **Android:** the colour before Go starts. See [android.md](android.md#the-shell-on-a-phone).
- **Extensions: the checks left out, and what is open.** Releases carry extensions since 0.2.25,
  with their flag off by default; turned on, they install from the Chrome Web Store. Every
  check skipped on purpose (hashes, publisher proof, blocklist, policy, content verification,
  host scrubbing) and every open behaviour is listed in
  [extensions.md](extensions.md#checks-to-discuss), to decide before the flag is on by default,
  along with the URL policy above covering what extensions request.
- **Checking store packages against Google's signature.** Today a Web Store package is held to
  the plain CRX3 check only, not to Google's publisher proof. No checks for now, by the owner's
  call; to discuss later.
- **The window's drag region.** Whether it can be simplified. The current fix is in 0.2.23.

## Decided, not started

- Move focus decisions into Go (fixes Cmd+W pressed twice quickly).
- An engine stress test against other engines, once the owner calls the engine solid.
- **Extensions, later** ([extensions.md](extensions.md#open-for-the-owner)): updates on a
  timer, from the store each extension came from; an update that asks for more permissions
  turned off until the user agrees, as Chrome does; keys for extension commands, with the
  keymap; and trying Bitwarden, Okta, 1Password (needs `nativeMessaging`) and Grammarly
  (needs `identity`) past their install.
- A comparison of JS frameworks for the shell, and the final language decision.
- **The mock as a small desktop.** A desktop with one Project B icon that launches the app;
  the mock OS moves the window from the drag regions Go reports, resizes it from its edges
  and corners (each size a WindowChanged, so live resize is testable without a VM), and
  its close, minimise, zoom and full screen buttons send the window requests the real
  window does.
- **The mock as close to the real engine as it can be.** The mock stands in only for
  Chromium and the native window; the Go app and the pages run unchanged. Known gaps: keys
  come from the mock page's buttons, not through the window's browser-or-page-first
  routing; focus requests are ignored; the mock page is buttons, not a site, and runs no
  extension pages. Contract tests, the same scenarios run against the mock and the real
  engine, are how a gap gets found and closed.

## The standard shell

- See the command box over a page in the real app (0.2.69 on): the page through the 80%
  scrim, light and dark. Seen on the mock only.
- For the owner: a pinned tab is marked only by its place, first with a rule after the pins.
  A pin glyph on the row is the next step if that's not enough; recommended to wait for his
  use of it.

## Shells, the contract and the shell menu

- **The shell menu in the other shells.** Standard's ☰ and Mobile's ⋯ draw Go's menu
  ([menus-shortcuts.md](menus-shortcuts.md#the-shells-menu)); Fox, Minimal, Terminal and Glide
  still have a Settings button or a menu of their own. Next: move Standard's `AppMenu.vue` to
  `shell/src/ui` for Fox and Terminal; a Svelte one for Minimal and Glide. Ask the owner whether
  each shell should have it (recommended: yes, it is the "everything you can do" menu).
- **A submenu's own submenu** shows as its items between lines in the shared menu (View's
  Shell). Make `ui/ContextMenu.vue` open a third level only if the owner asks.
- **Pinned tabs, later steps** ([windows.md](windows.md#pinned-tabs)): reordering pins, a pin
  that follows its tab, extensions seeing `pinned`, and a contract rule (40 to 49 are free for
  it), once more shells draw pins.
- **Rules not yet met:** rule 19 (our tab's title) by Glide and mobile-native; rule 30's
  prompt by Glide, Mobile and mobile-native; the lists in `shell/e2e/shell-contract.cjs` name who
  brings each.
- **Minimal's Svelte parts are ports** of the shared Vue ones (`PageDialog`, `PageMenu`,
  `Downloads`, `FindBar`): a change to one is made to the other, or both become shared Svelte
  parts once a second Svelte shell wants them.
- **The memory bench's GPU figure swings.** `tools/bench/memory.py` reads macOS's footprint,
  which counts the GPU process's purgeable graphics memory (about 640 MB) only when it isn't
  volatile at the sample: the same build read 821 MB and then 38 MB. Read the footprint by
  category without the reclaimable graphics memory, or take several samples, before trusting
  a GPU number.

## Small fixes

- The first real click after leaving full screen doesn't land; keys work.
- `tools/vm/check.py` times clicks by pixels, so it measures the VM's display; time inside the
  app instead.
- Quitting a dev build sometimes aborts with "A freed allocation is still referenced by a
  dangling pointer at exit" (partition_alloc_support.cc): something still holds a `raw_ptr` to a
  `BrowserContext` when `BrowserMainParts::PostMainMessageLoopRun` frees it, though the engine
  and extensions go first. Users never see it: the check exists only where
  `enable_dangling_raw_ptr_checks` is on (`out/pb`, non-official), not in the official build
  every release comes from. Finding the holder takes a build with
  `enable_backup_ref_ptr_instance_tracer = true` in an out dir of its own, a near-full rebuild.
- On Android 17 a cold start now and then shows the frame colour and no shell, the app running.
  The log of one (drive_android's GPU part on 0.2.69, emu-37) shows why: Android started the
  shell's renderer, `NativeOnlySandboxedProcessService0`, and killed it ten seconds later for
  a start timeout ("failed to attach"), and the browser heard nothing, so the shell's page
  waits for a renderer for good. It shows only deep in full runs on a loaded emulator. The fix
  is in how a launch that never attaches reaches the browser: as a dead renderer, which the
  shell's page already recovers from. Branch `shell-attach` (4a58733e, on 0.2.72) does it: the
  shell page watches each load, and one whose renderer isn't ready after 15 s is shut down and
  goes the crash path (Go hears SHELL_CRASHED, the page reloads). It builds on the Mac; it is
  not yet checked on Android. To check it on an emulator with root (`su 0`), stop the first
  sandboxed process as the app starts (`kill -STOP $(pgrep -f sandboxed_process0 | head -1)`)
  and see the shell come back about 15 s later; the stop needs no `pidof`, whose name match
  misses the zygote child's longer name.
- Closing a tab (Cmd+W) takes Go's part of the span about 3.4 ms at 10 tabs and 3.8 at 50 on
  0.2.70 (`tools/bench/tabs.py`, 2026-11-16), against 0.3 to 0.7 in 0.2.64. It isn't Go's own
  work: that is 126 µs for an open and a close on `core/perf`'s rig, where the engine does
  nothing, and it doesn't grow with the tabs open. So it is inside the engine's `CloseTab`,
  which Go calls in that span. The likely cause: since 0.2.65 each tab has its own renderer,
  which bought about 12 MiB a tab, so closing one now tears its renderer process down inside
  that call. Timeline marks around `CloseTab` would prove it; tearing the renderer down after
  `CloseTab` returns is the fix, if 3 ms ever matters. The Go side's own part, a session
  saved twice per close, is gone (1e5b1932, and 5c9f4e58 in 0.2.70).

## Terminal shell

The UX pass of 2026-11-14 (owner: "terminal stays... but it gets a good research ux pass"):
steps 1 to 4 and every contract rule are on main, described in
[shells.md](shells.md#the-terminal-shell). Left:

- **Step 5, waiting for the owner to try steps 1 to 4** (he asked that it wait). Each piece,
  in Terminal's look:
  - The prompt as a command line: `:open <url>` (`navigate`), `:tab <url>` (`newTab` with
    `text`), `:split` and `:vsplit <url>` (a split, then the address), `:ws <name>` and
    `:ws new <name>`, `:group <name>` (`editGroup` on the window's group), `:close`,
    `:only`, `:zoom`, `:run <action>` (by name from the snapshot's `actions`, as Glide's
    Command Bar does), `:b <words>` to jump to a pane by title. History in the shell's
    profile data (Up, Ctrl+R), Tab to complete. Shift+Cmd+; (reads as Cmd+:) focuses it.
  - Workspaces shown one at a time or all, all by default: a pill at the strip's left with
    the workspace's colour and name; new tabs open in the one shown.
  - Go's tab groups as coloured brackets over their chips, the group's name on the bracket.
  - Pinned tabs (Go has them now) as favicon squares after the pill, never closed by `:only`.
  - The research and mocks: `project-b-ux-audit/terminal/` beside the repo (summary,
    research, audit, mocks.html and PNGs). An opt-in, unbound `terminal.prefix` action for
    tmux users was proposed; the owner chose chords only.
- **Check in the real app:** the chips, Split down disabled under 321px, and rules 15 to 32
  have run only on the mock. The chords were checked with real keys in the VM on 0.2.64:
  none is taken by macOS.
- **Cmd+1 to 9 count tabs, the chips count windows.** Go's `tab.select` picks the Nth tab in
  the strip's order; a chip's number is its window's. Recommendation: leave it unless the
  owner finds it confusing; then Terminal's own `terminal.window1`..`9` on Ctrl+1 to 9.
- The shared page parts (dialog, find bar, menu, downloads, prompt, notice) have the design
  system's look, not Terminal's square monospace; restyling them means Terminal's own copies.

## Site features (the site panel, Links to other sites, the ad blocker, Sparks)

How they work is in [site-features.md](site-features.md).

- The site panel is in Standard, Fox and Mobile (contract rule 33); Minimal goes straight to
  settings; Terminal and Glide don't have it yet.
- The ad blocker's list (`core/adblock/hosts.txt`) is a few hundred hosts kept by hand, with no
  cosmetic rules. A published list (EasyList's hosts) is the next step, once its licence fits.
- Links to other sites, later if wanted: separate settings for a page's navigations in place
  and for its popups; a popup let open hidden and closed after about a second, for sites that
  wait on `window.opener`; a general popup blocker counting in the same `popupsBlocked`.
- Sparks' editor is a plain text area, without highlighting; no sync, no enterprise lists.
- The ten-path check of Links to other sites (a link, `target=_blank`, noopener,
  `window.open`, `location.href` and `replace`, a form, a popup then its location, a same-site
  link redirecting off-site, a popup and a location change together) was run by hand on the Mac
  and an emulator for 0.2.72. It belongs in `drive_first_app.py` and `drive_android.py` as an
  automated part; the page is in [site-features.md](site-features.md#checking-it).

## Build, release and test tooling

- The mock holds six connections per host, so four mock pages on screen hang a test
  ([shells.md](shells.md#testing-one)): serve the mock pages from a second host
  (`localhost` beside `127.0.0.1`).

- The desktop and the emulators have no lock, unlike `out/` and the Android VM. Give
  `tools/test.sh android` and the drivers one (per emulator, and one for the desktop).
- The README's screenshots show the Standard shell from before 0.2.63: retake them from a
  release run (`tools/readme-shots.py`).
- Each checkout's `build/pb/` copy costs about 455 MB once `out/pb` is rebuilt. A merged
  worktree's copy can go with the worktree.

## Daily driver

What a browser needs to be used all day ([daily-driver.md](daily-driver.md)).

- **Bookmarks, next steps.** Folders, import from another browser, and bookmarks among the
  address bar's suggestions. Today the list is flat.
- **History and Bookmarks on a phone.** The pages work in a desktop tab of ours; the Mobile
  shells don't reach them yet, nor draw a star.
- **Retry keeps the name.** A download that starts over can take the next free name
  ("big (1).bin") when an earlier attempt's file is still there; it should reuse its own.
- **For the owner:** where a phone's downloads go. Built: the shared Downloads folder, so
  other apps and the Files app see them. The other choice is the app's own storage.
- **For the owner:** suggestions fetched from the search engine as you type. Left out by the
  owner's call; if wanted, a setting, off by default, since every keystroke goes to the engine.
- **For the owner** (rows 12, 13 and 15 of [daily-use.md](daily-use.md)), in this order:
  printing, a new tab page, then saved passwords and autofill. Each is discussed with mocks
  before it is built, Standard and the Mobile web shell only.
- **Error pages in Glide and the native Mobile shell.** Rule 32 exempts both until they draw
  Go's notice.
