# Upgrading Chromium

`tools/chromium-upgrade.sh` runs the first two steps below with the fetch and the build; the
section after the list says how. By hand, this is what to run when a new version of content
is taken, besides the build itself. Each step's own doc has the detail; this is the list, in
order.

1. `python3 tools/web_features_bump.py`: what the web-features registry lacks and what it names
   that Chromium dropped ([web-features.md](web-features.md), "After a Chromium bump"). It
   compares with the hooks table, so it goes before the next step, and it needs no build.
2. `python3 tools/gen_hooks.py` against the fresh `out/pb`: content's hooks as Go knows them
   ([content-hooks.md](content-hooks.md), "When Chromium is upgraded"). Check the result in and
   rebuild.
3. `tools/form-matching.sh update`: DuckDuckGo's latest field matching, which finds a page's
   sign-in fields ([password-autofill.md](password-autofill.md), "Detection"). It is checked
   in a real Chromium, so it is measured again on the new one: the script prints the scores
   before and after. Then `tools/test.sh mock`, and commit the three files it names.

## tools/chromium-upgrade.sh

One command moves a checkout and the Chromium tree it builds on to another version: the newest
Stable by default. A person runs it and answers it.

What he asked for, 2026-11-24: "a new security release should not ideally need an llm or
human to do manual stuffs... if our architecture holds this should be possible"; "the
upgrade is manually triggered, we can later build notifications".

His decisions, 2026-11-24:

- "no work trees involved in this... a human is gonna run that... so we bump the current
  stuff and not work trees"; "we do not have to complicate right now with vm builds".
- "build one platform build at one go... if another platform is supported the script takes a
  param". Mac only for now; Android and Linux are a param each, later.
- "I need to be able to interact with the script... it first tells me we are currently on this
  and this is available as stable... we can build in these platforms from here".
- "we should just be building the engine and down parts... rest should theoretically be
  working... so testing also should be only for the engine interface; if the interface is
  good then we should be good."
- It commits nothing; it ends with what changed, and a person commits it with the Notes line.
- On moving Android and Linux too, rather than a Mac-only release, 2026-11-24: "i would say
  we upgrade everyone".

### Running it

    tools/chromium-upgrade.sh [platform] [version]

It talks first and changes nothing before a yes:

    Project B is on Chromium 155.0.8059.41.
    Newest Stable is 158.0.8127.89, out 2026-11-24 (chromiumdash, Mac).
    From this Mac you can build: mac. Not yet in this script: android linux.
    ChromeDriver: 158.0.8127.89.
    Platform [mac]:
    Upgrade mac to 158.0.8127.89? [y/N]

The newest Stable is chromiumdash's for Mac. ChromeDriver is the exact version when Chrome for
Testing has its file (its list lags the files), else the newest of the same major, which
drives any browser of its major. Every step prints what it does and keeps its log in
`build/chromium-upgrade/<version>/`. A stop prints `RESULT=stopped <step>: <why>`; fix the
cause and run the same line again, and it carries on (a fetched tree isn't fetched again, and
the build is incremental). The end is `RESULT=ok <version>` and `git status` of what changed.

### The version lives in one file

`CHROMIUM_VERSION` at the root, one line, as `VERSION` is for the app. The fetch scripts,
`fetch-extensions.sh`, `drive_chromedriver.py`, `package.sh`'s bundle version, the README and
`requirements.prompt` read it; `android-content-fetch.sh` is given it by its caller (`TAG=`).
`tools/build.sh` stops when the tree's `chrome/VERSION` is another version, which is what keeps
the one place true. The generated hooks' headers carry the version gen_hooks.py read; docs
that record a past measurement keep theirs.

### What it does, for mac

Chromium is under the engine interface ([engine-interface.md](engine-interface.md)), so it
builds the engine and what is under it, and tests that interface; the core and the shells
above it don't change.

