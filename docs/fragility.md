# Where Project B is fragile

One short entry for every real bug we fix: what was seen, the real cause in plain words, the
seam that gave way, the fix, and the check that now catches it. Newest first. Written to be
read aloud. Every fix handed to release brings its entry (roles/feature.md); the tester keeps
this page (roles/tester.md).

The seams:

- **Engine and adapter**, our C++ between Chromium and Go. Chromium has rules it doesn't
  check until run time, and breaking one kills a process, and duties it leaves to us; and
  the order in which each side hears of a thing. Seven so far: StackBlitz, the quit crash,
  the Android blank shell, file uploads on Android, sparks on strict sites, a duplicated
  tab that failed to load, a View Source tab named as its page.
- **The network across sleep.** A download that stops receiving waits for ever unless
  something gives up on it. Two so far: downloads after sleep, the stuck updater.
- **Shell layers.** What lies over the page has to stay under the shell's own menus, and
  back has to close what is open before it leaves. Two so far: the menu under the crash
  page, the back key out of Settings.
- **Tooling and test machines.** Shared folders and long-lived emulators. Two so far: the
  mixed package, the emulators' memory.
- **Narrow widths.** What sits next to something, or under it, has to be measured against
  a piece that doesn't wrap. One so far: the update line on a phone.
- **A page's own navigations**, which Go rules on, against Chromium's way of starting,
  committing and aborting a load. Two so far: Stay on this site, the back key in full
  screen.
- **The phone's shared storage.** What the app puts in Android's shared folders meets
  other apps' files there, which it can't see. One so far: a download named as a file
  already in Downloads.
- **Android's accessibility tree.** What a page says in ARIA or HTML has to reach the tree
  TalkBack and the driver read, with its name and its state; Chromium hands it over only as
  fully as the embedder has set it up to. Three so far: the phone's star and folders with no
  names, TalkBack hearing every switch as off, switches named by other words heard with no
  name.

## The back key out of Settings, 2026-11-19

- **Seen:** on a phone, in Settings at Flags, a list inside it, the back key closed
  Settings itself instead of going back to where Flags was opened from. A dialog open over
  Settings went the same way.
- **Cause:** our pages go to their lists and open their dialogs inside the page, with no
  entry in the tab's history. The back key goes back in the tab's history, and on a tab with
  none closes it, so from anywhere inside one of our pages it left the page. The desktop's
  Back did the same.
- **Seam:** shell layers: what a page opens inside itself, against the tab's history that
  back walks.
