# Blocking: lists, the ad blocker and Safe Browsing

**Status: a spec for the owner's approval. Nothing here is built yet.**

The owner, 2026-11-27: "we need to maintain a list of lists for safebtowsing and also the
adblocking ... similar lists can be ised for safe browsing right the the hook would be
navigation events", then "ok lets use brave lib", then "the lib should be outside our engine
interface... meaning no new content build for adblocker update and leaves room for us to
maybe replace it later".

So one blocking library, Brave's adblock-rust, decides three things from published filter
lists: which of a page's requests go, what parts of a page are hidden, and which sites get a
warning before they open. It lives on the core side, not in the engine. The engine gains a
few generic hooks that would serve any blocker, and the lists come from our own GitHub
releases, built by a scheduled workflow.

## What there is today

- `core/adblock/hosts.txt`: 187 hand-kept hosts, bundled and updated with the app.
- Go hands the engine the hosts and the sites where the blocker is off
  (`SetAdRules`, `core/app/sitefeatures.go`). The browser passes them to every renderer, and
  each renderer's request throttle (`engine/common/renderer_client.cc`, `BlockThrottle`)
  refuses a third-party request to a listed host as `ERR_BLOCKED_BY_CLIENT`, counted for the
  tab over `AdReports` ([site-features.md](site-features.md#the-ad-blocker)).
- **Block ads** (`blockAds`) in Settings, and the per-site switch in the site panel.
- No hiding of page parts, no scriptlets, no Safe Browsing. The basics check 28 (a malware
  test page warns) is a known failure, and the onboarding took Safe Browsing out: "no
  safebtowsing should not use google" ([onboarding.md](onboarding.md)).

All of that stays as the person sees it. What changes is where the rules come from and what
they can say.

## The pieces

```
GitHub (scheduled workflow) ──lists release──▶ core: lists manager (Go)
                                                 │ downloads, checks the signature,
                                                 │ keeps the files, picks what is on
                                                 ▼
                                   libpbblock (Rust, adblock-rust inside)
                                     ▲             ▲             ▲
                 request verdict ────┘   page extras┘  navigation │ (Go asks it)
                        │                     │                   │
   engine: renderer throttle ──mojo──▶ browser blocker sequence   │
           frame commit + watcher ─────────────┘           Go's navigation gate
```

1. **`libpbblock`**, our small Rust library in `core/blocking/lib/`, a few hundred lines
   around adblock-rust. It holds the loaded engines and answers through a plain C interface.
2. **The lists manager**, Go in `core/blocking`: the catalogue, the person's choices, the
   downloads, the files on disk, and which engines are loaded.
3. **The engine's generic hooks**: a request verdict, page extras, and the class and id
   watcher's query. The engine knows nothing of filter syntax or of adblock-rust.
4. **The lists release** on GitHub, made by a scheduled workflow.

## The library, outside the engine

**Decided by the owner:** adblock-rust is not compiled into `engine/`. A new version of the
library, a fix in our wrapper, or a different library altogether never needs Chromium
compiled again, and a new list needs no build at all.

- **A shared library, not a static one.** `libpbblock` is a Rust `cdylib`: `libpbblock.dylib`
  in the Mac bundle's Frameworks, `libpbblock.so` beside `libcore.so` on Linux and in the
  APK's `lib/arm64-v8a` on Android. Chromium 158 links its own Rust standard library into our
  binary; a Rust static library linked beside it would bring a second copy, and the two
  clash at link time. A shared library keeps its Rust to itself and exports only C symbols.
- **Built with core.** `engine/build_go.py` runs `cargo build --release --locked` for the
  target (the NDK's clang as the linker on Android, as it does for Go). The crates are pinned
  by `Cargo.lock` and its checksums and fetched as Go's modules are, not copied into the
  repo. A change to it rebuilds core and relinks, as a Go change does today. The Rust
  toolchain is pinned in `core/blocking/lib/rust-toolchain.toml` (1.100.0), apart from
  Chromium's own, whose standard library is built inside the content build.
- **Core links it**, and is the only part that does: Go calls it through cgo for the lists
  manager and for Safe Browsing, and hands the engine the addresses of its hot-path functions
  as one table (below). Replacing the library means another library with the same C
  interface; nothing in the engine or the shells changes.
- **Its C interface** (`core/blocking/lib/pbblock.h`), in outline:

```c
// Loading, from Go, on any one thread. compile turns a group's list texts into the
// library's own serialized form, which Go keeps on disk and loads at the next start.
PBBlocker* pbblock_new(void);
int  pbblock_compile(const uint8_t* const* lists, const size_t* lens, size_t n,
                     uint8_t** out, size_t* out_len);
int  pbblock_load(PBBlocker*, const char* group, const uint8_t* data, size_t len);
void pbblock_unload(PBBlocker*, const char* group);
int  pbblock_resources(PBBlocker*, const uint8_t* json, size_t len);

// The hot path, from any thread, never blocking: each reads one snapshot of the rules.
int   pbblock_request(PBBlocker*, const char* url, const char* source, const char* type,
                      char** redirect);           // 0 allow, 1 block, 2 redirect
char* pbblock_page(PBBlocker*, const char* url);  // JSON: css, scripts, generic
char* pbblock_hide(PBBlocker*, const char* page_url, const char* classes_ids_json);
void  pbblock_free(char*);
```

- **Snapshots.** Loading a group builds the new engine aside and swaps it in at once
  (`arc-swap`); a check under way finishes on the rules it started with. Nothing on the hot
  path takes a lock that loading holds.
- **Two blockers.** Go makes one `PBBlocker` for the network and page rules, and a second
  for Safe Browsing, so a Safe Browsing list never hides parts of a page or blocks a request,
  and an ad list never warns before a site.

## The engine's hooks

Every call across `pb_abi` is on the main thread today ([engine-interface.md](engine-interface.md)).
The request verdict can't be: a page asks for hundreds of things, from renderer threads,
while the main thread is busy drawing. So it is the one new kind of call, and the header
says so: **a blocker table, whose functions the engine may call on any thread, which must
not block and must not reach Go.**

```c
typedef struct PBBlockerHooks {
  void* blocker;
  int   (*request)(void*, const char* url, const char* source, const char* type, char** redirect);
  char* (*page)(void*, const char* url);
  char* (*hide)(void*, const char* page_url, const char* query_json);
  void  (*free)(char*);
} PBBlockerHooks;

// Main thread. NULL hooks: no blocking anywhere (Block ads off).
void set_blocker(const PBBlockerHooks* hooks);
// Main thread. The rules changed: renderers drop what they remember.
void blocker_changed(void);
```

`set_ad_rules` goes. The sites where the blocker is off stay the engine's to hold as now
(`set_blocking_off(origins)`), pushed to every renderer, so a page where it is off asks
nothing at all.

### 1. Requests

- **Where it is asked.** The renderer's `BlockThrottle` stays where it is. For an http(s) or
  ws(s) request from a page where blocking is on, `WillStartRequest` defers the request and
  sends `(url, source, type)` to the browser over a new `project_b.mojom.Blocker` pipe, a
  `SharedRemote` so a worker's or a synchronous load's thread can use it too. `source` is the
  page's site, as today (`site_for_cookies`, else the initiator). `type` is the request's
  destination in adblock's words: `script`, `image`, `stylesheet`, `xhr`, `sub_frame`,
  `font`, `media`, `websocket`, `ping`, `other`.
- **Where it runs.** In the browser the pipe is bound on its own sequence in the thread pool
  (`USER_BLOCKING`), not on the main thread, which may be busy, and not on the IO thread,
  which carries every other message. There the engine calls `hooks->request` directly: C++
  to Rust, no Go, no thread hop.
- **The answer.** Allow resumes the request. Block fails it as `ERR_BLOCKED_BY_CLIENT` and
  counts it for the tab, as now. Redirect is in the interface from the start but treated as
  block until the resources file is served (below): a page script that a list replaces with a harmless stand-in is
  blocked instead.
- **The administrator's block rules** (`block_rules`) stay in the throttle, checked first,
  in the renderer, unchanged.
- **Redirects of a request** are asked again in `WillRedirectRequest`.

**What it costs.** One round trip from renderer to browser per request that isn't
remembered, plus the check:

- The check: adblock-rust's request match is a few microseconds on a desktop (Brave's
  published figures); call it 10 to 30 µs on a mid-range phone. To be measured.
- The round trip: two mojo messages and two thread wake-ups. On a desktop it is tens of
  microseconds; on a mid-range Android phone a few hundred, up to a millisecond or two when
  the phone is loaded. Requests go in parallel, so this is paid once per step of a chain
  (a script that loads a script), not once per request: a page five steps deep pays about
  five round trips, a few milliseconds on a phone.
- **Remembered answers.** Each renderer keeps the last 512 answers, by `(url, source,
  type)`, and asks nothing for a repeat: the same tracker on every frame, a page polling
  the same address. `blocker_changed` clears them.
- **No batching.** Requests arrive one at a time; holding one back to send several together
  would add the delay the batch saves. Mojo already carries several in flight.
- **Not asked at all:** our own pages, `data:`, `blob:`, `file:` and the app's scheme; any
  request on a page where blocking is off; everything when Block ads is off (no hooks).
- **The gate.** Before the branch ships, `docs/benchmarks.md` gains a page-load run (the
  benchmark pages, cold and warm, on the Mac and on emu-37 and a real phone) with the list
  set on and with Block ads off. If the median load is more than 5% slower than with the
  hosts list today, the check moves into the browser's URL loader factory (a proxy, as
  Brave's), which saves the round trip, before it ships.

### 2. Parts of a page (cosmetic filtering)

- **Per page.** When a frame's navigation has its response (`WillProcessResponse` in a
  navigation throttle), the engine defers it, calls `hooks->page(url)` on the blocker
  sequence, and sends the result to the frame's renderer with the commit, on the frame's
  own associated pipe, so it is there before the document starts. The renderer then, at
  document start: adds the hide selectors as one user stylesheet (`display: none
  !important`), and runs the scriptlets in the page's own world, before any page script.
- **The generic watcher**, as Brave's: most hide rules are by class or id on any site. A
  small script in an isolated world of ours (as the form script, `worlds.h`) reads the
  classes and ids in the page and, from a `MutationObserver`, those added later, at most
  once per animation frame, and sends only names it hasn't sent before. The engine passes
  them to `hooks->hide` on the blocker sequence and adds what comes back to the frame's
  stylesheet. The watcher's script is core's (`core/blocking/page/watcher.js`), handed to the
  engine at start as the form script is, so a different blocker can bring its own.
- **Off where the blocker is off,** and where a list says `generichide` or `elemhide` for
  the site.
- **What it costs.** `hooks->page` runs once per frame load, off the main thread, in the
  time the response takes to arrive; on a phone a few hundred microseconds. The watcher
  sends a handful of messages per page and does nothing on a still page.

### 3. Safe Browsing

- **A second blocker,** loaded with the badware and phishing lists only.
- **Open lists only, ours to keep.** The owner: "lets try to do the same for safebrowsing...
  we cannot and should not send data to google... so we maintain some opne lists". The
  badware and phishing groups sit in the same catalogue, kept the same way, published and
  patched the same way. To start, after each one's licence is read: uBO badware (GPL-3.0),
  URLhaus filter (from abuse.ch's URLhaus), Phishing URL Blocklist (phishing-filter,
  CC-BY-SA-4.0, from OpenPhish, PhishTank and others), and Phishing.Database (MIT) as a
  candidate. No list that needs an account or an API key, or that asks to be told what was
  checked.
- **Checked at each release too.** With no Brave catalogue to compare against here,
  `tools/lists-vs-brave.py` reports for these groups each list's last change and size, so a
  list that stopped updating or shrank sharply is seen and replaced.
- **Asked by Go, at the gate it already has.** Every navigation of a tab's main frame, and
  each of its redirects, already asks Go (`PB_EVENT_NAVIGATION_STARTING`,
  `engine/browser/events.cc`). Go's handler asks the Safe Browsing blocker whether the
  address matches (a cgo call, microseconds, on the main thread). On a match Go refuses the
  navigation and shows the tab's notice, as for a bad certificate: kind `unsafe`, "example.com
  may harm your device" or "example.com may be trying to steal your information", which list
  said so, and Go back (primary) and Open anyway. Open anyway lets that host through until
  the app quits, kept nowhere, as the certificate warning does. **The engine changes not at
  all for this.**
- **Not checked:** frames inside a page and a page's requests. The badware lists' hosts also
  sit in the network rules (uBO's badware list is in the standard set), so a page's request
  to one is blocked there.
- **What it can't do.** Lists are hours old, and many phishing pages live for hours. It
  catches what the public lists know, which is less than a service that asks about every
  address as it opens. It sends nothing anywhere: the whole check is on the device.
- **A switch,** "Warn before dangerous sites", in Settings, Privacy and in the welcome, on by
  default. Off, Go doesn't ask and the lists aren't downloaded.

## The lists

### The catalogue

**Decided by the owner, 2026-11-27:** "lets then maintain the list ourselves and during a
release we check that against brave for any aditions and removals and the releaser can
check if it needs to change our list".

- **Ours, kept by hand:** `core/blocking/catalog.json`, in the repo, started once from
  Brave's public catalogue ([brave/adblock-resources](https://github.com/brave/adblock-resources),
  `filter_lists/list_catalog.json`, 99 sources: uBlock Origin's, EasyList, EasyPrivacy, the
  regional lists). Only a commit changes it; the workflow reads it and never writes it.
- **Checked against Brave's at each release.** `tools/lists-vs-brave.py` fetches Brave's
  catalogue and prints what differs from ours: lists Brave added or dropped, lists it turned
  on or off by default, and sources whose address moved. `release.sh` runs it and shows the
  result. It never stops a release: the releaser reads it and, where ours should follow, sends
  the catalogue change as its own commit. Lists we chose to leave out are named in the
  catalogue's `ignored` section with a reason, so the check doesn't raise them again.
  `docs/releasing.md` and `roles/release.md` gain the step.

```json
{
  "id": "easyprivacy",
  "title": "EasyPrivacy",
  "group": "trackers",
  "tier": "standard",
  "languages": [],
  "sources": [{"url": "https://easylist.to/easylist/easyprivacy.txt", "format": "standard"}],
  "licence": "GPL-3.0-or-later OR CC-BY-SA-3.0",
  "home": "https://easylist.to/"
}
```

**Groups,** each one loaded engine. The owner: "extreme once on a seperate group", so the
lists that break sites more often are a group of their own, off, rather than a tier inside
each group:

| Group | Blocker | Lists | On at first |
|---|---|---|---|
| Ads | network and page | EasyList, uBO filters, uBO unbreak, Brave's unbreak | yes |
| Trackers | network and page | EasyPrivacy, uBO privacy, Peter Lowe's | yes |
| Regional | network and page | one per language: the app's languages' on, the rest off | the app's languages' |
| Annoyances | network and page | cookie notices, pop-ups, social widgets (Fanboy's, uBO annoyances) | no |
| Extreme | network and page | AdGuard URL tracking, uBO's resource abuse, and any list that breaks sites often | no |
| Dangerous sites: malware | Safe Browsing | uBO badware, URLhaus filter | yes |
| Dangerous sites: phishing | Safe Browsing | Phishing URL Blocklist, Phishing.Database | yes |
| test | both | our own list: one test address per hook, for the checks; never shown | only in tests |

Our own `hosts.txt` stays as the built-in list (below) and as list `pb-hosts` in group ads.

### The person's choices

The owner, 2026-11-27: "warning default on but welcome screen offers choice same as the
adblocker... also we should be able to configure the lists i want... shown cleanly as groups
and individual lists".

- **Settings, Privacy:**
  - **Block ads**, as today: all of the network and page rules on or off.
  - **Warn before dangerous sites**: Safe Browsing, on by default.
  - **Lists**, one row that opens a page of its own.
- **The Lists page:** the groups, in the table's order, each a row with its switch and how
  many of its lists are on. Opened, a group shows its lists, each a row with its own switch,
  its title, one line on what it blocks, its home, when it last changed and its size.
  A list's own switch leaves the rest of its group as it is. **A group's switch** (ux's
  rule, the owner's "nice go ahead", 2026-11-27) is on while any of its lists is on, and its
  count says when only some are; off turns all its lists off; on brings back the group's
  defaults, for Regional the app's languages, not every list. Each group's line is what it
  blocks, then " · N of M on", the count never broken across lines. Extreme carries one line: "These block more and may break some sites."
  The Safe Browsing groups show under Warn before dangerous sites, greyed while it is off.
  The test group is never shown.
- **The welcome** offers both, on the Protection step: the ad blocker and Warn before
  dangerous sites, each a switch, both on ([onboarding.md](onboarding.md)).
- **The site panel's switch** stays as it is, and turns off requests and page parts for that
  site. Safe Browsing has no per-site switch; Open anyway is the way past one warning.
- Prefs per profile, as every pref (one process per profile, [profiles.md](profiles.md)):
  `blockAds`, `warnDangerous`, and `lists`, only the lists the person changed from the
  catalogue's default, so a list the catalogue adds later takes its own default.
- The Lists page, the welcome's switches and the unsafe-site notice go to ux for review
  before they are built, with mocks.

### Building and publishing them

The owner: "the apps cna then download our signed package.. we need to keep it so that
deltas can be downlaoded or we split those up so small change should not send the whole
thing".

So the package is split by list, and a list that changed sends only its change:

- **A scheduled workflow,** `.github/workflows/lists.yml`, every hour and by hand. It
  reads the catalogue, downloads each source (refusing one that shrank by more than half or
  failed to parse, and keeping the last good one), checks each parses with the `libpbblock`
  the app pins, and publishes the release, tag `lists`.
- **One file per list, as text.** Each list is its own file, `<list>-<sha>.txt.zst`, named
  by its content's SHA-256 and compressed with zstd. Text, not the library's serialized form:
  a few changed lines stay a few changed lines, where a rebuilt engine changes all through.
- **A patch per change.** For each list that changed, the workflow also publishes a patch
  from each of its last eight versions to the new one, `<list>-<old>-<new>.zst`: zstd with
  the old text as its dictionary ("patch from", klauspost/compress in Go on both sides, so
  no diff format of our own). A day's change to EasyList is then kilobytes, not its whole
  megabyte. An app further behind than eight versions takes the whole file.
- **The resources,** `resources-<sha>.json.zst`, the scriptlets and stand-ins, published and
  patched the same way: put together as Brave does, from pinned commits of uBlock Origin
  (its redirect resources, read by adblock-rust's assembler, and its scriptlets, read with
  node, `tools/lists/ubo-scriptlets.mjs`) and of Brave's `dist/resources.json`
  (`core/blocking/lib/examples/resources.rs`).
- **Trusted lists.** uBO's `trusted-*` scriptlets set cookies and rewrite what a page
  fetches, so, as uBO allows only its own lists to use them, only a list the catalogue marks
  `trusted`, one all of whose sources are uBO's or Brave's own, may; the library compiles
  each list with or without that permission.
- **`manifest.json`**: each list's current SHA-256 and size, and the patches there are to it.
  **`manifest.json.sig`**: its Ed25519 signature. **`LICENSES.txt`**: each list's licence and
  home. Files no manifest of the last eight names are deleted from the release.
- **Signed.** Lists carry scripts that run in every page, so the app takes only a manifest
  signed with our key, whose public half is compiled into core (Go's `crypto/ed25519`). The
  owner: "the keys can be stored in our keykosh or reuse the app sogning keys if possible".
  A new key of its own, `lists-signing.key`, sealed in the kosh beside the app's, and the
  workflow's one secret. Not the app's keys: the workflow runs on GitHub every hour, so its
  key sits in GitHub's secrets, and the APK's keystore, the Mac identity or the Linux repo
  key there would let whoever took it sign app updates. This one can only sign lists, and a
  new one goes out in the next app. Every list the app ends with, whole or patched, must match the signed manifest's
  SHA-256, or it is thrown away and the whole file fetched.
- **Only GitHub.** No server of ours. The app fetches
  `github.com/debkosh/project-b/releases/download/lists/...`.

### Fetching them

- **After the shared start-up wait** ([startup.md](startup.md)): `later.Run(lists.Start)`.
  Then every hour while the app runs, so a phishing list is at most about two hours old. It
  asks for `manifest.json` with the last ETag, so most checks are one small reply that
  nothing changed. On a metered connection the ad lists come at most once a day; the Safe
  Browsing ones still come each hour, being small patches.
- **Only what changed, of the lists that are on:** for each, the patch from the version it
  has, else the whole file.
- **Decided by the owner, 2026-11-27:** "phones and desktop download the lists obly those
  whoch changes and builds rhe list and stores it locally as one or more files for quick
  load on restart". Every platform the same; no ready-made engines in the release.
- **Compiled on the device, a group at a time.** A group whose lists changed is compiled
  again (`pbblock_compile`) on a background goroutine at low priority, written to disk in the
  library's serialized form, and swapped in. The serialized form never leaves the device, so
  a new library version only means the app compiles once more after the update; no release
  waits on the lists.
- **On disk,** in the app's shared folder beside the profiles list (`lists/`): the texts and
  each group's compiled engine. Profiles share them. A file is written aside and renamed into
  place; two profiles fetching at once only fetch twice.
- **Loaded at start,** before the first window's page, from the compiled engines on disk. A
  serialized engine loads in milliseconds (a FlatBuffer, read in place), start-up work in
  the same sense as the hosts list today. One compiled by another library version is left
  unloaded and compiled again after the start-up wait; until then that group's rules are off,
  the built-in hosts stay on.
- **Turning a group on** fetches and compiles it then, not after the wait: it is something
  the person did.

### The first run, and offline

- **Requests:** the built-in `hosts.txt`, made into rules (`||host^$third-party`) and loaded
  as group ads, so blocking is never weaker than today, even with no network ever.
- **Page parts:** none until the lists arrive.
- **Safe Browsing:** none until the lists arrive, usually within a minute of the first start.
  Settings says "Lists not downloaded yet" until then. No list is bundled: the public ones
  are copyleft and change by the hour.
- **No network for weeks:** the last lists stay in force. Settings shows each group's age.

## Size and memory, and Android

| | Desktop | Android |
|---|---|---|
| App size | `libpbblock` about 2 to 3 MB | about 2 to 3 MB in the APK, arm64 only; no list data in the APK |
| Download | the standard set's texts compressed, a few MB, once; after that the patches, tens of kilobytes a day | the same; over a metered connection, at most once a day |
| Compiling | a changed group, in the background, under a second (to be measured) | a few seconds at low priority for the whole set on the first run (Brave compiles its lists on phones too); a changed group after that |
| Memory | the standard set's engines, in the browser process only: about 15 to 20 MB (Brave's figure for its default set after its 2026 FlatBuffers work, which cut it by three quarters) | the same, once; Annoyances and Extreme add to it, so the Lists page shows each group's size |
| In each renderer | the 512 remembered answers and each frame's stylesheet: kilobytes | the same |

- **One copy.** The rules live only in the browser process. Putting a blocker in each
  renderer would save the round trip but cost a copy per renderer, which a phone can't pay.
- **Measured on the Mac** (M-series, adblock-rust 0.13.5, `cargo run --release --example
  measure`): EasyList, EasyPrivacy and uBO's filters and privacy lists, 4.3 MB of text,
  compile in 51 ms to a 6.4 MB engine, which loads in 1 ms; a request check takes 1.7 µs,
  a page's parts 85 µs. The library is 2.9 MB on the Mac and 3.3 MB for Android arm64 before
  the APK's compression. A phone is perhaps five to ten times slower; to be measured on one.
- **Loaded by copy.** adblock-rust copies the serialized engine into its own memory, so a
  group costs about its file's size in the browser process; mapping the file would save
  nothing.
- **The phone's checks** run with the same numbers in `docs/benchmarks.md`: page load,
  memory of the browser process with the standard set, and start-up time, on emu-37 and a
  real phone.

## Licences

- **adblock-rust: MPL-2.0.** File-level copyleft, which our MIT allows beside it. Its
  vendored files stay MPL, unchanged; ours are MIT. Its own dependencies are checked by
  `cargo deny` in the build (MIT, Apache-2.0, BSD, MPL-2.0 allowed).
- **The lists are never in the repo or the app,** except our own `hosts.txt` (MIT). They are
  data the app downloads from our release. EasyList and EasyPrivacy are GPL-3.0 or
  CC-BY-SA-3.0, uBO's lists and its scriptlets GPL-3.0, phishing-filter's lists CC-BY-SA-4.0,
  the regional ones each their own. The release carries each list unchanged in meaning,
  with `LICENSES.txt` naming its licence and home, and the workflow's sources say where each
  came from.
- **A licences page** in the app (Settings, About, Licences; there is none today) lists
  adblock-rust and its crates, and each list in use with its licence and home. Chromium's
  own notices go on the same page when it is made.
- **Before the branch ships,** the owner reads the licences table once; this is the reading of a
  developer, not a lawyer's.

## Delivery

The owner: "build it in one go". One branch, handed to release once, with all of it:
`libpbblock` and its build on the three platforms; the blocker table and the throttle; page
parts at commit, scriptlets and the watcher; Safe Browsing and its notice; redirect answers
served by the engine from the resources file; the lists manager with the built-in list; the
catalogue and its check against Brave's; the workflow with its patches and the signing key;
the settings, the Lists page and the welcome's switches; the page-load gate passed. Inside
the branch the work goes in that order, one change per commit, and the engine's part first,
as it is the one that needs a content build.

Its `Notes:` lines: "Ads and trackers are blocked from up-to-date public lists, which you
choose in Settings", "Hides the empty spaces ads leave on pages", "Warns before opening a
site known for malware or phishing, checked on your device".

## Tests

- **Go,** with the mock engine: the manifest's signature and hashes, a damaged or unsigned
  file refused, a patch applied and a bad patch falling back to the whole file, an engine of
  another library version compiled again, the swap, the ETag, the choices, the first-run
  hosts. `tools/lists-vs-brave.py` against a saved copy of Brave's catalogue.
- **Rust:** `cargo test` on `libpbblock`'s C interface, and that a snapshot holds across a
  swap.
- **The real app:** a test page under our own test list's addresses, for each hook: a
  request blocked and counted, an element hidden, a scriptlet run, a generic class hidden,
  an unsafe-site notice with Go back and Open anyway; the per-site switch and Block ads off
  for each. Desktop drivers and `drive_android`. On the desktop, `tools/check_basics.py`
  28 and 42 to 48: each starts with a `prefs.json` that turns the test group's lists on and
  every other list off, maps `*.blocking.test` to its own server, and waits up to 90 s for
  the timeline to say each test unit compiled.
- **The workflow:** a run on a branch publishes to a draft release, twice, so the second
  has patches; an app pointed at the draft takes the patches.

## Not doing

- No Google Safe Browsing, nor any service that is told the addresses a person opens.
- No blocker inside the engine or in renderers.
- No server of ours for the lists.
- No catalogue that follows Brave's by itself: the releaser decides each change.
- A list the person adds by address is a later question.