1. Writes `CHROMIUM_VERSION`.
2. Fetches `chromium/src` at the tag in place (`tools/fetch-chromium.sh`), under the build
   lock: every checkout on this Mac builds on this tree, so other builds wait.
   With a remote build server answering, it puts the new clang there
   (`tools/remote/toolchain.sh`).
3. `web_features_bump.py`: exit 1 stops it; a person decides.
4. Builds (`tools/build.sh`), then `gen_hooks.py`. Generated files that differ in more than
   their version line mean content's API moved: it stops and shows what changed. A compile
   error stops it with build.sh's known-failure hints. Otherwise it builds again with the new
   hooks.
5. Fetches the test tools for the version, ChromeDriver and the mock store's extensions
   (`tools/fetch-test-tools.sh`), then packs the app (`tools/package.sh`), which the test
   runs as people get it. Every other checkout's `.tools/` is still for the old version:
   `tools/fetch-test-tools.sh` there brings it to the tree's.
6. `tools/test.sh app`: the real engine through its driver and through ChromeDriver. It opens
   the app's windows on the desktop, so it asks first; a no stops there, and the same line
   again later runs it (the build is already done by then).

A patch within a major goes through with no hands after the yes. A new major stops at 3, 4 or
6 when it needs adapter work. After it: commit with `Notes: Chromium <version>, with its
security fixes`, and hand the branch to release, which runs the release run
([releasing.md](releasing.md)) and takes DuckDuckGo's field matching as at every release.

### Then android and linux

`tools/chromium-upgrade.sh android`, then `linux`, once the Mac's run has ended green: the web
features and the hooks are checked on the Mac's tree, and `CHROMIUM_VERSION` follows it. Both
build in container pb from one tree, `/work/chromium`, so `android` moves it
(`tools/android-content-fetch.sh`, under the container's build lock, with the pb-android VM
held up and lowered to nice 15) and `linux` syncs it again, which is quick. Each builds
(`tools/build.sh <platform> arm64`), and after asking, runs its driver: the APK on Android 17
(`tools/test.sh android`), the Linux app headless (`tools/linux/headless.sh`). Both need the
tester's slot. The container's builds stop when `/work/chromium` isn't at `CHROMIUM_VERSION`,
as the Mac's does.

Not in it, for now: worktrees, the macOS VM, watching for releases, notifications,
publishing.

### The first run: 155.0.8059.41 to 158.0.8127.89, 2026-11-24

Three majors at once, so it stopped where a new major should, and each stop that a later run
would meet again became a step of the script or a fix in the tool it called:

- chromium.googlesource.com answered 429 to the sync at 32 jobs: `fetch-chromium.sh` syncs 8
  at a time and tries again up to 3 times.
- A sync that failed had already checked out `src` at the new tag, so `chrome/VERSION` said
  done: the script fetches every time.
- The new clang wasn't on the remote build server, which runs on this Mac:
  `toolchain.sh` copies locally with `PB_RBE_SSH=local`, and the script runs it.
- 158's siso refused the remote compile of libc++'s clang module, whose depfile names the
  worker's clang folder: `build.sh` builds with `use_clang_modules = false`.
- The old `hooks_gen.h` was itself among what didn't compile: `gen_hooks.py` runs after a
  failed first build too, and the stop names what moved.
- The adapter moved by hand, in one commit with the version and the hooks: find in page
  (content numbers its own requests; the reply is the observer's `DidReceiveFindReply`),
  `ChildProcessId`, `CreateLoginDelegate`'s `do_not_prompt_for_login`, `EventArgs` for
  extension events, `GetBrowserContextLifetimeTracker` (none), and the window dispositions'
  new names.
- The test needs the mock store for the version and the packed app: the script fetches the
  one and packs the other.

`web_features_bump.py` on 158 found no missing interface and no gone name, and 17 new
delegate methods, which the web features session takes to him as rows.

Still to do: Android and Linux build in container `pb` from its own tree, `/work/chromium`,
still at 155, and `engine/` now compiles against 158 only. Until that tree moves, main can
build one side or the other, not both, and `tools/release.sh` builds both.
