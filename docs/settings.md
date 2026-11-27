# The settings page

The settings page is one of our pages, in a tab of ours (`shell/src/apps/settings/`). Go owns
everything on it (`core/settings` and the parts other managers add); the page shows it and
sends the person's changes back.

Like Chrome's settings, it is a list of sections, one showing at a time. It had a second
layout, every setting on one page by task, to choose between by using them (2026-11-18); the
owner chose the sections (2026-11-20): "we are gonna also ditch the one page settings and only
ship sectioned settings from now on.. i ahve used them and feel sections make better sense".

## Sections

| Section | What is in it |
|---|---|
| Profile | Profile name, Template, Import from other browsers (desktop) |
| Sync | Sync's card ([sync.md](sync.md#the-screens)) |
| Passwords | Passwords and Passkeys, each opening as a page of its own; named Passwords alone, as "Passwords and passkeys" took two lines in a desktop's list |
| Appearance | Theme, Frame colour, Tab layout (desktop), Address bar (phone), Shell |
| Start and search | On start, Search engine |
| Workspaces | The workspaces |
| Bookmarks | Where bookmarks show, and the bar |
| Downloads (desktop) | Save to, Ask where to save each file |
| Privacy and sites | Protection, a card under its heading: Block ads, Warn before dangerous sites, and Block lists, which opens as a page of its own; Clear browsing data, Clear cookies when the app quits (desktop), Sites, which opens as a page of its own; Secure DNS, a card of its own |
| Extensions, or Sparks | Extensions (with its flag on, desktop), and Sparks; named Sparks without extensions |
| Keyboard (desktop) | Keys, and the keyboard shortcuts |
| About Project B | The version and its update, Automatic updates, What's new ([About](#about)), Run setup again |
| Flags, Web features | Under a line at the foot of the list |

The order, also from 2026-11-20 ("the sequence of sections needs some love"): who you are
first, as Chrome and Safari start with the account, with its Sync and its Passwords after it; then how the app looks, and how its
windows and tabs open, Start and search and Workspaces; then your things, Bookmarks and
Downloads; then Privacy and sites; what you add to it; Keyboard, a page of reference seldom visited; and
About last, as every browser has it. ux's review set Workspaces and Keyboard where they are.

