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
| `links` | `allow`, `newtab`, `block` (a page's links to other sites) | `allow` |
| `spark:<id>` | `on`, `off` (this spark on this site) | `on` |

A site is its origin, as for permissions. A value that is the kind's default is never kept,
so `sites.json` holds only what the person changed. The ad blocker's default is one constant
in Go, `sites.AdsDefault`: on, the owner decided.

## The site panel

The site-settings icon at the start of the top bar opens a panel there, over the page,
instead of going to settings. On a phone it is a sheet from the bottom. It is drawn from
Go's state: `state.openSites`, by origin, each site a web tab is open on, `{settings:
[{kind, title, value, choices?}], sparks: [{id, name, on}]}`, with the tab's own
`connection`, `connectionNote` and `adsBlocked`; every change is one message back,
`setSiteSetting {origin, kind, value}`, as Site settings uses. Rows, top to bottom:

- **The site:** its host and its connection (secure, or not secure and why).
- **Ad blocker** (switch), with how many requests it blocked on this page.
- **Links to other sites:** Allow, New tab or Block, one three-way picker (`Segmented`).
- **Sparks on this site:** each spark that matches the page, with a switch for this site.
  Shown only when one matches.
- **Permissions:** each the site has set or asked for, Allowed or Blocked.
- **Details**, which opens settings at the site (`sites/<origin>`), as the icon does today.

A change that needs the page again (the blocker, a spark) says "Changes apply when the
page reloads" in the panel's foot. Escape or a press outside closes it. The panel is the
shell's drawing, so each shell draws its own; Standard's is the design system's floating
surface, 360px wide (`@/ui/SitePanel.vue`, which Fox shares); Mobile's is
`SiteSheet.svelte`, opened from the site's mark in the pill, a 48px target. Contract rule 33
checks it by role and name.

## Sparks

Userscripts, as the reference browser's Site Spark: JavaScript only, no extension APIs, each in a world of
its own on the page.

- **Kept by Go**, the profile's `sparks/` folder, one `.user.js` per spark as the person
  wrote or imported it, and `sparks.json` for each one's id and whether it is on. At most
  25 sparks, each at most 140 KB.
- **The header**, as the reference browser's: `// ==UserScript==` with `@name` (needed), `@description`,
  `@match` (needed, one or more, Chrome's match patterns), `@exclude-match`, `@run-at`
  (`document-start` or `document-end`, the default) and `@noframes`. Go parses it, and
  refuses a file without a name or a match, saying why.
- **Matching is Go's** (`core/sparks`). When a frame of a tab is about to commit a page,
  the engine asks Go which scripts go in it (`PB_EVENT_SPARKS_FOR`, 47: the tab, the
  frame's address, whether it is the tab's own frame), and Go answers with the sparks that are on, match, aren't excluded, aren't off
  for the tab's site, and aren't `@noframes` for a subframe: each its id, its source and
  when it runs.
- **The adapter injects** each into an isolated world of its own, numbered from 2^28 up, at
  document start or document end: the browser hands the frame its sparks over
  `project_b.mojom.Frame` before the commit, and the renderer runs each when its moment
  comes, or at once when they arrive after it. It knows nothing of headers or patterns.
- **Settings** gets a Sparks section: the list with a switch each, New (an editor: a plain
  text area for the first version), Import (the file chooser) and Export (the Save panel),
  and Delete.

## Links to other sites

The owner: "links to other site can be blocked or new tab or allowed". One setting per site,
`links`, with three values: **Allow** (the default, as every browser does), **New tab** and
**Block**.

- **What counts as going to another site:** a navigation of the tab's main frame, started
  by the page (a link, a form, a script, `window.location`, a meta refresh), to an address
  whose site, the registrable domain, isn't the page's. Frames inside the page are
  untouched. The shell (the address field, the command box, back and forward) goes
  anywhere.
- **New tab:** the navigation is refused, and when a press led to it the address opens in
  a new tab beside this one, in front; with no press (a script, a redirect the page
  started), it is refused and nothing opens.
- **Block:** the navigation is refused, and nothing opens.
- **A redirect** from the site's own server to another site counts too, so New tab and
  Block break a sign-in that goes through another site (accounts.google.com). The setting
  is the person's, per site, and Allow by default.
- **The engine** asks Go, before Go hears the navigation start, when a page-started
  top-level navigation goes to an address whose registrable domain isn't the page's
  (`PB_EVENT_LEAVING_SITE`, 46: the tab, the page it leaves, the address, whether a press
  led to it). Go
  answers as for any navigation; it needs no public suffix list of its own.

## The ad blocker

On by default (one constant in Go, while the owner decides), switchable per site from the
panel: it's what a person expects of a browser they choose today, and a site that breaks is
one switch away.

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

## Not in the first version

Cosmetic filtering, list updates between app updates, an editor with highlighting, Sparks
sync, and the reference browser's enterprise policy lists. Each is a later step, if wanted.

## One release

All four in one release, as the owner asked: the Go and the adapter's parts are shared by
the Mac and Android (`engine/browser/site_features.cc`, `engine/common/ad_rules.cc`, the
renderer client and `engine/mojom`).
