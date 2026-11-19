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
| 1 | The thin wrapper, `PBInit`, Mac then Android | building |
| 2 | `core/system`, the OS managers' door, with its fake | |
| 3 | Notifications through Go | |
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

To fill in at step 2, from the brief's table.

## The exceptions: where the engine still touches the OS

Only these, so OS code doesn't creep back into the adapter:

1. The window (`window_mac.mm`), which holds Chromium's page views and shares input with
   them.
2. Panels attached to a window: print, Save as, Share, the page's file chooser, the folder
   chooser.
3. The Mac menu bar, and quitting from the Dock or at logout.

The clipboard stays in the engine: Chromium has it for every platform.

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
