# The settings page

The settings page is one of our pages, in a tab of ours (`shell/src/apps/settings/`). Go owns
everything on it (`core/settings` and the parts other managers add); the page shows it and
sends the person's changes back.

It has two layouts, the same settings arranged two ways, and the flag Settings layout
(Advanced, Flags) switches between them while the page is open, with no relaunch: `page`, one
page by task, the default and what Off means, and `sections`, sections in a list. It is a live
flag ([flags.md](flags.md)): Go saves it and sends the flags again, and the page takes the other
layout at once, showing Flags, where the person is. The owner asked for both, to choose
between by using them (2026-11-18).

## One page by task (`page`)

Every setting is on one scrolling page, in five groups by task:

| Group | What is in it |
|---|---|
| Look | Theme, Frame colour, Tab layout (desktop), Address bar (phone), Shell |
| Browsing | On start, Search engine, Keys, Keyboard shortcuts (desktop), Workspaces, Bookmarks, Downloads: Ask where to save (desktop) |
| Privacy | Sites, Block ads |
| Your data | Profile name, Template, Import from other browsers, Run setup again |
| Advanced | Automatic updates, Sparks, Extensions (with its flag on), Web features, Flags |

On a desktop a rail at the left names the groups and marks the one at the top; a press scrolls
the page to it. On a phone the same names are chips over the page. The long lists, Workspaces,
Sites, Keyboard shortcuts, Sparks, Extensions and Web features, are each a row on the page that
opens the list as a page of its own, in its place, with a back button to its group. The tab's
title is Settings, or Settings › and the list's name on a list's page.

## Sections in a list (`sections`)

Like Chrome's settings: a list of sections, one showing at a time.

| Section | What is in it |
|---|---|
| Appearance | Theme, Frame colour, Tab layout (desktop), Address bar (phone), Shell |
| Start and search | On start, Search engine |
| Workspaces | The workspaces |
| Bookmarks | Where bookmarks show, and the bar |
| Downloads (desktop) | Ask where to save each file |
| Privacy and sites | Block ads, Sites, which opens as a page of its own |
| Keyboard (desktop) | Keys, and the keyboard shortcuts |
| Profile and import | Profile name, Template, Import from other browsers, Run setup again |
| Extensions and sparks | Extensions (with its flag on, desktop), Sparks; Sparks alone without them |
| About Project B | Automatic updates |
| Flags, Web features | Under a line at the foot of the list |

On a desktop the list is at the left, with an icon for each section; the tab's title is
Settings › and the section's name. On a phone the page is the list, each section with a line
saying what it is set to (System · Teal, Where you left off · DuckDuckGo); a press opens the
section under its own heading, with a back button to the list.

## The status line

Under the search, in both layouts, one line says whether the app is the default browser and
which version runs. Not the default, it says so with **Make default browser**, which asks the
system as the welcome's last step does (`AskDefaultBrowser`: macOS's question, Android's
sheet); the answer comes back as `DefaultBrowserChanged`, which Go sends to the welcome pages
and the settings page alike, `isDefault`. The snapshot reads `IsDefaultBrowser` as the page
opens (`core/app/default_browser_settings.go`). The version has what the update manager is
doing beside it, Up to date, Downloading or Relaunch to update, with Check for updates, or
Relaunch (Install update on a phone) once a release is ready. Automatic updates is a row of
its own, under Advanced, or About Project B in the sections layout. The page shows the line
over every group; the sections layout over every section on a desktop, and over the list of
sections on a phone.

## Search

Both layouts search every setting from one field: at the top of the page, at the top of the
list of sections on a desktop, and over the list of sections on a phone. A row is found by its
label, its second line, or the name of its group or section; what was found is marked. The
page shows only the rows found, each working in place, under the group or section they are in
(in the sections layout every section is shown, one after the other, under its name), with
how many were found over them, or that none was. The rail or the list of sections counts the
rows found in each, and dims those with none. A long list, and Import, which aren't rows,
show as the row that goes to them while there are words. Escape empties the field and the page
is as it was; going anywhere, from the rail, the list or a row found, ends the search. Every
`Row.vue` says whether it matches and counts itself in its section (`search.js`).

## On a phone

The phone reaches every setting. What makes no sense there stays off it: Tab layout, Keys and
Keyboard shortcuts, Extensions, and Ask where to save (a phone has no Save panel).

## Parts, and how they are arranged

Both layouts are made of the same parts (`arrange.js`): a row (`Rows.vue`, `KeysRow.vue`,
`ProfileRows.vue`), or a block that brings its own cards (`Bookmarks.vue`, `Import.vue`,
`Flags.vue`, and the long lists). Every row is drawn by `Row.vue`: its label, a second line
under it, and its control. A layout is data: its groups or sections, each a run of cards, a card
being a list of parts; `PageLayout.vue` and `SectionsLayout.vue` only arrange them. A list among
a card's parts is drawn as the row that opens it. `arrange.js` also says where each part shows
(desktop, phone, or only when the page has what it needs, as Profile needs a listed profile).

## Addresses

What the page shows is an address, `state.section`, which Go may name as it opens the page
(`OpenSection`): a group or section (`look`, `browsing`, `privacy`, `data`, `advanced`;
`appearance`, `start`, `profile`, `about`), a part (`import`, `flags`), or a list (`workspaces`,
`sites`, `sites/<origin>`, `keyboard`, `sparks`, `extensions`, `webFeatures`). Both layouts take
every address: the page opens a list's own page, or scrolls to the part or its group; the
sections show the section that is it or holds it, or Sites' own page. The site panel's Details
opens `sites/<origin>`, and an extension's Manage opens `extensions`.

## Tests

`shell/e2e/settings.cjs` drives the page on the mock engine, once in each layout, each run on a
profile whose flag names it; it goes to each part as a person does, by the rail, the list of
sections and the rows that open the lists. Each run also switches the flag to the other
layout and back, in the same page, searches (rows found working in place, the counts, a
section's name, nothing found, Escape, a list found), and walks a phone-sized page through
every group or section and a search. `core/flags/flags_test.go` has the live flag.