- **Fix:** on dd-page-back: each place one of our pages goes to inside itself, and each
  dialog or menu while open, is an entry in its tab's history, so back closes the dialog,
  then goes to the place before, and leaves the page last. One module for every page and
  layer: `pageHistory` and `pushEscape` (shell/src/ui/escape.js, docs/shells.md, "Back on
  our pages").
- **Guard:** `shell/e2e/settings.cjs`, both layouts: Back from a list to where it was,
  Forward to it again, and Back closing a dialog; `shell/e2e/pages.cjs`: Back and Forward
  through a bookmarks folder; and drive_android's settings_back on a phone.
## A View Source tab named as its page, 2026-11-19

- **Seen:** View Source opened the page's source in a new tab, but the tab's address was
  the page's own, without view-source:, in the address bar and so in History, a duplicate
  and a restore, each of which then had the page and not its source. The tester's new check
  of the app's actions found it.
- **Cause:** Chromium keeps a source page's address as the page's own and marks its entry
  as showing the source. The engine reported each navigation's address as Chromium has it,
  so Go never heard of the view-source: at all.
- **Seam:** engine and adapter: what Chromium keeps apart, an address and how it shows,
  the engine has to put back together for Go.
- **Fix:** on dd-view-source: the engine reports a navigation in view-source mode as
  view-source: and the page's address (`ShownURL`, engine/browser/page_tab.cc), starting,
  finished and failed alike. Go's policy judges a page's source as the page, so a blocked
  host's source stays blocked, and View Source is off on a source page.
- **Guard:** drive_first_app.py viewsource, on the real app; `TestIsAllowedURL` and
  `TestViewSourceOpensInItsTabsWorkspace` in Go.

## A duplicated tab that failed to load, 2026-11-19

- **Seen:** on the Mac, Duplicate Tab on a google.com tab opened the copy on our error
  page, "Can't reach google.com", ERR_BLOCKED_BY_CLIENT, with Failed in the address bar and
  the bare host for a title. Reload or Try again loaded it.
- **Cause:** Go asks the engine for the copy and only then adds it to its tabs. The engine
  started the copy's first load inside that call, and when that navigation asked Go
  whether it may go ahead, Go didn't know the tab yet and said no, as it does for any tab
  it doesn't know. The engine's word for that refusal is the one an ad blocker uses. A new
  tab never raced so: Go makes it, adds it, and then asks it to load.
- **Seam:** engine and adapter: an engine call whose own events come back before the call
  has returned to Go.
- **Fix:** on dd-dup-tab: the engine starts the copy's load once DuplicateTab has
  returned, so Go knows the copy by then (`Engine::DuplicateTab`).
- **Guard:** drive_first_app.py duplicate: three duplicates of a loaded tab each load
  their page, with no error.


## A download named as a file already in Downloads, 2026-11-19

- **Seen:** on Android 16, a 60 MB big.bin downloaded to its end, and the app closed at
  once: Android's log said the copy into the phone's Downloads broke a rule of its own,
  "UNIQUE constraint failed: files._data". The phone's Downloads already had a big.bin,
  from an earlier test's run. Found while testing download notifications.
- **Cause:** a finished download is copied into the phone's shared Downloads under its own
  name, first as a pending file Android names itself, and made public as the copy ends. Only
  then does Android check the name, and it already had that file, which this app can't see,
  as another app's files are hidden from it. Android 14 refused it the same way for a file
  deleted behind its back, by a file manager that skips its index, as the sparks session
  found: the index still held the name. The refusal came back as an exception on the
  copying thread that nothing caught, which ends the app.
- **Seam:** the phone's shared storage: a name the app can't check before Android does.
- **Fix:** on dd-dl-samename: a name Android refuses is tried again as big (1).bin, big
  (2).bin and so on, as the phone names its own copies, up to 99; past that the copy goes
  and the file stays in the app's list (`Downloads.copyToShared`).
- **Guard:** drive_android's downloads part downloads the same name twice: the second
  reaches the phone's Downloads as name (1) and the app keeps running.

## Switches named by other words heard with no name, 2026-11-19

- **Seen:** on Android 17 with TalkBack on, after the fix for switches heard as off, the
  welcome's switches for the built-in sparks and for automatic updates reached Android with
  no name. A bare page showed which markup: a switch named by aria-label was named, but one
  named by aria-labelledby, or by a label wrapped around it, or both, had no name at all.
  Chrome on the same emulator named all four.
- **Cause:** our Chromium, 155, puts a name it takes from other elements into the field
  Android 16 added for a supplemental description, not into the control's text, and
  TalkBack doesn't speak that field on a control. An aria-label still goes to the control's
  description, which TalkBack speaks. The Chrome on the emulator, 133, predates that change,
  so it named all four. The daily driver found it from TalkBack's own log, which said "On,
  Switch" for Bravo.
- **Seam:** Android's accessibility tree: where an engine newer than the Android Chrome we
  compare with puts a name.
- **Fix:** on switch-labels: every switch is named by aria-label, from the same words its
  visible label shows: the settings rows, the welcome's, Sites' keep-cookies switch, and
  the site panel's and site sheet's. The line under a switch still reaches Android, by
  aria-describedby. On dd-a11y-names: the engine puts such names in the control's text
  again, as Chrome does, so selects and fields named by a label are named too.
- **Guard:** `shell/e2e/shells/mobile.cjs` counts a switch or checkbox without aria-label as
  nameless. For the engine, the tester's drive_android talkback_states turns TalkBack on and
  reads a page's switch named by a wrapping label, one by aria-labelledby and a field named
  by `<label for>`, each with its name; all three failed on 0.2.103 and pass since 0.2.105.

## TalkBack heard every switch as off, 2026-11-19

- **Seen:** on Android 16 with TalkBack on, every switch and checkbox in Project B reached
  Android named but not checkable and not checked, a page's native checkbox too, so
  TalkBack would say each one is off and can't be turned. Chrome on the same emulator read
  them checked. The tester found it; the phone star's check had seen the same of the
  bookmark sheet's folders.
- **Cause:** Chromium on Android gives pages the screen reader's mode, the one with checked
  states in it, only while it knows TalkBack is running. It asks which services run once, at
  its first question, and again only if the embedder registered its observers of the
  system's settings at start, as Chrome does. We never registered them, so the app kept what
  it saw first, and a screen reader turned on later, or still starting then, never counted.
- **Seam:** Android's accessibility tree: something Chrome sets up at start that an
  embedder has to set up too.
- **Fix:** on dd-a11y-checked: the app registers Chromium's accessibility observers as it
  starts, before anything asks (`BrowserApplication`).
- **Guard:** the tester's drive_android part, which turns TalkBack on and reads a checked
  switch and checkbox as checked.

## The phone's star and folders with no names, 2026-11-19

- **Seen:** the new phone check for the star, drive_android.py bookmark_star, couldn't find
  the Mobile shell's Bookmark in its menu on either emulator, though it was on screen. Past
  it, the bookmark sheet's folders showed as radio buttons one pixel across, with no name
  and never checked.
- **Cause:** the star was a button marked pressed or not, aria-pressed, as Standard's is.
  Chrome on Android gives such a button to Android as a toggle button, and in the tree the
  driver reads that toggle button carried no name at all: not Bookmark, not Edit bookmark.
  The extensions sheet's Pin was built the same way. Each folder was a real radio input,
  hidden at one pixel inside its row's label: the tree got the hidden input, a target one
  pixel across, without the label's words. TalkBack itself wasn't tried; it asks Chrome for
  a fuller tree, which may have named them.
- **Seam:** Android's accessibility tree.
- **Fix:** on dd-native-star: the Mobile shell's star and Pin drop aria-pressed. Their
  names already say which they are, Bookmark or Edit bookmark, Pin or Unpin, and Pin draws
  its pressed look from a class. Each folder is its whole row, a button with the radio
  role, named by its title and checked while it is the folder.
- **Guard:** drive_android.py bookmark_star finds the star and the folders by their names
  in Android's tree, and every control of the sheet is 48dp or more (the checked folder too,
  in the native shell; the web shell's checked state reaches the tree only with a screen
  reader on, see "TalkBack heard every switch as off"); `shell/e2e/shells/mobile.cjs` checks the star on is named Edit
  bookmark, with no aria-pressed.

