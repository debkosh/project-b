# The settings page

The settings page is one of our pages, in a tab of ours (`shell/src/apps/settings/`). Go owns
everything on it (`core/settings` and the parts other managers add); the page shows it and
sends the person's changes back.

It has two layouts, the same settings arranged two ways, and Look's Settings row switches
between them while the page is open, with no relaunch: One page, by task, the default, and
Sections, in a list. It is Go's `settingsLayout` pref, `page` or `sections`: Go saves it and
sends the prefs again, and the page takes the other layout at once, showing Look, where the
person is. The owner asked for both, to choose between by using them (2026-11-18), and to have
the choice under Look rather than in Flags.

## One page by task (`page`)

Every setting is on one scrolling page, in five groups by task:

| Group | What is in it |
|---|---|
| Look | Theme, Frame colour, Tab layout (desktop), Address bar (phone), Shell, Settings (this page's layout) |
| Browsing | On start, Search engine, Keys, Keyboard shortcuts (desktop), Workspaces, Bookmarks, Downloads: Save to and Ask where to save (desktop) |
| Privacy | Clear browsing data, Sites, Block ads |
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
| Appearance | Theme, Frame colour, Tab layout (desktop), Address bar (phone), Shell, Settings (this page's layout) |
| Start and search | On start, Search engine |
| Workspaces | The workspaces |
| Bookmarks | Where bookmarks show, and the bar |
| Downloads (desktop) | Save to, Ask where to save each file |
| Privacy and sites | Clear browsing data, Block ads, Sites, which opens as a page of its own |
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
Relaunch (Install update on a phone) once a release is ready; while a release downloads,
Cancel and Retry, Retry last and a little apart, with its progress as a line under the words. On a narrow page the line
breaks between the version and the update, never inside the update, so its buttons stay
beside what they act on and the progress line stays under its words. Automatic updates is a row of
its own, under Advanced, or About Project B in the sections layout. The page shows the line
over every group; the sections layout over every section on a desktop, and over the list of
sections on a phone.

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
- **Block ads** is the ad blocker on every site ([site-features.md](site-features.md#the-ad-blocker)).
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
  the system's folder panel on the settings tab's window, through the engine's `ChooseFolder`,
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
Keyboard shortcuts, Extensions, Ask where to save (a phone has no Save panel), Save to (a phone
saves to its shared Downloads) and Clear cookies when the app quits (a phone's app has no quit
of its own).

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
`sites`, `sites/<site>`, `sites/<origin>`, `keyboard`, `sparks`, `extensions`, `webFeatures`). Both layouts take
every address: the page opens a list's own page, or scrolls to the part or its group; the
sections show the section that is it or holds it, or Sites' own page. The site panel's Details
opens `sites/<origin>`, the page of that origin's site with that origin's settings whether
or not it has any, and an extension's Manage opens `extensions`.

## Tests

`shell/e2e/settings.cjs` drives the page on the mock engine, once in each layout, each run on a
profile whose `settingsLayout` pref names it; it goes to each part as a person does, by the
rail, the list of sections and the rows that open the lists. Each run also switches Look's
Settings row to the other layout and back, in the same page; checks the status line (not the default browser, Make default
browser, then the default; the version, and Check for updates reaching the release server
once, and a download's Cancel and Retry on its own line at a phone's width), Block ads, Ask
where to save, Save to with a folder chosen and Use Downloads, Clear browsing data's dialog,
Sites with what two
sites keep, a site's Clear data and a site's page with its Clear, and Clear cookies when
the app quits with a site kept from its page; searches (rows found
working in place, the counts, a section's name, nothing found, Escape, a list found); and walks
a phone-sized page through every group or section, and a search.

In Go, `core/prefs/prefs_test.go` has the layout pref; `core/app/default_browser_settings_test.go`
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
