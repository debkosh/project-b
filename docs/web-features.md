# Web features

> A web feature here is something a page can call or do whose browser side Chrome provides
> from `//chrome` or `//components`, not content. Project B has no `//chrome`, so each one is
> missing, half there, turned off, or, at worst, gets the page's renderer killed.

Every such feature is one row of `core/webfeatures/registry.json`, and that file is the one
record of what a page gets of it. Everything that needs to know reads it:

| Reader | What it takes |
|---|---|
| GN (`engine/web_features.gni`) | The features the engine turns off, which `engine/buildflags` hands to `DisableChromeOnlyBlinkFeatures` in `engine/common/main_delegate.cc` |
| The probes (`tools/web_features.py`) | The calls to make in a page, and what each row's status lets a call give |
| `tools/web_features_bump.py` | The interfaces, delegates, features and permissions the rows name |
| Go (`core/webfeatures`) | The whole file, embedded; it refuses a row that doesn't hold together, and sends the rows to the settings page |

It lives in `core/` because Go embeds it, and `go:embed` can't reach outside the module; GN
and the tools read it from there, as they read `core/flags/registry.json`.

The research it came from, with file and line evidence for every row, is on the branch
`web-features-research`, in [docs/research/web-features/](https://github.com/debkosh/project-b/tree/web-features-research/docs/research/web-features):
`inventory.md` (the 76 features), `tracking.md` (the proposal this follows) and the scripts.
The inventory is a reading of Chromium 155 and Project B 0.2.61; the registry is kept current.

## A row

```json
{
  "id": "badge",
  "name": "App badge",
  "api": "navigator.setAppBadge(), navigator.clearAppBadge()",
  "interface": {"name": "blink.mojom.BadgeService", "scope": ["frame"]},
  "status": {"mac": "quiet", "linux": "quiet", "android": "works"},
  "why": "On Mac the call goes to an empty binder, so no badge shows; Android doesn't ask the browser",
  "needs": ["go-manager"],
  "owner": "engine/browser/content_browser_client.cc:ContentBrowserClient::RegisterBrowserInterfaceBindersForFrame",
  "probe": [
    {"name": "setAppBadge", "call": "navigator.setAppBadge(3)"},
    {"name": "clearAppBadge", "call": "navigator.clearAppBadge()"}
  ],
  "test": ["probe:setAppBadge", "probe:clearAppBadge"]
}
```

| Key | | Meaning |
|---|---|---|
| `id` | required | Lower-case words joined by `-`; the inventory's id |
| `name` | required | What a person calls it, for the settings page |
| `api` | required | What a page calls or does |
| `status` | required | What a page gets, for `mac`, `linux` and `android` (below) |
| `why` | required | One line a person can read: why it is so |
| `interface` | | The Mojo interface Blink asks the browser for, and its scopes: `frame`, `service_worker`, `dedicated_worker`, `shared_worker` |
| `delegate` | | The content method that decides it, `Class::Method` |
| `permissions` | | The `blink::PermissionType` values that decide it |
| `needs` | | What it would take to work as in Chrome: `stay-off`, `binder`, `delegate`, `go-manager`, `prompt`, `build` |
| `owner` | | Our file and function that answers for it; none when nothing of ours does |
| `disable_blink_features`, `disable_features` | | The Blink runtime features and base features the engine turns off for it |
| `probe` | | The calls the probes make: `name`, `call`, `expect` (below), `sees_nothing` for a call with no answer a page can see, and `tap_races` for one whose answer on a tap rides a race |
| `test` | | The checks that prove the status: `probe:<name>`, `basics:<number>` (`tools/check_basics.py`), or `check:<a check's name>` |

A probe's `expect` says what its call gives on each platform, as the probes write it: the kind
(`absent`, `resolved`, `rejected`, `pending`, `killed`), then a colon and the error's name or
the start of the value where the kind alone doesn't tell (`rejected:NotAllowedError`,
`resolved:false`, `resolved:[]`). A call passes when what it gave starts so. `android_tap` is
what it gives on a tap, where that differs from load. Every probe but a `sees_nothing` one
gives one for each platform, and its kind must be one the row's status allows there (below).

A key Go doesn't know, a status that isn't one of the list, a probe name used twice, an
`expect` missing or out of its status, or a `test` that names a probe the row doesn't have fails `TestTheRegistry` (`go test -tags sqlite_fts5 ./...` in
`core`), and stops the app at its start.

## In settings

Settings, Web features lists every row, read only, with a filter by name, reason or status.
The rows are grouped by what a page gets on the platform the app runs on, worst first, each
heading with its count: Fails quietly, Partly, Turned off, Absent, Works, then any other
(Crashes the page, Not checked). The heading carries the status, so a row has none of its
own: the feature's name (its API in the tooltip) and the `why` line, which wraps. The Fails
quietly heading is in `--danger`; the others are `--muted`. Go sends the rows in the settings
snapshot (`webfeatures.View()`, for `runtime.GOOS`); the page never reads the file. The
section is `shell/src/apps/settings/WebFeatures.vue`, and `shell/e2e/settings.cjs` checks it
on the mock engine: every row in its group with the counts, the reasons, the heading
colours, nothing to change, and the filter.

## The statuses

