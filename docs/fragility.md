# Where Project B is fragile

One short entry for every real bug we fix: what was seen, the real cause in plain words, the
seam that gave way, the fix, and the check that now catches it. Newest first. Written to be
read aloud. Every fix handed to release brings its entry (roles/feature.md); the tester keeps
this page (roles/tester.md).

The seams:

- **Engine and adapter**, our C++ between Chromium and Go. Chromium has rules it doesn't
  check until run time, and breaking one kills a process, and duties it leaves to us; and
  the order in which each side hears of a thing. Nine so far: StackBlitz, the quit crash,
  the Android blank shell, file uploads on Android, sparks on strict sites, a duplicated
  tab that failed to load, a View Source tab named as its page, a copied image pasting
  empty, dangling pointers at quit on 158.
- **The network across sleep.** A download that stops receiving waits for ever unless
  something gives up on it. Two so far: downloads after sleep, the stuck updater.
- **Shell layers.** What lies over the page has to stay under the shell's own menus, and
  back has to close what is open before it leaves. Three so far: the menu under the crash
  page, the back key out of Settings, a download bar lost under the keyboard.
- **Tooling and test machines.** Shared folders and long-lived emulators. Three so far: the
  mixed package, the emulators' memory, a pushed file another install's.
- **Narrow widths.** What sits next to something, or under it, has to be measured against
  a piece that doesn't wrap. One so far: the update line on a phone.
- **A page's own navigations**, which Go rules on, against Chromium's way of starting,
  committing and aborting a load. Two so far: Stay on this site, the back key in full
  screen.
- **The phone's shared storage.** What the app puts in Android's shared folders meets
  other apps' files there, which it can't see. Two so far: a download named as a file
  already in Downloads, Delete on the older of two same-named downloads.
- **Android's process lifecycle.** Android ends an app's process when it likes, and
  says why only if asked. One so far: a crash notice after runs that never crashed.
- **Android's accessibility tree.** What a page says in ARIA or HTML has to reach the tree
  TalkBack and the driver read, with its name and its state; Chromium hands it over only as
  fully as the embedder has set it up to. Three so far: the phone's star and folders with no
  names, TalkBack hearing every switch as off, switches named by other words heard with no
  name.
- **A shell is a page.** Our shells are web pages, and Chromium holds them to a page's
  limits: what a page may do only after a click, a shell can't do for an action that comes
  from the menu bar or a key. One so far: Glide's Copy Address.
- **The phone's keyboard.** It takes half the screen or more, by an amount that differs
  between Android versions, and what a person needs next has to stay in sight above it, and
  what it is composing is its own until the word ends; and when it goes, what stood over it
  goes down with it, from under a finger still on the glass. Four so far: a Dialog's buttons
  under the keyboard, the keyboard left up by the native star's sheet, the first letter typed
  twice, a tap on a saved login's chip lost.
- **Work that takes time.** A rule made for work nobody asked for, such as waiting before
  saying it is busy, doesn't fit a press, which needs its answer at once; and a clock that
  measures how long the work has gone on is not one that measures how long nothing has
  arrived. And a call that may wait on the person must be reached from no thread and no
  job that others wait on. And a wait that is the person's, at a question the OS put up, is
  not ours to end: the question stays, and its answer goes nowhere. Nor is a question put
  to the person for nothing they did. And an answer that depends on the person isn't final:
  what they refused once they may allow at the next asking. And what a person takes back
  while the work is still going is taken back once it is done, not forgotten because there
  was nothing yet to take. Seven so far: Sync now that showed nothing, a sync called out of
  reach that wasn't, the app frozen while the desktop asked for its keyring, GNOME's keyring
  broken by our giving up, the keyring's question at every start, a keyring that gave no key
  and nothing said or to try, a backup code kept that nobody saw.
- **An error as text.** A library's words for what went wrong hold more than we would say
  ourselves: Go names the whole address a request went to, and an address can be a secret.
  What goes to a log, a tooltip or a report has to be looked at as text a stranger may read.
  One so far: the sync token in the log.
- **One sentence for every failure.** A row with one sentence for "it failed" says that
  sentence for failures it doesn't fit, and the person checks the wrong thing. Each failure
  the code can tell apart gets its own words, the rest say no more than that it failed, and
  the error's own words are where a person can read them. One so far: a sync called out of
  reach that wasn't.
- **An id used as a name elsewhere.** An id made in one place, with its alphabet, becomes a
  name in another that allows less. One so far: a passkey whose id starts with an underscore.
- **A spec's model taken as our rule.** A standard describes a device as it models one, and
  that model can be poorer than what a person expects of a manager they can look into. A
  rule copied from it that deletes or hides something of the person's has to be asked of
  them first. One so far: earlier passkeys for a site gone.
- **A trigger runs under its caller's rules.** What a trigger's own statement says about a
  conflict gives way to the statement that fired it. One so far: a row changed twice before a
  run.
- **What a platform doesn't have.** A thing one OS has no way to do is not a setting the
  person left off. Answered as that, it tells them to fix what they can't, in words written
  for another OS. One so far: saved passwords Linux couldn't show.

