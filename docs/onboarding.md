# The first-run onboarding

What a new user sees on the first start, and how they bring their things over from another
browser. Item 13 of the [UX comparison](ux-comparison.md#what-to-do-ranked). Mocks approved
2026-11-17. Slice 1 went out in 0.2.78 and slice 2 in 0.2.80; **slice 3, the importers, is
built** (see [Built](#built)).

The owner, 2026-11-17: "if we are missing features then we add those but the onboarding needs to
be nice and thats not shell its our internal page". So it is a page of ours in a tab, as Settings
is: the app `shell/src/apps/welcome`, at `chrome://welcome/`. Every shell shows it the same way,
and it opens once, on the first start. Every step can be skipped, and everything it sets is in
Settings later.

The owner on the first mocks, 2026-11-17: "nice mocks... the only thing is not to choose a default
color i donot like the current green thing... also first time the welcome should come as full app
screen... with a nice splash screen and stuff". So:

- **No colour is chosen for the user.** The welcome and a new profile are neutral (a light grey
  frame, ink for the accent) until a colour is picked; "None" is the first swatch and the one
  selected. The dusty teal default (`#4f9e9e`) goes from the profile templates and the presets.
  Built in slice 1: `frameColor` `""` is None (`tones.None`), a grey frame for light and one
  for dark, which the window paints by its appearance, and no dot on the Dock icon.
- **The welcome takes the whole window,** with no browser UI around it: a splash (the mark draws
  itself, the name fades in), then the welcome, with a window behind the words that changes colour
  as it waits. The browser's UI comes in around the page at Start browsing. The phone's welcome
  takes the whole screen in the same way.

And on the second mocks, 2026-11-17:

- **Choosing a colour or the theme changes the page and its window at once,** the frame too.
  The mock page does it: open `mocks.html?m=d3` and press the swatches, the theme and the tabs.
- **The same page opens again later in a tab** of ours, inside the browser, from Settings' Run
  setup again ([mock](screenshots/onboarding/mock-d7.png)).
- **No Safe Browsing from Google.** "no safebtowsing should not use google... we should have
  discussed the implementation". It is out of the onboarding; nothing in the app does Safe
  Browsing today, and how it would work is for a discussion first.
- **Approved:** no "How you work" step, five steps on the desktop; the style step offers tabs on
  the side, on top, and Glide.
- **A flag to show the welcome again:** "we should expose a flag that changes it to false so that
  we can test the welcome full screen". Go keeps whether the welcome was shown; a flag in
  Settings, Flags, "Show the welcome at the next start", clears it, and the next start opens the
  welcome over the whole window as a first start does.
- **The browsers' own logos** on the import rows, for easy recognition. The app reads each
  browser's icon from its installed app (`NSWorkspace`'s `iconForFile:`), so the logo is the
  browser's own and nothing of theirs ships with us. The mock draws stand-ins.
- **Reading another browser's data is a module of its own,** "so that we can use it as a
  separate project too if required": see [The importer](#the-importer-a-module-of-its-own).
- **Sync** is a discussion of its own, in its own session ("[project-b] sync"): many ways to
  sync, to places the user controls.

## The pages

The mocks are drawn in [screenshots/onboarding/mocks.html](screenshots/onboarding/mocks.html)
(`?m=d0` plays the splash into the welcome, `?m=d3` is the style step working; `d0s` to `d7`,
`p0` to `p2` are the stills); the splash as a [video](screenshots/onboarding/splash.webm).

**Desktop**

1. **Welcome:** the splash ([mock](screenshots/onboarding/mock-d0s.png)), then the mark, the
   name, one line, Get started and Skip setup over a window that cycles through the colours
   ([mock](screenshots/onboarding/mock-d1.png)).
2. **Bring your things over:** each browser and profile found on the Mac, a row with what it
   holds. Turned on, the row opens to what to bring: bookmarks (with their folders), history,
   passwords, open tabs, the search engine, extensions ([mock](screenshots/onboarding/mock-d2.png)).
   Safari's row asks for Full Disk Access once ([mock](screenshots/onboarding/mock-d2b.png)): the
   page opens that pane of System Settings and goes on by itself when access is given. Then
   what came over ([mock](screenshots/onboarding/mock-d2c.png)). Open tabs come over as a
   workspace named after the browser.
3. **Pick your style:** a picture of the browser beside the choices: tabs on the side, on top,
   or Glide; the colour, None first; theme; keys like Chrome's or Project B's. The picture and
   the page itself, with its window's frame, take each choice at once
   ([none](screenshots/onboarding/mock-d3.png), [blue](screenshots/onboarding/mock-d3b.png),
   [dark](screenshots/onboarding/mock-d3c.png)).
4. **Protection:** the ad blocker, Links to other sites, the search engine, automatic updates
   ([mock](screenshots/onboarding/mock-d4.png)).
5. **You're set:** three things worth knowing (the command box, the site panel, workspaces),
   Make default browser, which macOS confirms with its own question, and Start browsing, which
   turns the Welcome tab into a new tab ([mock](screenshots/onboarding/mock-d5.png)).

**Phone** ([splash](screenshots/onboarding/mock-p0.png), [mocks](screenshots/onboarding/mock-p1.png), [more](screenshots/onboarding/mock-p2.png)).
No import: Android keeps each app's data from the others.

1. Welcome.
2. Make it yours: the address bar at the foot or on top (`addressBarPosition`), and the colour,
   None first.
3. The search engine and the ad blocker.
4. Default browser, through Android's own sheet (`RoleManager`, `ROLE_BROWSER`).
5. Download notifications asked here, with a reason, instead of in the middle of the first
   download as today (`Downloads.java`).
6. Start browsing.

**Settings** gets an Import section, the same rows with Import… on each, and Run setup again,
which opens the welcome in a tab ([mock](screenshots/onboarding/mock-d6.png),
[in a tab](screenshots/onboarding/mock-d7.png)). Run setup again became Show the welcome again, in Flags, which
closes the app for the next start to open the welcome whole (see Built).

## What it needs that we don't have

| Feature | Where | Slice |
|---|---|---|
| A first-run marker, the Welcome page, Show the welcome again, and the flag that shows it at the next start | Go (`core/app`, `core/flags`), `shell/src/apps/welcome`, settings | 1 |
| The welcome over the whole window, the splash, the UI coming in at the end | the window's layout gives the welcome tab all of it, with no shell regions, until Start browsing; on the phone, the Mobile shells hide their bar for it | 1 |
| No colour until one is picked | a neutral frame in `core/tones` (light grey, ink accent, and its dark twin), the templates' and presets' teal gone | 1 |
| Make default browser | Mac: the app can't be a browser yet. Its Info.plist (`tools/package.sh`) declares no `CFBundleURLTypes` for http and https nor `CFBundleDocumentTypes` for html, and nothing in the engine takes a link macOS hands over (`application:openURLs:`), as Android's `PBOpenURL` does. Then `NSWorkspace`'s `setDefaultApplicationAtURL:toOpenURLsWithScheme:`, which asks the user itself. Android: the manifest already takes http and https links, so `RoleManager`'s `ROLE_BROWSER` request is all | 1 |
| Download notifications asked on the phone's step | an engine call that asks for `POST_NOTIFICATIONS` and reports whether it was given. Today `Downloads.java` asks the moment a download starts, so Android's prompt covers the page; that ask goes, or stays only for a user who skipped setup | 1 |
| A preview of each layout in the chosen colour and theme | drawn by the welcome page from the tones Go sends, so it follows every choice; no pictures to keep up | 1 |
| Bookmark folders, built ([bookmark-folders.md](bookmark-folders.md)) | `core/bookmarks`, the Bookmarks page, the star's panel | 2 |
| Importers: the Chromium family (Chrome, Edge, Brave, Arc), Firefox, with each browser's own icon: bookmarks, history and the search engine, built | `browserdata`, a module of its own (below); the welcome's Bring your things over; Settings' Import | 3 |
| History import, built | `core/history` | 3 |
| Open tabs, as a workspace named after the browser | `browserdata`: Chromium's session files, Arc's sidebar, Firefox's session store | not yet given one |
| Safari, behind Full Disk Access | the same package | 4 |
| Passwords | the store is the daily driver session's (`core/passwords`, `Import([]Login)`); the importers hand logins to it. Chromium's are sealed with the Keychain's "<Browser> Safe Storage" key, which macOS asks about once per browser; Firefox's with `key4.db` | 5 |
| Extensions reinstalled by id | the Web Store install path, while their flag is on | 5 |

Each slice goes out through a release of its own.

## Built

**Slice 1, the welcome itself** (branch `onboarding`):

- **None, no colour,** is where every profile starts: `frameColor` `""`, `tones.None`. Its
  palette is a light grey frame, or a dark grey in dark, with ink for the accent (white text
  on it) in light and the light text (ink on it) in dark. `WindowLayout` carries `FrameColor`
  and `FrameDark`, the same for any colour picked, and the window paints the one for its
  appearance: on the Mac by `effectiveAppearance`, repainting when it changes; on Android by
  the theme and the phone's night mode. The Dock icon has no dot. Settings' swatches and the
  welcome's share the presets, `shell/src/ui/colours.js`, None first, struck through.
- **The page,** `shell/src/apps/welcome`, at `chrome://welcome/`, one of `tabs.Pages`, with
  Go's side in `core/app/welcome.go`. Desktop: Welcome, Pick your style, Protection, You're
  set; Bring things over joins in slice 3 with the importers. Every choice is a pref, set as it is made (`setPref`, `setSearchEngine`,
  `setAutoUpdate`), and comes back as a change: the page repaints from the tones Go sends,
  the window's frame with it, and the picture of the browser (`Preview.vue`) is drawn in
  those tones, so nothing is a picture to keep up. Skip setup and Start browsing both
  finish it.
- **When it opens:** at the start that made the profile (`app.Config.Made`, the first start
  with no profile list), again at a start after one that left it unfinished, and at every
  start while the flag **Show the welcome at each start** is on. `welcome.json` in the
  profile says `{"done": false}` while it shows and `{"done": true, "at": ...}` once finished.
  A folder named with `--user-data-dir` was made by no start, so the drivers' profiles show
  none; their roots made fresh with `--root` do.
- **Over the whole window:** Go tells every shell which tab is the welcome with the window
  to itself (`welcome` in the shell's state). While it is the active tab, Standard hides its
  bars and keeps the frame's band, with a top edge, and a strip across the top to drag the
  window by; Glide shows the page alone with that strip; the Mobile web shell hides its bars
  as for a page in full screen, keeping the system's. Finishing closes the tab, the shell
  comes back, and a shell that opens a tab at the start gets one.
- **The splash** plays once over the whole window: the mark grows and draws itself, the name
  fades in, and the welcome comes; less motion skips it. The hello's window rises from the
  page's foot, its frame turning through the presets as it waits.
- **Protection** offers the search engine and automatic updates, and shows the ad blocker on.
  Not here yet: a switch for the ad blocker and for Links to other sites across every site
  (today each is a site's own setting, the ad blocker on by default, links allowed), and the
  keys like Chrome's or Project B's (Settings, Keyboard, has it).
- **Show the welcome again,** first in Settings' Flags (on a phone, below General), marks
  the welcome unfinished in `welcome.json` and closes the app. The next start opens it over the
  whole window, as a first start does, with the tabs On start, open brings back under it. It
  doesn't open in a tab: the welcome is the whole-window first run, and a tab showed less of it.
- **Make default browser,** on the last step. Go asks the engine (`AskDefaultBrowser`), and
  the system asks the person: on the Mac `NSWorkspace`'s `setDefaultApplicationAtURL:`
  for http then https, on Android `RoleManager`'s `ROLE_BROWSER` sheet. The answer comes
  back as `PB_EVENT_DEFAULT_BROWSER`, and the row says so. The Mac app declares itself a
  browser in its Info.plist (`tools/package.sh`: http and https, html and xhtml), and a
  delegate takes the links and pages macOS hands it to Go (`PBGoCalls.open_urls`), which
  opens them as a second copy's are. Checked on the mock; on a real Mac only in the VM, as
  pressing it changes the Mac's own default browser.
- **Download notifications,** on a phone's last step: `AskNotifications` asks Android for
  `POST_NOTIFICATIONS`, and `PB_EVENT_NOTIFICATIONS` brings the answer back. The ask at the
  first download stays, for a person who skipped setup; once allowed here it never comes.
- **Checked on the real Mac app,** 2026-11-17, on a fresh `--root`: the welcome over the whole
  window, the grey frame, Blue and Dark repainting the page and the window's own border at
  once, None in dark painting the dark grey (`#3a3f4a`), and Start browsing giving the
  window back with its command box and noting `{"done": true}`.
- **Make default browser checked in a VM clone** (pb-onboard, 2026-11-17): macOS's own
  question, Use "Project B", the row saying so, then `open https://example.com/` from the
  guest opening a tab in the running app, and with the app quit, starting it on the link.
- **Checked on Android 14** (emulator-5600, 2026-11-17), after `pm clear`: the welcome over the
  screen (the first build packed no welcome page: `engine/android/BUILD.gn` lists each page),
  the grey status bar, Red repainting the status bar and the page, the address bar on top,
  RoleManager's sheet setting Project B as default, Android's notifications question, and
  the Mobile shell back in red with its bar on top.

**Slice 3, bringing things over** (branch `importers`):

- **Bring your things over,** the welcome's second step on the desktop, lists each profile
  of another browser on the Mac, with the browser's own logo and what it holds: "4
  bookmarks · 3 pages · Bing for search". A row turned on opens to what it brings, each
  ticked: Bookmarks, History, Search engine. Bring over sends each row, and the step
  becomes Brought over, a line for each thing as it lands ("4 bookmarks, in 5 folders,
  under From Chrome · Personal"); Continue goes on. With no row on, the way on is Continue.
  While Go looks the step says so; when it finds nothing the step is left out. A phone
  has no such step. ([shot](../build/e2e/welcome-bring.png) after `shell/e2e/imports.cjs`)
- **Settings' Import** (after Keyboard) lists the same rows, each saying what it holds or
  what it last brought over ("Brought over today: 4 bookmarks, 3 pages, Bing"), with
  Import…, which asks what to bring in a dialog, all ticked at first. Go looks again each
  time the section opens.
- **Bookmarks** come over in a folder at the end of the top, "From Chrome", or "From Chrome
  · Work" for a browser with more than one profile, holding the browser's own top folders
  (Bookmarks bar, Other bookmarks; Firefox's toolbar and menu) and all in them, in their
  order, each with when it was added. An address bookmarked already is left out, as an
  address is bookmarked once, and a folder left with nothing isn't made; made again, an
  import goes into the folders it made and adds only what is new
  (`bookmarks.Manager.Import`).
- **History** joins as pages: an address here already keeps the most visits of the two and
  the later visit, so an import made again counts nothing twice (`history.Manager.Import`).
  Only web pages, as history keeps.
- **The search engine** is taken when it is one of ours, matched by the host of its
  address (`search.ByAddress`); a browser that never chose one has its own: Google for
  Chrome, Arc and Firefox, Bing for Edge, Brave Search for Brave. One we don't have says
  so on its line.
- **What came** is kept in `imports.json` in the profile, by profile (`chrome/Default`), with
  when, for the row's line. Go reads on a worker and adds on the main thread, and says so
  in the timeline (`imports:`).
- **Messages:** a page sends `findBrowsers` and `import {key, what}`; Go sends `browsers`
  (null in a snapshot until a look ends), `importing` (the keys busy) and `imported {key,
  imported}`. The welcome's and the settings page's both (`core/app/imports.go`).
- **Testing it:** the mock's `-browsers <home>` names the home folder to look in, and the
  e2e library gives every start an empty one unless a test names browserdata's
  `testdata/home`, which `browserdata/testdata/make.sh` makes in the browsers' own
  formats. `core/app` (`TestBringingThingsOverFromAnotherBrowser`), `core/bookmarks`,
  `core/history` and `shell/e2e/imports.cjs`.
- **Checked in the real app** in a VM clone (pb-onboard, 2026-11-17), with the test home's
  profiles copied into the guest's home: on a fresh `--root`, Get started led to the three
  rows, each with the icon macOS gave its stand-in app, and Bring over brought Chrome ·
  Personal's 4 bookmarks in 5 folders, 3 pages and Bing, as `imports.json` and `prefs.json`
  said. No real Chrome or Firefox was on hand, so their own files and logos are untried.
- **Not yet:** open tabs, passwords (slice 5), extensions (slice 5) and Safari (slice 4).

## The importer, a module of its own

Reading another browser's data is the Go module `browserdata/` at the repo's root, with its
own `go.mod`, its own tests and a README, and no import of anything in `core`: it could move to
a repo of its own as it is. `core` uses it through a `replace` line in its `go.mod`. It only
reads; what to do with what it finds is the app's.

- **What it finds:** `Find()` lists the browsers installed and each one's profiles, with the
  folder, the profile's name, the icon's path and counts. Chrome, Edge, Brave and Arc share
  Chromium's files; Firefox has its own; Safari's need Full Disk Access, and `Find` says so
  instead of failing.
- **What it reads,** in plain types of its own: the bookmark tree with folders, history visits
  with their times, open and pinned tabs (Arc's Spaces as groups), the default search engine,
  extension ids, and saved logins.
- **Logins:** Chromium's are sealed with the Keychain's "<Browser> Safe Storage" key, which
  macOS asks the user about once per browser; Firefox's with `key4.db`, opened with Go's own
  crypto. Safari keeps its passwords in iCloud Keychain, which no other app can read, so
  Safari's row brings bookmarks and history only.
- **SQLite** (Chromium's and Firefox's history, Safari's) through macOS's own `libsqlite3`
  over cgo: nothing fetched at build time, no third-party code. Each database is copied first,
  as the browser may have it open.
- **A command** `cmd/browserdata` prints what `Find` sees and dumps one profile as JSON, for
  checking an importer against a real browser: `go -C browserdata run ./cmd/browserdata`,
  `-dump chrome/Default`, `-icon out.png chrome`.

Built in slice 3: `Find(home)`, a profile's `Bookmarks`, `History`, `Search` and `Counts`,
and `Icon(app, px)`, which draws `NSWorkspace`'s `iconForFile:` as a PNG. Each database is
read from a copy with its write-ahead log; Firefox's `search.json.mozlz4` is opened by a
small LZ4 reader of its own. SQLite and the icon need macOS (`*_mac.go`, cgo); elsewhere
they say `ErrNotOnThisSystem`, so the Android build compiles it and never calls it. Its
README says the rest.

How it builds into the app: `core/go.mod` replaces the module with `../browserdata`; the
Mac binary links `sqlite3` and AppKit for it (`engine/BUILD.gn`, `go_core_lib`), as a
c-archive leaves cgo's libraries to the binary; `engine/build_go.py` and the Android one
list its files for ninja; the Android build copies it in with `core`
(`tools/android-release.sh`, `tools/android-content-build.sh`); `tools/test.sh mock` runs
its tests.
