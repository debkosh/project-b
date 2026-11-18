# The first-run onboarding

What a new user sees on the first start, and how they bring their things over from another
browser. Item 13 of the [UX comparison](ux-comparison.md#what-to-do-ranked). Mocks approved
2026-11-17. Slice 1 went out in 0.2.78, slice 2 in 0.2.80, slice 3 in 0.2.81 and slice 4,
Safari, in 0.2.82; **extensions, the browsers' logos and the counts of what came are built**
(see [Built](#built)). How import works, to read aloud: [import.md](import.md).

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
  the side, on top, and Glide. Changed 2026-11-18, the owner: "it should show the stuff we have
  built so shell and keyboard stays". So the style step shows the shells as they look, and How
  you work is back as the fourth of six: starter workspaces, Personal and Work ticked, which
  the welcome makes, and the keys, "since we sonot differe from chrome.. make the other keymap
  firefox... but keep project b as one": Project B's (which are Chrome's) or Like Firefox,
  with the chosen keymap's keys shown under the choice
  ([style](screenshots/onboarding/mock-d3s.png), [how you work](screenshots/onboarding/mock-d4w.png)).
- **A flag to show the welcome again:** "we should expose a flag that changes it to false so that
  we can test the welcome full screen". Go keeps whether the welcome was shown; a flag in
  Settings, Flags, "Show the welcome at the next start", clears it, and the next start opens the
  welcome over the whole window as a first start does.
- **The browsers' own logos** on the import rows, for easy recognition. The owner,
  2026-11-17: "jist default chrome firefox logos.. not profile specific", then yes to
  shipping them; 2026-11-18, "ship all the imports" with Edge's, Brave's and Arc's too, and
  "remove the machinery for installed app icons... just ship the icons". So all six ship
  with us (`shell/src/ui/logos`), the same for every profile, and nothing reads an
  installed app's icon. Edge's (MIT), Brave's (MPL 2) and Arc's (public domain) are the
  SVGs Wikimedia Commons keeps; Safari's, which Commons doesn't have, is the app's own icon
  at 128 pixels.
- **Reading another browser's data is a module of its own,** "so that we can use it as a
  separate project too if required": see [The importer](#the-importer-a-module-of-its-own).
- **The search engine doesn't come over** (the owner, 2026-11-18: "remove the search engine
  machinery altogether we donot thinga that mich of an inport... anywasy we show the search
  engine as wlakthrough"). The welcome's Protection step offers it; nothing reads another
  browser's choice, so the readers of Chromium's Preferences, Firefox's
  `search.json.mozlz4` (and its LZ4 reader) and macOS's global preferences are gone.
- **Import is a wizard, with nothing kept** (the owner, 2026-11-18: "the import page needs
  to be like a simple wizrads.. we donot maintain the import state... click chrome when we
  show the profile list... which would have counts and stuff then import... show importing by
  checking items one after other as they are imported then finished"). Which browser, then
  which profile and what to bring, then each kind ticked as it lands, then what came
  ([mock](screenshots/onboarding/mock-i1.png)). One profile at a time, Import another for the
  next; the same wizard in the welcome and in Settings (his picks). imports.json is gone.
- **Forks are a line of data each,** with their own logo, and **another browser is a folder
  the person chooses** ("we should have chrome forks and firefox forks... which asks the use to
  select a folder and then try from there"; "if our backend code is not changing and we are
  just storing name icon and default folder in shell we can have many builtin options"; ship
  each fork's logo; the folder in the welcome and in Settings).
- **Passwords come over** (the owner, 2026-11-18): read directly from Chrome's family and
  Firefox's, "and password import as a second option under imports settings... so we have
  improt from browsers and imports from file.. like exported bookmarks password etc"; a
  Firefox primary password is asked for. The store is the passwords session's: the API was
  agreed with it (`Store.Import([]Login)` with Origin, Username, Password, Created and
  LastUsed, and Added, AlreadySaved and Skipped back); the importer makes no store of its own.
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
2. **Bring your things over:** the import wizard ([mock](screenshots/onboarding/mock-i1.png)):
   each browser found on the Mac, forks among them, and another browser's folder to choose;
   then a browser's profiles, each with what it holds, and what to bring: bookmarks (with
   their folders), history, extensions; then each ticked as it lands. Earlier, rows with
   switches ([mock](screenshots/onboarding/mock-d2.png)).
   Safari's row asks for Full Disk Access once ([mock](screenshots/onboarding/mock-d2b.png)): the
   page opens that pane of System Settings and goes on by itself when access is given. Then
   what came over ([mock](screenshots/onboarding/mock-d2c.png)). Open tabs come over as a
   workspace named after the browser.
3. **Pick your style:** a picture of the browser beside the choices: tabs on the side, on top,
   or Glide, each drawn as that shell draws itself; the colour, None first; theme. The picture and
   the page itself, with its window's frame, take each choice at once
   ([none](screenshots/onboarding/mock-d3.png), [blue](screenshots/onboarding/mock-d3b.png),
   [dark](screenshots/onboarding/mock-d3c.png)).
4. **How you work:** the workspaces to start with, as the workspace switcher shows them:
   Personal and Work ticked, Reading not, and Add one…; Continue makes those ticked. Then the
   keys, Project B or Like Firefox, with a few of the chosen keymap's under the choice
   ([mock](screenshots/onboarding/mock-d4w.png)). The phone has no such step.
5. **Protection:** the ad blocker, Links to other sites, the search engine, automatic updates
   ([mock](screenshots/onboarding/mock-d4.png)).
6. **You're set:** three things worth knowing (the command box, the site panel, workspaces),
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
closes the app for the next start to open the welcome whole (see Built); since the settings
page's layouts ([settings.md](settings.md)) it is Run setup again once more, under Your data.

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
| Importers: the Chromium family (Chrome, Edge, Brave, Arc), Firefox, with each browser's logo: bookmarks and history, built | `browserdata`, a module of its own (below); the welcome's Bring your things over; Settings' Import | 3 |
| History import, built | `core/history` | 3 |
| Open tabs, as a workspace named after the browser | `browserdata`: Chromium's session files, Arc's sidebar, Firefox's session store | not yet given one |
| Safari, behind Full Disk Access: bookmarks and history, built | `browserdata` (`safari.go`, `plist.go`); the welcome's and Settings' Allow access | 4 |
| Passwords, read, built; they come over once the passwords store is there | `browserdata` (`chromium_logins.go`, `firefox_logins.go`, `keychain_mac.go`); the store is the passwords session's (`core/passwords`, `Store.Import`) | 5 |
| Files a person exported, built: bookmarks (HTML) now, passwords (CSV) with the store | `browserdata` (`files.go`); Settings' Import | 5 |
| Extensions installed again by id, in Settings' Import while their flag is on, built | `browserdata` (`extensions.go`); the engine's `InstallExtensionFromStore`; the extension manager's queue | 5 |

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
  (Settings has Block ads for every site since 2026-11-18; links are each a site's own
  setting, allowed), and the keys like Chrome's or Project B's (Settings' Keys has it).
- **Run setup again,** in Settings' Your data (Profile and import in the sections layout), marks
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
  of another browser on the Mac, with the browser's logo and what it holds: "4
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
- **The search engine** (removed 2026-11-18, above) was taken when it was one of ours, matched by the host of its
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
- **Not yet:** open tabs and passwords (slice 5). Safari came in slice 4, extensions after.

**Slice 4, Safari** (branch `safari`):

- **Safari's row** is in the list on every Mac whose user has opened Safari, as
  `~/Library/Safari` is there; macOS keeps what is in it behind Full Disk Access, so until
  the app has that the row is locked: "Needs Full Disk Access, in System Settings", nothing
  counted. `Profile.Locked` tries to list the folder; macOS refuses with "operation not
  permitted".
- **Turned on in the welcome** ([shot](../build/e2e/welcome-allow.png)), the row opens
  System Settings at Privacy & Security › Full Disk Access and the step shows how, as the
  approved mock: a picture of the switch, the steps, and "Waiting for access…" with Open
  System Settings to open it again. Go tries Safari's folder each second from then, for ten
  minutes, and looks again once it can be read: the step comes back to the list with
  Safari's row on and its choices ticked. Back and Not now go back to the list, the row off.
- **Settings' Import** has Allow access… on Safari's row in place of Import…; its dialog
  shows the same, and turns into Import from Safari's choices once the access is given
  ([shot](../build/e2e/settings-allow.png)).
- **What macOS does,** found in a VM clone with SIP on (pb-safari, 2026-11-17): an app that
  has tried Safari's files is listed in Full Disk Access by itself, switched off, so "Turn
  on Project B" is all there is to find. Turning it on asks for the Mac's password, then
  macOS says the app "will not have full disk access until it is quit", with Quit & Reopen
  and Later. It has it at once: the running app read Safari's files about eight seconds
  after the switch, without a restart. So the steps say to choose Later, and the page goes
  on by itself. A VM with SIP off doesn't enforce Full Disk Access at all.
- **Bookmarks** come from `Bookmarks.plist`, a binary property list, which `plist.go` reads
  (Go's standard library has no reader for one): Favorites (`BookmarksBar`), Bookmarks Menu
  and the Reading List, each a folder under From Safari, as Safari names them; its History
  proxy isn't a bookmark. Safari's own starting Favorites (Apple, iCloud, Yahoo and the
  rest) come with them, as they are bookmarks like any other.
- **History** from `History.db`: each page's visit count, its last visit, and the title of
  its last visit that had one. Times are seconds since 2001.
- **The search engine** (removed 2026-11-18, above) was macOS's own choice, `NSPreferredWebServices` in the global
  preferences, which Safari has used since it moved its choice there; Google until one is
  picked. Its passwords are in iCloud Keychain, which no other app can read.
- **Messages:** a page sends `openAccess`; Go opens System Settings
  (`engine.OpenFullDiskAccess`, `open` with the pane's `x-apple.systempreferences` address)
  and starts trying. A row from Go has `locked` while it is.
- **Testing it:** the test home has a Safari (`make.sh` writes its files with Python's
  `plistlib` and Safari's own tables); a folder with no permissions stands in for macOS's
  refusal. `browserdata` (`TestSafariIsLockedUntilItsFolderCanBeRead`, `TestBplist`),
  `core/app` (`TestSafariWaitsForFullDiskAccess`) and `shell/e2e/imports.cjs`, which locks
  and unlocks its copy of the home. The reader was also run on this Mac's own Safari
  history and on a fresh VM's bookmarks, and over every binary property list in
  `~/Library/Preferences` (284, all read).
- **Checked in the real app** in pb-safari with SIP on, on a fresh `--root`: Get started
  showed Safari's row locked; turning it on opened System Settings at Full Disk Access with
  Project B listed, off; turning that on (the password, then Later) brought the welcome
  back to the list by itself, Safari's row on with 13 bookmarks, 1 page and Google; Bring
  over brought 13 bookmarks in 2 folders under From Safari, the page and Google, as
  `imports.json` said.

**Passwords, and files a person exported** (branch `password-import`):

- **A Chromium browser's passwords** are in its profile's Login Data and Login Data For
  Account (those saved to the person's account), SQLite: "v10" then AES-128-CBC, the key
  PBKDF2-SHA1 of a random secret in the login keychain, "<name> Safe Storage", with the salt
  "saltysalt" and 1003 rounds, the IV sixteen spaces, as Firefox's own importer has it
  (`ChromeMacOSLoginCrypto.sys.mjs`). Each known browser's keychain names are a line of the
  table: "Chrome", "Microsoft Edge", "Brave Browser", "Opera" (Opera GX's too), "Vivaldi",
  and so on. The importer reads the secret through Apple's Security framework from the
  app itself (`KeychainSecret`), so macOS's question names Project B: Allow, Always Allow
  or Deny, and the Mac's password. "Never save" entries, federated ones and HTTP auth ones
  aren't brought.
- **A Firefox profile's** are in logins.json, each value sealed by NSS with the key in
  key4.db, which is sealed under the primary password, "" when none is set: PBES2 (PBKDF2
  with HMAC-SHA256 of SHA1(global salt + password), then AES-256-CBC), or the older
  SHA1-and-3DES; the logins are 3DES-CBC, or AES-256-CBC in newer ones. Go's own crypto,
  no NSS. A profile with a primary password says so, and the wizard asks for it under the
  choices; it goes with the import, is used once, and is kept nowhere. Wrong, the line
  says so.
- **Safari's** are in iCloud Keychain, which no other app can read: they come as a file.
- **Counts:** a profile's passwords are counted without unlocking anything, from the rows
  and logins.json; the row says "· 36 passwords" and Passwords is a choice, ticked.
- **Into the store:** the importer hands what it read, in memory only, to the passwords
  store through `PasswordStore` (`App.UsePasswordStore`), which says how many it added, had
  already (a login for the same origin and username is kept as it is) and left out. Until
  the store is there, nothing names passwords. The mock's `-password-store` gives a store
  that only counts, and `-keychain` a test secret, for the tests.
- **From a file,** in Settings' Import, under the browsers: Bookmarks file, the HTML every
  browser exports (Netscape's bookmark file), into a folder named after the file; and,
  with the store, Passwords file, the CSV of Chrome, Edge, Brave, Firefox or the Passwords
  app (Safari's), its columns found by name. The page reads the file the person picks and
  hands Go what it holds; nothing else sees it. After a passwords file the wizard says to
  delete it, as it holds them in plain text.
- **Testing it:** `browserdata/testdata/logins.py` seals the test home's passwords as each
  browser does (Python's hashlib and openssl): Chrome · Personal's under "test-safe-storage",
  Firefox's with no primary password, Zen's under "letmein". `browserdata`
  (`TestLogins`, `TestSHA1And3DESKey`, `TestReadExportedFiles`), `core/app`
  (`TestPasswordsComeOver`) and `shell/e2e/imports.cjs`. The keychain itself is untried:
  this Mac has no Chrome with saved passwords.

**Import as a wizard, forks, and a folder chosen** (branch `import-wizard`):

- **The wizard** (`shell/src/ui/ImportWizard.vue`) is the welcome's Bring your things over
  and Settings' Import. Which browser: each browser Go found, with its logo and its
  profiles' count, or what its one profile holds, and Another browser, which asks for its
  profiles folder. A browser shows its profiles, the first picked, each with what it holds,
  and what to bring, ticked but extensions; Choose a different folder… is there too. Import
  shows each kind, waiting, coming (a spinner; extensions say how many of how many), then
  ticked with what came; then Import another. Safari asks for Full Disk Access first, and
  shows its profile once given. The welcome's step is always there on the desktop, as a
  folder can be chosen with no browser found; its Continue goes on at any time.
  ([welcome](../build/e2e/welcome-bring.png), [brought](../build/e2e/welcome-brought.png),
  [Settings, a folder chosen](../build/e2e/settings-import.png))
- **Nothing is kept.** No imports.json, no "brought over today": the wizard shows what an
  import brought while it is on screen. Bookmarks say what is there already, and a second
  import adds only what is new, as before.
- **Messages:** a page sends `findBrowsers`, `chooseFolder`, `openAccess` and `import {key,
  what}`; Go sends `browsers` (each `{id, name, locked, profiles: [{key, name, counts}]}`,
  a profile's key its folder), `folder` (`{browser}`, `{error}` or `{cancelled}`),
  `importStep {key, step, state, result, error}` as each kind starts and ends, and
  `imported {key}` at the end.
- **Forks** are lines of `browserdata`'s table, each an id, a name and the folder its
  profiles are in under Application Support, and the family whose files it keeps: Chrome
  Beta and Canary, Edge Beta and Dev, Chromium, Vivaldi, Opera, Opera GX and Yandex
  Browser, which keep Chromium's; Zen, LibreWolf, Waterfox and Floorp, which keep Firefox's.
  The Chromium ones' folders are those Firefox's own importer uses
  (`ChromeMigrationUtils.sys.mjs`); the Firefox forks' were taken from their own docs and
  are untried. Opera can keep its one profile in its folder itself, which is read as a
  profile with no name.
- **Their logos** ship in `shell/src/ui/logos`, one file named by the browser's id, with a
  README of where each is from and its licence; a browser with none shows its family's
  (`chrome-beta`, `chrome`'s), and one no one named, a plain mark.
- **A folder chosen** goes through a new engine call, `ChooseFolder` (the system's open
  panel, as sheet on the window, showing Library; `choose_folder` and
  `PB_EVENT_FOLDER_CHOSEN` in the ABI), whose panel the extensions' Load unpacked also uses
  now (`engine/browser/choose_path_mac.mm`); the driver's `choose` answers either.
  `browserdata.Open` reads the folder: a Chromium browser's folder of profiles or one
  profile, or a Firefox one's (`profiles.ini`) or one profile (`places.sqlite`). A known
  browser's folder is that browser; another is `other-chromium` or `other-firefox`, named
  after its folder ("User Data" names the folder above). A folder of neither says so.
- **Testing it:** the test home has an Opera of one profile, a Zen, and a Thorium outside
  Application Support. `browserdata` (`TestOpenAFolderChosen`, `TestFind…`), `core/app`
  (`TestBringingThingsOverFromAnotherBrowser`, `TestAFolderChosenIsABrowser`,
  `TestSafariWaitsForFullDiskAccess`, and with extensions `TestExtensionsComeOverFromTheStore`),
  `shell/e2e/imports.cjs`, and `tools/drive_extensions.py import` on the real app.

**How you work, and the shells as they look** (branch `welcome-how-you-work`):

- **Pick your style** draws each shell as it draws itself (`Preview.vue`): Standard's address
  bar and buttons, New Tab with ⌘T, tabs with their titles and the workspace switcher, on the
  side or along the top; Glide's address, its Space with its dot, its tabs and its dots at the
  foot; a page. On a 640 by 400 grid scaled by the picture's width, so a choice's card is the
  same picture as the large one, in the page's tones as before.
- **How you work** ([shot](../build/e2e/welcome-work.png)) lists Personal ("Everyday
  browsing"), Work and Reading, each with its colour, Personal and Work ticked, and Add one…
  for a name of the person's own. Beside them, the standard shell's workspace menu with the
  ticked ones. Continue sends `setWorkspaces`: the first ticked takes Main's place, renamed
  and recoloured, while Main is as a new profile has it, and the rest are made. Back and
  Continue again undo what is no longer ticked: one the step made goes, and Main gets its
  name back when none is ticked. Skip setup makes none.
- **The keys:** Project B or Like Firefox, `useKeymap`, which makes `keymap.json` follow
  that template, as Settings' Keyboard does. Under it, eight of the chosen keymap's keys:
  New tab, Address, Reopen tab, Next tab, History, Downloads, Bookmarks, Page source. Go
  sends both keymaps' keys in the snapshot (`commands.TemplateKeys`).
- **Firefox's keymap** (`templates/keymap/firefox.json`) is Project B's with Firefox's Mac
  keys where they differ, from Firefox's own `browser-sets.inc.xhtml`: Downloads ⌘J (ours
  ⇧⌘J); Bookmarks ⇧⌘O, its Library, and ⌘B, its sidebar (ours ⌥⌘B); Page source ⌘U as
  well as ⌥⌘U; History ⌘Y and ⇧⌘H, its sidebar; and Ctrl+Tab goes to the next tab in order,
  as Firefox's does, not to the one used last (`tab.next`, and `tab.recentNext` has none).
  Windows and Linux keep Project B's.
- **Testing it:** `core/app` (`TestHowYouWorkMakesTheWorkspacesAndChoosesTheKeys`) and
  `shell/e2e/welcome.cjs`.

**Extensions, the logos, and what came** (branch `extensions-import`). The owner, 2026-11-18:
"ship all the imports... the import should cleanly show how many it imported". Extensions
come over in Settings' Import alone, while the Extensions flag is on, and with no prompt for
each (the owner chose both, 2026-11-18).

- **What a profile has:** `browserdata`'s `Profile.Extensions` reads `extensions.settings`
  from a Chromium profile's Secure Preferences (and Preferences, where an older Chrome kept
  it), with each one's manifest, kept there or in its folder under `Extensions/`, its name
  looked up in its messages when the manifest says `__MSG_name__`. Only what its person
  installed: what the browser installed by default, an administrator's policy or Chrome
  itself put there is left out. Each says why it can't be installed again from the Chrome
  Web Store by its id, if it can't: loaded from a folder, from another store (Edge's) or a
  file, or Manifest V2. `Counts.Extensions` counts those that can. Firefox's and Safari's
  have none a Chromium browser runs.
- **Settings' Import** says "· 2 extensions" on a row while the flag is on, and its Import…
  dialog has Extensions, unticked at first. The welcome doesn't offer them: at a first
  start the flag is always off.
- **Installing them:** Go hands each id to the extension manager, which installs one at a
  time through the engine's `InstallExtensionFromStore` and answers its prompt itself, as
  ticking Extensions was the yes. The engine fetches the package from the store's download
  address with the profile's own network (`--apps-gallery-download-url`'s template, or the
  Chrome Web Store's update address), then installs it as a package file is, held to the
  id asked for and marked as the store's ([extensions.md](extensions.md#installing-the-store-is-a-website)).
  An extension installed already isn't installed again. While they install the row says
  "Installing 2 of 5 extensions…".
- **What came,** in the welcome and in Settings alike, is a line per kind with its count:
  bookmarks and folders, "1 Reading List item, in its own folder" for Safari's, pages of
  history, the search engine, then extensions installed (by name), installed already,
  each that didn't install and why, and each left out and why, quieter
  ([shot](../build/e2e/settings-extensions.png)). `imports.json` keeps the same.
- **The logos** are the six that ship (above); `browserdata.Icon` and its AppKit link are
  gone, as is the rows' `icon`.
- **Messages:** `import`'s `what` takes `extensions`; `importing` and `imported` carry
  `installing`, each import's `{done, total}` by key.
- **Testing it:** the test home's Chrome · Personal has eight extensions in its Secure
  Preferences, of which two can come. `browserdata`
  (`TestExtensionsAreThoseItsPersonInstalled`), `core/app`
  (`TestExtensionsComeOverFromTheStore`, with the mock engine's store, `-store` on the
  mock: a folder of `crx/<id>_<version>.crx`) and `shell/e2e/imports.cjs`, which installs
  the test extension and uBlock Origin Lite from `.tools/extensions`.

## The importer, a module of its own

Reading another browser's data is the Go module `browserdata/` at the repo's root, with its
own `go.mod`, its own tests and a README, and no import of anything in `core`: it could move to
a repo of its own as it is. `core` uses it through a `replace` line in its `go.mod`. It only
reads; what to do with what it finds is the app's.

- **What it finds:** `Find()` lists the browsers installed and each one's profiles, with the
  folder, the profile's name and counts. Chrome, Edge, Brave and Arc share
  Chromium's files; Firefox has its own; Safari's need Full Disk Access, and a profile's
  `Locked` says so instead of failing.
- **What it reads,** in plain types of its own: the bookmark tree with folders, history visits
  with their times, open and pinned tabs (Arc's Spaces as groups),
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
  `-dump chrome/Default`.

Built in slice 3: `Find(home)`, a profile's `Bookmarks`, `History`, `Search` and `Counts`;
`Extensions` came later. Each database is read from a copy with its write-ahead log;
SQLite needs
macOS (`*_mac.go`, cgo); elsewhere it says `ErrNotOnThisSystem`, so the Android build compiles it and never calls it. Its
README says the rest.

How it builds into the app: `core/go.mod` replaces the module with `../browserdata`; the
Mac binary links `sqlite3` for it (`engine/BUILD.gn`, `go_core_lib`), as a
c-archive leaves cgo's libraries to the binary; `engine/build_go.py` and the Android one
list its files for ninja; the Android build copies it in with `core`
(`tools/android-release.sh`, `tools/android-content-build.sh`); `tools/test.sh mock` runs
its tests.