## The back key in full screen, 2026-11-18

- **Seen:** on 0.2.95's candidate, on Android, the back key on a page in full screen closed
  its tab. Release's Android tests caught it before it shipped.
- **Cause:** the engine sends the back key to Go as "go back" when the tab has a page behind
  it, and as "close this tab" when it hasn't, and only "go back" left full screen first. A
  new tab used to load a blank page, so a page opened from it always had one behind it. Once
  a new tab started empty, the first page in it had none.
- **Seam:** a page's own navigations: a rule that counted on every tab's history starting
  with a blank page.
- **Fix:** 954d9cb1 on newtab-back-key: the back key's close leaves full screen first too.
  c0cc9fdd: the renderer-crash test waits for the page to show before it kills its renderer,
  as an empty tab's renderer now starts only as the page loads.
- **Guard:** core/app's full screen test sends the back key's close in full screen, and
  drive_android.py page_full_screen presses the back key on a page in a new tab.

## The update line on a phone, 2026-11-18

- **Seen:** on a phone, while an update downloaded, Settings' version line put Cancel on a
  line of its own, with a short stray underline under it.
- **Cause:** the version, the update's words, Retry and Cancel were one box that wrapped
  anywhere, so Cancel fell to the next line alone. The underline was the download's progress
  line: it was pinned to the bottom left of that whole box, so once the box took two lines it
  sat under Cancel instead of under the words.
- **Seam:** narrow widths.
- **Fix:** on dd-update-line-narrow: the line breaks only between the version and the
  update, the update's words never wrap, and the progress line is measured against those
  words alone.
- **Guard:** the settings page's test narrows the line to a phone's width while an update
  downloads, and checks Retry and Cancel sit on the words' line with the progress line under
  the words (`shell/e2e/settings.cjs`, both layouts).

## Sparks import, and file uploads on Android, 2026-11-18

- **Seen:** Settings' Sparks Import did nothing on a phone. On the Mac it took one file only,
  no zip, and a spark imported again was added as a second copy.
- **Cause:** on Android no web page could open a file picker at all, our own pages and every
  site's upload button alike: only the Mac's window asked Go and opened one, and without
  that Chromium says no. Import itself had been built for one file into the editor, with
  nothing that knew a spark it already had.