| Status | A page gets | A probe's `expect` may be |
|---|---|---|
| `works` | What Chrome gives | resolved |
| `quiet` | A rejection, an empty or "denied" answer, or no answer; the page carries on | resolved, rejected, or no answer in 2 s |
| `partly` | Some of it works (`why` says which) | resolved, rejected, or no answer |
| `off` | Nothing: the engine turns it off, so the API isn't there | throws at once |
| `absent` | Nothing, without our turning it off (not on for this platform, or a Chrome-only build part) | throws at once |
| `kills` | Its renderer killed, and every tab sharing it | the renderer dies |
| `unknown` | Not run, and the code doesn't settle it | anything but a killed renderer; the probe prints what it saw |

A call with `sees_nothing` (adding a `<link>`, say) is held only to its renderer living,
whatever the status. So is a call with `tap_races` on a tap, and it has no `android_tap`. The
browser reads the tap's user activation as the call reaches it, on another pipe from the one
that tells it of the tap and of the calls that used the activation up, so which comes first
is a race. `Notification.requestPermission()` on a tap opens the site's prompt when the
activation is there first, which waits for a person (`pending`), and gives `"default"` when it
isn't (seen on Android 14, 2026-11-26).

## Changing it

- **A status changed** (a fix landed, a feature came or went): change the row's `status`,
  `why` and its probes' `expect` in the same commit as the change. The probes fail until they
  agree with what a page gets.
- **Turning a feature off**: add its Blink runtime feature to `disable_blink_features`, or a
  base feature that sets it to `disable_features`, and make its status `off` on both
  platforms; the switches are global, so an entry with either list must be off everywhere,
  and an `off` entry must name what turns it off. Rebuild: GN reads the file at `gn gen`, and
  `main_delegate.cc` passes the lists as `--disable-blink-features` and `--disable-features`,
  after any the command line already gave.
- **A new feature**: add a row in its group (turned off; Mojo binders; delegates; permissions;
  build parts). Give it a probe when a page can call it without a gesture or a system picker,
  and the `test` that proves its status.
- **A probe call** throws at once only when the API isn't there: write it so (call the
  method, don't test `typeof`), and keep it from opening anything over the page.

## How the tests use it

`tools/web_features.py` loads the file for the probes. Each call runs through one wrapper
that says what it gave: absent or rejected with the error's name, resolved with the start of
the value, or pending after 2 s. A killed renderer is read from content's "bad Mojo message"
in the log. Each call is held to its probe's `expect`.

- **Mac and Linux**, `python3 tools/drive_first_app.py webapis` (part of `tools/test.sh app`),
  each held to its own column: every
  probe call on a fresh page, one at a time. Per call, "a page that calls `<name>` keeps its
  renderer"; then one check that every call gave what its `expect` says for the platform, naming
  each row that didn't. Calls of `unknown` rows print what they gave, to fill the row in. The
  service worker that sets the badge is held to `badge-sw`, and the copy button to
  `clipboard-async`.
- **Android**, the `web_apis` part of `tools/drive_android.py` (`tools/test.sh android`): all
  calls on one page, on load and again on a tap. Per pass, the page keeps its renderer, and
  every registry call gave what its `expect` says for Android (`android_tap` on the tap). A few calls whose browser side is
  content's own (`CONTENT_APIS`) are made too, for the renderer's sake only.
- **Basics**: a row's `basics:<n>` names the `tools/check_basics.py` check that shows it, and
  the run holds the two together on its platform: a check that works under a row that isn't
  `works`, `partly` or `unknown`, or fails under one that is `works`, prints a `REGISTRY` line
  naming the row and fails the run.

## After a Chromium bump

    python3 tools/web_features_bump.py           # reads chromium/src; no build needed
    python3 tools/web_features_bump.py --list    # and everything it read

Run it before `tools/gen_hooks.py` ([content-hooks.md](content-hooks.md), "When Chromium is
upgraded"), since it compares with the hooks table that run rewrites. It reads:

- **Chrome's binders**, for each scope content lets an embedder bind for
  (`ContentBrowserClient::RegisterBrowserInterfaceBindersFor<Scope>`): frames, from
  `PopulateChromeFrameBinders`, and service workers, from chrome's
  `RegisterBrowserInterfaceBindersForServiceWorker`. Dedicated and shared workers have no such
  hook in content, so chrome binds nothing for them; a new scope is reported until
  `CHROME_BINDERS` in the script says where chrome fills it. Each interface comes with the
  platforms its `#if` lets it in on.
- **Content's binders** (`content/browser/browser_interface_binders.cc`) and **ours**
  (`engine/browser/content_browser_client.cc`, `engine/extensions/browser/content_hooks.cc`).
- **What Blink's renderer asks for**: every `mojom::` name in `third_party/blink/renderer`.
- **The virtual methods** of `WebContentsDelegate` and `ContentBrowserClient`, with the
  platforms each is on.

And reports three lists:

1. **Chrome binds it, Blink asks for it, and nothing here binds it or names it.** A page that
   asks for one is killed. Add a row, and turn the feature off or give it an empty binder.
2. **Delegate methods new since `docs/content-hooks.md`**, in no row. The table is generated
   for the Mac, so only the methods the Mac has are compared. A person decides whether a page
   can tell; if it can, add a row.
3. **What a row names that Chromium no longer has**: an interface no `.mojom` defines, a
   delegate method gone from its class, a Blink or base feature that isn't declared, a
   permission type gone from `blink::PermissionType`. Fix the row.

It exits 1 when the first or the third list has anything, so a bump can't pass it quietly.
On Chromium 155.0.8059.41 all three are empty.
