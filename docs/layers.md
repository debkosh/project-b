# Three layers: wrapper, Go core, engine

The feature doc for the work. The brief, agreed on 2026-11-19 while discussing other
platforms, is `docs/research/layers.md` on branch `platforms-research`; this doc is where the
work stands and what was decided on the way. A choice in the brief that turns out wrong goes
back to the platforms session before it changes here.

## His words, 2026-11-19

- "we have a wrapper for each platform... desktops have main.cc android has main.java... then
  go core and engine... the engine built contains our adaptor and content... go core contains
  the managers and shell"
- "go can have managers which talk to the adapter and managers who talk to the os... engine
  only talks to go core"
- main.cc "should just be start gocore start content and thats it"
- "ok no rust for now"

## The model

- **The wrapper** starts the app and nothing else: `engine/app/main.cc` on the Mac,
  `BrowserActivity.java` on Android.
- **The Go core** (`core/`): the managers and the shell. Some managers talk to the engine
  through `engine.Engine`, some to the OS through `core/system`. Go decides everything.
- **The engine**: our adapter and Chromium's content, one library. It talks only to the Go
  core, and touches the OS only where Chromium's objects force it (the exceptions below).

## The order of work

Each step leaves Mac and Android working and the release run green.

| | Step | State |
|---|---|---|
| 1 | The thin wrapper, `PBInit`, Mac then Android | tested: Mac suites, Android 16 and 17; to release |
| 2 | `core/system`, the OS managers' door, with its fake | tested: Mac suites, Android 16; to release |
| 2b | Save as and the folder chooser through Chromium's own file dialog | |
| 3 | Notifications through Go | built on the Mac, branch `layers-notify` |
| 4 | The Mac's open-link events into `core/system`, or a fourth exception | |
| 5 | The shell embedded in the Go core | |
| 6 | The three-piece build, with the build-flow restructure below | |

Linux and Windows are not in this work: they come later, from the platforms session.

## 1. The thin wrapper

What `ProjectBMain` decided in C++ before content (`--version`, claiming the profile, the
saved flags, the switch that leaves crash signals to Go) moves into one Go call. The Mac's
`main`:

```c
int main(int argc, const char** argv) {
  const char** args = PBInit(argc, argv);  // the Go core starts
  if (!args) return 0;                     // it printed the version, or handed over
  return ProjectBMain(args, &go);          // content starts
}
```

- `PBInit` (`core/cmd/lib`) reads the command line as Chromium does (`core/cmdline`), prints
  `--version` itself ("Project B 0.2.x, Chromium 155.0.…"), claims the profile, and returns
  the finished command line: the one given, with the saved flags not given on it and, on the
  Mac, `--disable-in-process-stack-traces`. NULL means this copy quits. The list lives as
  long as the process, as `main`'s own `argv` does.