- **Seam:** engine and adapter: a browser duty Chromium leaves to the embedder, done on one
  platform only.
- **Fix:** e87f84bb: Android opens the phone's own picker, through Go as the Mac does. 20a96452
  and 21739d34: Import… takes any number of .user.js files and zips, asks Replace, Rename or
  Ignore for a spark already there, by @name and @namespace, and says what it did; Export…
  makes a zip.
- **Guard:** the settings e2e imports the owner's own sparks from a zip a Mac made, settles
  each clash, and reads an export back (21739d34); the Android driver's file_inputs part opens
  the picker from a page and from Settings, and imports a zip (f508ae91).

## The file picker on Android 10, 2026-11-19

- **Seen:** on Android 10, a page's file input and Settings' Import… opened no picker, and
  the page never heard it was closed. On Android 17 both worked.
- **Cause:** where the phone has a camera app, Chromium asks for the camera before it opens the
  picker. Android answered, but the activity passed its answers on only for its own
  notifications question, never to Chromium's window, which waited for good. From Android 11
  an app sees only the other apps its manifest names, and ours names no camera, so Chromium
  finds none there and asks nothing.
- **Seam:** engine and adapter: an answer Android gives the activity, which the embedder has
  to hand on to Chromium.
- **Fix:** 27ebb318: the activity hands every permission answer to the window, as it hands
  every activity result.
- **Guard:** the Android driver's file_inputs part, on Android 10 and on 17 (f508ae91).

## Sparks unstyled on strict sites, 2026-11-18

- **Seen:** on a site that allows only its own styles, as GitHub does, the downloader spark's
  button drew without its styles.
- **Cause:** the page's security policy blocked the <style> the spark added, as it would
  the page's own.
- **Seam:** engine and adapter: the page's rules applied to what we add to the page.
- **Fix:** bf72288d: sparks run in the page's own world, as the owner chose, and where one
  runs, the page's policy has its style rules eased before the page commits; every other
  rule stays.
- **Guard:** drive_first_app's sparks part: a spark's <style> applies on such a page, and the
  owner's Page Cleanup and Limit Screen Time draw there, styled (ea330554).

## Sparks' HTML on sites that want Trusted Types, 2026-11-19

- **Seen:** on a page whose policy wants Trusted Types, as YouTube's does, a spark's
  `innerHTML` threw a TypeError; the owner's downloader sets it four times.
- **Cause:** the spark runs in the page's own world, so the page's rule for its HTML holds
  for the spark too.
- **Seam:** engine and adapter: the page's rules applied to what we add to the page.
- **Fix:** 52562760: where a spark runs, the response's policies lose
  `require-trusted-types-for` and `trusted-types` before the page commits, as their style
  rules give way.
- **Guard:** drive_first_app's sparks part: a spark's innerHTML works on such a page
  (12f8bc42).

## The emulators' memory, 2026-11-18

- **Seen:** this Mac said it was out of application memory, and Safari and Terminal stopped.
- **Cause:** the four Android test emulators had grown to 266 GB together over two to three
  days of test runs. Nothing restarted them.
- **Seam:** tooling and test machines.
- **Fix:** restart the emulators after each release's Android tests and once a day, in
  roles/tester.md (c903d79c, 9d1d2e4e). A restarted emulator whose saved state breaks its GPU
  boots cold (ee8f4a5e).
- **Guard:** the test results log any process on the Mac over 4 GB during a test, and a
  watcher logs them between tests (8ed6f4c8).

## StackBlitz crashed, 2026-11-18

- **Seen:** a StackBlitz project showed "This page crashed".
- **Cause:** our blocker for a page's requests to blocked hosts didn't allow itself to be
  moved to another thread. Chromium moves it for every synchronous load, a synchronous
  XMLHttpRequest or a worker's importScripts, and when it refused, Chromium killed the page.
  Any page that loads that way crashed, not only StackBlitz.
- **Seam:** engine and adapter: a Chromium rule we didn't know, checked only at run time.
- **Fix:** 8083139d, one line: the blocker says it may move.
- **Guard:** the real app's browsing checks load a cross-origin isolated page that does both
  kinds of synchronous load (`drive_first_app.py browsing`).

## The menu under the crash page, 2026-11-18

- **Seen:** on a crashed tab, the ☰ menu opened under the "This page crashed" notice, with
  the tabs drawn over it.
