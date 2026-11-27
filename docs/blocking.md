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
  target (the NDK's clang as the linker on Android, as it does for Go), with the crates
  vendored in the repo (`cargo vendor`) so a build needs no network. A change to it rebuilds
  core and relinks, as a Go change does today. The Rust toolchain is pinned in
  `rust-toolchain.toml`, beside Chromium's own, which we don't use for it.
- **Core links it**, and is the only part that does: Go calls it through cgo for the lists
  manager and for Safe Browsing, and hands the engine the addresses of its hot-path functions
  as one table (below). Replacing the library means another library with the same C
  interface; nothing in the engine or the shells changes.
- **Its C interface** (`core/blocking/lib/pbblock.h`), in outline:

```c
// Loading, from Go, on any one thread.
PBBlocker* pbblock_new(void);
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
  block until step 4 (below): a page script that a list replaces with a harmless stand-in is
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
- **The gate.** Before step 1 ships, `docs/benchmarks.md` gains a page-load run (the
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
- **A switch,** "Warn before dangerous sites", in Settings, Privacy, on by default. Off, Go
  doesn't ask and the lists aren't downloaded.

## The lists

### The catalogue

`core/blocking/catalog.json`, in the repo, kept by hand, started once from Brave's public
catalogue ([brave/adblock-resources](https://github.com/brave/adblock-resources),
`filter_lists/list_catalog.json`, 99 sources: uBlock Origin's, EasyList, EasyPrivacy, the
regional lists). A script, `tools/catalog-from-brave.py`, prints Brave's catalogue in our
form for a person to compare; it never writes ours.

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

**Groups,** each one loaded engine:

| Group | Blocker | Standard (on) | Strict (opt-in) |
|---|---|---|---|
| ads | network and page | EasyList, uBO filters, uBO unbreak, Brave's unbreak | none yet |
| trackers | network and page | EasyPrivacy, uBO privacy, Peter Lowe's | AdGuard URL tracking |
| annoyances | network and page | none | cookie notices, pop-ups, social widgets (Fanboy, uBO annoyances) |
| regional | network and page | the list for each of the app's languages | other regions, by hand |
| badware | Safe Browsing | uBO badware, URLhaus filter | none |
| phishing | Safe Browsing | Phishing URL Blocklist (phishing-filter) | none |
| test | both | our own list: one test address per hook, so the checks have something to hit | none |

Our own `hosts.txt` stays as the built-in list (below) and as list `pb-hosts` in group ads.

### The person's choices

- Settings, Privacy, **Block ads**: as today, all of the network and page rules on or off.
- Under it, **Lists**: each group with a switch and, opened, its lists with theirs; Strict
  lists marked as such, with one line on what they may break. Regional lists follow the
  app's languages unless the person picks.
- **Warn before dangerous sites**: Safe Browsing.
- The site panel's switch stays as it is, and turns off requests and page parts for that
  site. Safe Browsing has no per-site switch; Open anyway is the way past one warning.
- Prefs per profile, as every pref (one process per profile, [profiles.md](profiles.md)).
- UI for ux to review before it is built.

### Building and publishing them

- **A scheduled workflow,** `.github/workflows/lists.yml`, every six hours and by hand. It
  reads the catalogue, downloads each source (refusing one that shrank by more than half or
  failed to parse, and keeping the last good one), builds each group's serialized engine
  with the same `libpbblock` the app pins (`tools/pbblock-build`, a small Rust binary in the
  same crate), and publishes a release, tag `lists`, replaced each time:
  - `manifest.json`: for each group, its lists' versions, the file's name, size and
    SHA-256, and the library's format version;
  - `<group>-<format>.dat`: the serialized engine, compressed;
  - `resources-<format>.json`: the scriptlets and stand-ins (uBO's, from Brave's repo);
  - `LICENSES.txt`: each list's licence and home;
  - `manifest.json.sig`: the manifest's Ed25519 signature.
- **Signed.** Lists carry scripts that run in every page, so the app takes only a manifest
  signed with our key, whose public half is compiled into core (Go's `crypto/ed25519`). The
  private key is in the kosh (`project-b/lists-signing.key`) and is the workflow's one
  secret. A file whose SHA-256 differs from the signed manifest's is thrown away.
- **The format version.** adblock-rust's serialized form changes between its versions. The
  manifest names the format each `.dat` was built for; an app takes only its own. A release
  of the app that moves to a new library version is published after the workflow builds
  that format, so the files are there when it starts. The workflow builds every format a
  supported release uses, the current Stable's and Dev's, no more.
- **Only GitHub.** No server of ours. The app fetches
  `github.com/debkosh/project-b/releases/download/lists/...`.

### Fetching them

- **After the shared start-up wait** ([startup.md](startup.md)): `later.Run(lists.Start)`.
  Then every twelve hours while the app runs. It asks for `manifest.json` with the last
  ETag, so most checks are one small reply that nothing changed, and downloads only groups
  whose SHA-256 changed and that the person has on.
- **On disk,** in the app's shared folder beside the profiles list (`lists/`), so profiles
  share one download. A file is written aside and renamed into place; two profiles fetching
  at once only fetch twice.
- **Loaded at start,** before the first window's page, from the files on disk. A serialized
  engine loads in milliseconds (it is a FlatBuffer, read in place), so this is start-up work
  in the same sense as the hosts list today. New files mid-run are loaded at once and swapped
  in; open pages keep the page rules they had until they load again.
- **Turning a group on** fetches it then, not after the wait: it is something the person
  did.

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
| Download | the standard set compressed, a few MB on the first run; changes after | the same; over a metered connection, at most once a day |
| Memory | the standard set's engines, in the browser process only: about 15 to 20 MB (Brave's figure for its default set after its 2026 FlatBuffers work, which cut it by three quarters) | the same, once; strict lists add to it, so the Lists page shows each group's size |
| In each renderer | the 512 remembered answers and each frame's stylesheet: kilobytes | the same |

- **One copy.** The rules live only in the browser process. Putting a blocker in each
  renderer would save the round trip but cost a copy per renderer, which a phone can't pay.
- **Read in place.** If adblock-rust reads its serialized engine without copying it, Go maps
  the file instead of reading it, so the system can drop and reread those pages under
  memory pressure. To be checked against its API in step 1.
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
- **Before step 1 ships,** the owner reads the licences table once; this is the reading of a
  developer, not a lawyer's.

## Steps

1. **Requests:** `libpbblock`, its build on the three platforms, the blocker table and
   throttle, the lists manager with the built-in list, the catalogue, the workflow and the
   signing key; groups ads and trackers; the page-load gate.
2. **Safe Browsing:** the second blocker, badware and phishing groups, the notice, the
   switch; basics check 28 changed to our own test list's address.
3. **Page parts:** page extras at commit, scriptlets, the watcher.
4. **Strict lists and stand-ins:** annoyances, the strict tier in Settings, and redirect
   answers served by the engine from the resources file.

Each step has a `Notes:` line ("Ads and trackers are blocked from up-to-date public lists").

## Tests

- **Go,** with the mock engine: the manifest's signature and hashes, a damaged or unsigned
  file refused, the format version, the swap, the ETag, the choices, the first-run hosts.
- **Rust:** `cargo test` on `libpbblock`'s C interface, and that a snapshot holds across a
  swap.
- **The real app:** a test page under our own test list's addresses, for each hook: a
  request blocked and counted, an element hidden, a scriptlet run, a generic class hidden,
  an unsafe-site notice with Go back and Open anyway; the per-site switch and Block ads off
  for each. Desktop drivers and `drive_android`.
- **The workflow:** a run on a branch builds every group and publishes to a draft release.

## Not doing

- No Google Safe Browsing, nor any service that is told the addresses a person opens.
- No blocker inside the engine or in renderers.
- No server of ours for the lists.
- No list text compiled on the device: the workflow builds the engines. A list the person
  adds by address is a later question.