- Chromium's version is stamped into Go at build time from `chrome/VERSION`.
- `ProjectBMain(argv, go)` takes a NULL-terminated `argv`, which both binaries' `main`
  already have, and keeps only: the command line, Go's table, the copy's folder (from the
  table's `copy_dir`), the timeline, `ContentMain`. The helper's path is unchanged.
- `PBGoCalls` loses `claim_profile`, `version` and `launch_switches`, which only the wrapper
  needed, and gains `copy_dir`.
- Android: `BrowserActivity` calls `GoCore.init` before content starts, with the data folder
  as `--root` and its `profile` folder as `--user-data-dir`. It appends to content's command
  line the switches Go added (none today: Android has never applied saved flags or the
  signal switch, and still doesn't). `StartApp` no longer claims the profile.
- Read the same on both: `main` → `PBInit` → content → `PBStart` → `app.Start`.

## 2. `core/system`

The calls on `engine.Engine` that are really the OS's move to `system.System`, built per OS
in `core/system`, and their C++ leaves the adapter:

| Off `engine.Engine` | C++ that goes | `core/system` on the Mac | on Android |
|---|---|---|---|
| `OpenFile`, `RevealFile` | `downloads_mac.mm`, `downloads_android.cc` | NSWorkspace, in a `.m` beside the Go | Java, through JNI |
| `IsDefaultBrowser`, `AskDefaultBrowser` | `mac_app.mm`, `BrowserWindow.java`'s | NSWorkspace | Java's RoleManager sheet |
| `AskNotifications`, `NotificationsAllowed` | `BrowserWindow.java`'s | allowed, nothing to ask | Java's permission question |
| `SetDockColor` | `mac_app.mm` | NSApp's icon | nothing: no Dock |
| `OpenFullDiskAccess` | none, already Go | `open` System Settings | nothing |

- An answer comes back as a callback the caller passes, run on the main thread:
  `AskDefaultBrowser(func(isDefault bool))`, `AskNotifications(func(allowed bool))`. The
  `PB_EVENT_DEFAULT_BROWSER` and `PB_EVENT_NOTIFICATIONS` events, and the Listener calls
  `DefaultBrowserChanged` and `NotificationsAnswered`, go.
- On Android the OS's door is Java, so `core/system` reaches it through JNI in the Go
  library's own C (`go_table.c`, which already calls Java for the device key), not through
  the engine. The answers come back from the activity to Go the same way.
- On the Mac some of it is Objective-C, in `.m` files beside the Go, built by cgo. macOS
  allows its UI only on the main thread: `core/system` hops there itself, so no caller can
  get it wrong.
- `core/system` is the interface alone, so the managers build and test with no cgo; each OS's
  is `core/system/platform`, which only `core/cmd/lib` links; `core/mocksystem`, beside
  `core/mockengine`, is the tests' and the dev server's. It answers as a person who says yes,
  or, with `Dismiss`, as one who closes the question.
- The crash report's device section (`Report`) and the downloads folder come along from
  `core/cmd/lib`'s `system_*.go`, with the Full Disk Access and Dock files there.
- The drivers and tests watch through the app, not the events, so they stay as they are: the
  e2e reads the dev server's `dock`, `openedFiles` and `shown`, now the mock system's; the
  Mac driver's `dockdot` reads the real Dock icon; `drive_android.py` answers Android's own
  notifications question.

Agreed with the platforms session, 2026-11-19 (Linux will be the same shape, pure Go over
D-Bus, and Windows Win32 from Go):

1. Each callback runs on the main thread exactly once. An answer that never comes (a sheet
   dismissed, the activity recreated or destroyed while asking) calls it once with no answer.
2. JNI only from a thread attached to the JavaVM: `core/system` hops to the main thread
   before calling Java, so callers never think about it.
3. The drivers and tests that watched the old events (`drive_android.py`, the Mac driver,
   the e2e) move to the new path in the same commit, or a test-fix commit just before it.
4. The fake keeps the same callback behaviour, so Go's tests cover a dismissed or
   never-answered request.
- Then, in later commits, what is per OS in Go today moves behind it too: `core/instance`,
  `core/relaunch`, `core/update/bundle.go`, and `core/cmd/lib`'s `*_darwin.go` and
  `*_android.go`, so a reader finds every OS call in one place.

## 3. Notifications through Go

Today the adapter asks Go whether a page's notification shows (`PB_EVENT_NOTIFICATION_SHOW`),
then posts it to macOS itself (`notifications_mac.mm`), and a click goes from macOS to the
adapter, which fires the page's click event and tells Go (`PB_EVENT_NOTIFICATION_CLICKED`).
After:

- The engine still asks Go (`NotificationShown`): content's notification service is the
  engine's, and it needs the answer to fire the page's show event.
- Go posts a shown one itself, through `core/system` (`ShowNotification`: its id, title, the
  site's host under it, body), and takes it away there (`RemoveNotification`) when the page
  closes it, which the engine now says (`NotificationClosed`, a new event).
- A click comes from the OS to Go (`OnNotificationClicked`, set once at start). Go tells the
  engine (`ClickNotification`, a new call), which fires the page's click event, then brings
  its tab forward, as today. `PB_EVENT_NOTIFICATION_CLICKED` and the Listener's
  `NotificationClicked` go.
- The Mac's notification centre code, its delegate and the no-bundle check move from
  `notifications_mac.mm` into `core/system/platform`; the file goes. A phone shows no page's
  notification yet, so Android's `ShowNotification` does nothing, as today.

## The exceptions: where the engine still touches the OS

Only these, so OS code doesn't creep back into the adapter:

1. The window (`window_mac.mm`), which holds Chromium's page views and shares input with
   them.
2. Print and Share, attached to a window.
3. The Mac menu bar, and quitting from the Dock or at logout.

Plus the links and files the OS hands the app, if step 4 can't move them into Go.

The clipboard stays in the engine: Chromium has it for every platform.

## 2b. Chromium's own file dialog

His decision, 2026-11-19, through the platforms session: "yes lets do that". Downloads' Save
panel (`downloads_mac.mm`) and Go's `ChooseFolder` (`choose_path_mac.mm`, which Load unpacked
shares) go through `ui::SelectFileDialog`, as `file_chooser.cc` already does for a page's file
input (`SELECT_SAVEAS_FILE`, `SELECT_FOLDER`). Both Mac files go, and Linux and Windows get
both choosers for free. The drivers that answer them move with it.

Why we wrote our own, from their history and docs: a sheet on the window, a folder to start
in, the `.crx` type for Load unpacked, the driver's `choose` answering in place of the panel,
and, for Import's folder, showing hidden files so a browser's folders under `~/Library` show
(docs/onboarding.md). Chromium's Mac dialog (`select_file_dialog_bridge.mm`) does the first
four: a sheet on its owning window, a default folder, file types, and the driver's answer stays
in front of it. It has no way to show hidden files.

So Import's chooser starts where the browsers keep their folders, which shows them at once,
agreed with the platforms session, 2026-11-19: `~/Library/Application Support` on the Mac
(nothing inside `~/Library` is hidden, only `~/Library` itself; Cmd+Shift+. still shows the
hidden ones). On Linux, later: `~/.config`, where Chrome's, Chromium's and the forks' folders
are; Firefox's is `~/.mozilla`.

## The build-flow restructure

Given to this work by the orchestrator on 2026-11-19, after step 1, as the platforms session
planned it:

1. One entry point, `tools/build.sh <platform> <cpu>`, mac arm64 by default; `out/pb` and
   `build/pb` stay as they are.
2. The three pieces named in the build: the wrapper, `libcore`, the engine library.
   `build_go.py` takes GOOS and GOARCH from the target. (Step 6.)
3. `tools/package.sh <platform>`: the Mac as today, Android folding in
   `android-content-package.sh`.
4. Later: Linux in container pb, with a slot from the tester.
5. `out/<platform>-<cpu>`, switched in one commit together with release's scripts.

Release and the orchestrator hear before any change to the `out/` or `build/` paths.