- **Cause:** the notice lay over the page in a layer above the top bar's, so anything the top
  bar opened sat under it.
- **Seam:** shell layers.
- **Fix:** b3d0db95 on popups-over-crash, in 0.2.89: the page's area is a layer of its own,
  under the shell's bars and everything they open, in every shell.
- **Guard:** the Standard and Fox shell tests open the menu over a crashed tab.

## The mixed package, 2026-11-18

- **Seen:** the 0.2.86 installer held an older app core with the new interface. Release
  caught it before it shipped.
- **Cause:** the packaging script took the app from the build folder that every checkout
  shares. Another session built into it between the release's build and its packaging.
- **Seam:** tooling: one folder shared by everyone.
- **Fix:** 6d84f38d: packaging takes the checkout's own copy of its last build, and refuses
  one built from another commit.
- **Guard:** the packaging script itself refuses a mismatch, and the test results record the
  commit of the build each test tested.

## The stuck updater, 2026-11-17

- **Seen:** 0.2.76 said "Downloading 0.2.85" all night, with nothing to press.
- **Cause:** the update's download had stopped receiving, across the Mac's sleep, and nothing
  gave up on it; a manual check also waited for ever on the update folder's lock.
- **Seam:** the network across sleep.
- **Fix:** dfbc90c2: the download gives up after 30 seconds with nothing. 0a82e4ea: Retry
  and Cancel while it runs, Try again once it fails, and a check waits at most 15 seconds for
  the lock.
- **Guard:** the updater's Go tests (core/update).

## Downloads stuck after sleep, 2026-11-16

- **Seen:** a download stopped moving after the Mac woke, and stayed that way.
- **Cause:** the connection died in the sleep, and Chromium waits for data with no time
  limit.
- **Seam:** the network across sleep.
- **Fix:** 52ee4412: the engine interrupts a download that receives nothing for 30 seconds,
  Chromium resumes it on a new connection, and one that still fails shows Retry and Copy link.
- **Guard:** Go's downloads tests (core/app/downloads_test.go) and the mock engine's failed
  downloads.

## The quit crash after a download, 2026-11-16

- **Seen:** Project B crashed as it quit, after anything had been downloaded.
- **Cause:** Chromium's download manager outlives the profile. Our part that decides where a
  download goes belonged to the profile, so at shutdown the manager still pointed at it after
  it was gone.
- **Seam:** engine and adapter: the order Chromium tears things down.
- **Fix:** 4a3252d9: the profile takes its part back from the manager before it goes.
- **Guard:** `tools/test.sh quit`, in each release run: download a file, quit, and look for a
  crash.

## Stay on this site let pages through, and blanked them, 2026-11-16

- **Seen:** with a site set to stay on itself, some clicks still took the page to another
  site. When one was blocked, the page went empty, with the tab's title and icon gone, and
  only Back brought it back. Popups opened, then went blank. The Blocked chip looked wrong in
  dark mode.
- **Cause:** the engine guessed which navigations the page started and which the person did,
  and guessed wrong in some cases. And a blocked navigation was stopped after Chromium had
  begun to replace the page, so what was left was a blank page instead of the old one.
- **Seam:** a page's own navigations against Chromium's start, commit and abort.
- **Fix:** 1410035e and 577f4436: Go decides every navigation as it starts, Allow or Block,
  and the engine stops guessing. f67fc155: a page's new tabs and popups go by the page that
  opens them, and a blocked popup closes. 618da686: a tab whose navigation ends with nothing
  shown gets its page's title and icon back.
- **Guard:** the links checks in the real app, on the Mac and Android: the ten ways a page
  leaves, each refused under Block with the page kept and each going under Allow
  (7c637cc7, 695db34f).

## The Android blank shell, 2026-11-17

- **Seen:** on Android, the app now and then started to a blank screen, with no toolbar.
- **Cause:** Android kills a sandboxed process that doesn't attach in time, and the browser
  hears nothing of it, so the shell's page waited for ever for a renderer that was gone.
- **Seam:** engine and adapter: a process dying silently on the OS's side.
- **Fix:** b6bf890f: if the shell's renderer hasn't attached within 15 seconds, the engine
  makes the shell page again.
- **Guard:** `tools/android-attach-check.sh`, which starts the app again and again and checks
  the shell draws each time.
