# Where Project B is fragile

One short entry for every real bug we fix: what was seen, the real cause in plain words, the
seam that gave way, the fix, and the check that now catches it. Newest first. Written to be
read aloud. Every fix handed to release brings its entry (roles/feature.md); the tester keeps
this page (roles/tester.md).

The seams:

- **Engine and adapter**, our C++ between Chromium and Go. Chromium has rules it doesn't
  check until run time, and breaking one kills a process. Three so far: StackBlitz, the quit
  crash, the Android blank shell.
- **The network across sleep.** A download that stops receiving waits for ever unless
  something gives up on it. Two so far: downloads after sleep, the stuck updater.
- **Shell layers.** What lies over the page has to stay under the shell's own menus. One so
  far: the menu under the crash page.
- **Tooling and test machines.** Shared folders and long-lived emulators. Two so far: the
  mixed package, the emulators' memory.
- **Narrow widths.** What sits next to something, or under it, has to be measured against
  a piece that doesn't wrap. One so far: the update line on a phone.
- **A page's own navigations**, which Go rules on, against Chromium's way of starting,
  committing and aborting a load. Two so far: Stay on this site, the back key in full
  screen.
- **Android's accessibility tree.** What a web shell says in ARIA has to reach the tree
  TalkBack and the driver read, with its name. One so far: the phone's star
  and folders with no names.

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
  in the native shell; the web shell's checked state isn't in the tree the driver reads); `shell/e2e/shells/mobile.cjs` checks the star on is named Edit
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
