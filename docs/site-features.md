# Site features: Sparks, links to other sites, the ad blocker, and the site panel

The owner's three per-site features, designed and built 2026-11-15. Go owns each:
what is kept, what matches, what the answer does. The engine's adapter only carries out
what Go says, behind our engine interface. Each is one Go package's worth, shared by the
Mac and Android. Mocks: `project-b-ux-audit/site-panel/` (desktop and phone, light and
dark).

All three keep their per-site state in `core/sites`, the home of every per-site setting
([daily-driver.md](daily-driver.md#permissions)): new rows in the kinds table, not page
requests (`Requested` off), with their own values.

| Kind | Values | Default |
|---|---|---|
| `ads` | `on`, `off` (the blocker on this site) | `on` |
| `links` | `allow`, `block` (a page's own navigations to other sites) | `allow` |
| `sparks` | `on`, `off` (the person's sparks, all of them, on this site) | `on` |
| `popups` | `block`, `allow` (a tab the page opens with no tap first) | `block` |

A site is its origin, as for permissions. A value that is the kind's default is never kept,
so `sites.json` holds only what the person changed. The ad blocker's default is one constant
in Go, `sites.AdsDefault`: on, the owner decided.

## The site panel

The site-settings icon at the start of the top bar opens a panel there, over the page,
instead of going to settings. On a phone it is a sheet from the bottom. It is drawn from
Go's state: `state.openSites`, by origin, each site a web tab is open on, `{settings:
[{kind, title, value, choices?}], sparks: [name]}`, with the tab's own
`connection`, `connectionNote` and `adsBlocked`; every change is one message back,
`setSiteSetting {origin, kind, value}`, as Site settings uses. Rows, top to bottom:

- **The site:** its host and its connection (secure, or not secure and why).
- **Ad blocker** (switch), with how many requests it blocked on this page.
- **Sparks** (switch): whether sparks run on this site, all of them or none, with the names
  of those that are on and match the page ("A and B", "A, B and 2 more"). Shown only when
  one does.
- **Links to other sites:** Allow or Block (`Segmented`).
- **Permissions:** each the site has set or asked for, Allowed or Blocked.
- **Details**, which opens settings at the site (`sites/<origin>`), as the icon does today.

A change that needs the page again (the blocker, sparks) says "Changes apply when the
page reloads" in the panel's foot, with Reload beside it, which reloads the page. Escape or a press outside closes it. The panel is the
shell's drawing, so each shell draws its own; Standard's is the design system's floating
surface, 360px wide (`@/ui/SitePanel.vue`, which Fox shares); Mobile's is
`SiteSheet.svelte`, opened from the site's mark in the pill, a 48px target. Contract rule 33
checks it by role and name.

## Sparks

Userscripts, as other browsers' site scripts: JavaScript only, no extension APIs, each run in
the page's own world, as other userscript managers run a script that asks for no extension APIs.

- **Kept by Go**, the profile's `sparks/` folder, one `.user.js` per spark as the person
  wrote or imported it, and `sparks.json` for each one's id and whether it is on. At most
  25 sparks, each at most 140 KB.
- **The header**, the common one: `// ==UserScript==` with `@name` (needed), `@namespace`,
  `@version`, `@description`, `@match` (needed, one or more, Chrome's match patterns),
  `@exclude-match`, `@run-at` (`document-start` or `document-end`, the default) and
  `@noframes`. Go parses it, and refuses a file without a name or a match, saying why.
  `@name` with `@namespace` says which spark a file is, as other userscript managers have
  it: an import of the same pair is that spark again.
- **Matching is Go's** (`core/sparks`). When a frame of a tab is about to commit a page,
  the engine asks Go which scripts go in it (`PB_EVENT_SPARKS_FOR`, 47: the tab, the
  frame's address, whether it is the tab's own frame), and Go answers with the sparks that
  are on, match, aren't excluded and aren't `@noframes` for a subframe, unless sparks are
  off for the tab's site: each its id, its source and when it runs.
- **The adapter runs** each in the page's own world, each in a function of its own so no two
  clash over a name, at document start or document end: the browser hands the frame its
  sparks over `project_b.mojom.Frame` before the commit, and the renderer runs each when
  its moment comes, or at once when they arrive after it. It knows nothing of headers or
  patterns. Run in the page's world, a spark sees and can change what the page does: it can
  replace one of the page's functions, or the browser's own, such as `window.open`.
- **The page's policy gives way for styles and HTML.** A site's security policy that allows
  only its own styles would block the styles a spark adds, and one that wants Trusted Types,
  as YouTube's does, would make a spark's `innerHTML` throw. Where a spark runs, the browser
  asks Go for the frame's sparks as the response comes (a navigation throttle in
  `engine/browser/site_features.cc`), and when there are any, each of the response's
  policies has `style-src`, `style-src-elem` and `style-src-attr` set to allow any style,
  inline ones too, and loses `require-trusted-types-for` and `trusted-types`, before the
  page commits with it. Every other rule stays as the site set it: scripts, frames and
  connections.
- **Settings** gets a Sparks section: New on its heading line, with a ⋯ menu of Import…
  and Export… beside it, the count of 25 under it, and a row for each spark: its name,
  where it runs ("every site" for `<all_urls>`), its switch, and a menu (⋯, or a
  right-click) of Edit…, Export and Delete. A tap on the row edits it. New and Edit open
  the editor, a plain text area for the first version, under the list.
- **Import…** takes any number of files at once, from the system's picker on the Mac and on
  a phone: each `.user.js`, and each `.zip` of them. A zip gives every `.user.js` in it, at
  any depth, and leaves out the rest, such as a Mac's `__MACOSX` folder, hidden files and
  READMEs. The page sends Go each file as base64 (`sparkImport`); Go reads the sparks out
  of them (`core/sparks/archive.go`) and sorts them (`import.go`):
  - one the profile already has, with the same source, is already here;
  - one the same import holds twice is used the first time;
  - one the profile has with another source is a clash. Go sends the clashes
    (`sparkClashes`), and one dialog asks about each: **Replace** (the source goes in place
    of the one kept, which keeps its id, its switch and its switches for each site),
    **Rename** (both are kept, the new one as "Name 2", its header's `@name` rewritten) or
    **Ignore**. Replace is chosen to start with. Import (`sparkImportKeep`) keeps them;
    Cancel or Escape (`sparkImportCancel`) leaves everything as it was;
  - the rest are added, on, while there is room for 25.
  Go then says what it did (`sparkImported`), and the same dialog shows it: the title says
  it once ("Added dhd 2 · 3 already here"), and a row for each spark gives its name and
  what became of it, or why it was not imported.
- **Export**: a row's Export downloads that spark's `.user.js`. Export… is a dialog listing
  every spark that reads, ticked, under All; Export N sparks downloads `Sparks.zip` of
  those ticked (`sparkExport`, `sparkZip`): each kept file as it is, named by its spark,
  which Import here and other userscript managers read back. Both go through our
  downloads.

### The built-in sparks

Three of the owner's sparks come with the app, to give a feel of what sparks do: **Video
downloader** (his `dhd`), **Screen time limit** and **Page cleanup**. They are samples, not a
feature of ours:

- **Added once, off.** A profile's first start on the version that has them, a new one or
  one updated to it, adds the three to its sparks, off, after any it has, while there is room
  for 25. A profile notes that it has had them, so they never come back once deleted, and an
  update never changes them (`AddBuiltins`, `core/sparks/builtin.go`; `sparks.json` keeps
  the ids they were given as).
- **Then they are the person's.** Each is an ordinary spark: on or off, Edit…, Export,
  Delete, and its switches for each site, as any other. Nothing marks it as ours.
- **Their sources** live in `core/sparks/builtin/` as `.user.js` files, embedded in the app,
  each with `@name`, `@description`, `@version` and `@namespace`, and a short doc beside it
  (`core/sparks/builtin/README.md`). They are the owner's originals cleaned up after a UX
  pass. The runtime still runs his originals unchanged, as the driver's sparks part checks.
- **Each stands alone:** a floating overlay of plain HTML, drawn in a shadow root of its own,
  with its own styles, light and dark by `prefers-color-scheme`, in our None frame's tones and
  our type, the same block in all three. Nothing
  comes from the shell or the runtime. The UX pass, for all three: sentence-case names, no
  emoji (small line icons drawn inline), no pulse, bounce or shake, what is tapped low on a
  phone and small floating panels on the desktop, Escape closing the innermost.
  - **Video downloader:** nothing until a video is found; then one round button bottom
    right, which opens "Videos on this page": a row per video, its type and size or "Video
    1", its host, and Download, a plain link to the file ending in `#pb-download` (below):
    a download from the page, which our Downloads takes, with its Pause, Cancel and Retry,
    another site's file too. A tap turns it to a tick in the accent. A stream says "Streams
    can't be saved as one file". "Hide on
    this page" is in the panel's menu.
  - **Screen time limit:** 30 minutes a day on each site its `@match` takes: every site, as
    it comes, and fewer once the person edits its patterns in Settings › Sparks. "Daily limit reached", "Limit
    reached again" and "It's late", one message each; "More time…" leads to the sum, then
    "How much more?". The countdown shows only in the last 5 minutes.
  - **Page cleanup:** a 40px button docked to the edge; "Hide parts of this page" outlines
    what is under the finger or pointer, a tap hides it at once with "Hidden. Undo", and
    Done ends it. "Show hidden parts (3)" when there are some.
- **Their storage** is the site's own `localStorage`, as in his originals, so a site can read
  or clear what they keep there.
- **`#pb-download`:** a link whose address ends in `#pb-download` is saved as a download,
  from the page it is on, whatever its type, another site's file too, under the name its
  server gives it (`ContentBrowserClient::ShouldForceDownloadResource`). Chromium ignores
  `download` on another site's link and opens the file in a tab, whose player asks for it
  again as from the file's own address; a site that serves its files only to its own pages
  (a video host, by `Referer`) refused both, and the video downloader's Download failed
  there while Chrome's own Download, from the page, got it (2026-11-27). The fragment never
  reaches the server. The suggested name is the server's, not the page's, so a page can't
  pass a file off under another name, which is why Chromium ignores `download` there.
- **The phone's welcome** has a step after Make it yours, "Extras for any page": a switch
  for each of the three the profile still has, with its name and `@description`, all off
  ([mock](screenshots/onboarding/mock-p3.png)). The page hears them in its snapshot and as
  `sparks`, and a switch sends `setSparkOn {id, on}`. The desktop shows them in Settings ›
  Sparks only.

### Sparks kept for sync

A spark is an item sync can carry ([synckosh](https://github.com/debkosh/synckosh)'s
SPEC): collection `sparks`, one item per spark, the spark's id inside it, its data the
source and whether it is on. So each spark has:

- **an id made once and never changed**: a UUIDv7 for a new spark; one kept since 0.2.70
  keeps its eight hex digits;
- **`added` and `modified`**, Unix milliseconds: when it was kept, and when an edit or a
  switch last changed it. The list's order is the order they were added in; a place key
  comes when sparks can be moved.

`sparks.json` is `{version: 2, sparks: [{id, on, added, modified}], builtins: [id]}`. The
first version, a list of `{id, on}`, reads in its order, each added a millisecond after the
one before, and is written as version 2 at the next change.

A delete leaves no tombstone yet, as with bookmarks; sync adds what it needs when it is
built. A spark's switches for each site stay with the site's settings.

### Decisions, 2026-11-18

- Where sparks run: in the page's own world. The owner: "yes we would like to be running on
  the page itself.. since that gives more possibilities right... we can override stuff and
  what not".
- What gives way: "ok to styling ease only".
- His block popup spark is not a built-in: "the block popup we do not need since we already
  built the stay on site feature... so drop that".
- Import and export: "import and export should work.. and we should be able to import and
  export zip... import overwrites after user prompt... replace rename ignore".

### Decisions, 2026-11-19

- Trusted Types give way where a spark runs, so his downloader's HTML works on sites that
  want them: "yes".
- One switch for sparks in the site panel, for all of them: "sparks also get quick switch
  from the site setting popup... not individual sparks... any sparks enabled or disabled to
  run on this page". Each spark's own switch for a site, which there was before, goes.
- The built-ins: "builtin are just for giving some feel of stuff... only added to the
  browser store on welcome or update we do not worry about updating them for now.. make
  sure they are also a syncable entity... editing is allowed the user owns it once
  installed".
- No help from the shell for them: "sparks should not need view related stuffs from the
  shell... they stay floating simple html overlays".
- Which sites Screen time limit covers: its own `@match`, as every spark's, with no list or
  setting of its own (the owner, through UX).

## Links to other sites

One setting per site, `links`: **Allow** (the default) or **Block**. A site kept at New tab,
which 0.2.70 had, reads as Allow.

**What it holds: the page's own navigations, never the person's.**

- **The person's** reach the engine through Go: the address field, the command box, a
  bookmark, Back, Forward, Reload, Open link in new tab from our menu, a tab restored. The
  tabs manager sends each through one wrapper of the engine (`core/tabs/pagenav.go`, `own`),
  which marks the tab before the engine hears it.
- **The page's** is every other navigation that starts in a tab: a link pressed in the
  page, a script, a form, a redirect of one of those, and a tab the page opens
  (`target=_blank`, `window.open`). A redirect belongs to its navigation's start.

**Where it is decided: one place, in Go, as a navigation starts.** The tabs manager's
`HandleNavigationStarting` (event 60) puts each of the page's navigations to the page
guard, `App.pageMay` (`core/app/sites.go`), which says go or not:

| Setting | To the page's own site | To another site, or the blank page |
|---|---|---|
| Allow | go | go |
| Block | go | refused |

- **The parent site.** In its own tab, the page the tab shows. A tab the page opened goes
  by its opener's page until it has a page of its own (event 2, `TAB_OPENED`, names the
  opener); the blank page a popup opens on isn't one.
- **A site** is one registrable domain, over http or https alike, by the public suffix list
  (`sites.SameSite`, `golang.org/x/net/publicsuffix`).
- **Refused in place:** the navigation goes nowhere (`NavigationElsewhere`): nothing
  commits, no history, the page stays live, its title and icon kept.
- **Refused in a tab the page opened:** the tab closes as it opens, its opener shows again,
  and the opener's `popupsBlocked` counts it; a new page in the opener starts the count
  again. A blank popup closes at once; one the page later sends to another site closes then.
- **The address field** shows the count with an icon, a button named "1 pop-up blocked",
  which opens the site panel (contract rule 33). It is the place a general popup blocker
  will count too.

**The engine** has no part of it: it asks Go about every navigation start as it always has.
One Android fix came with it: a tab is one of ours by how it was made, not by its browser
context, which Android shares between our pages and the web. Before it, every Android web
tab counted as ours, and a popup its page opened loaded over the page itself: the blank page
of an ad's `window.open()` blanked the page.

**Checking it.** The real-app drivers do it: `python3 tools/drive_first_app.py links` on the
Mac and `python3 tools/drive_android.py links_to_other_sites` on an emulator, each part of its
full run, with the page in `tools/links_page.py`. The page is served on one host and the other
site is the same test server by its other name (the Mac: `127.0.0.1` and `localhost`; the
emulator, through `adb reverse`: `localhost` and `127.0.0.1`), so no request leaves the machine
and the server sees whether the other site was ever asked. Block is set as a person sets it:
the site's panel message on the Mac, the site sheet's Block on a phone, waited for in
`sites.json`. A press is followed 1.5 s later by the page asking its own host for
`/alive?<n>`, so a page still there says so. A pass under Block is the page still there, its
count risen, one tab, nothing of the other site in history (on the Mac, Back still off), the
other site never asked, and the address field counting the popups: five of the ten open one.
The person's own way, an address typed, still goes under Block, and every way goes under
Allow. The ten ways are a link, `target=_blank`, `rel=noopener`, `window.open`,
`location.href`, `location.replace`, a form, a blank popup sent on, a same-site link the
server redirects (a 302) and a popup opened as the page leaves.

**What was learned on the way** (0.2.70 to 0.2.72):

- Go misses a navigation start that comes before it listens (the start URL at launch), so a
  mark of "the person asked" has to match what starts, not stand as a flag: a load's mark is
  its host.
- Android's popups loaded over their page because `PageTab::internal()` came from the browser
  context, which Android shares between our pages and the web; it is now fixed at creation.
- Chromium commits `about:blank` with no navigation throttle, so nothing gate-side can stop a
  blank page; what stops it is the popup not being made over the page in the first place.
- A renderer's request throttle never reaches the browser's `ResourceLoadComplete`, so the
  ad blocker's count comes over mojom (`AdReports`); Blink drops script run from its own
  observer callbacks, so Sparks run at `RunScriptsAtDocumentStart/End`.

**Later, if wanted:** separate settings for a page's navigations in place and for the
tabs it opens after a tap; a popup let open hidden and closed after a second, for sites that wait on
`window.opener`.

## The ad blocker

On by default (one constant in Go, while the owner decides), switchable per site from the
panel: it's what a person expects of a browser they choose today, and a site that breaks is
one switch away. **Block ads** in Settings (Privacy; Privacy and sites in the sections layout)
turns it off or on for every site: a pref, `blockAds`, on by default. Off, Go hands the engine
rules with no hosts, so nothing is kept from any page, and leaves the `ads` setting out of
each open site's settings, so the panel has no switch, nor a site's view in Settings; each
site's own setting is kept, and counts again once it is on.

- **The rules are Go's** (`core/adblock`): a list of ad network, ad server and tracker
  hosts, kept by hand and bundled in the app, a few hundred hosts that serve only ads or
  tracking, so no site's own content goes with them. Updated with the app. A larger
  published list (EasyList's hosts) is a later step, once its licence is settled.
- **The engine blocks**, beside the policy's block rules: Go hands over the hosts and the
  origins where the blocker is off (`set_ad_rules`, at start and on each change), the
  browser passes them to every renderer over `project_b.mojom.Renderer`, and each
  renderer's request throttle refuses a request to a listed host, or one under it, from a
  page whose site isn't the host's own and where the blocker isn't off. A refused
  request fails as `ERR_BLOCKED_BY_CLIENT` (the policy's as `ERR_BLOCKED_BY_ADMINISTRATOR`),
  which the browser counts for the tab (`PB_EVENT_AD_BLOCKED`, 48), and Go clears at the
  tab's next page.

## Pop-ups

**Decided 2026-11-26, the owner:** "just have a per app simple setting.... allow popups ...
default block... show something somewhere on blocking a popup", then "no no per site
setting". Asked which pop-ups, once he heard the engine knows whether a real tap came first:
only those opened without one, Chromium's rule. The sign is the address field's.

- **One setting per site,** `popups`: **Block** (the default) or **Allow**, a row of the
  kinds table like Links to other sites. It is set in the site panel and in Site settings,
  which lists it as it lists every kind of ours. There is no switch for the whole app.
- **A pop-up is a tab a page opens with no tap first:** `window.open`, `target=_blank` or a
  form to a new tab, when no real tap, click or key reached the page just before. That is
  Chromium's user activation, which a page's script can't fake. The engine passes content's
  `user_gesture` as `TAB_OPENED`'s last value, `tapped` (`PageTab::AddNewContents` and
  `OpenURLFromTab`). A tab no page opened, such as one from our menus, the debugging port or
  an extension, is always tapped. A link pressed, or a `window.open` in the moments after a
  press, is tapped and opens, so sign-in windows keep working.
- **Decided in Go, in the page guard** (`App.pageMay`). The tabs manager puts a tab a page
  opened to the guard as it opens (`PageNavigation.Untapped` for an untapped one). The guard
  refuses it unless the opener's site has Pop-ups at Allow; a page that isn't a site has
  none, so it is refused. Only the opening is put to the guard this way: a pop-up let open
  is a tab like any other after it. A refused one goes the way Links to other sites'
  refused tabs go. The tab closes as it opens, its opener shows again, and the opener counts
  it in `popupsBlocked`, with the address field's "1 pop-up blocked". The opener also keeps
  the latest address, `blockedPopup`, that isn't the blank page. A new page in the opener
  clears both.
- **The site panel** (and Mobile's sheet), which that count opens, has a **Pop-ups** row
  after Links to other sites, its choice in Links' order, Allow then Block, Block chosen by
  default. While the page has blocked some, the note under it says "*n* blocked on this
  page", and under that, on a line of its own so no panel width breaks it, **Open it**, or
  **Open the last one** when there were more (ux's words, the same day). It closes the panel and opens the kept address in a tab beside the opener, as the
  person's, so no guard holds it, and the count drops by one. Its message is
  `openBlockedPopup`, for the window's active tab. The address field's count says "1 pop-up
  blocked" and "3 pop-ups blocked", for Links' refused tabs too. A kind of ours whose default
  isn't its first choice names it (`Kind.Initial`), and Site settings has it as the kind's
  value.
- **Checked by** Go's tests (`TestAPopupMeetsTheGuardAsItOpensAndOpensOnce`,
  `TestASitesPopupsAreBlockedUntilAllowed`), the contract's rule 33 on the mock engine (its
  `popup` page action is an untapped `window.open`), and the real-app drivers' `popups`
  parts. The drivers run their own script in a page with no tap, so a check that wants its
  script's window opened runs it as if tapped (the driver's `tappage` and `taptab`).

## Not in the first version

Cosmetic filtering, list updates between app updates, an editor with highlighting, Sparks
sync, and enterprise policy lists. Each is a later step, if wanted.

## One release

All four in one release, as the owner asked: the Go and the adapter's parts are shared by
the Mac and Android (`engine/browser/site_features.cc`, `engine/common/ad_rules.cc`, the
renderer client and `engine/mojom`).
