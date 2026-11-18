# The settings page

The settings page is one of our pages, in a tab of ours (`shell/src/apps/settings/`). Go owns
everything on it (`core/settings` and the parts other managers add); the page shows it and
sends the person's changes back.

## One page by task

Every setting is on one scrolling page, in five groups by task:

| Group | What is in it |
|---|---|
| Look | Theme, Frame colour, Tab layout (desktop), Address bar (phone), Shell |
| Browsing | On start, Search engine, Keys, Keyboard shortcuts (desktop), Workspaces, Bookmarks |
| Privacy | Sites |
| Your data | Profile name, Template, Import from other browsers, Run setup again |
| Advanced | Version, Automatic updates, Sparks, Extensions (with its flag on), Web features, Flags |

On a desktop a rail at the left names the groups and marks the one at the top; a press scrolls
the page to it. On a phone the same names are chips over the page. The long lists, Workspaces,
Sites, Keyboard shortcuts, Sparks, Extensions and Web features, are each a row on the page that
opens the list as a page of its own, in its place, with a back button to its group. The tab's
title is Settings, or Settings › and the list's name on a list's page.

The phone reaches every setting. What makes no sense there stays off it: Tab layout, Keys and
Keyboard shortcuts, and Extensions.

## Parts, and how they are arranged

The page is made of parts (`arrange.js`): a row (`Rows.vue`, `KeysRow.vue`,
`ProfileRows.vue`), or a block that brings its own cards (`Bookmarks.vue`, `Import.vue`,
`Flags.vue`, and the long lists). Every row is drawn by `Row.vue`: its label, a second line
under it, and its control. The layout is data: groups, each a run of cards, a card being a list
of parts. A list among a card's parts is drawn as the row that opens it. `arrange.js` also says
where each part shows (desktop, phone, or only when the page has what it needs, as Profile needs
a listed profile).

## Addresses

What the page shows is an address, `state.section`, which Go may name as it opens the page
(`OpenSection`): a group (`look`, `browsing`, `privacy`, `data`, `advanced`), a part
(`import`, `flags`), or a list (`workspaces`, `sites`, `sites/<origin>`, `keyboard`, `sparks`,
`extensions`, `webFeatures`). A list's address opens its page; any other scrolls the page to the
group that holds it. The site panel's Details opens `sites/<origin>`, and an extension's Manage
opens `extensions`.

## Tests

`shell/e2e/settings.cjs` drives the page on the mock engine; it goes to each part as a person
does, by the rail and the rows that open the lists.