Each section's name says what is in it (2026-11-24). Sync had been under Profile, and
Passwords and Passkeys under Privacy, where nobody looks for them ("inside profile we have
most of the stuff related to sync... inside privacy we mostly have data"); the owner chose
this arrangement from three, one section per thing, over fewer broader groups and Chrome's
names. The arrangement is one list in shell/src/apps/settings/arrange.js. Everything else
finds a setting by what it is, never by the section that holds it today: Go opens the page
at a part's address (sync, passwords, sites/…), and each section's place in the list names
its parts (data-parts), which the tests click by. Moving a part is that list alone.

On a desktop the list is at the left, with an icon for each section; the tab's title is
Settings › and the section's name, the first section, Profile, when Go names none. On a phone
the page is the list, each section with a line saying what it is set to (System · Teal, Where
you left off · DuckDuckGo; Sparks, how many are on, or None yet); a press opens the section
under its own heading, with a back
button to the list. Each section's name, and its line, stay on one line however narrow the
phone: what doesn't fit ends in an ellipsis. "Profile and import" wrapped on his phone; the
names are short now, and `settings.cjs` checks no row wraps at 360px.

## The status line

Under the search, one line says whether the app is the default browser. Not the default, it
says so with **Make default browser**, which asks the system as the welcome's last step does
(`system.System.AskDefaultBrowser`: macOS's question, Android's sheet); the answer comes back
to its callback, which Go sends to the welcome pages and the settings page alike, `isDefault`.
The snapshot reads `IsDefaultBrowser` as the page opens (`core/app/default_browser_settings.go`).
The line tops every section on a desktop, and the list of sections on a phone.

## About

The version and its updates, in one card (`Version.vue`), the About Project B section; the
owner asked for them out of the status line, in a section of their own (2026-11-20). Its first
row is Project B and the version running, with what the update manager is doing beside it, Up
to date, Downloading or N is ready, and one button: Check for updates, or Relaunch (Install
update on a phone) once a release is ready; while a release downloads, Cancel and Retry, Retry
last and a little apart, with its progress as a line under the words
([updates.md](updates.md#settings)). On a desktop this is the one place a download shows. The
menu bar's Check for Updates… opens Settings here. Under it, **Automatic updates**, then
**What's new in N**: this release's notes, only this release's, the owner's ask (2026-11-20:
"release notes only gets the current relase notes not rhe whole history"). They are
`shell/src/apps/settings/release-notes.md`, built into the page, and the release carries the
same file beside its manifest ([releasing.md](releasing.md)). A phone's list of sections says
the version on About's line, and Update ready while one is.

## Privacy

- **Clear browsing data…** asks in a dialog from when, the last hour, day, week or four
  weeks, or All time to start, and what goes: History, Cookies and site data, and Cached
  images and files ticked, The downloads list not, History saying how many pages the range
  takes. Go clears it (`core/app/clear_data_settings.go`): history and the downloads list
  are Go's own, the pages last visited and the downloads listed since then (the files stay);
  cookies with site storage, and apart from them the cache, are erased in every workspace and
  the profile's own storage by the engine's `EraseWorkspace`, from the same time, with their
  tabs left open. The row then says what went, and from when.
- **Clear cookies when the app quits**, a desktop's (a phone's app has no quit of its own),
  erases every workspace's cookies and site storage as the app quits, but the sites kept;
  the cache stays. A site is kept from its page in Sites, by a switch there while this is
  on, and its row then says Kept; the prefs `clearCookiesOnQuit` and `keepCookies`, the kept
  sites by registrable domain. Go tells the engine what a quit erases at start and whenever
  the switch, the kept sites or the workspaces change (`SetQuitErasure`,
  `core/app/quit_erasure.go`); the engine's quit, from Cmd+Q, the Dock, a logout or Go,
  runs it and ends the run loop once every workspace's erase is done, or after five
  seconds. A crash erases nothing: the next quit does.
- **Protection** comes first, a card under that heading ([blocking.md](blocking.md#the-persons-choices)):
  - **Block ads** is the ad blocker on every site, "Ads and trackers, from public lists
    kept up to date" ([site-features.md](site-features.md#the-ad-blocker)).
  - **Warn before dangerous sites** is Safe Browsing, "Malware and phishing, checked on this
    device; nothing is sent anywhere", on by default (the pref `warnDangerous`). Off, Go
    asks nothing and those lists aren't downloaded.
  - **Block lists** says how many lists are on and when they were updated ("N of M lists
    on · updated 20 minutes ago", or "not downloaded yet"), and opens a page of its own.
- **The Block lists page** (`BlockLists.vue`, the address `blockLists`): a line on what the
  lists are, then the groups, each a row with its switch and a line of what it blocks and
  " · N of M on", the count never broken. A group opened shows its lists, each with its
  title, what it blocks, its home and its own switch, and when it last changed and its size,
  or Not downloaded yet. A group's switch is on while any of its lists is on; off turns all
  of them off, on brings back the group's defaults (`setBlockGroup`); a list's switch
  leaves the rest of its group be (`setBlockList`). Go keeps only the lists changed from the
  catalogue's default, the pref `lists`. The Safe Browsing groups, Malware and Phishing, are
  under Dangerous sites, greyed while Warn before dangerous sites is off. The test group
  never shows. On a phone a tap on a group's words opens it, a hairline before its switch.
- **Secure DNS** sends the profile's lookups to a DNS-over-HTTPS provider: Off, Automatic or
  Secure only, a listed provider or a custom template, and the provider's last check
  ([secure-dns.md](secure-dns.md)).
- **Sites** is a row per site, its registrable domain, as Safari's Website Data and
  Chrome's All sites have it (`core/app/site_data_settings.go`): the settings of each of its
  hosts, and what every workspace's storage keeps for them, its cookies and bytes, which the
  engine counts by host as the page opens (`ListSiteData`: cookies by domain, the quota
  manager's usage by host and local storage's). The row's line says the settings, how many
  hosts keep something and how much; the list's foot, how much every site keeps, with Clear
  all site data. A site's own page says what it keeps on which hosts, with Clear, and then
  each host with a setting has its settings under its name. Clear, a row's Clear data, and
  Clear all ask first and erase the site's storage and cache, or every site's, in every
  workspace: the engine's `EraseWorkspace` with the site named, a filter by registrable
  domain, since cookies are kept by domain and no host's can go alone. The settings stay.

## Downloads

- **Save to**, a desktop's, is the folder downloads are saved in, by its name on a button
  with the whole path under it, the home folder as ~ and a long one cut in the middle: the
  Downloads folder, or the pref `downloadFolder`, a whole path. The button asks for another in
  a folder dialog on the settings tab's window, opening at the folder now, through the engine's `ChooseFolder`,
  the one Import's "Choose a folder" uses (`choose_folder`, `PB_EVENT_FOLDER_CHOSEN`): the
  panel's answer is Save to's when Save to opened it on that window, else the importer's
  (`App.FolderChosen`, `core/app/folders.go`). The folder chosen becomes the pref, and Go's
  downloads manager saves there from then on (`core/app/download_folder_settings.go`). Use
  Downloads, there while another folder is chosen, goes back. A phone saves to its shared
  Downloads and has no Save to. The owner, 2026-11-18: "in phones no save to... less code to
  maintain" (a folder of its own would come from Android's Storage Access Framework as a
  content URI, which the engine would have to write each download through).
- **Ask where to save each file** opens the Save panel for every download, at the file Go
  would have saved.

## Search

One field searches every setting: at the top of the list of sections on a desktop, and over
the list on a phone. A row is found by its label, its second line, or the name of its section;
what was found is marked. The page shows only the rows found, each working in place, under the
section they are in, every section one after the other under its name, with how many were
found over them, or that none was. The list of sections counts the rows found in each, and
dims those with none. A long list, and Import, which aren't rows, show as the row that goes to
them while there are words. Escape empties the field and the page is as it was; going
anywhere, from the list or a row found, ends the search. Every
`Row.vue` says whether it matches and counts itself in its section (`search.js`).

## On a phone

The phone reaches every setting. What makes no sense there stays off it: Tab layout, Keys and
Keyboard shortcuts, Extensions, Ask where to save (a phone has no Save panel), Save to (a phone
saves to its shared Downloads) and Clear cookies when the app quits (a phone's app has no quit
of its own).

## Parts, and how they are arranged

The sections are made of parts (`arrange.js`): a row (`Rows.vue`, `KeysRow.vue`,
`ProfileRows.vue`), or a block that brings its own cards (`Bookmarks.vue`, `Import.vue`,
`Flags.vue`, and the long lists). Every row is drawn by `Row.vue`: its label, a second line
under it, and its control. The sections are data, each a run of cards, a card being a list of
parts or a block; `SectionsLayout.vue` only arranges them. A list among
a card's parts is drawn as the row that opens it. `arrange.js` also says where each part shows
(desktop, phone, or only when the page has what it needs, as Profile needs a listed profile).

## Addresses

What the page shows is an address, `state.section`, which Go may name as it opens the page
(`OpenSection`): a section (`profile`, `appearance`, `start`, `privacy`, `about`, ...), a part
(`import`, `sync`, `flags`, `version`), or a list (`workspaces`, `sites`, `sites/<site>`,
`sites/<origin>`, `blockLists`, `keyboard`, `sparks`, `extensions`, `webFeatures`). The page shows the
section that is it or holds it, or a list's own page, as Sites'. The site panel's Details
opens `sites/<origin>`, the page of that origin's site with that origin's settings whether
or not it has any, an extension's Manage opens `extensions`, and a `project-b://sync` link opens
`sync`. Each address the page goes to is an entry in its tab's history, and each dialog or row
menu while it is open, so the phone's back key and Back go to the one before, not out of
Settings ([shells.md](shells.md#back-on-our-pages)).

## Tests

`shell/e2e/settings.cjs` drives the page on the mock engine; it goes to each part as a person
does, by the list of sections and the rows that open the lists; checks the status line (not the default browser, Make default
browser, then the default) and About (the version, Check for updates reaching the release
server once, and a download's Cancel and Retry in its row at a phone's width), Protection's
rows and the Block lists page (a group opened, a list and a group switched, Dangerous sites
greyed with Warn before dangerous sites off, the test group never shown), Block ads, Ask
where to save, Save to with a folder chosen and Use Downloads, Clear browsing data's dialog,
Sites with what two
sites keep, a site's Clear data and a site's page with its Clear, and Clear cookies when
the app quits with a site kept from its page; searches (rows found
working in place, the counts, a section's name, nothing found, Escape, a list found); and walks
a phone-sized page through every section, in order, none of whose names or lines wraps at
360px, and a search. `shell/e2e/update-notice.cjs` checks About with a release ready, on a
desktop and a phone.

In Go, `core/app/default_browser_settings_test.go`
the default browser's state in the snapshot, Make default browser and its answer;
`clear_data_settings_test.go` what each choice clears, site data in every workspace with the
tabs open, and a range: the pages each would take, and what the last hour clears and leaves;
`site_data_settings_test.go` Sites by site, in every workspace, without an extension's
storage, and a site's Clear and Clear all; `quit_erasure_test.go` what a quit erases as
the switch, the kept sites and the workspaces change; `sitefeatures_test.go` Block ads;
`downloads_test.go` Ask where to save; `download_folder_settings_test.go` Save to's Change…, a
panel closed with none or a path that is no folder, and back to Downloads. On the real engine, `tools/drive_first_app.py
sitedata` counts what pages in two workspaces kept, clears the site in both, and clears
from the last hour; `quiterase` quits with a site kept and finds, after a restart, the
other site's cookies and storage gone in both workspaces and the kept site's there.
