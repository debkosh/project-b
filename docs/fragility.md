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
- **A page's own navigations**, which Go rules on, against Chromium's way of starting,
  committing and aborting a load. One so far: Stay on this site.

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
- **Fix:** b3d0db95 on popups-over-crash, in 0.2.88: the page's area is a layer of its own,
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
- **Guard:** the quit check in each release run: download a file, quit, and look for a crash.
  It lives outside the repo for now, to be moved in.

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