## Dangling pointers at quit on Chromium 158, 2026-11-24

- **Seen:** by release, in the quit check on the 158 candidate: the download finished and
  the app quit cleanly, but its log had 92 reports that "a raw_ptr is dangling", all at quit,
  where 155 had none in every run that day.
- **Cause:** our main delegate held content's client first and the renderer and utility
  clients after it, so C++ destroyed it last. Content's client keeps raw_ptrs to those two,
  and by then they were freed; its destructor let go of pointers to freed memory, which
  Chromium's dangling-pointer check reports. The order was the same on 155; 158 is what
  made the check fire.
- **Seam:** engine and adapter: the order our objects end in, against pointers content keeps.
- **Fix:** content's client is declared last in MainDelegate, so it goes first, before
  anything it points at.
- **Why it was missed:** my miss. The upgrade tests the engine interface, tools/test.sh app,
  as he scoped it; the quit check reads the log after quitting and is in release's run only.
  I didn't run it on 158 before handing over.
- **Guard:** tools/check_quit_after_download.py fails on "dangling" in the app's log after
  quit (tools/test.sh quit); it caught this one, and passes with the fix.

## A backup code kept that nobody saw, 2026-11-24

- **Seen:** by release, as a test of Settings' Sync that failed now and then while the Mac
  was busy with a build: after Skip for now on "Save your backup code", the card said
  "Backup code, Made 24 Nov 2026" where it should have offered to make one.
- **Cause:** the code is made as its card opens, which takes a round trip to the server.
  Leaving the card dropped the code only if the page already had it. Left sooner, by Skip
  for now, Cancel or Escape while the card still said "Making your backup code…", the code
  was made all the same and never dropped. The bucket then had a backup code nobody had
  seen, and the row said there was one. A person who skipped would believe they had a way
  back in, and after New code… and Cancel the bucket had two.
- **Seam:** work that takes time: what a person takes back while the work is still going.
- **Fix:** leaving the card sends the drop whether the code has come or not. Go does what is
  asked of sync in turn, so the drop follows the making, and with nothing made it does
  nothing.
- **Why it was missed:** my miss. I wrote the page to drop what it held, and thought of the
  making as instant: every test's server answered at once, so Skip always came after the
  code. Only a loaded machine made the press come first.
- **Guard:** `shell/e2e/sync.cjs`: with the mock's sync server slow, Cancel while the card
  says it is making the code, then the card opened again and left; the row still offers to
  make one. Seen to fail by its name without the fix, and to pass with it; and the whole
  test, under a load average of 24, failed before the fix at the check release saw.

## Saved passwords Linux couldn't show, 2026-11-24

- **Seen:** by the Linux session on GNOME, while it checked the keyring's line. In Settings'
  Passwords on Linux, Show and Copy answered "Set a screen lock on this device to show
  passwords", whatever the person did, and the page said they would be asked for "Touch ID
  or your Mac's password". Nobody on Linux could see or copy a password they had saved.
  Filling one into a site worked.
- **Cause:** Linux has no check of the person a Flatpak can ask, and its `verifyPerson`
  answered as a Mac with no password set does: no screen lock. The page's sentence had two
  wordings, a phone's and a Mac's, and Linux got the Mac's.
- **Seam:** what a platform doesn't have.
- **Fix:** his decision, 2026-11-24: on Linux show them, the keyring being what is asked.
  `system.CanVerifyPerson` says whether the OS has a check: false on Linux. There Show and
  Copy ask none and nothing holds; opening the password asks the keyring for the device
  key, which the desktop has the person unlock. The page says "Your keyring unlocks them."
  where it spoke of the check.
- **Why it was missed:** Passwords was built for the Mac and the phone, each with a check,
  and its tests' device always had one. When Linux came, the stub was written as "until a
  portal does it" and nobody went through Passwords' own page there; I didn't either, until
  the Linux session's note.
- **Guard:** core/app's `TestSettingsPasswordsWhereTheOSChecksNobody`, and
  `shell/e2e/password-settings.cjs` run once as Linux: the page's words, Show and Copy with
  no check asked, and the keyring's line on the row when the keyring gives no key. Checked
  on GNOME 46 by the Linux session, 2026-11-24, on a build of f3fe73b5: the line over the
  list; Show and Copy, the password pasting from the system's clipboard; with the keyring
  locked at a start nothing asks until Show, which brings the desktop's question, the app
  answering meanwhile, and Unlock shows the password; Cancel leaves the keyring's line on
  the row, and Show asks again.

## A keyring that gave no key, and nothing said or to try, 2026-11-24

- **Seen:** reading the code while the two entries below were fixed, not in a report. On
  Linux the desktop's keyring gives no device key when the person says no to its unlock
  question, and when GNOME's keyring is refusing everything until the next login. With sync
  on, Settings' Sync then showed "Set up sync" as though sync were off, and nothing asked
  again until a restart. Passkeys came up as an empty list. Passwords said only that a
  password couldn't be opened.
