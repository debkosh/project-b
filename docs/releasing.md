# Releasing

How a release is put together, tested and published, as the release session runs it (role in
[roles/release.md](../roles/release.md)). What `tools/release.sh` itself does is in
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
- Then, once per release candidate: `go vet` and `go test ./...` in `core`, with `-tags sqlite_fts5` as every Go build has it ([store.md](store.md#how-it-works)),, `tools/build.sh`,
  `tools/test.sh mock`, `tools/package.sh` into a copy, and `tools/build.sh android arm64` then
  `tools/package.sh android` for the APK. Push the branch so the owners can rebase onto it.
- A release that waits on a fix stays open; new work goes into the next one instead, so the
  test run isn't started over. While the candidate hasn't been tested yet, a branch that
  applies cleanly and adds no engine or Android risk can still join it.
- Two handed-over branches that conflict: the first in is the base. Push `next-<version>` with
  it and have the later owner rebase onto that and settle the conflict.
- Check the diff and the commit messages for the names in CLAUDE.md's rule before building:
  `git diff origin/main <tip> | grep -ciE '<names>'`, and the same over `git log --format=%B`.

## The release run

The cut-back run, Sudipta's standard from 0.2.64, on the packaged app and the APK, not on
`out/pb`:

- `PB_BIN=<pkg>/Contents/MacOS/project_b tools/test.sh app`: the driver, ChromeDriver and
  extensions.
- `PB_DMG=<dmg> tools/test.sh vm`: the VM. Run it after the app group, not beside it, since the
  app's windows can cover the VM's.
- `PB_APK=<apk> EMULATORS='project-b-emu-37:5606 project-b-emu-29:5604' tools/test.sh android`.
  emu-29 (Android 10) has one known failure, the full-screen gap ((0, 0, 1080, 2274) on a
  1080x2400 screen).
- `tools/test.sh basics` (32 of 41 areas; it exits 1 by design) and the tester's
  `quit_after_download.py` on the packaged app.

The APK step starts only once the candidate build is done: `tools/build.sh android` runs
`npm run build`, which writes the same `shell/dist`, and clears `/work/release` before it takes
its lock. The desktop run and the APK build then go side by side, and the Android run once the
APK exists.

Run each as an amarit job (`amarit job -name N -log F -- ...`), never an `amarit run` unit: a
unit is a service amarit starts again after its own restart, and on 2026-11-20 an update
replayed 311 finished ones. A failure is either a test the change didn't follow (a selector or a
label it removed) or the app; the test's owner fixes the first with a tools-only commit and only
the failed parts run again. Android 17's one known flake is the blank-shell start-up race in
[todo.md](todo.md).

## Dev builds

A finished branch lands on main as it comes, rebased, once Go's tests and the mock suite pass
on it, and `tools/release.sh dev` publishes it to the Dev channel from the main checkout,
as the `pb-release` job below ([release-channels.md](release-channels.md)). It runs Go's tests
and the mock suite again on main first, and stops at a failure; no tag, no test report, no
release notes. Send the orchestrator the DMG and APK links from its `RESULT=ok` line. Stable
is cut from main later, with the full run.

## Publishing

    amarit job -name pb-release -dir <main checkout> -log ~/PW_TOOLS/project-b-release/<v>.log -- \
      env KEYKOSH_ROOT=$KEYKOSH_ROOT caffeinate -i tools/release.sh

Before it: if main moved since the branch was cut, rebase the branch onto it. When the new
commits only touch docs or tests, check `git diff <tested tip> HEAD -- . ':!<those paths>'` is
empty and publish without a new run; when they touch the app, run the release again. Then
fast-forward main to the release branch, bump `VERSION`, make the release notes this
version's (`shell/src/apps/settings/release-notes.md`: `# <v>` and its changes, one `- ` line
each in a user's words; the branches in it add theirs under the heading, a branch with none
gets one here, and the last release's lines go), commit "Bump the version to <v>" and push.
Only this release's notes: the app shows them in Settings' About, What's new in <v>, and
`tools/release.sh` refuses notes of another version, publishes the file beside `releases.json`
as `release-notes.md`, and puts its lines in the GitHub release. `tools/release.sh` takes `chromium/src/out/.pb-build.lock` for its whole
run, waiting for any build that holds it, and releases the checkout the job starts in. Don't
merge into main while it runs; it reads that checkout. It takes about four minutes. Wait for `RESULT=ok v<version>`: `tools/package.sh` prints a
`RESULT=ok` of its own partway through.

## Shared machines

- **out/** is one build folder for every checkout. `tools/build.sh` holds its lock for the
  whole build and copies the app it made into the checkout's own `build/pb/`; test that copy or
  a package, never `out/pb` directly. A copy stops sharing blocks with `out/pb` once `out/pb` is
  rebuilt, so each checkout's copy costs its full size (about 455 MB).
- **The Android VM** (colima `pb-android`, container `pb`): `tools/android-content-build.sh`
  takes `/work/.android-build.lock`, so a second build waits, and `tools/build.sh android`
  stops the VM only when no build holds that lock. A checkout older than those scripts doesn't
  take the lock.
- **The desktop, the VM, container pb and the emulators** have no lock. One session at a
  time: the tester hands out slots. Ask once per release for all of them, and give each back
  as its step ends (force-stop `dev.projectb.browser` on an emulator first), or at once,
  unused, when the candidate fails. Three drivers on one emulator made false failures more than once.

## Traps

- A job's `-log` file is appended to, and its last line says how it ended (`amarit job N: exit
  0`, or `killed: ...`). Wait for that line, and give each run a new log file: in an old one a
  wait returns at once over the last run's end. A log with no such line, and the job gone from
  `amarit jobs`, means a reboot or a kill -9 ended it. Twice that nearly passed off an old APK as a
  new one: check a new build's file is newer than its start, or differs from the last.
- A commit id given in a message may be a typo: check out the branch's tip from origin, or
  check the id against it, before building.
- GitHub sometimes refuses a push ("missing necessary objects", "failed"); pushing again works.
- Never a bare `git stash`: the stash is shared with every worktree.
- The main checkout is the release's: never check out, reset or build there. `tools/release.sh`
  refuses unless it is on main and is origin/main. Before the bump, check `git branch
  --show-current` says main and `git status --porcelain` is empty.
- A release that adds a shell dependency (`shell/package.json`) needs `npm install` in the main
  checkout's `shell/` before `tools/release.sh`, and in any release worktree with its own
  `node_modules`; worktrees that link `shell/node_modules` to the main checkout's share it. Use
  `npm install`, not `npm ci`, there: other checkouts link to it.
- Go holds the shell's built pages (`core/shellfiles`, [layers.md](layers.md#5-the-shell-inside-go)),
  so a shell change rebuilds Go, and the app and the APK carry no `shell/` folder.
- A Go module marked `// indirect` that core imports directly is rewritten by every build
  (`-mod=mod`), leaving the checkout dirty; `go mod tidy` in `core` is the fix, in its own commit.
- A test killed from outside (SIGTERM) is not a failure of the app: rerun it alone through
  `tools/results.py run --rerun yes`, so the report takes the rerun.
- Android: never `adb kill-server`; the adb server is the amarit unit `android-adb` (`amarit
  restart android-adb`). If `tools/build.sh android` prints "already running, ignoring" and
  then a docker.sock error, `colima stop -p pb-android && colima start -p pb-android
  --activate=false`.
- To tell a flaky Android check from a regression, run the same parts on the previous release's
  APK on the same emulator (`ADB_SERIAL=... PB_APK=... python3 tools/drive_android.py <parts>`).
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
