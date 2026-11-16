# Releasing

How a release is put together, tested and published, as the build and release session ran
0.2.51 to 0.2.74. What `tools/release.sh` itself does is in
[updates.md](updates.md#publishing).

## Putting a release together

Feature sessions hand over branches. Each release is a branch `next-<version>` cut from main,
in a worktree of its own (main's checkout stays on main, since the release reads it):

- Take each branch's commits that main hasn't got yet, oldest first: `git log --reverse
  --cherry-pick --right-only HEAD...<tip>`, then cherry-pick each. A commit already picked
  with a resolved conflict has a different patch, so skip any whose subject is already in the
  last few hundred commits.
- Resolve conflicts that are two sides adding at the same place by keeping both: a message
  name added to the SDK's update list, an export, a docs row, a test function. A Go test file
  where both sides appended a test often loses a closing brace at the join; `gofmt -l` finds it.
  A conflict that changes behaviour on both sides goes back to the branch's owner to rebase.
- Then, once per release candidate: `go vet` and `go test ./...` in `core`, `tools/build.sh`,
  `tools/test.sh mock`, `tools/package.sh` into a copy, and `tools/android-release.sh` for the
  APK. Push the branch so the owners can rebase onto it.
- A release that waits on a fix stays open; new work goes into the next one instead, so the
  test run isn't started over.

## The release run

The cut-back run, Sudipta's standard from 0.2.64, on the packaged app and the APK, not on
`out/pb`:

- `PB_BIN=<pkg>/Contents/MacOS/project_b tools/test.sh app`: the driver, ChromeDriver and
  extensions.
- `PB_DMG=<dmg> tools/test.sh vm`: the VM. Run it after the app group, not beside it, since the
  app's windows can cover the VM's.
- `PB_APK=<apk> EMULATORS='project-b-emu-37:5606' tools/test.sh android`, adding
  `project-b-emu-29:5604` when the change is Android's.
- `tools/test.sh basics` only when a daily-use feature lands.

Run each under amarit. A failure is either a test the change didn't follow (a selector or a
label it removed) or the app; the test's owner fixes the first with a tools-only commit and only
the failed parts run again. Android 17's one known flake is the blank-shell start-up race in
[todo.md](todo.md).

## Publishing

    amarit rm pb-release
    amarit run -name pb-release -restart never -dir <main checkout> /bin/bash -c \
      "env HOME=$HOME KEYKOSH_ROOT=$KEYKOSH_ROOT PATH=$PATH caffeinate -i python3 -I \
       ~/PW_TOOLS/project-b-release/locked-release.py > ~/PW_TOOLS/project-b-release/<v>.log 2>&1"

Before it: fast-forward main to the release branch, bump `VERSION`, commit "Bump the version
to <v>" and push. `locked-release.py` takes `chromium/src/out/.pb-build.lock` and runs
`tools/release.sh` in the checkout amarit starts it in. Don't merge into main while it runs; it
reads that checkout. It takes about four minutes.

## Shared machines

- **out/** is one build folder for every checkout. `tools/build.sh` holds its lock for the
  whole build and copies the app it made into the checkout's own `build/pb/`; test that copy or
  a package, never `out/pb` directly. A copy stops sharing blocks with `out/pb` once `out/pb` is
  rebuilt, so each checkout's copy costs its full size (about 455 MB).
- **The Android VM** (colima `pb-android`, container `pb`): `tools/android-content-build.sh`
  takes `/work/.android-build.lock`, so a second build waits, and `tools/android-release.sh`
  stops the VM only when no build holds that lock. A checkout older than those scripts doesn't
  take the lock.
- **The desktop and the emulators** have no lock. One session at a time: the release session
  hands out slots. Three drivers on one emulator made false failures more than once.

## Traps

- After `amarit rm X; amarit run -name X ...`, check the output says `running, pid` and the
  log's first line is new. If it printed `unit X: ...` instead, nothing started, and a wait for
  it returns at once over the last run's output. Twice that nearly passed off an old APK as a
  new one: check a new build's file is newer than its start, or differs from the last.
- A commit id given in a message may be a typo: check out the branch's tip from origin, or
  check the id against it, before building.
- GitHub sometimes refuses a push ("missing necessary objects", "failed"); pushing again works.
- Never a bare `git stash`: the stash is shared with every worktree.
- A new worktree has no `chromium` or `depot_tools`: link both to the main checkout's
  (`ln -s ../project-b/chromium chromium`, the same for `depot_tools`) before
  `tools/build.sh`, which otherwise reports them missing; a `chromium/` folder the build's
  lock left behind goes first. `.tools` links to the main checkout's too, for the mock suites.
- When a shell control goes away, search the tools for it by every name they may use: its
  label, its class (`button.settings`) and its text. The real-app driver and the VM check find
  controls by CSS selectors, and a missed one stops their run early.
- Test races seen in this repo, each fixed in the test, not the app: a tab closed right after
  its page sent a message (the message goes on the tab's channel, which a closed tab no longer
  has); a setting read before a shell's background prefs write landed; a tab navigated before
  its first page loaded, so the page replaced it in history. Wait for Go's state, a file Go
  wrote, or the page's title, never a fixed time.