- **Cause:** the answer was one more error with no name. Sync's start ran once, so an
  answer that a second asking would have changed was final for that run of the app.
- **Seam:** work that takes time: an answer that depends on the person taken as final.
- **Fix:** the answer has a name, `keys.ErrKeyring`, made of oskit's `ErrNoPortal` where the
  app asks for the device key. Sync stays on and says it can't start, with one line for
  both causes, as the app can't tell them apart: "Your keyring is locked. Unlock it when
  asked; if it doesn't ask, log out and back in." Try again asks the keyring again. The
  same line is on Passkeys with Try again and in the set-up Dialog. A password's row has it
  too, and on Linux reaches it since the entry above: until then Show and Copy stopped
  before the keyring, at the screen-lock line.
- **Why it was missed:** every test's device key always came. The Linux checks answered the
  keyring's question each time.
- **Guard:** core/keys' `TestAKeyringThatGaveNoKeyIsAskedAgain` and core/sync's
  `TestAKeyringThatGivesNoKeyAtStart`: sync on with no host and the problem named, and its
  server opened once the keyring gives the key. Not guarded: the pages' words, which no
  mock can reach, as the mock's device key always comes. Checked on GNOME by the Linux
  session, 2026-11-24, on a build of a03b3cf5: `keyring_linux.go` compiles; Cancel gives
  "Sync can't start" with the line, and Try again asks again and sync runs once unlocked;
  Passkeys says the line with Try again; a keyring broken by a killed app says the line at
  once, at each Try again. A person's own quit while the question is up no longer breaks
  the keyring: the app stays, with no window, until the question is answered, and ends
  then. Nothing is said on a web page where a fill or a passkey doesn't happen for this.

## The keyring's question at every start, 2026-11-24

- **Seen:** by the Linux session reading 0.3.10 on GNOME: a profile that had ever sealed
  anything, a saved password or a passkey, asked the OS for the device key at every start,
  with sync off too. On a desktop that logs in by itself the keyring is locked, so its unlock
  question came up from nowhere at each start: the question most likely to be left up while
  the person quits, which breaks gnome-keyring (the entry below).
- **Cause:** sync's start opened the profile key first and only then looked whether sync was
  on, because what says so, its address, is sealed.
- **Seam:** work that takes time: a question put to the person for nothing they did.
- **Fix:** whether sync is on is told by its address being kept at all, which opens nothing.
  With sync off a start asks for no key; the first ask follows the person's own action. A
  profile key that doesn't open is then found when sync is turned on, and says so there.
- **Why it was missed:** on a Mac and on a desktop whose keyring opens at login the ask is
  silent, so it cost nothing anyone saw; no test counted the asks.
- **Guard:** core/sync's `TestAStartAsksForTheDeviceKeyOnlyWithSyncOn` counts them: none at
  a start with sync off, one or more with sync on. `TestALockedProfileKeyFoundWhenTurningSyncOn`
  wants a locked key said when sync is turned on, and mended by starting fresh. Both fail on
  the code as it was. With sync on, the question still comes at start, as the run does.
  Checked on GNOME by the Linux session, 2026-11-24, on a build of main with the keyring
  locked: with sync off and a saved password a start put up no question, and the passwords
  list came without one.

## GNOME's keyring broken by our giving up, 2026-11-24

- **Seen:** on Linux 0.3.10 under GNOME (Ubuntu 24.04, gnome-keyring 46): the "unlock the
  login keyring" question left up for more than two minutes and then answered. From then on
  nothing could be sealed or opened: an import said "skipped 1", for that person and every
  later start of the app, until they logged out and in.
- **Cause:** the request for the device key gave up after two minutes when its caller set
  no limit, and the app sets none. Giving up closed the portal's Request, but GNOME's
  question stayed up. Once the person answered it, gnome-keyring answered every later
  request for a secret, from any app, "The operation was cancelled", at once and with the
  keyring unlocked, until it started again at the next login. The Linux session's probe took
  the two halves of giving up apart: closing the Request alone sets it off, closing the pipe
  the secret is written to does not. The fault is gnome-keyring's; we set it off.
- **Seam:** work that takes time: a limit of our own on a wait that is the person's.
- **Fix:** `oskit/secret` on Linux waits for as long as the question is up and never closes
  its Request, whatever its caller's context says, as the Mac's keychain request already
  did. Nothing freezes while it waits, since the app asks off the main thread and outside
  the store's jobs (the entry below).
- **Why it was missed:** the limit was there against a portal that never answers, which
  nobody had seen, and nobody asked what ending the wait leaves behind. The tests' portal
  answers at once, and no check on a real desktop waited two minutes at the question.
- **Guard:** the reason is written at the call in `oskit/secret/secret_linux.go`, and the
  call can no longer be ended from outside. No automated test holds a real keyring's
  question; the Linux session's probe on GNOME is the check: the question up for three
  minutes, then unlocked, and the key arrived, as it did for two requests after. The same
  on a build of main, 2026-11-24, with sync on: the question came at start and stayed up
  three minutes while the tabs and Settings kept answering, sync ran after the unlock, and
  an import begun meanwhile finished after it, with one question for the two. Not guarded,
  and not ours to guard: a process that quits or is killed while the question is up, whose
  requests the portal closes itself.

