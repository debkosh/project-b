# Site features: Sparks, links to other sites, the ad blocker, and the site panel

A design, 2026-11-15, for the owner's three per-site features. Not built. Go owns each:
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
| `spark:<id>` | `off` (this spark not on this site) | not kept: on wherever it matches |

## The site panel

The site-settings icon at the start of the top bar opens a panel there, over the page,
instead of going to settings. On a phone it is a sheet from the bottom. It is drawn from
Go's state for the active tab (`state.site`: the origin, the connection, the toggles, the
matching sparks, the permissions set), and every toggle is one message back
(`setSiteSetting`, as Site settings already uses). Rows, top to bottom:

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
surface, 360px wide. A contract rule in r10's range checks it by role and name.

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
- **Matching is Go's.** When a frame of a tab is about to commit a page, the engine asks Go
  which scripts go in it (one event: the tab, the frame's address, whether it is the main
  frame), and Go answers with the sparks that are on, match, aren't excluded, aren't off
  for the tab's site, and aren't `@noframes` for a subframe: each its id, its source and
  when it runs.
- **The adapter injects** each into an isolated world of its own, numbered from a range of
  ours, at document start or document end. It knows nothing of headers or patterns.
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
- **The engine** already asks Go before each top-level navigation. It adds two facts to
  that question, in our terms: whether the page or the person started it, and whether a
  press led to it; and it gives the site (registrable domain) of the address, so Go needs no
  public suffix list of its own.

## The ad blocker

On by default (one constant in Go, while the owner decides), switchable per site from the
panel: it's what a person expects of a browser they choose today, and a site that breaks is
one switch away.

- **The rules are Go's:** a list of ad and tracking hosts, bundled in the app as a file Go
  reads at start (a host list, as uBlock's lists boil down to; the hosts only, no cosmetic
  rules in the first version). Updated with the app.
- **The engine blocks**, extending the block rules it already has: today Go hands over the
  hosts its policy blocks (`set_block_rules`) and the adapter refuses every request a page
  makes to them, frames, images, scripts and fetches included. The ad hosts go through the
  same call, as a second set marked as ads, and with them the sites where the blocker is
  off. A request to an ad host is refused unless the page it comes from (the tab's site) has
  the blocker off. The adapter counts what it refused per tab, for the panel.
- **The size:** a host list of some 50,000 entries doesn't fit on a renderer's command line
  as the policy hosts do. The adapter hands the sets to each renderer as one read-only
  shared memory region, as Chromium's own subresource filter does, and a new one when the
  per-site list changes.

## Not in the first version

Cosmetic filtering, list updates between app updates, an editor with highlighting, Sparks
sync, and the reference browser's enterprise policy lists. Each is a later step, if wanted.

## Order of building

The panel and the kinds first (with the toggles it can already show: permissions), then
links to other sites (smallest engine change), then the ad blocker, then Sparks. Mac and
Android together, since the Go and the adapter's parts are shared; the Mobile shells draw
the sheet.
