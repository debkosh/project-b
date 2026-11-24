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
3. `web_features_bump.py`: exit 1 stops it; a person decides.
4. Builds (`tools/build.sh`), then `gen_hooks.py`. Generated files that differ in more than
   their version line mean content's API moved: it stops and shows what changed. A compile
   error stops it with build.sh's known-failure hints. Otherwise it builds again with the new
   hooks.
5. Fetches ChromeDriver into `.tools/chromedriver`.
6. `tools/test.sh app`: the real engine through its driver and through ChromeDriver. It opens
   the app's windows on the desktop.

A patch within a major goes through with no hands after the yes. A new major stops at 3, 4 or
6 when it needs adapter work. After it: commit with `Notes: Chromium <version>, with its
security fixes`, and hand the branch to release, which runs the release run
([releasing.md](releasing.md)) and takes DuckDuckGo's field matching as at every release.

Not in it, for now: worktrees, the VM, Android and Linux, watching for releases,
notifications, publishing.