## A pushed file labelled for another install, 2026-11-23

- **Seen:** on the test emulators after a long day of runs, drive_android's bookmark_star failed
  on every build: the menu's Bookmark saved nothing, then "the app is gone". A restart of the
  emulator cleared it, and it came back after more runs.
- **Cause:** the driver, not the app. profile_file writes bookmarks.json and prefs.json into
  the app's data as root with `adb push`, then fixes their owner. adbd labels each file it
  pushes for the app's SELinux category, from a list of packages it reads once and keeps; once
  an upgrade check had uninstalled and installed the app, the app had a new category and adbd
  went on using the old one. The app was then denied its own files: the store never made its
  bookmarks table, and nothing could be bookmarked. Proven on emu-16k: after a reinstall the
  app's folder was c221, and a file pushed next was c220.
- **Seam:** a driver writing into an app's private data as root, fixing one kind of ownership
  and not the other.
- **Fix:** profile_file runs `restorecon -RF` on the app's folder after its chown (8ca72878).
  restorecon is a new process each time, so it labels with today's category.
- **Why it was missed:** the failure looked like the app dying, and a restart cleared it, so it
  passed as built-up emulator state for a day. The passwords session found the SELinux denial
  in the run's saved logcat.
- **Guard:** the fix itself, on every profile_file call, and the tester's restart of the
  release emulators before each release's Android group (roles/tester.md).

## Earlier passkeys for a site gone, 2026-11-23

- **Seen:** by Sudipta, on webauthn.io: he made several passkeys for the site over some days,
  and only the latest was there.
- **Cause:** saving a passkey deleted the one already kept for the same site and the same
  account, by the site's user handle, and a sign-in listed only the newest of two for one
  account. That was the design, not a slip: WebAuthn models an authenticator as holding one
  passkey per site and account, and Apple's and Google's managers replace as well. A site
  normally stops a second one by naming the first as one to leave out; webauthn.io forgets a
  registration after a day and then names none, and its user handle is made from the
  username, so the same username on a later day replaced the earlier passkey. The delete
  synced, so it went from every device.
- **Seam:** a spec's model taken as our rule.
- **Fix:** `Make` in `core/passkeys` deletes nothing, and `List` has every passkey of the
  site. A sign-in's card keeps one account's passkeys together, each saying when it was
  made (`ownOrder` and `ownRows`, `core/app/passkeys_own.go`). The passkeys already replaced can't be brought back.
- **Why it was missed:** the test said the wrong thing on purpose: it wanted the first passkey
  gone after a second for its account. The rule was in the design's table of calls, as one
  clause, and was never put to Sudipta as a choice.
- **Guard:** `TestMakeKeepsListAndCount` (`core/passkeys`) makes two for one account and a
  third for another, and wants all listed and each signing;
  `TestTwoPasskeysForOneAccountAreBothOffered` (`core/app`) saves two through the Save card
  and wants both rows at a sign-in, and the older one signing;
  `TestOwnRowsOrderAndWhenMade` holds the rows' order and words;
  `TestSeveralPasskeysOnOneSiteSync` (`core/sync`) wants all three on a second device;
  `shell/e2e/passkeys.cjs` saves a second for one account and wants both rows on the card.
  Each fails on the code as it was.

## The app frozen while the desktop asked for its keyring, 2026-11-23

- **Seen:** on Linux 0.3.9 under KDE: importing a passwords file from Settings brought
  KWallet's question for its password, and while it waited the desktop marked Project B's
  window "Not Responding" and nothing in the app answered.
- **Cause:** a profile's key is opened the first time something is sealed with it, and
  opening it asks the OS for the device key, which may wait on the person: a keyring or
  wallet to unlock on Linux, the Mac's keychain after an update. Two things then waited with
  it. The import kept its logins on the main thread, though keeping them was written to run
  on a worker; and every seal ran inside a store job, so the store's one worker waited too,
  and with it anything that waits on the store. On a Mac and on most Linux desktops the OS
  answers at once, so nothing showed.
- **Seam:** work that takes time: a call that may wait on the person, reached from a thread
  and from a job that must not wait.
- **Fix:** the import keeps its logins on a worker and answers on the main thread, its
  step's clock stopped meanwhile. `keys.Ready` makes or opens the profile key, and saving a
  login, changing one, bringing logins over, making a passkey and turning sync on each call
  it before their store job, so the job itself asks the OS nothing.
- **Guard:** core/app's `TestKeepingPasswordsNeverHoldsTheMainThread` has a store that waits
  as a keyring does, from a browser and from a file, and wants the main thread still running
  and the import done after the answer. core/passwords'
  `TestTheDeviceKeyIsAskedOutsideTheStore` has a device key that waits, and wants the store
  still answering while it does, for saving, changing and bringing over. Both fail on the
  code as it was. Checked on KDE with 0.3.10, 2026-11-24: with KWallet's question up, for a
  new wallet and for a closed one, the window and Settings keep answering and the login
  arrives once the wallet opens.

## A field pressed before its page was told, 2026-11-22

- **Seen:** before filling was released, on the real Mac app: a press on a sign-in's field
  right after its page appeared brought no list, and nothing brought it later. The real
  app's check had passed in the morning, and failed three times in three once main had moved
  under it.
- **Cause:** the form script tells a page's fields a tenth of a second after the page
  changes, as pages change in bursts, and told a press at once. A field pressed inside that
  tenth, or focused by the page as it appears, as an autofocus is, was a press on a field Go
  hadn't heard of, so Go offered nothing. The morning's runs happened to press later.
- **Seam:** the order in which each side hears of a thing: the page's script and Go.
- **Fix:** the form script tells the fields, as they are at that moment, before every press
  it tells (`tellFields` in `core/forms/page/forms.js`).
- **Guard:** `shell/e2e/form-script.cjs` puts a form on a page and focuses its field in the
  same breath, and wants the fields told, that field among them, before the press; it fails
  on the script as it was. `tools/drive_first_app.py filling` presses a real page's field as
  soon as it has loaded.

## A tap on a saved login's chip lost, 2026-11-22

- **Seen:** on a phone, before filling was released: a tap on a saved login's chip, in the
  bar over the keyboard, filled nothing. The keyboard went down and the chips stayed. A
  second tap filled. The emulators' check passed most times and missed now and then.
- **Cause:** a touch on the shell takes the keyboard from the page, so the keyboard goes
  down and the bar over it goes down with it, while the finger is still on the glass. The
  click that follows a tap is aimed at where the finger is, and the chip was no longer there.
  `adb shell input tap` is down and up at once, so it nearly always beat the keyboard; a
  press of 150 milliseconds, as a finger's, missed four times in four.
- **Seam:** the phone's keyboard, and a shell control that stands over it.
- **Fix:** the chips and Manage take a tap as the finger coming up where it went down, told
  to the control it went down on, wherever that control has gone (`tap` in
  `shell/src/apps/shell/mobile/PasswordBar.svelte`), and stop the click that would follow
  it onto whatever the bar left there. A click with no finger before it, a screen reader's,
  still picks.
- **Guard:** `shell/e2e/password-fill-mobile.cjs` holds a finger on a chip, takes the
  keyboard down under it, and lifts it: the login fills and the address field isn't opened;
  it fails on the bar as it was. `tools/drive_android.py` taps the chips with `finger`, down
  for 150 milliseconds, on the emulators.

## The first letter typed twice, 2026-11-20

- **Seen:** on Android 10, typing an address in the phone's field showed its first letter
  twice: "localhost" became "llocalhost". Found by drive_android on Dev 0.3.9-dev.202611201508,
  where 24 parts of Android 10's run failed at typing; Android 17 passed.
- **Cause:** the field completes what is typed with the rest of an address, put in the field
  after the caret. Android 10's Gboard holds the word it is composing as its own, and when the
  field's text changed under it, it typed the word again over the completion.
- **Seam:** the phone's keyboard: what it composes is its own until the word ends, and the
  page may not rewrite the field under it.
- **Fix:** while the keyboard composes, Go's answer lists rows and completes nothing; the
  input that ends the word asks again and completes. The same in Standard, for a desktop's
  input methods (`Address.svelte`, `AddressBar.vue`, 0c5e1630).
- **Guard:** mobile.cjs composes a letter and checks the field holds only it while Go's answer
  lists a completion, and that it completes once the word ends; it failed without the fix.
  drive_android's browsing on Android 10 types into the field.
- **Missed:** the branch was tested with Gboard on Android 16 only, which composes otherwise,
  and drive_android wasn't run on it before handover, as its handover said. A phone shell
  change now runs drive_android in full on both emulators before handover (roles/feature.md).

## The keyboard left up by the native star's sheet, 2026-11-19

- **Seen:** in the native Mobile shell, Enter in the bookmark sheet's name saved the bookmark
  and closed the sheet, but the keyboard stayed up over the page with nothing to type into.
  With the bars stepping aside for the keyboard, the toolbar went too, so Menu was out of
  reach until the back key put the keyboard away. Found by drive_android's bookmark_star on
  Android 14, once the driver could find the shell with the toolbar aside.
- **Cause:** the native shell closes a sheet by taking its views away. Android doesn't put
  the keyboard away when the view that has it goes, so it stayed up for a field no longer
  there. The web shell's sheets are a page's, and Chromium does put it away.
- **Seam:** the phone's keyboard: it follows the focused view, and a view taken away with the
  focus leaves it behind.
- **Fix:** on keyboard-bars-again: a sheet closing with its field focused puts the keyboard
  away (`MobileNativeShell.kt`, open).
- **Guard:** bookmark_star in the native shell opens the menu after Enter in the name, which
  needs the toolbar back.

## The sync token in the log, 2026-11-19

- **Seen:** nobody saw it. Reading the code for Sync details showed that a sync run that
  failed wrote the bucket's whole address into the profile's timeline, and showed it in the
  Sync row's tooltip. An address from the memd page carries the bucket's token in its path,
  and the timeline's last lines go into a crash report, which a person may post in public.
- **Cause:** sync logged and showed a failed run's error as Go worded it, and Go's error for a
  request names the whole address it went to. Status showed only the host everywhere else.
- **Seam:** an error as text: a library's words for what went wrong hold more than we would
  say ourselves.
- **Fix:** on sync-token-out-of-log: `sync.Words` gives an error's text with a request's
  address cut down to its host, and the status, the page's answers and the timeline use it.
- **Guard:** `TestAFailedRunsWordsLeaveTheTokenOut` in `core/sync` signs a device out and
  finds no token in the status or the timeline, having first seen that Go's own error has it.

## A sync called out of reach that wasn't, 2026-11-20

- **Seen:** on the Mac, in a new profile, he turned sync on against a bucket that already held
  his other devices' data. The passphrase was taken, the dialog closed, and the Sync row said
  "Couldn't reach the server" with Try again. The server answered him at once in a browser.
- **Cause:** the row had one sentence for two things that aren't it. Any run that failed, for
  whatever reason, read "Couldn't reach the server", the server's own 500 and a row that
  couldn't be written among them. And any run still going after 30 seconds read the same,
  from a clock in the page that measured how long the run had gone on, not how long nothing
  had arrived; a first sync against a full bucket can take longer, and Try again then cut the
  working run short and started it over. His Mac hit the first: the run failed on a passkey
  (the next entry), which the server's log, every request answered 200, and then the
  profile's own log showed. Sync details showed a failed run's own words only once it was
  opened.
- **Seam:** one sentence for every failure; and work that takes time, a clock on the wrong
  thing.
- **Fix:** on sync-says-what-failed. `core/sync` names a server's own failure (`ErrServer`)
  apart from no answer, and ends a request with nothing going or arriving for 30 seconds as
  stalled (`ErrStalled`, in the http client it wraps), so a long run whose requests are
  answered never stalls. The row says what a failed run was by the status's `problem`
  (`syncRun.js`), keeps no clock of its own, and goes on saying "Syncing…" while a run does.
  Sync details shows a failed run's own words under its row and on the state's card, and the
  run going on with how long and which request it waits on. The words are the request and
  then the cause (`sync.Words`), without the sentence over again.
- **Guard:** `core/sync` (`TestAFailedRunSaysWhatItWas`,
  `TestARunStallsOnlyWithNothingArriving`): a 500 is `server`, a 400 has no name, no answer is
  `unreachable`; a run slower than the stall wait whose requests are answered ends well, one
  with nothing arriving ends `stalled`, and Sync details names the request it waited on.
  `shell/e2e/sync.cjs`: the row's words for each, the tooltip, a held run that goes on saying
  it syncs and then ends as stopped, and Sync details showing each failed run's words with
  nothing opened and a run going on with its count.

## A passkey whose id starts with an underscore, 2026-11-20

- **Seen:** the failed run above. The profile's log: "sync: passkeys _bNi…: guid: letters,
  digits and ._:~- up to 128, not starting with underscore", at every run, with the wait
  between them growing to half an hour. Nothing of the profile synced.
- **Cause:** a passkey's guid in sync was its id, random bytes named in base64url, whose
  first letter is an underscore one time in 64. synckosh keeps guids that start with an
  underscore for its own items and refuses one, and one refused item fails the whole run.
  The passkeys' tests made a few passkeys, never one of those.
- **Seam:** an id from one place used as a name in another, each with its own alphabet.
- **Fix:** at first, in 0.3.6, `core/sync` put a `~` before such a key. Since the move to
  random guids the seam is gone: an item's guid is `synckosh.NewGUID`'s, the credential ID
  is in the item's sealed private part, and no id of ours is used as a name of synckosh's
  ([sync.md](sync.md)).
- **And so no one row can do it again:** a row sync can't hand over is set aside with why,
  and the rest goes (`drain` in `core/sync/run.go`, the marks' `stuck` column); Sync details
  names it. `TestARowThatCantBeSentIsSetAside` and `TestAnItemTheServerRefusesIsNamed`.
- **Guard:** `TestAPasskeyWhoseIDStartsWithAnUnderscoreSyncs` in `core/sync` makes passkeys
  until one's id starts with an underscore, syncs it to a second profile, signs nothing away,
  and deletes it there. `TestTheServerHoldsNothingReadable` has every item's guid a UUIDv7.

## A row changed twice before a run, 2026-11-20

- **Seen:** in the same log, between the failed runs: "store: UNIQUE constraint failed:
  sync_changes.collection, sync_changes.key", many times. Each was a write the store refused:
  a page visited again, or typed again, while its first visit still waited to go out. The
  visit wasn't counted. With runs failing the wait was long; with sync well it is the ten
  seconds between sync's looks, so it has lost repeat visits since sync was released.
- **Cause:** the trigger that marks a changed row wrote the mark with `INSERT OR REPLACE`.
  SQLite runs a trigger's statements with the conflict handling of the statement that fired
  it when that statement has one, and an upsert's `DO UPDATE` is an update that aborts. So a
  second change of a marked row, through an upsert, failed on the mark's unique pair and took
  the feature's own write with it. History, typed addresses and bookmarks all write that way.
  The tests changed each row once between runs.
- **Seam:** a trigger's body runs under its caller's rules, not its own.
- **Fix:** on sync-says-what-failed: the trigger deletes the row's mark and writes it again
  (`triggers` in `core/sync/run.go`). Triggers are made again at each start, so a profile has
  the new ones once it runs the new app.
- **Guard:** `TestARowChangedTwiceBeforeARunKeepsBoth` in `core/sync`: a page visited and
  typed three times and a bookmark edited twice between two runs keep every change, the
  store logs no refusal, and the run leaves no mark.

## Sync now that showed nothing, 2026-11-19

- **Seen:** on a phone with sync on against a real server, pressing Sync now changed nothing
  on screen. He couldn't tell whether it had synced.
- **Cause:** the row said "Syncing…" only once a run had gone on for a second, so that sync's
  own runs every few seconds wouldn't flicker. A run a person asked for took the same rule, and
  against a quick server it was over before the second passed. The row already read "Last
  synced just now" from the run before, so it stayed word for word the same.
- **Seam:** work that takes time: a rule made for work nobody asked for, applied to a press.
- **Fix:** on sync-now-feedback: a run a person asked for, with Sync now, Retry or Try again,
  says "Syncing…" at once and for a second at least, with Sync now still under the finger but
  disabled, then when it last synced or what went wrong (`Sync.vue`).
- **Guard:** `shell/e2e/sync.cjs` presses Sync now against a quick server and checks the row
  says "Syncing…" at once with the button disabled, then "Last synced just now" with Sync now
  back. On a phone, drive_android's sync part presses Sync now and sees the server asked.

## A Dialog's buttons under the keyboard, 2026-11-19

- **Seen:** on Android 17, in Settings' Sync, with the passphrase typed twice and the
  keyboard up, the Dialog's Cancel and Turn on sync were out of sight. A person had to put
  the keyboard away, or scroll inside the Dialog, to find them. The release run for 0.2.107
  found it: the Android driver's tap landed beside the button and only put the keyboard
  away, so sync was never turned on. On Android 10, whose keyboard is shorter, they stayed
  in sight.
- **Cause:** a Dialog was one box that scrolled as a whole when it had less height than it
  needed. With the lower field focused, the page scrolled that field into view and the
  buttons under it went below the edge. Android 17's keyboard leaves that little room.
- **Seam:** the phone's keyboard: how much height is left over it is known only on the
  device.
- **Fix:** on dialog-phone-keyboard: a Dialog keeps its title and its buttons in place and
  only what is between them scrolls (`ui/Dialog.vue`), and the Mobile shell's dialog for a
  page's prompt or sign-in does the same (`PageDialog.svelte`), as the design system's "On
  a phone" says.
- **Guard:** `shell/e2e/sync.cjs` gives the passphrase Dialog 380px of height, and
  `shell/e2e/shells/mobile.cjs` a page's sign-in 300px; each checks the title and the
  buttons are in sight and only the body scrolls. On the device, drive_android's
  sync_on_the_phone taps Turn on sync with the keyboard up.

## Delete on the older of two same-named downloads, 2026-11-19

- **Seen:** on a phone, with two downloads of one name in the list, the second shown as
  name (1) in the phone's Downloads, Delete on the older one deleted the newer one's copy
  and left its own. Open and Share on the older one had opened and shared the newer one
  since 0.2.106. The phone check of Delete found it.
- **Cause:** a phone moves a finished download's file to the shared Downloads, so its
  place in the app's own folder is empty again. Go gave the next download of that name
  the same place, and the app finds each download's copy in the shared Downloads by that
  place, so the second copy took over the first's.
- **Seam:** the phone's shared storage: a name the app's own folder no longer holds is
  still a download's.
- **Fix:** on dd-batch-0.2.109: a new download's place skips every one a download in the
  list has, not only running ones (`free`, core/downloads/downloads.go), so each keeps
  its own.
- **Guard:** `TestTheSameNameAfterItsFileMovedGetsItsOwnPath` in Go; drive_android's
  downloads part deletes the older one and checks the newer copy stays.

## A download bar lost under the keyboard, 2026-11-19

- **Seen:** in the batch for 0.2.109, on a phone in the Mobile shell, a download started
  from the address bar showed no bar at all, and so never offered notifications. The phone
  check of the offer found it.
- **Cause:** two features met on the bar. The keyboard work hides the bars under the page
  while the soft keyboard is up, and the download bar drops the news that comes while it
  is hidden, so it never comes back over what the person went on to. Enter in the address
  ends editing a moment before the keyboard goes, and the download's first news came in
  that moment.
- **Seam:** shell layers: hidden because the person moved on, against out of the way for
  a moment.
- **Fix:** on dd-batch-0.2.109: the keyboard puts the bar away rather than hiding it
  (`away`, shell/src/apps/shell/mobile/DownloadBar.svelte): it keeps its news out of
  sight and shows it once the keyboard goes, its time and the offer's one showing starting
  then.
- **Guard:** shell/e2e/shells/mobile.cjs: a download starting with the keyboard up keeps
  its bar out of sight, then offers notifications; drive_android's
  download_notifications on a phone.

## Glide's Copy Address that couldn't copy, 2026-11-19

- **Seen:** in the real app, Glide's Copy Address from the menu bar said "Could not copy",
  and the clipboard kept what it had. The new real-app check found it while the action
  coverage's owed tests were worked off; no test had run it from the menu before.
- **Cause:** Glide wrote the address with the page's own clipboard call. Chromium lets a
  page write the clipboard only just after a click or a key press in that page, and the menu
  bar's action reaches the shell as a message from Go, with neither, as Shift+Cmd+C's does.
  The sidebar's copy button worked, as it is a click.
- **Seam:** a shell is a page: a page's limit, met by an action that isn't the page's.
- **Fix:** on glide-copy-address: the shell sends Go `copyText`, and Go puts the text on the
  clipboard, as it does a context menu's Copy Link. Every Copy Address of Glide's goes that
  way: the menu bar's, the sidebar's, and its menus' (`menus.js` `copy`).
- **Guard:** glide.cjs runs Copy Address from the menu and finds the address on Go's
  clipboard; in the real app, drive_first_app's glide part runs it and check_clipboard.py
  finds the address on the Mac's clipboard.

## A copied image pasting empty, 2026-11-19

- **Seen:** on the long-press menu's branch, on Android 14, Copy image put the picture on the
  clipboard, and Android's own preview and the keyboard showed it, but pasted into one of our
  pages it arrived as a PNG of no bytes. The driver's new check found it; the mock can't, as it
  has no clipboard.
- **Cause:** Chromium on Android reads an image from the clipboard through its content URI, and
  refuses a URI of its own app unless it is the copy it recorded making, a defence against a
  page reading the app's files through the clipboard. It records its copies only through an
  image file provider the embedder sets, as Chrome does. We set none, and our menu put the image
  there with Android's clipboard directly, so Chromium had no record of it. By the same gap,
  from Chromium's code and not tried, a page's own copy of an image was dropped.
- **Seam:** Engine and adapter: a duty Chromium leaves to the embedder, at its seam with
  Android's clipboard.
- **Fix:** on long-press-menu: the app gives Chromium's clipboard an image file provider
  (`ClipboardImages`), which keeps its copies in the profile's shared folder, and Copy image goes
  through Chromium's clipboard, which records it.
- **Guard:** drive_android.py long_press_menu pastes the copied picture into a page, which must
  get a PNG with bytes.

## A crash notice after runs that never crashed, 2026-11-19

- **Seen:** on a real phone, Android 17 with 0.2.99, "Project B crashed last time"
  showed after a run that hadn't crashed. Its report's last run
  restored its two tabs and painted; a minute later its update check found no network,
  the phone asleep; then nothing more. Its crash.log held only its first line, no panic,
  and Android kept no native crash of it.
- **Cause:** a crash.log left behind was the one sign of a crash: a clean stop deletes it,
  and only a crash was thought to skip that. On a phone Android ends an app in the
  background whenever it wants the memory, and when it's swiped away, with no clean stop,
  so those runs left crash.log too.
- **Seam:** Android's process lifecycle: an end that isn't a crash, which only Android
  can tell from one.
- **Fix:** on dd-exit-reason: at the next start Go asks Android why the process crash.log
  names ended (ApplicationExitInfo, Android 11 and later, `ExitReason`). Only a crash, a
  native crash or not responding bring the notice; other reasons go in the timeline, and
  crash.log with them. Before 11 it stays as it was.
- **Guard:** drive_android's crash_notice: Android ending the app in the background and a
  swipe-away bring no notice, a native crash of the app's process does; and
  `TestAProcessTheSystemEndedIsNoCrash` in core/crash.

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

## The keychain asks after every Mac update

**What he sees.** After an update, macOS asks "Project B wants to access key 'Project B device key'";
Always Allow answers it until the next update.

**The cause.** The app is signed with our own certificate, which has no Team ID. Since macOS 10.12 a
keychain item's access list carries a partition list, and for an app with no team that is
`cdhash:<the build that made it>`. Every build has a new cdhash, so it is outside the list until a
person allows it; Always Allow adds that build's cdhash. The designated requirement is already the
same for every build (`identifier "dev.projectb.browser" and certificate root = H"…"`), so signing
differently can't help.

**The decision.** Keep the sheet and say so (Sudipta, 2026-11-23): a line in every Stable release's
notes and a hint at first start. A Developer ID's Team ID would give a stable `teamid:` partition.

**The check.** `tools/vm/run.sh <0.3.9 DMG> upgrade` (PB_VM=pb-secret, PB_UPGRADE_TO=<new DMG>) prints
the item's partition list before and after the update.
