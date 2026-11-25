# Shells: choosing one, building one, testing one

A design note, 2026-11-13, for the fifth step of [profiles.md](profiles.md), with the
owner's decisions. A shell that passes its tests on the mock engine is expected to work in
the real app; "What the real app adds" lists the little the mock can't show.

## Decided

- The shell is only the representation of Go's data and the surface the user acts on.
- There are several shells to choose from. The choice is per profile and switches live.
- A shell may keep data only it understands, such as how it splits a window into panes.
  It is kept per window. Changing shell loses it, and the next shell starts from Go's
  data alone.
- A concept moves into Go only when every shell is expected to have it.
- Building a shell is friendly to its developer: it is written and tested on the mock
  engine, with no Chromium build.
- **A shell is held to the messages on channel `shell` and to the rules in "What every
  shell does", and to nothing else.** The SDK is a convenience the browser gives a shell's
  author on top of them: a shell may use it, or talk to Go itself.
- How a shell shows the extensions is the shell's own, and so is the band around the
  window.
- 2026-11-14, the owner: "think of the shells as external code which can interact with our
  go core... the go core has features which are shell independent so every shell gets to
  use those... tab groups... workspaces... if a shell doesnot want to support work spaces
  it can choose to merge everything or use only one workspace or something". Go's
  features, workspaces and tab groups when they land, are the shell's to use, not the
  shell's to own: every shell gets them through the messages. A shell that doesn't want a
  feature still copes with Go's data: for workspaces it can keep to one, for tab groups it
  can draw them or take every tab out of its group itself.
- **A shell's id is a code name, not its name** (the owner, 2026-11-24: "lets name the ids
  like ... code name like crshell arshell fxshell pbshell mwshell mnshell"). The id is what
  profiles save, folders and action ids carry, and it never changes; the name and the
  words about it are what people see, written once, in the registry, and free to change.
  Glide became Curve that day by one line there. The ids: `pbshell` Standard, `arshell`
  Curve, `tmshell` Terminal, `fxshell` Fox, `mnshell` Minimal, `mwshell` Mobile, `mkshell`
  Mobile (native).

## Where a shell lives

- **A shell is one folder,** `shell/src/apps/shell/<id>/`, with an `index.html` and a
  `main.js`, and **one line in `core/shells/registry.json`:** its id, the name the user
  sees and a line about it (the only place either is written: Settings, View, Shell, the
  menus, the welcome and the homepage read them from there, and a Go test fails on a copy),
  `"phone": true` for a shell made for a phone, and its own actions and those of
  Go's it takes, if it has any ([below](#a-shells-own-actions)). The id is lower-case letters,
  digits and dashes.
- Each shell builds into `shell/<id>/` beside the other pages, and loads at
  `chrome://shell/<id>/` in the real app and at `/shell/<id>/` on the mock. Nothing in the
  engine names a shell: it serves `chrome://shell/<id>/` from that folder.
- Every shell speaks on the one channel, `shell`. Go doesn't know which shell is on the
  other end.
- The shells: Standard (`pbshell`), the default and the reference shell (below); Curve
  (`arshell`), a sidebar of Spaces with the page as a card ([arshell.md](arshell.md));
  Terminal (`tmshell`), panes that split, as a terminal's do; Fox (`fxshell`), laid out as
  Firefox is; Minimal (`mnshell`); and on a phone Mobile (`mwshell`) and Mobile (native)
  (`mkshell`).

## The reference shell

The owner, 2026-11-14: Standard (`pbshell`) is the neutral, feature-complete reference shell, and
the default for new profiles and both profile templates.

- **Every Go feature shows there first.** A feature that a shell can show is drawn by
  Standard when it lands; the other shells follow when they choose to.
- **Tests run against it.** The checks on the mock engine and in the real app are written
  for Standard, and other shells have their own on top.
- **The internal pages share its look:** settings and the rest are drawn from the same
  design system ([design-system.md](design-system.md)), so they and Standard read as one.
- Curve and fox are styles to choose, Minimal the stripped-down one, and Mobile the phone's.

## Choosing a shell

- **Which shell a profile loads is a pref,** `shell`, whose value is a shell's id. A
  template can set it, and a reset to a template sets it too.
- The user chooses in View, Shell (a list read from the registry, with a tick on the one
  in use) and in the Profile section of settings.
- **Switching is a reload.** When the pref changes, every window's shell page loads the
  other shell. It says `ready` and gets the snapshot, as a shell does after a crash. The
  tabs keep running: their views stay where they were until the new shell reports its
  layout, unseen until it has drawn it. Go drops what the old shell claimed of the window,
  and its data.

## The SDK

`shell/src/sdk/` is what the browser gives a shell's author. Its core has no framework in
it: plain functions and plain objects. A Vue shell and a Svelte shell each use a thin
adapter over it; a shell with no framework uses the core as it is. A shell keeps no copy
of it.

| From | What |
|---|---|
| `@/sdk/shell.js` | `state`: everything Go sends, as a plain object; a field is replaced whole when it changes, never changed in place, except that a `tabs` message's tabs keep the favicon each had. Its `loaded` is false until Go's snapshot is in, and true from then on ([the splash](#the-splash)). `onChange(fn)`: calls `fn(fields)` after each change, with the fields that changed. `connect(handlers, afterDraw)`: says `ready`, keeps `state`, calls the shell's `handlers[kind](message)` after each message, and once the snapshot is drawn reports the layout and says `shown`. `send(msg)`. `saveData(value)`. `saveProfileData(value)`. `setPref(name, value)`. `focusPage()`. `setFrame(width)`. The helpers, each given the state it reads: `activeTab(state)`, `workspaceOf(state, tab)`, `controlSpace(state)`, `keyLabel(state, action)`, `tabLabel(tab)`, and `isNewTab(tab)`, true for a tab with nothing loaded, which has no page under the shell: the shell draws its new tab there, or a backdrop in the page's colour (`Canvas`). It puts the theme's colours on the root. |
| `@/sdk/regions.js` | The markers a shell marks its elements with, each with `mark(el)` and `unmark(el)`. `shell`: this element is the shell's, so presses on it reach the shell. `drag`: this element moves the window wherever it has no control: an empty space, or a whole bar of controls, whose gaps then drag; its controls, and anything in it marked `shell`, stay the shell's. `page`: `mark(el, tab)` and `update(el, tab)`, this tab's page shows here; 0 shows none. |
| `@/sdk/contextmenu.js` | What a shell sends about the right-click menu, `state.contextMenu`: `chooseContextItem(menu, id)`, `closeContextMenu(menu)`, and `contextMenuPoint(menu)`, where in the window the press was. `@/ui/PageMenu.vue` draws it in the design system's look, at the click; a Vue shell mounts it once. |
| `@/sdk/bookmarks.js` | `bookmark(url, title)`, `saveBookmark(id, title, parent)`, `removeBookmark(id)`, `isBookmarked(state, url)` and `folders(list)`, for a star on `state.bookmarks` and the panel `state.bookmarkPanel` asks for ([bookmark-folders.md](bookmark-folders.md)). |
| `@/sdk/suggest.js` | `suggest(id, text, kinds)`, `chooseSuggestion(row, newTab)` and `suggestionKinds`, every kind. `state.searchEngine` names the engine, for a placeholder. |
| `@/sdk/find.js` | `findText(tab, text)`, `findNext(tab, forward)` and `closeFind(tab)`, for `state.find`, each naming the tab whose bar it is. `@/ui/FindBar.vue` draws the bar, hung from the top right of the box that holds the page; a shell gives `connect({ focusFind: () => findBar.focus() })`, `findBar` from `@/ui/find.js`. |
| `@/sdk/dialogs.js` | `answerDialog(id, ok, text, password)`, `answerPasskey(id, action, text)` for a passkey request's step ([passkeys.md](passkeys.md); `@/ui/PasskeyCard.vue` and `.svelte` draw one), and `dialogFor(state, tab)`, the dialog a tab's page waits on. `@/ui/PageDialog.vue` draws a page's dialog and a site's sign-in over the box that holds the active page, which is `position: relative`. |
| `@/sdk/downloads.js` | What a shell sends about `state.downloads`: `downloadAction(id, action)` and `clearDownloads()`. `@/ui/Downloads.vue` is the Downloads button, shown once there is one, with its list; with `peek`, the list also opens by itself when a download begins, for a shell that gives `connect({ downloadStarted: () => downloadsPanel.started() })`, `downloadsPanel` from `@/ui/downloads.js`. |
| `@/sdk/sites.js` | A site's settings ([daily-driver.md](daily-driver.md#permissions)): the permission prompts, `promptFor(state, tab)`, `answerPermission(prompt, choice)`, and the words `permissionHost(prompt)` and `permissionAsk(prompt)`; and `hasSite(tab)` and `openSiteSettings()`. `@/ui/PermissionPrompt.vue` draws a prompt in the design system's look. |
| `@/sdk/notice.js` | A tab's notice, `tab.notice`: `runNotice(tab, action)`. `@/ui/PageNotice.vue` draws one in the design system's look. |
| `@/sdk/menu.js` | The shell's own menu, `state.menu`: `requestMenu()` as it opens, and `runMenuItem(item)` ([menus-shortcuts.md](menus-shortcuts.md#the-shells-menu)). Both adapters give them too. |
| `@/sdk/pins.js` | What a shell sends about `state.pins`, the profile's pinned tabs, and each tab's `pin`: `pinTab(id)`, `unpin(pin)` and `openPin(pin)` ([windows.md](windows.md#pinned-tabs)). Both adapters give them too. |
| `@/sdk/extensions.js` | What a shell sends about extensions: `pressExtension(id, el)`, `pinExtension(id, on)`, `manageExtensions()`, `answerExtensionPrompt(on)`. |
| `@/sdk/controls.js` | `controls`: the one selector for what a control is, something the user presses or types in (buttons, links with an address, fields, selects, text areas, `role="tab"` and `role="button"`, anything with a `tabindex`). The SDK finds a drag bar's controls by it, and the check reads it for rule 6. |
| `@/sdk/vue.js` | The Vue adapter: `state`, a reactive copy that follows the SDK's; `activeTab` and `controlSpace` as computeds; `workspaceOf(tab)` and `keyLabel(action)`; `connect(handlers)`, which waits for Vue's `nextTick`; the directives `v-shell`, `v-drag` and `v-page="tab.id"`; and `send`, `saveData`, `focusPage`, `setFrame`, `tabLabel`, `isNewTab`. |
| `@/sdk/svelte.svelte.js` | The Svelte 5 adapter: `state`, a `$state` copy that follows the SDK's; `connect(handlers)`, which waits for Svelte's `tick`; the actions `use:shell`, `use:drag` and `use:page={tab.id}`; and the SDK's helpers, given that `state`. |
| `@/theme.js`, `@/design.css` | Painting Go's tones, light or dark, and the shared controls ([design-system.md](design-system.md)). |
| `@extensions` | `ExtensionsToolbar`: the Vue shells' extensions buttons, their menu and the install prompt, drawn from `state.extensions`. `extensionsBuilt`: whether the build has extensions. |

The SDK measures the marked elements and sends the layout by itself, once a frame and only
when it changed. It watches each `drag` element's subtree, so a control that comes or goes
in a bar, such as a tab added to a strip, is measured with nothing from the shell. A shell
on the SDK never sends `layout`. It does not measure on a scroll: a strip that scrolls is
marked `shell`, with `drag` spaces beside it, as the fox shell's is.

Go's window manager orders the regions, and the first that holds a point decides: the
controls in the drag bars (the shell's), then the drag bars, then the rest of the shell's,
and the pages under everything. In full screen nothing drags, so a bar that is also `shell`
is the shell's there.

## The messages

One JSON object each way, with its kind in `t`. A shell sends one with
`chrome.send('shell', [JSON.stringify(msg)])`, which the engine gives the page, and Go
calls `window.__shellReceive(json)` with each of its own, a JSON string. Nothing reaches a
shell before it has set `__shellReceive` and said `ready`. On the mock, `/mock/webui.js`
gives the page the same two.

**Go to the shell**

| `t` | Carries | When |
|---|---|---|
| `snapshot` | Everything below at once, and `data` | In answer to `ready` |
| `tabs` | `tabs`, `active`, `groups`: this window's tabs in order, with no `favicon`, the active one's id (0 when it has none), and its tab groups | Tabs open, close, move, another becomes active, or a group changes |
| `tab` | `tab`: one tab's new state, its favicon with it | Its title, address, loading, favicon or history changed; or it is new to this window's list, as a tab moved in from another window is, with a favicon: then after the `tabs` that lists it |
| `theme` | `theme`: `system`, `light` or `dark` | The pref changed |
| `windowColor` | `windowColor`: `#rrggbb`, and `tones`: `{light, dark, hues}`, every colour made from it, each palette the design system's tokens by name (`core/tones`), and the seven hues a workspace or group keeps, `{hue, name, light, dark}`, drawn for this frame | The frame colour changed |
| `controls` | `controls`, `cornerRadius` | The native window buttons moved, or full screen |
| `keys` | `keys`: the key each action shows, by action id | The keymap changed |
| `workspaces` | `workspaces`: id, name and colour of each | The list changed |
| `pins` | `pins`: the profile's pins, each `{id, workspace, url, title, favicon}`, in their order | A pin was made or removed |
| `menu` | `menu`: the menu bar's menus for this window, each item `{title, action, enabled, checked}`, `{separator}` or `{title, items}` | The shell asked, with `menu` |
| `update` | `update`: what the updater is doing | It changed |
| `crashLog` | `crashLog`, `crashReport` | The notice of the last run's crash was closed |
| `tabLayout` | `tabLayout` | The standard shell's tab position changed |
| `addressBarPosition` | `addressBarPosition`: `bottom` or `top` | The Mobile shell's address bar moved, from settings on a phone |
| `focusAddressBar` | | The user asked for the address bar |
| `action` | `id`: one of the shell's own actions | Its key was pressed, or its menu item chosen, while this shell is on |
| `suggestions` | `id`, `items`: rows `{kind, title, detail, url, tab, action, inline, typed}` | In answer to `suggest`: one ranked list, each row an `address`, a `search`, a `tab`, a `history` page, a `bookmark`, an `action` or, with nothing typed, a `closed` tab. The first row may carry `inline`, the rest of the address the typed text starts, for the field to show selected after the caret ([daily-driver.md](daily-driver.md#search-engines-and-one-suggestion-source)) |
| `bookmarks` | `bookmarks`: the profile's bookmarks, newest first, each `{id, title, url, added}` | One was added, edited or removed ([daily-driver.md](daily-driver.md#history-and-bookmarks)) |
| `searchEngine` | `searchEngine`: `{id, name}` | Another search engine was chosen in settings |
| `paint` | `span` | Go wants a timeline span ended once this is drawn |
| `find` | `find`: the active tab's search while its find bar is open, `{tab, text, matches, active}`, or `null` | A search ran, its count changed, the bar opened or closed, or another tab shows ([daily-driver.md](daily-driver.md#find-in-page)) |
| `pageFullScreen` | `pageFullScreen`: the tab whose page fills the screen in this window, or 0 | A page asked to fill the screen, as a video's button does, or it ended ([daily-driver.md](daily-driver.md#video-full-screen)). While it is the active tab, the shell lays that page across the whole window and drops its band too (`setFrame(0)`), and puts both back after |
| `focusFind` | | The find key was pressed: the keyboard goes to the find bar |
| `dialogs` | `dialogs`: the dialogs pages wait on, oldest first | A page asked, or one was answered or dropped ([daily-driver.md](daily-driver.md#javascript-dialogs)) |
| `downloads` | `downloads`: the profile's downloads, newest first | One started, moved on, ended or left the list ([daily-driver.md](daily-driver.md#downloads)) |
| `downloadStarted` | `id`: the download's id in `downloads` | A tab of this window began a download while the window was in front; after the `downloads` that lists it. Standard opens its Downloads list by itself on it |
| `contextMenu` | `contextMenu`: the open right-click menu, or `null` | A right click in a tab's page, or a long press on a link or an image on a phone: what it was on and the actions that can run there ([daily-driver.md](daily-driver.md#right-click)); `null` once it is answered, or its tab closes, navigates or is no longer shown |
| `backKey` | | The phone's back key, while the shell said it closes something (`backCloses`) |
| `keyboard` | `on` | The phone's soft keyboard came up or went: while it is up, the bars under the page step aside, but for the shell's own field that has it (docs/design-system.md, On a phone) |
| `scroll` | `tab`, `direction`: `down` or `up` | A tab's page began to scroll down, or back up (content's `DidChangeVerticalScrollDirection`) |
| `extensions` | `extensions`: the buttons, the open popup and the prompt | While the Extensions flag is on: an extension changed, or the active tab did |
| `profileData` | `profileData`: what this shell saved for the profile | A shell in any window of the profile saved it |
| `prefs` | `prefs`: every one of Go's prefs by its name in the file (`frameColor`, `theme`, `tabLayout`, `shell`, `addressBarPosition`); the snapshot carries it too | Any pref changed, after the message of its own (`windowColor`, `theme`, ...) |
| `permissionPrompts` | `permissionPrompts`: every prompt open in the profile, `{id, tab, origin, kinds, once}` | A site asked to use its camera, microphone or notifications, or a prompt was answered or dropped |
| `passwordOffer` | `passwordOffer`: the saved logins offered at a field of a tab's page the person pressed, `{id, tab, box, site, framed, insecure, field, logins: [{id, username, savedFor}], at}`, or null; the snapshot carries it too. `box` is the field in the tab's page; `at` the login the keys or the pointer are at, -1 for none. It holds no password ([password-autofill.md](password-autofill.md)) | A sign-in's field was pressed, the keys moved in the list, or the offer went: filled, dismissed, a press elsewhere, the tab going on |
| `passwordSave` | `passwordSave`: the offer to save a login a person just signed in with in a tab, or to update a saved one's password, `{id, tab, site, username, update}`, or null; the snapshot carries it too. It holds no password ([password-autofill.md](password-autofill.md)) | A sign-in was seen to go through, or the offer went: answered, another sign-in sent in its tab, its tab gone to another site, or closed |
| `passwordSaveShown` | `passwordSaveShown`: `{offer, password}`, what the person typed, to the shell that asked with `passwordSaveShow` alone; null once the offer has gone | A shell asked to show it, or the offer went |
| `sync` | `sync`: sync's status, `{on, syncing, host, lastSynced, error, problem, locked}`; the snapshot carries it too ([sync.md](sync.md#the-functions-coresync)) | It changed: sync turned on or off, a run began or ended, or the profile's keys don't open |

The snapshot's `profileData` is what this shell last saved for the profile, `null` when
nothing. Its `actions` is every action in the menus that can run then, `{id, title}`, in
the menus' order, for a shell that lists them to run by name (`run`).

A tab is `{id, url, title, loading, progress, blocked, crashed, error, errorCode, favicon,
thumb, canGoBack, canGoForward, workspace, group, page, pin, connection, connectionNote, notice}`,
`thumb` the address of a picture of its page as it last looked, a small image for an `<img>`
that changes with each new picture, absent before the first and once the tab commits a page at
another address (a reload or a new `#fragment` keeps it); a shell shows the favicon without it.
Only a phone shell's windows get pictures (`captureTab`). `group` the id of its tab group, absent when it is in none; `pin` the id of the pin it is open
for, absent when it is open for none (a window's pinned tabs come first in its list, in
the pins' order); `connection` how its page reached it, `secure` or `insecure`, absent when
it isn't a site, and `connectionNote` why, in one line
([daily-driver.md](daily-driver.md#permissions)). `error` says why its page failed to load, absent when it didn't:
`not-found`, `offline`, `refused`, `unreachable`, `timeout`, `tls`, `proxy`, `redirects`,
`bad-response`, `blocked`, `file` or `other` (engine.Failure). `errorCode` is the engine's
own name for the error, such as `ERR_NAME_NOT_RESOLVED`, to show under the sentence and
never to match on. `page` says the tab is one of ours, and which of our pages it shows:
`settings`, the only one so far; it is absent for a web tab. A tab of ours is drawn,
switched, laid out and closed as any tab is ([windows.md](windows.md#our-own-pages-are-tabs-decided-2026-11-14)),
but it is in no workspace and no group: its `workspace` is the one it was opened from, where
New Tab from it goes, so `workspaceOf` gives a shell no dot for it. Rule 19 says how a shell
shows it. `notice` is what to draw over its page while it has nothing to show, `{kind,
title, detail, actions}`, absent otherwise
([daily-driver.md](daily-driver.md#error-pages-and-certificate-warnings)). The snapshot also has `running` (the version), `platform`
(`mac`, as the keymap names it) and `data`, and `extensions` only while their flag is on.

- `controls` is `{x, y, w, h}`, where the native window buttons are, zero size when there
  are none; `cornerRadius` is the radius the OS rounds the corners with, 0 in full screen.
- `update` is `{phase, version, error, done, total}`: `phase` is `""`, `downloading`,
  `ready` or `failed`.
- `keys` is the key each action shows, by action id, written as the keymap writes it
  (`Cmd+T`); `@/keys.js` and `keyLabel` show it as the menus do.
- `workspaces` is each workspace's `{id, name, color, tones}`, in the user's order, one at least. `color`, as a group's, is one of the seven hues in whole degrees (`"181"`), drawn as `tones.hues` says, or a picked `#rrggbb` ([design-system.md](design-system.md#colour)); `tones` is its colour's, as `windowColor` carries the frame's, a hue filled at gulal's middle.
  Main, the profile's own storage, has the id `""`. A tab's `workspace` is its workspace's
  id. Every shell and the settings page hear the list again whenever it changes, whoever
  changed it. The workspace messages are the settings page's, and keep its rules: a name
  no other workspace has, whatever its case, and never deleting the last. A shell names
  the workspace by `workspace`, as `newTab` does, where the settings page says `id`.
- `pins` is every pin of the profile, in every workspace, `{id, workspace, url, title,
  favicon}`, the title and icon its tab had when pinned. Every window hears the same list;
  a pin is open in a window when one of its tabs has its id in `pin`, and has no tab after
  a restart, or once its tab closed, until `openPin`. Go has no rule that a shell draw
  them: one that draws none still lists the pinned tabs first ([windows.md](windows.md#pinned-tabs)).
- `groups` is each of the window's tab groups, `{id, name, color, collapsed, tabs}`, in the
  order they were made, `tabs` its tabs' ids in the group's own order
  ([groups.md](groups.md)).
- `suggestions` has the `id` the shell asked with and `items`, each `{url, title}`.
- `extensions` is `{buttons, popup, prompt}`. A button is `{id, name, title, icon, badge,
  badgeColor, badgeTextColor, enabled, pinned}`, in the order the menu lists them; `popup`
  is the id of the extension whose popup is open, or `""`; `prompt` is an install or a
  removal waiting for an answer, `{id, name, version, icon, permissions, remove}`, or
  `null`.

**The shell to Go**

| `t` | Carries | Does |
|---|---|---|
| `ready` | | Asks for the snapshot. Nothing reaches a shell before it |
| `shown` | | The snapshot is drawn and the layout sent |
| `layout` | `shell`, `controls`, `drag`, `tabs`, `viewW`, `viewH`, `frame` | The regions, the pages shown, and the band. Below. The SDK sends it |
| `newTab` | `workspace`, optional: a workspace's id; or `groupWith`, a tab's id; optional `text`, `typed` and `background` | Opens a tab in this window and makes it active, in the workspace named, or without one in the active tab's, or the first in the list when the window has no tab. With `groupWith`, in that tab's workspace and group, starting a group of the two when it has none. With `text`, the tab goes there, an address or a search, and for one of our pages' addresses, such as `chrome://settings`, the window's tab of that page shows instead; with `background`, it opens behind the active tab, as a link opened in a new tab does; with `typed`, as `navigate` has it |
| `createWorkspace` | `text`, `color` | Makes a workspace with that name and `#rrggbb` colour, at the end of the list |
| `renameWorkspace` | `workspace`, `text` | Renames a workspace, by its id |
| `editWorkspace` | `workspace`, `text`, `color` | Renames and recolours a workspace together; a name refused changes neither |
| `moveWorkspace` | `workspace`, `by`: -1 or 1 | Moves a workspace one place up or down the list |
| `deleteWorkspace` | `workspace` | Deletes a workspace: Go takes it out of the list, closes its tabs in every window, a window whose every tab was in it staying open with none, and erases its storage. The last one is refused |
| `pinTab` | `id` | Pins a web tab with an address: a pin of it at the end of its workspace's pins, with the tab open for it, which goes among the window's pinned tabs and out of its group. A tab of ours, or one with no address yet, isn't pinned |
| `unpin` | `pin`: a pin's id | Removes the pin. A tab open for it stays, as an ordinary tab first after the pinned ones |
| `openPin` | `pin` | Shows this window's tab for the pin, or opens one for it in its workspace and at its address, in its place among the pinned tabs |
| `createGroup` | `ids`, optional `name` and `color` | Makes a group of the tabs: the first's window and workspace, and the rest that share them |
| `addToGroup`, `removeFromGroup` | `id`, and `group` to add to | Puts a tab in a group, or takes it out |
| `editGroup` | `group`, and any of `name`, `color`, `collapsed` | Changes what is sent |
| `ungroup`, `closeGroup` | `group` | Ends a group, its tabs staying; or closes its tabs |
| `moveTab` | `id`, `index`, optional `window` | Moves a tab to `index` in its window's order, the place it has after the move, clamped; the active tab stays. Between two tabs of a group it joins the group, and out of its group's run it leaves it ([groups.md](groups.md)). With `window`, another window's id, it moves into that window at `index`, which it leaves if it was its last tab; an unknown window changes nothing. The standard shell's dragging sends it; a tab let go outside the window goes to a window of its own, by the Move Tab to New Window action. With pins: an unpinned tab stops after the pinned ones, and a pinned tab stays where it is, as pins keep their order; a pinned tab moved into a window that has a tab for its pin shows that tab instead ([windows.md](windows.md#pinned-tabs)) |
| `closeTab` | `id`, optional `then` | Closes a tab. With no `id`, or 0, it is Close Tab: the active tab. Closing a window's last tab closes the window. With `then: "recent"` it closes the tab given or the active one, and when that was active the tab used just before it takes its place rather than its neighbour (Go keeps each window's tabs in the order they were last used) |
| `activateTab` | `id` | Makes a tab the active one |
| `navigate` | `text`; optional `typed` | Takes the active tab to an address or a search. One of our pages' addresses shows the window's tab of that page instead, and one of our tabs takes any other address to a new tab beside it. `typed`, sent by an address field for what was typed and entered, makes an address a typed one once it loads, which suggestions list first ([store.md](store.md#history)); a bookmark or a tile sends none |
| `back`, `forward`, `reload` | | Act on the active tab |
| `focusTab` | `id` | Gives the active tab's page the keyboard. One for a tab no longer active, as when the address typed for it went to another tab, does nothing |
| `focusShell` | | Gives the shell the keyboard |
| `captureTab` | | Asks for a picture of the active tab's page, for its `thumb`, as the Mobile shell's switcher opens: send it before the layout that covers or hides the page. Only a phone shell's windows get one, and Go itself pictures the tab a window leaves as another becomes active. A window gets one picture each half second at most, one asked for sooner being dropped; never one of a tab with no address, which shows the shell's new tab page, nor of a tab with a `notice` |
| `suggest` | `id`, `text`, `kinds`, `rows` | Asks for suggestions with rows of the kinds named, at most `rows` of them besides the actions (left out: 6 under three letters, 8 from three; docs/daily-driver.md, What the address field suggests). The bare message without `kinds` gets history alone, as it always did; the SDK's `suggest()` asks for every kind unless told otherwise. With no text, what a new tab offers: with `closed`, the tabs closed lately; then the sites visited most and the newest bookmarks (docs/daily-driver.md, A new tab) |
| `forgetSite` | `url` | Takes the site at `url` out of history, every page of it, as the new tab page's × does |
| `bookmark` | `url`, `title`, `on` | Bookmarks the address, or takes its bookmark away: the address bar's star |
| `chooseSuggestion` | `row`, `newTab` | Runs a row: Go loads its address or search in the active tab, or a new one, shows its tab, reopens a `closed` tab, or runs its action. A row of a list for typed text carries `typed`, and its address is a typed one once it loads |
| `run` | `text`: an action's id | Runs an action, as its menu item would |
| `copyText` | `text` | Puts the text on the clipboard, as a context menu's Copy Link does. A shell page's own write needs a click, which an action from the menu bar or a key hasn't |
| `shareText` | `text` | Hands the text to the system's share sheet, as a context menu's Share Link does: a phone's field shares its page's address |
| `menuOpened` | | A phone shell's menu sheet opened: Go starts the window's settings page out of sight, as Settings is a tap away, so the tap shows it drawn ([windows.md](windows.md#settings-loads-as-the-phones-menu-opens-decided-2026-11-19)). A desktop shell doesn't send it |
| `menu` | | Asks for the menu bar's menus for this window, as the shell opens its own menu; Go answers with `menu` ([menus-shortcuts.md](menus-shortcuts.md#the-shells-menu)) |
| `window` | `text`: `close`, `minimise`, `zoom` or `fullScreen` | Asks the window |
| `data` | `data`: any JSON | Replaces this window's copy of the shell's own data |
| `profileData` | `data`: any JSON | Replaces what this shell keeps for the whole profile, on disk |
| `relaunch`, `dismissCrash` | | The update bubble's and the crash notice's buttons |
| `setPref` | `name`: a pref's name in `prefs`; `value` | Sets one of Go's prefs through its own setter, so with the same checks as the settings page's writes: a value it refuses, or a name that isn't a pref, changes nothing. Every window hears it |
| `setTabLayout` | `text`: `vertical` or `horizontal` | Sets the tab position pref, which only the standard shell reads |
| `setAddressBarPosition` | `text`: `bottom` or `top` | Sets the address bar position pref, which only the Mobile shell reads |
| `contextMenuChoose` | `id`, `item` | Runs action `item` of right-click menu `id`; Go closes the menu |
| `contextMenuClose` | `id` | The menu went without running anything |
| `findText` | `tab`, `text` | Searches the tab's page afresh; nothing once its bar has closed |
| `findNext` | `tab`, `forward` | Shows the next match in the tab's page, or the one before |
| `findClose` | `tab` | Closes the tab's bar, keeping the match shown selected |
| `answerDialog` | `id`, `ok`, `text`, `password` | Answers a page's dialog; `text` is a prompt's, or a sign-in's username with its `password`. For a passkey request, no cancels it and yes does nothing |
| `answerPasskey` | `id`, `action`, `text` | What the person did at a passkey request's step: `cancel`, `pin` with the PIN, `account` with the row's index, `phone`, `key`, `bluetoothOn`, `bluetoothSettings` or `retry` ([passkeys.md](passkeys.md)). The card stays until Go sends the next step or drops the dialog |
| `downloadAction` | `id`, `action`: `open`, `reveal`, `cancel`, `pause`, `resume` or `remove` | Acts on one download; `remove` takes it off the list and leaves the file |
| `clearDownloads` | | Takes every finished download off the list |
| `shareReport` | `rect`: where the control that asked is | Writes a crash report in the profile (core/crash/report.go) and hands it to the system's share sheet, anchored there. Nothing leaves the device unless the user sends it |
| `saveReport` | | Writes the same report and keeps it as a finished download (`downloads.Keep`): copied into the downloads folder, listed with `from` "Crash report" and no `url`, shown by the system as a finished download (on a phone its notification, whose tap opens it), published (engine `PublishFile`: on a phone, into the shared Downloads), and told to the window as `downloadStarted`, so a shell shows it as it shows a download begin. Android's share sheet may have nowhere to save a file |
| `backCloses` | `on` | While on, the phone's back key comes to the shell as `backKey`, before the page's history: the shell has a sheet open, or the address in edit |
| `extensionClick` | `extension`, `rect` | A press on an extension's button at `rect`: Go opens its popup under it, closes it when it is the one open, or tells the extension it was clicked |
| `extensionPin` | `extension`, `on` | Puts the extension's button on the toolbar, or takes it off |
| `extensionsManage` | | Opens settings at its Extensions section |
| `extensionClosePopup` | | Closes the extension popup that is open |
| `extensionAnswerPrompt` | `on` | Answers the install or removal prompt |
| `permissionAnswer` | `id`, `choice`: `allow`, `once`, `block` or `dismiss` | Answers a permission prompt; `once` only where the prompt says `once` |
| `passwordPick` | `offer`, `login` | Fills a login of the offer showing into the page: its username and its password, into the fields Go found. Nothing asks the device to check the person |
| `passwordAt` | `offer`, `at` | The pointer is over a login of the offer's list: it is the current one, which Enter in the page's field fills |
| `passwordDismiss` | `offer` | Takes the offer away with nothing filled |
| `passwordsManage` | | Opens settings at Passwords |
| `passwordSaveAnswer` | `offer`, `choice` | Answers the offer to save: `save` saves the login, or gives the saved one its new password; `never` has the site not offered on again; `dismiss` only takes the offer away |
| `passwordSaveShow` | `offer` | Asks for the password the offer would save, to show the person; answered with `passwordSaveShown` |
| `openSiteSettings` | | Opens settings at the site of the window's active tab |
| `noticeAction` | `id`: the tab, `action`: one of its notice's actions' ids | Does what the person chose on a tab's notice: `reload`, `back` or `proceed` |
| `timeline` | `kind`, `name`, `text`, `at` | An entry on the app's timeline ([logging.md](logging.md)): `kind` is `mark`, `begin`, `end`, `info`, `warn` or `error`, and `at` the page's `performance.now()`. The SDK sends them |

**The layout.** Each rectangle is `{x, y, w, h}` in CSS pixels of the shell's page, which
covers the window. `shell` is the shell's own areas; `drag`, the bars and spaces that move
the window; `controls`, the controls inside those bars, which stay the shell's; `tabs`,
`{tab, rect}` for each tab whose page shows, in the order the shell shows them; `viewW` and
`viewH`, the page's size it measured against; `frame`, the width of the band the window
draws around its edge in the frame colour, over the shell and the pages, 0 for none. A
window has no band before its shell asks for one, and keeps it while a shell that crashed
or was switched loads again.

The active tab is the one the address bar, back, forward and reload act on. A shell that
shows several tabs makes the one the user works in active: it sends `activateTab` when the
user picks one in the shell, and Go makes a tab active by itself when a press in its page
gives it the keyboard.

### A shell without the SDK

The SDK does these for a shell on it. A shell that talks to Go itself does them itself.

1. It sets `window.__shellReceive`, sends `ready`, and keeps what each message says: the
   snapshot has it all, a message of state sets the fields it carries, and `tab`
   replaces that one tab in `tabs`. A `tabs` message's tabs carry no favicon: each keeps
   the one it had, by its id. Go leaves them out of the list, a data URL each that it
   would send again on every change to every window; the snapshot and `tab` carry them.
2. It draws the snapshot, sends `layout`, then sends `shown`. It sends a layout only once
   it has drawn what it measured: the window shows the pages a layout first places after
   the shell's next frame, so that frame has to hold it.
3. It sends `layout` again whenever anything in it changes: a resize, a tab shown in
   another place, a control that comes or goes in a drag bar, something that moves in an
   animation. The SDK sends it at most once a frame, and only when it changed.
4. It sends `focusTab` only after a layout that shows that tab, and `focusShell` before it
   puts the keyboard in a field of its own while a page has it.
5. While `extensions.popup` names an open popup, a press anywhere in the shell sends
   `extensionClosePopup`, except a press on that popup's own button, which goes to Go as
   `extensionClick` and closes it there.
6. It paints its own colours from `tones` and `theme`: `tones` is Go's two palettes of the
   frame colour, light and dark, and the theme pref, or the system while it says `system`,
   picks one, as the SDK does.
7. For the timeline only: it answers `paint` with a `timeline` entry of kind `end` and that
   span once it has drawn. Without it the span stays open.

## Colours

The owner, 2026-11-14: "colors live in go... shells can choose to maintain there own... or
change the browser pref via some shell config page if they want to... standard uses the go
prefs itself".

- **The profile's colour is Go's:** the frame colour pref (`frameColor`), which the window's
  band, the Dock and the profile list follow, and which reaches a shell as `windowColor`.
  The theme is Go's too (`theme`). The settings page is where the user changes them.
- **Standard uses Go's prefs and keeps no colour of its own,** painting the `tones` Go makes
  from `windowColor`, light or dark as `theme` says.
- **Another shell may keep colours of its own** for its own look, in its own data, but the
  frame colour stays the profile's.
- **A shell may change Go's colour pref** from a page of its own: shells are privileged,
  and set any of Go's prefs with `setPref` ([Go's prefs](#gos-prefs)).

## Go's prefs

The owner, 2026-11-14: "shells and internal pages all are privileged so the shells get to
control everything... we can start with a simple api interface for the shell to the prefs".
A shell reads Go's prefs from `state.prefs` and sets any of them with `setPref(name,
value)`, with the checks the settings page's writes get; every window hears the change.
The profile's frame colour and theme are among them, so a shell with a page of its own for
colours changes Go's, which the window, the Dock and the profile list follow.

## A shell's own data

`saveData(value)` sends the whole value; Go keeps the latest for the window and never
reads it. It comes back as `state.data` in the snapshot, so a shell that was reloaded lays
itself out as it was. A new window's shell gets none. It is dropped when the shell
changes, the profile is reset to a template, or the window closes. It is in memory for
now, and will be saved with the windows and tabs when sessions are.

**What a shell keeps for the profile.** `saveProfileData(value)` sends the whole value as
`profileData`; Go writes it to `shells/<id>.json` in the profile folder and sends it to the
shell in every window, and it comes back as `state.profileData` in every snapshot. Go never
reads it. It survives a restart, a closed window and a switch to another shell and back; a
reset to a template keeps it, as it keeps the workspaces. It names no tabs, since a tab's id
lasts only as long as the tab: Curve keeps its Favorites here.

A shell's data names tabs by id. A tab in the data that Go no longer lists is gone, and a
tab Go lists that the data doesn't have is new: Go's list is the truth.

## A shell's own actions

A shell may have actions of its own, with keys, such as the terminal's splits. Each is an
ordinary action in the keymap ([menus-shortcuts.md](menus-shortcuts.md#a-shells-own-actions)):

- **The shell declares them** in its entry in `core/shells/registry.json`, each an id and
  the title its menu item shows: `"actions": [{"id": "tmshell.splitRight", "title":
  "Split right"}]`. The id starts with the shell's id and a dot, and never changes once
  released.
- Go registers them, and each is enabled only while its shell is the profile's. Their keys
  are in the keymap templates and their place in the menu tree, as any action's.
- **Running one sends the shell `action`** with its id. The shell handles it as it would
  its own control: with the SDK, `connect({ action: (m) => ... })`. With another shell on,
  the action is disabled: its key goes on to the page and no shell hears it.
- **A shell may take one of Go's actions,** to run it its own way while it is the
  profile's: `"takes": ["tab.new"]` in its entry. The action then reaches the shell as
  `action` with its id, whichever way it was run (its key, its menu item, anything else
  that runs it), in place of what Go does. The key and the menu item stay Go's, so
  the user's own key for it still applies. Go begins a span named for the action and asks
  the shell to end it (`paint`), which the SDK does once the shell has drawn. What a
  shell asks of Go by its own messages is unchanged: a shell that takes New Tab still
  opens a tab with `newTab`. A window opens with one new tab whatever the shell, at the
  app's start and for New Window. Standard's New Tab goes to the window's new tab, one
  with nothing loaded, in the workspace shown, opening one when there is none, and its box
  shows on it ([windows.md](windows.md#every-window-has-a-tab-decided-2026-11-18)).

## What every shell does

The check holds every shell to these, knowing nothing else about it.

**Rule numbers.** A rule's number is its name in the check and in reports, and never
changes once given. They are handed out by range, so builders working at once don't take
the same one (settled 2026-11-14):

| Numbers | What, or whose |
|---|---|
| 1–13 | The first rules, below. |
| 14 | The splash before Go's data. |
| 15 | JavaScript dialogs. |
| 16 | The right-click menu. |
| 17 | Downloads. |
| 18 | A new tab, and the last tab closing. |
| 19 | Our own pages' tabs: a title, not an address field. |
| 20–29 | The daily-driver features still to come: find in page, video full screen, HTTP sign-in, file upload and what follows them. |
| 30–39 | Permissions and their prompt, tabs restored after a crash, certificate warnings, error pages. |
| 40–49 | Pinned tabs, and Go features after them. |

A rule whose feature a shell doesn't have yet lists that shell in the check, with who brings
it there; a phone shell may be left out of a rule for a desktop feature.

1. It says `ready`, draws the snapshot, and says `shown`. The window shows its pages only
   after the shell's frame with the layout that places them, never on the bare window.
2. **The active tab's page shows,** after every change: a new tab, another made active, a
   close, a tab a page opened, a resize; a new tab, with nothing loaded, has no page, and
   the shell draws its new tab there (rule 18). It asks for a page's
   keyboard (`focusTab`) only once the layout it sent shows that page: the window can't give
   the keyboard to a page it hides, and no key reaches Go until something takes it. A tab
   with no address holds no page of anyone's yet, so a shell may show its own view there
   instead, as Standard's command box does over a new tab; the check's tabs hold pages.
3. The pages it shows lie inside the band it asked for, don't overlap, and are all tabs of
   this window.
4. **Each tab of the window is an element with `role="tab"` and `data-tab="<id>"`,** the
   active one with `aria-selected="true"`. A click on it makes that tab active. It may
   lie in a sheet one of the shell's controls opens, as a phone's tab switcher does: it is
   always in the document, hidden while the sheet is closed. Tabs in a group
   ([groups.md](groups.md)) are no different: a shell that draws no groups lists them as
   any other, and every tab stays one click or one sheet away.
5. **`focusAddressBar` puts the keyboard in the field named Address,** where an address
   and Enter take the active tab there, while a web tab is active.
6. **Every control it draws is the shell's,** a control as `@/sdk/controls.js` says: it
   lies in a region it marked `shell`, or in a `drag` bar, so a press reaches it and not
   the page beneath, nor the window's drag.
7. A press in the middle of a page it shows reaches that page.
8. Some space is marked `drag`, and no page lies under the native window buttons.
9. Reloaded, it comes back showing the same pages: what it needs to do so it saved.
10. Switched to and away from, it starts from Go's data alone.
11. **Settings is one press away, or two:** a control named Settings, on the shell or in a
    menu one of its controls opens, opens the settings tab, or shows it when the window
    has one.
12. **The basic controls are somewhere in the shell, found by role and name, and each does
    its job.** The check finds each one only as a screen reader would, by its role and
    its accessible name, never by a class or where it sits, and presses it or types into
    it to see Go do what it says.
13. It logs no errors.
14. **Before Go's data comes it draws a splash of its own, marked `aria-busy="true"`, with
    none of its controls; when the data comes the splash goes and the UI shows.** The
    check holds the shell's `ready`, so the snapshot doesn't come, and finds the splash and
    none of rule 12's controls by role and name; then it lets the snapshot through, and
    once the transition is over finds no `aria-busy="true"` left and the controls there.
    The shells that don't have it yet are listed in the check, with who brings each.
15. **A page's alert, confirm and prompt show as a `role=alertdialog`** with Go's words and
    buttons by their names (`ok` and `cancel` in `state.dialogs`), and a text box for a
    prompt; answering reaches the page. Another tab shown hides it, and showing its tab
    again brings it back. Every shell, a phone's too, since the page waits until answered
    ([daily-driver.md](daily-driver.md#javascript-dialogs)). The shells that don't have it
    yet are listed in the check, with who brings each.
16. **On a desktop, a right click in a page opens a `role=menu` from Go's
    `state.contextMenu`;** choosing a `menuitem` whose name is one of Go's action titles for
    that press makes Go do it, and Escape, or a press outside, closes it. Which of Go's
    actions it shows, and in what order, is the shell's ([daily-driver.md](daily-driver.md#right-click)).
    A shell that draws nothing shows no menu, which this catches. The shells that don't have
    it yet are listed in the check, with who brings each.
17. **A control named Downloads shows once the profile has a download,** and opens a list
    where each download is a `listitem` by its file name, a finished one with Show in
    Finder, which makes Go show the file ([daily-driver.md](daily-driver.md#downloads)). The
    list may be the shell's own, or the Downloads page, one of our tabs, which the control
    opens by running `downloads.show`. On a phone the control may be an item of the menu one
    of the shell's controls opens, and Show in Finder is Show in Downloads, the phone's
    Downloads app, which the phone's Downloads page keeps in the row's ⋮, a sheet named by
    the file. The shells that don't have it yet are listed in the check, with
    who brings each.
18. **A new tab has nothing loaded** ([windows.md](windows.md#every-window-has-a-tab-decided-2026-11-18)):
    no renderer and no page, so the shell draws its new tab where the page goes, as its
    own, so a press there reaches the shell: its new tab page, its box, or at least a
    backdrop in the page's colour (`isNewTab`). A control named New Tab shows one, the
    active tab. What New Tab does is the shell's: Standard goes to the window's new tab.
    A window always has a tab: closing its last one closes the window, so a shell draws no
    empty state.
19. **For one of our tabs, the shell shows the page's title where the address goes, with a
    way to close the tab, and no field to type in.** The title is the tab's, as Go sends
    it, which the page may change, such as "Downloads › In progress". The check holds every
    shell to it but those not drawing it yet, each named with who brings it there
    (`shell/e2e/shell-contract.cjs`).
20. **The find key (`find.open`), or on a phone the menu's item for it, opens a text box
    named Find in page with the keyboard in it;** what is typed searches the active tab's
    page and the count shows, Enter shows the next match, and Escape, or a phone's back
    key, closes it and ends the search ([daily-driver.md](daily-driver.md#find-in-page)). The shells that don't have it yet are
    listed in the check, with who brings each.
21. **A page in full screen (`state.pageFullScreen`, the active tab) covers the whole
    window with nothing of the shell on it,** no band and no controls; when it ends, on
    Escape, which Go takes, or a phone's back key, the window is as it was
    ([daily-driver.md](daily-driver.md#video-full-screen)). The shells that don't have it yet
    are listed in the check, with who brings each.
22. **A site's sign-in (`kind` `signin` in `state.dialogs`) shows as a `role=alertdialog`
    with text boxes named Username and Password,** and Go's Sign in button answers with both
    ([daily-driver.md](daily-driver.md#sign-in-to-a-site)). Every shell that draws dialogs;
    the check skips the shells rule 15 lists as not yet.

30. **A site's question shows on its tab** ([daily-driver.md](daily-driver.md#permissions)):
    while the active tab has a prompt in `permissionPrompts`, the shell shows a `dialog`
    named by its question, "*site* wants to use your camera and microphone", with buttons
    named Block and Allow, or, where `once`, Block, Always allow and Allow this time; an
    Allow grants it, Block denies it, and Escape dismisses it. Standard, Fox, Minimal and
    Terminal have it. Not yet in Curve, whose own session brings it, nor in Mobile,
    which gets it as a sheet from the Android session, nor in Mobile Native, from its session.
31. **A site's settings are one press away on a web page:** a control named Site settings
    opens settings on the active tab's site (`openSiteSettings`): at once, or from Details in
    the site panel where the shell has one (rule 33). Not yet in the shells rule 30 lists.
32. **A page with nothing to show says why:** while the active tab has a `notice`, the
    shell covers its page with an `alert` holding Go's title, its detail, and a button for
    each of its actions, by Go's titles, which sends `noticeAction`. Standard, Fox, Minimal
    and Terminal have it. Not yet in Curve, whose own session brings it; Mobile and
    Mobile Native draw a notice of their own until their sessions move them to Go's.

33. **The site panel** ([site-features.md](site-features.md)): the Site settings control
    opens a `dialog` named Site settings for the site's host over the page, drawn from
    `state.openSites` (`@/sdk/sites.js`): a switch named Ad blocker, a switch named Sparks
    with the names of the sparks that run on the page, a choice of Links to other sites
    (Allow, Block), the site's permissions, and Details.
    Each change is `setSiteSetting`; Escape (a phone's back key) closes it. A tab's
    `popupsBlocked`, when above 0, shows in the address field as a button named by its count
    ("1 pop-up blocked"), which opens the panel. Standard
    and Fox draw the shared `@/ui/SitePanel.vue`; Mobile, a sheet. Minimal goes straight to
    settings; Terminal and Curve, from their sessions.

**The names a shell gives its basic controls.** Each is a button, or a menu item in a menu
one of the shell's controls opens; a menu item's name may end with its key.

- **Back**, **Forward**, and **Reload**, which may be **Stop** while a page loads.
- **New Tab**: a way to a new tab. A press makes one, or puts the keyboard in a field of
  the shell's where an address and Enter make one, as the standard shell's box does.
- A way to close a tab: **Close Tab**, for the active one, or **Close** and the tab's
  label as `tabLabel` gives it, such as Close Mock example.com.
- **Address**: a text box or a combo box, where an address and Enter take the active tab
  there, while a web tab is active: for one of ours, rule 19.
- **Settings**.
- **Extensions**, while their flag is on, whose menu has **Manage extensions**.

**The extensions.** Every shell shows the extensions' buttons and the install prompt, from
`state.extensions`: the Vue shells with `ExtensionsToolbar`, `mnshell` with its
own. What a shell draws carries the hooks the real app's extensions tests
(`tools/drive_extensions.py`) find it by:

- the Extensions button, `aria-label="Extensions"`;
- its menu, `.pbx-menu`, which Escape closes, with Manage extensions as `.pbx-manage` and
  each extension's pin as a button labelled `Pin <name>` or `Unpin <name>`;
- each pinned button, `.pbx-action`, labelled with the extension's title or name, with
  `data-extension="<id>"`, `.pbx-open` while its popup is open, its icon an `img` and its
  badge `.pbx-badge`;
- the prompt, `.pbx-prompt`, its yes button `.primary` and its no button another.

A shell that draws them itself draws them only when `extensionsBuilt` from `@extensions`
says the build has them, so a build without extensions has none of it.

### Back on our pages

Our pages, Settings, Bookmarks, History and Downloads, keep their tab's history for what
they open inside themselves, so the phone's back key and Back go to where the person was,
not out of the page. Each place a page goes to inside itself, as a list of Settings or a
bookmarks folder, is an entry in its tab's history, by `history.pushState`; and each layer
open over it, a dialog or a row's menu, is one while it is open. Back closes the innermost
layer, as Escape does, then goes back through the places. It is one module,
`pageHistory` and `pushEscape` in `shell/src/ui/escape.js`: a page calls `pageHistory` with
where it is and how to show a place, and calls what it returns with each place it goes to;
every layer that already pushes its Escape gets its entry with nothing more. A layer that
closes itself, by a button or Escape, takes its entry with it. An arrow up to the place
that holds this one, as a list's "Back to Privacy" or a bookmarks path, calls `upTo`, which
works as Android's Up: back, when the entry before is that place, or in this entry's stead,
so it never adds one, and back never returns to where it left. A shell never calls
`pageHistory`: the phone's back key reaches a shell only while it said something of its
own is open (`backCloses`), and goes back in the page otherwise, where these entries are.
A page of a shell's own, made the same way, gets the same back by calling it.

### The splash

Until Go's snapshot is in, a shell knows nothing of the window: not its tabs, its layout,
its theme or its frame colour; the SDK's `state.loaded` says so. Each shell draws a splash
then: the app's name, centred, in white at 85% on the window's own colour, which the
engine paints under the transparent shell; the minimal shells set it in their Space
Grotesk and Terminal in its monospace. When the data comes, the UI is made at once and
takes input at once, and the two cross-fade in 100 ms, `var(--motion) var(--ease)`
([design-system.md](design-system.md#motion)): the splash fades out as the UI fades in,
only their opacity moving. It is plain CSS, with no timer. The splash stays in the
document, `aria-busy` while `loaded` is false, and goes with `display: none` once it
isn't, a discrete transition so it fades first; the UI's root starts from `opacity: 0` in
`@starting-style`. With reduced motion `--motion` is 0, so it is a cut. As the UI is made
with the data already in, what it did on the first change from the empty state it does on
mount too: the address field takes the keyboard for an empty tab, a strip scrolls the
active tab into view.

## Building one

1. Make `shell/src/apps/shell/<id>/` with `index.html` and `main.js`, and add the line to
   `core/shells/registry.json`.
2. In `main.js`: import `@/design.css` and mount the app: a Vue app with the three
   directives from `@/sdk/vue.js`, a Svelte one with `mount`. The app calls `connect()` as
   it is made.
3. Draw a splash until `state.loaded` ([the splash](#the-splash)), and then the UI:
   `state.tabs` and the basic controls with their names; mark what is the shell's
   with `shell`, the top bar with `drag` (its gaps move the window, its controls stay the
   shell's), and each place a page shows with `page`. Ask for a band with `setFrame` if
   the shell wants one.
4. Run it on the mock (below) and make the check pass.

The shell to read first is Minimal, `shell/src/apps/shell/mnshell/`, the smallest, in
Svelte; Standard, `shell/src/apps/shell/pbshell/`, in Vue, is the reference that shows
every feature Go offers. `go run -tags sqlite_fts5 ./cmd/dev -use minimal` loads one. The SDK needs no
framework: a shell may draw with the DOM itself and `onChange` ([The adapters](#the-adapters)).
The owner, 2026-11-14: no hidden example shells, so none is kept for that path. In Vue,
a shell can be as small as this `App.vue`, beside an `index.html` and a `main.js` of a few
lines:

```vue
<script setup>
import { ref } from 'vue';
import { connect, controlSpace, send, setFrame, state, tabLabel } from '@/sdk/vue.js';
import { ExtensionsToolbar } from '@extensions';

setFrame(4);
const address = ref(null);
connect({ focusAddressBar: () => address.value.focus() });
</script>

<template>
  <div class="splash" :aria-busy="!state.loaded">Project B</div>
  <div v-if="state.loaded" class="window">
    <header v-shell v-drag :style="{ paddingLeft: controlSpace.left }">
      <button
        v-for="tab in state.tabs" :key="tab.id"
        role="tab" :data-tab="tab.id" :aria-selected="tab.id === state.active"
        @click="send({ t: 'activateTab', id: tab.id })"
      >{{ tabLabel(tab) }}</button>
      <button aria-label="New Tab" @click="send({ t: 'newTab' })">+</button>
      <button @click="send({ t: 'closeTab', id: state.active })">Close Tab</button>
      <button @click="send({ t: 'back' })">Back</button>
      <button @click="send({ t: 'forward' })">Forward</button>
      <button @click="send({ t: 'reload' })">Reload</button>
      <input ref="address" aria-label="Address"
        @keydown.enter="send({ t: 'navigate', text: $event.target.value })">
      <component :is="ExtensionsToolbar" v-if="ExtensionsToolbar" :extensions="state.extensions" />
      <button @click="send({ t: 'run', text: 'app.settings' })">Settings</button>
    </header>
    <main v-page="state.active"></main>
  </div>
</template>

<style>
.splash { position: fixed; inset: 0; display: grid; place-items: center; font: 500 15px/1 system-ui, sans-serif; letter-spacing: 0.02em; color: rgb(255 255 255 / 0.85); user-select: none; pointer-events: none; transition: opacity var(--motion) var(--ease), display var(--motion) allow-discrete; }
.splash[aria-busy='false'] { display: none; opacity: 0; }
.window { display: flex; flex-direction: column; height: 100%; transition: opacity var(--motion) var(--ease); }
@starting-style { .window { opacity: 0; } }
header { display: flex; align-items: center; gap: 4px; height: 44px; padding-right: 8px; background: var(--bar); color: var(--text); }
input { flex: 1; }
main { flex: 1; margin: 0 var(--frame-width) var(--frame-width); }
</style>
```

## Testing one

**While writing it.** `go run -tags sqlite_fts5 ./cmd/dev -use <id>` in `core/` and `npm run dev` in
`shell/`, then open `/mock/window` on vite's address. The page is the OS window: the shell
in a frame, and under it a mock page for each tab the layout shows, where the layout puts
it. A mock page has buttons that do what a real page can: navigate, change its title, open
a tab, crash. Edits to the shell reload in place.

**Seeing its regions.** Turn on Show window regions in settings, Flags, and start
`go run -tags sqlite_fts5 ./cmd/dev` again: the mock window then outlines every rectangle the window manager
tracks, as the real window does ([windows.md](windows.md#seeing-the-regions)): the shell's
areas red, the controls in a drag bar among them, the drag areas blue and dashed, each
shown tab's page green with its id, the window buttons purple. A gap that should drag and
doesn't, or a control that isn't outlined red, shows at a glance. As in the real app the
flag is read at launch, from the profile's `flags.json`.

**The check.** `node shell/e2e/shell-contract.cjs <id>` runs "What every shell does"
against one shell, in a real browser on the mock engine, with the Extensions flag on; with
no id it runs every shell in the registry. It ends with one line per rule, `ok` or `FAIL`.

**The shell's own tests.** `shell/e2e/shells/<id>.cjs`, for what only this shell does: its
panes, its menus, its keys. `shell/e2e/lib.cjs` starts the mock, built with extensions, on
a port of its own and a profile of its own, so tests run side by side, and has the helpers the other tests use:
`check`, `until`, `settle`, `regions`, `fire`, and the window with its frames
(`openWindow`). A test starts the mock with `-use=<id>`, and ends with `done()`.

`tools/test.sh mock` runs the check and every shell's own tests with the rest.

**The mock's limit: three mock pages on screen.** Each mock page shown keeps an event stream
open, and Chrome holds six connections per host, so with four or more mock pages showing, plus
the shell's own stream, the shell's next `POST /shell/send` waits for good and everything it
sends after is lost, with Go idle. A test of panes (Terminal's) keeps to three on screen.
Serving the mock pages from a second host would lift it ([todo.md](todo.md)).

**What the mock tells a test.** `/mock/regions?window=<n>` answers with the last layout:
`regions` (the shell's, a drag bar's controls among them) and `drag`, `visible` (each shown tab and its rectangle), `shell` (the id of the
shell the window loads), `focused` (the tab last given the keyboard), `unshownFocus` (the
tabs a shell asked to give the keyboard while no window showed them), Go's `tabs`,
`active` and `settings` (the window's settings tab, 0 for none), and what it had before;
with `x` and `y`, `hit` says what a press there does. The settings tab's page shows in a
frame of its own, `iframe.our-page[data-page=settings]`, the settings page at
`/settings/?tab=<id>`.
**Pitfalls the Standard tests met.**
- The shell page enforces Trusted Types: no `v-html` or `innerHTML`, so icons are SVG in the
  template.
- A mock page's Loading lasts a moment, so record its states from before the press (a
  MutationObserver on `#state`, as the contract's Reload rule does) rather than look after it.
- A navigation before the previous page has loaded replaces it, with no history entry: wait
  for each page's title before the next, or Back has nowhere to go.
- The layout reaches the mock a frame after the DOM changes: read `regions()` inside `until`.
- A control hidden with `visibility: hidden` (a row's ×) can't take even a forced click, and
  a row that moves under a pointer that stays still gets no `:hover`; focus the row instead.

`/mock/fire` does what a page or the user can: `kind=menu` with an action's id runs it,
`kind=focusTab` is a press in a tab's page, and each button of the mock page is a kind,
acting on the tab `tab` names. `go run -tags sqlite_fts5 ./cmd/dev -addr=127.0.0.1:0` takes a free port and
prints the address it got.

## What the real app adds

The mock can't show these, so the real app's own tests cover them once for every shell in
the registry (`tools/drive_first_app.py`): the app starts on the shell and marks
`app.ready`; switching to it live leaves the tabs loaded; the page shows through the
shell's unpainted part; a press in each shown page gives that page the keyboard.

How the real window decides a press: the shell's regions first, in their order (a drag area
moves the window, a shell area goes to the shell), then the shown page under the point
(`window_mac.mm`, the container's `hitTest:`). So anything a shell marks as its own over a
page, a suggestion list or Standard's command box over its translucent scrim, takes the
press there. The shell draws above the pages, so its translucent pixels tint the page; it
can't blur it, since the page is a view of its own. A layout reaches the window a frame or
so after the shell's DOM changes, so a real-app check of a region waits for it (up to 2 s).

## Bars that hide on scroll

A phone shell gives the page the whole screen while it is read, as every phone browser
does: its bars go while the page scrolls down and come back when it scrolls up. This is the
model to build that on, for any shell. The Mobile web shell follows it
(`apps/shell/mwshell/App.svelte`, `bars()`), and `e2e/shells/mwshell.cjs` checks each rule
below.

**What Go gives.** A `scroll` message, `{tab, direction}`, when a tab's page starts
scrolling the other way (content's `DidChangeVerticalScrollDirection`). It comes once per
change, not for every scroll, and doesn't say where the page is. The shell's layout (its
`page` region) says where the page draws, and any change to it resizes the page.

**One wish and two facts.** Keep `shown`, what was last asked for, and two facts that
follow it:
- `over`: the page has the bars' room, running on under them.
- `away`: the bars are off the screen.

`away` is only ever true while `over` is.

**The rule: the page's room changes only while the bars cover it.** So nothing but the page
ever shows where the bars were: no strip of the frame colour, and no snap as the page fills
it. The page is resized once each way.
- **Going:** set `over`, so the page grows under the bars. Wait a few frames for it to draw
  there (Mobile waits 3), then set `away`, so the bars slide off and uncover it.
- **Coming back:** clear `away`, so the bars slide in over the page from wherever they are.
  When their slides finish, clear `over`, so the page gives the room back under them.
  Mobile waits for the `finished` of each bar's `getAnimations()`. With no slide running,
  that's at once: under reduced motion, while the bars aren't drawn (a page in full
  screen), or when they never left. Don't wait on `transitionend`: no transition starts
  when `away` comes and goes within one frame, and the room would never come back.

**A change of mind mid-slide.** Set the wish and flip `away`. A CSS transition turns round
from where it is, so there's nothing to cancel. Number each ask, so that a slide that
finishes after a newer ask leaves the room alone.
- Called back while sliding off, the bars slide back in, and the room goes back when that
  slide ends.
- Sent away while sliding in, they slide off again, and the page keeps the room all along.
- Called back in the few frames before the slide starts, the room goes back at once.

**The resize's echo.** Resizing the page under a finger is a scroll of its own to content:
the page moves under the finger, or its end comes nearer, and content reports a change of
direction. Taken for the person's, that turned the bars back and forth near the top of a
page. So ignore any direction for 300 ms after `over` changes. The same pause keeps a
finger's small wobble from turning the bars at once. Mobile puts `data-settling` on its
window meanwhile, for tests to wait on. A slide moves the bars, not the page, so a direction
during a slide-in is the person's, and is taken.

**What else brings them back:** another tab, a page that starts loading in the active tab,
the address called for, find, and any sheet. Don't hide them while a sheet is open, the
address is being edited, or find is open.

**The top of a page.** The bars should always show while a page is at its top, as Chrome's
do. A shell can't do that yet, as `scroll` doesn't say where the page is. Until it does:
- A new page starts with the bars shown, since its load brought them back.
- Scrolling back up towards the top brings them back.

See [the proposal below](#proposed-go-and-the-engine-take-part).

**The motion.** Use the design system's `--motion` (100 ms) with `--ease`, and move the bars
with `transform` only. A height or margin that animates would resize the page every frame.
Under reduced motion `--motion` is 0 and the bars don't slide.

**Laying the bars over the page,** as Mobile does it in CSS:
- Each bar stays in the flow, and its height is measured.
- While `over`, a negative margin of the bar's own height lets the page's part run on under
  it. The strip at the foot also keeps the toolbar's height under it, so it stays above the
  toolbar.
- While `over`, the bars sit above the page's notices and below the switcher and the sheets.
- What floats at the page's foot, such as the download bar, keeps above the bars while
  they lie over the page in place. Mobile's `--covered` gives the height to keep clear.
- `away` moves each bar by its own height, plus the heights of the bars between it and the
  edge.
- A bar that joins them goes inside one of the parts, so the heights and the margins count
  it with no more of either: Mobile's bookmarks bar is in the head with the address on
  top, under the toolbar, and with the address at the foot in the strip's part, over the
  group strip, which that part then holds while either shows.

**With the bar on top,** the page's top edge moves when the room changes, so its content
moves by the bar's height once each way: as the bars start to go, and when they're back in.
A shell can't avoid that; only the engine can (proposal 3 below).

### Proposed: Go and the engine take part

1. **Go drops the resize's echo** (S). Go knows when a tab's page region changes size, from
   the layout the shell sends. It could drop that tab's `scroll` directions for 300 ms
   after, so no shell has to carry the rule. It can be tested on the mock.
2. **`scroll` says when the page is at its top** (S, engine). Add `top: true` to the
   message, from the frame's root scroll offset (`cc::RenderFrameMetadata`,
   `root_scroll_offset`), so a shell can keep the bars shown there.
3. **Chrome's way: the engine's browser controls** (L, engine). The content layer can move
   the page with the bars under the finger, so the bars follow the finger instead of
   sliding on a change of direction, and the page is never resized for them.
   - The engine gives it the bars' heights (`WebContentsDelegate::GetTopControlsHeight`,
     `GetBottomControlsHeight`, `DoBrowserControlsShrinkRendererSize`).
   - It hands the shell each frame's `top_controls_shown_ratio`, which the shell moves its
     bars to.
   - On Android the shell and the page share one compositor, so the two can move in step.
   - It's the only fix for the bar on top's moving content.

## The terminal shell

`tmshell`: a minimal shell laid out as a terminal emulator with split panes. Monospace,
dense and square.

- **Go's tabs are panes, in windows,** as tmux calls them. One window shows at a time and
  fills the body. **A window is one of Go's tab groups, or a tab in none** (owner,
  2026-11-14: "panes are a shell feature... termy should use tab groups for split paned
  tabs"): a split's tabs are a group, so every other shell sees them together. How a
  window's panes lie is the shell's own, a tree: a split is a row or a column of parts with
  their shares, a leaf is one tab. The strip on top has one chip per window: its number,
  its name (its group's name, else the host of the pane it shows, else that pane's label),
  and a square per pane, each the tab's `role="tab"` element, in the order the panes lie
  on screen, the pane the window shows filled. Pressing the name shows the window at the
  pane it showed last; pressing a square shows that pane. The
  chips follow Go's order, and the shell keeps Go's order the strip's: a tab Go puts
  elsewhere is moved (`moveTab`) to its place, so Cmd+1 to 9 and the next and previous tab
  keys go where the strip says. New Tab follows the chips, and at the strip's far end are
  the extensions and a gear that opens settings. Each pane has a title bar with split
  right, split down and close; a press on it makes the pane active. The prompt line at the
  foot is the address bar, with back, forward and reload, and the update when one is ready.
- **Its keys are chords, with no prefix key** (owner, 2026-11-14), each one of its own
  actions: Option+Cmd+D splits the active pane right and Shift+Cmd+D down
  (`tmshell.splitRight`, `tmshell.splitDown`), Cmd+D being Bookmark in every shell; Ctrl+Cmd and an arrow makes the pane that way active, the nearest
  sharing most of its height or width (`tmshell.focusLeft` and so on); Ctrl+Shift+Cmd and
  an arrow moves a divider of the active pane 32px that way, the one on that side of it,
  else the one on its other side (`tmshell.resizeLeft` and so on); Shift+Cmd+Enter zooms
  the active pane to fill the body and back (`tmshell.zoom`). A zoom ends when another
  pane becomes active, on a split, or when the pane closes, and is kept in the shell's
  data. A pane's title bar has Zoom beside the splits while its group has more than one
  pane; the buttons' titles show the keys.
- **The active tab is the focused pane,** and the window shown is the one that holds it.
  When the active pane closes, by its button or by Go's Close Tab (Cmd+W), the pane that
  takes its room becomes active, whichever tab Go chose from its own list, so its window
  stays shown; the body shows that pane's window at once, never Go's choice on the way.
- **A split** is `newTab` with `groupWith` the pane's tab: Go puts the new tab in that
  tab's group, starting one of the two when it is in none, and the shell puts its pane
  beside the one split. **A tab Go puts in a group**, such as one a page opens from a pane,
  is a pane of that window, beside the tab before it in Go's list: to its right when that
  pane is wider than tall, else under it. **A tab in no group** is a window of its own. A
  group ended anywhere breaks its window into a window per tab, as a tab taken out of one
  leaves it; a group made of lone tabs makes them one window. A window holds one pane
  when Go ends a group at one tab. **No pane is smaller than 240 by 160:** a split that
  would leave less is off (its button disabled, its key does nothing), and dividers drag
  and keys move them only that far.
- **Its own data** is `{windows, zoomed}`, each window's tree and the zoomed pane, naming
  tabs by id, and nothing Go has. It is fitted to Go's tabs and groups on the snapshot and
  on every `tabs`: a tab that is gone, or no longer in its window's group, leaves its tree,
  its share going to the part before it; a split left with one part becomes that part; a
  tree holding the tabs of two windows keeps the one most of them are in.
- **It leaves out** a prefix key, a key to close a pane other than Cmd+W, dimming the
  pages of panes that aren't active, dragging panes between windows, broadcasting input,
  and the notice of the last run's crash.

## The fox shell

`fxshell` puts the tabs in a strip on top, a navigation bar under it with a wide centred
address field, and a menu button at the far right, in our tones.

- **The tab strip** is the title bar. Tabs share it between a minimum and a maximum width;
  past the minimum it scrolls sideways, with arrows at its ends, New Tab fixed at its end,
  and the active tab always scrolled into view. "List all tabs" at its right opens a menu
  of every tab. While the profile has two or more workspaces, a tab has a line in its
  workspace's colour along its top.
- **The navigation bar** has back, forward, and reload, which is Stop (`run nav.stop`)
  while the page loads; the address field, centred, with the suggestions attached under it
  and the active tab's workspace at its right end; the extensions; and the menu button,
  whose commands each run an action Go has (`run`): New Tab, New Window, Zoom In, Zoom Out,
  Actual Size, Full Screen, Settings.
- The crash notice and the update notice are bars under the navigation bar.
- **Its own data:** none. The strip's scroll position follows the active tab, so it never
  calls `saveData`.
- **Left out:** dragging tabs to reorder or tear off, pinned tabs, a tab's context menu,
  a bookmarks bar, a sidebar, customising the toolbar, and print and find in the menu,
  which have no action yet.

## The mobile shell

`mwshell`: the phone's shell, in Svelte, everything at the thumb. Its guideline is
`project-b-research/mobile-shell/spec.md`.

- **The registry marks it `"phone": true`.** A phone offers and loads only phone shells,
  and the APK carries only those (owner, 2026-11-14: more phone shells can join it later),
  so it starts on Mobile even where a profile's pref names `pbshell`, as the templates do;
  a desktop offers none, though it loads one named outright, as the mock does for the
  shell's tests in a phone-sized window. With more than one phone shell, Settings, Look on
  a phone has Shell to choose between them (a phone has no View menu).
- **One bar at the foot, 48px:** the address pill, which shows the host and the lock and
  becomes the field when tapped, with the suggestions over it, the closest nearest the
  thumb; the tab count, which opens the switcher; and ⋯, the menu sheet. A swipe along
  the bar goes to the tab before or after. Back is the phone's back key, and in the menu
  while there is somewhere to go back to. The back key, in order (owner, 2026-11-14): it
  closes an open sheet or ends the address's editing (`backCloses`); else goes back in the
  page; else, on a tab with no history, closes the tab, and the one used before it shows
  (`closeTab` with `then: "recent"`); down to the last tab, whose closing closes the
  window, and on a phone the app (owner, 2026-11-18). Closing the last tab another way
  does too. For one of our tabs, settings, the
  pill is the tab's title with a close, and nothing to type (rule 19).
- **What the field suggests** follows
  [daily-driver.md](daily-driver.md#what-the-address-field-suggests-and-when). Tapped over
  a new tab it draws nothing, the new tab page being the list. Over a page, with nothing
  typed, it covers the page with the page's own row (Copy, Share, Edit) and, while
  `bookmarksCommandBox` is on, every bookmark as one row of tiles
  ([bookmark-folders.md](bookmark-folders.md#on-the-mobile-web-shell)). Typed, it asks Go
  for its list with `rows`, the rows that fit, and draws it without headings, the first
  row nearest the field, an open tab's row with Switch to tab, the others with ↖; the first
  row's completion shows selected in the field. Every suggest the shell sends takes its id
  from one count (`queries.js`), so the bar's and the new tab page's answers never mix.
  Until Go's answer for the latest text comes, the list on screen is an earlier text's; it
  carries the text it answers as `data-answers`, which the shell's tests wait on before
  they press a row, and Enter takes the first row only while it answers what is typed.
- **The bookmarks bar,** while `bookmarksBar` is on, is one of the bars: over the group
  strip and the toolbar, or under the toolbar on top
  ([bookmark-folders.md](bookmark-folders.md#on-the-mobile-web-shell)).
- **A download you can see.** A download that starts while the app is showing raises a
  small bar over the foot of the page, just above the toolbar: "Downloading big.zip". When
  it ends while the bar or the app is showing, the bar says "big.zip downloaded" with Open,
  which does what the Downloads page's Open does (`downloadAction` `open`), or "Couldn't
  download big.zip" with Retry (`retry`). Each goes after 2 seconds (design-system.md, "On
  a phone"); a press keeps it while held. A tap anywhere else in the shell, or a swipe down
  or sideways on it, puts it away, and so does a sheet opening or the Downloads page
  showing, which says it all already; it says nothing while either is there, so it never
  comes back over what the person went on to. The find bar, in the toolbar's place, is
  never under it.
  What it says is Go's download news (`downloadNews`, core/app/downloads.go), each change
  of a download's state told once, the same words and times for the web and the native
  phone shell, which only draw it and keep the rules above for when. The first download's
  start, while Android's notifications aren't allowed, offers them instead: "Downloading
  big.zip" with "Show progress in notifications?" on a line of its own, so a long name
  never hides it, and Allow only, for 8 seconds or until
  answered; the shell tells Go once it showed it (`notificationsOffered`), and it never
  comes again. Android's notifications are the engine's and Go's
  ([daily-driver.md](daily-driver.md)).
- **Find in page**, from the ⋯ menu, puts a find bar in the toolbar's place with the
  keyboard in it: the field with the count at its end, the match before and after, and
  ×, as the back key, closes it. The bars stay while it is open (rule 20).
- **Downloads** is always a tile of the ⋯ menu, with how many are running under it;
  it opens the Downloads page, a tab of ours ([daily-driver.md](daily-driver.md#downloads)),
  its title and a close in the pill (rule 19).
- **A page in full screen**, as a video's, has the whole screen: the bars, the strip and
  the notices go, and any sheet closes; Android hides its status and navigation bars, and
  the back key ends it (rule 21).
- **A long press on a link** in the page opens the shell's sheet for it, since a phone
  has no menu of its own: the dense sheet, with New tab, In group, Copy link, Download and
  Share as tiles, then Open in workspace and Copy link text as rows; on an image, New tab,
  Copy image, Download and Share, then Copy image address; a linked image has both
  ([daily-driver.md](daily-driver.md)). Tabs opened from it open behind, so the group's
  strip gathers them while the user reads on. A long press on text keeps Android's own
  selection.
- **A page's dialog** (alert, confirm, prompt, leave-page question or a site's sign-in,
  rules 15 and 22) is a card in the middle of the screen over the dimmed window, sheets and
  bars too, as Android's dialogs are, with Go's words and buttons 48px tall. Only the shown
  tab's shows; another tab's waits until its tab is shown. A prompt or a sign-in starts
  with the keyboard in its first field. The back key answers it no, before it closes a
  sheet (`backCloses` is on while one shows).
- **A page that failed, crashed or was blocked** gets a notice over it rather than a blank:
  one sentence (Can't find example.com, This page crashed, example.com is blocked), the
  error's own name under it, and Try again or Reload.
- **Share report**, on the last run's crash notice and on a crashed page's notice, hands
  one text report to the phone's share sheet (`shareReport`); **Save** beside it puts that
  report in the phone's Downloads (`saveReport`), as the share sheet may offer nowhere to save it.
- **The pill's end** says Blocked, Crashed or Failed when the page is, as the standard
  shell's address bar does.
- **Not secure** in words at the pill's start, with the warning mark, in `--danger`, when
  the active tab's `connection` isn't secure: an http page, or a certificate passed with
  Open anyway, as Chrome does. It is the site's mark, named "Not secure · Site settings",
  and opens the site's sheet, which says why. A secure page keeps the lock.
- **The switcher** lies over the page, the bar still under it: a card per tab in two
  columns, the workspaces as chips that choose which cards show and where + opens a tab.
  Its cards are the tabs' `role="tab"` elements, kept in the document while it is closed.
  Closing the last tab leaves a new one: the window never has none.
  A card shows its page as it last looked, the tab's `thumb`, in an upright 3:4 place the
  same for every card, the picture filling it without stretching and cropped from the top,
  as Chrome's are; without one, the favicon or the first letter. Opening, the switcher
  sends `captureTab` before it covers the page, so the active tab's picture is fresh.
  A search over the shown workspace's tabs, by title and address in any case, is at the
  top: there at once with four tabs or more, behind a search button with three or fewer.
  Its × clears it; Enter shows the first tab found; closing the switcher forgets it.
  While `bookmarksSidebar` is on, Tabs and Bookmarks above it all choose between the cards
  and the bookmarks ([bookmark-folders.md](bookmark-folders.md#on-the-mobile-web-shell));
  it opens on Tabs each time.
- **Tab groups** are Go's ([groups.md](groups.md)). While the active tab is in one, a strip
  over the bar shows the group: its dot, which opens the group's sheet (name, colour, its
  tabs, New tab, Ungroup, Close group with a confirm), its tabs' favicons, and + for a new
  tab in it. A tap on a favicon goes to its tab; a tap on the active one, which carries an
  ×, closes it; a long press on any closes it.
- **Every target is 48px or more each way,** Android's least, though what it draws may be
  smaller inside it ([design-system.md](design-system.md)).
  In the switcher a group is one card, its colour along the top and its tabs' favicons in
  it, which are their `role="tab"` elements. A long press on a card opens its menu: Add to
  group (the groups of its workspace, or a new one with a new tab), Remove from group,
  Close. Held and dragged onto another card, a card joins that card's group or starts one
  with it; swiped sideways, it closes. A swipe along the bar stays in the group.
- **The menu sheet** (⋯) is shaped for a phone, as Firefox's is: first a row of the page's
  actions, Back while there is somewhere to go back to, Forward, Bookmark (a star, filled
  while the page is bookmarked, through the SDK's `bookmark`), Reload or Stop, and New tab.
  Then a flat list: what Go's menu lifts to its top level (`"shell": "top"`, Print… the
  first: the tree's top-level actions the sheet has no place of its own for), New tab in
  group while the tab is in one, Find in page, and Workspaces, where new tabs open, while
  there are two or more. Then the places as tiles in one row, each running Go's action by
  its id: History (`history.show`), Bookmarks (`bookmarks.show`), Downloads
  (`downloads.show`, with how many are running) and Extensions while their flag is on,
  which opens its sheet. Then the update while one is ready, Settings (`app.settings`),
  and last a quiet More: the menu bar's whole tree as Go gives it, its submenus opening in
  the sheet with a row back, so nothing in it is out of reach (the owner, 2026-11-16: the
  shells draw Go's menu). Its accessible names are those words: Back, Forward, Bookmark,
  Reload, Stop, New Tab, Find in page, Workspaces, History, Bookmarks, Downloads,
  Extensions, Settings and More; `tools/drive_android.py` and `tools/measure.py` find them
  so.
- **The address bar on top** is a setting, `addressBarPosition`, in Settings, Look on a
  phone: the bar moves under the status bar, the suggestions grow down from it, and the
  group strip stays at the foot, as Chrome's does. Sheets stay at the foot.
- **The bars hide on scroll,** and come back when the page scrolls up, for another tab,
  a page that starts loading, the address, find or a sheet. The page's room changes only
  while the bars cover it, so they slide out as smoothly as they slide in. It's the
  reference for [Bars that hide on scroll](#bars-that-hide-on-scroll), where the whole
  model is.
- **The band:** none. A desktop window's buttons, as on the mock, get a strip of their
  own above the page, which a phone has no need of.

## Not in this step

Keys and menu items for a shell's own commands, such as a split, which came later
([A shell's own actions](#a-shells-own-actions)). Saving a shell's data across a restart. A
shell from outside the app. A shell per window. Dropping the tab position pref, which only
the standard shell reads.

## Order of building

Each commit builds, leaves a working app, and carries its tests.

1. The standard shell moves to `apps/shell/pbshell`, what every shell needs moves into
   the kit, and the engine and the mock load a shell by its id. No change the user sees.
2. The mock shows a page for every tab a layout lists.
3. The `shell` pref, the registry, View, Shell and the settings choice, the live switch.
4. A shell's own data.
5. The check, and the tests' shared helpers.
6. The terminal shell. 7. The fox shell.

## Proposed: the shell API without Vue

The owner asked that the shell's API not depend on Vue, and that a shell in Svelte show
where a shell's functional part ends and its presentation begins. The branch `shell-api`
builds it: the SDK above with no framework in it, a Vue and a Svelte adapter over it, and
`mnshell`, a bare shell in Svelte that keeps every rule. This is the line it draws,
and what is left to decide.

### The functional part: the SDK

What every shell needs, whatever it looks like:

- Talking to Go: the channel, `ready` and `shown`, and sending what the user does.
- Go's state as it arrives, and hearing what changed.
- The shell's own data.
- Giving a page the keyboard only once the layout shows it.
- Measuring the layout: the shell's areas, the drag bars and their controls, where the
  pages show, and the band.
- What a control is.
- Key labels, and the theme's colours on the root.
- The extensions' state, what goes back about them, and closing an open popup on a press
  elsewhere in the shell.
- The timeline's entries.

### The presentation: each shell's own

- What it draws and where: the tabs, the bars, menus, notices, and the extensions'
  buttons and prompt.
- Which elements it marks `shell`, `drag` and `page`.
- The band around the window, from none to any width. The window draws it, over the
  pages, so the shell keeps its pages inside it.
- Where the keyboard goes when a tab becomes active: the address, or the page.
- The shape of its own data, such as the terminal's panes.
- Its look: the design system's classes, or none.

### The SDK's names

- `state`: Go's state, a plain object; a field is replaced whole when it changes, except
  that a `tabs` message's tabs keep the favicon each had.
- `onChange(fn)`: calls `fn(fields)` after each change, with what changed.
- `connect(handlers, afterDraw)`: says ready, keeps the state, calls the shell's handler
  for each message, and says shown once the snapshot is drawn.
- `send(msg)`: one message to Go.
- `saveData(value)`: the shell's own data for this window.
- `focusPage()`: gives the active tab's page the keyboard, once the view has drawn and the
  layout shows it.
- `setFrame(width)`: the band the window draws, 0 for none.
- `shell`, `drag` and `page`: the markers, each with `mark(el)` and `unmark(el)`, and
  `page` with `update(el, tab)`.
- `activeTab(state)`, `workspaceOf(state, tab)`, `tabLabel(tab)`, `keyLabel(state,
  action)`, `controlSpace(state)`: helpers that read the state they are given.
  `workspaceOf` is the workspace a shell shows a tab in, by a dot or its name, and is
  undefined for one of our tabs, and while the profile has one workspace only, so every
  shell leaves the dot out
  then.
- `pressExtension(id, el)`, `pinExtension(id, on)`, `manageExtensions()`,
  `answerExtensionPrompt(on)`: what goes back about extensions.
- `controls`: the selector for what a control is.

"Once the view has drawn" is the one place a framework comes in. `connect` takes
`afterDraw(fn)`: Vue's `nextTick`, Svelte's `tick`. Without one, the SDK takes it that the
view draws as it hears of a change, as a shell with no framework does in its `onChange`.
The helpers take the state they read so that a framework's copy can be given to them, and
the framework then knows what was read.

### The adapters

- **Vue**, `@/sdk/vue.js`, gives the names the Vue shells already used: `state` as a
  reactive copy that follows the SDK's, `activeTab` and `controlSpace` as computeds,
  `workspaceOf(tab)`, `connect` with `nextTick`, and the three directives. The four Vue
  shells changed only where the SDK took a job from them: the import path; no
  `useTheme`, as the SDK paints the colours; `setFrame(4)` in place of reading the band
  from Go; and the extensions' toolbar given `state.extensions`. The example gained the
  basic controls it lacked.
- **Svelte 5**, `@/sdk/svelte.svelte.js`: `state` as a `$state` copy, `connect` with
  `tick`, the actions `use:shell`, `use:drag` and `use:page={id}`, and the SDK's helpers,
  given that `state`.
- **No framework**: the shell imports `@/sdk/shell.js` and `@/sdk/regions.js`, marks
  elements as it makes them, and draws again what `onChange` names.

### What a shell's author writes

In Vue, as above. In Svelte, as `mnshell` does:

```svelte
<script>
  import { activeTab, connect, drag, page, send, shell, state, tabLabel } from '@/sdk/svelte.svelte.js';

  let address;
  connect({ focusAddressBar: () => address.focus() });
</script>

<header use:shell use:drag>
  {#each state.tabs as tab (tab.id)}
    <button role="tab" data-tab={tab.id} aria-selected={tab.id === state.active}
      onclick={() => send({ t: 'activateTab', id: tab.id })}>{tabLabel(tab)}</button>
  {/each}
  <input bind:this={address} aria-label="Address" value={activeTab(state).url}
    onkeydown={(e) => e.key === 'Enter' && send({ t: 'navigate', text: address.value })} />
</header>
<main use:page={state.active}></main>
```

With no framework, nothing is built for it yet. It would read:

```js
import { connect, onChange, send, state, tabLabel } from '@/sdk/shell.js';
import { drag, page, shell } from '@/sdk/regions.js';

const header = document.querySelector('header');
const main = document.querySelector('main');
shell.mark(header);
drag.mark(header);
page.mark(main, 0);

onChange((fields) => {
  if ('tabs' in fields || 'active' in fields) drawTabs();
  page.update(main, state.active);
});
connect({ focusAddressBar: () => address.focus() });
```

where `drawTabs` makes a button for each of `state.tabs`, and keeps the ones it made by
tab id.

### The extensions

Drawing them is the shell's. The SDK keeps `state.extensions`, sends what goes back, and
closes an open popup on a press elsewhere in the shell; each pinned button carries
`data-extension="<id>"` so its own press isn't one. The Vue shells keep `ExtensionsToolbar`,
which now takes `extensions` as a prop and sends through the SDK, and looks and works as
it did. `mnshell` draws plain buttons and a plain prompt with the same hooks.
`extensionsBuilt` from `@extensions` lets a shell's own buttons leave a build without
extensions, and nothing of `@extensions` but that one constant reaches a Svelte shell, so
no Vue does.

What it costs: `mnshell` is 51 KB, 20 KB gzipped, of which its extensions part is
under 5 KB, and 47 KB without extensions. The example, the same shell in Vue, is 81 KB,
32 KB gzipped. The SDK alone is 13 KB, 5 KB gzipped.

### The band

The shell asks for it with `setFrame(width)`, and the SDK sends it in the layout as
`frame`. Go keeps it per window and the engine draws it from that width, as it already
took the width with every layout, so the engine didn't change. The four Vue shells ask for
4px and look as they did; `mnshell` asks for none and its page reaches the window's
edge. Rule 3 checks the pages lie inside the band the shell asked for. The snapshot no
longer has `frameWidth`; the settings page had a copy it never read, which went too.

### The two minimal shells

`mnshell`, and `minimal-js` until it was dropped with the hidden shells, were to be, in the owner's words, a tab bar, a toolbar, a
vertical list too, the simplest button placements, a simple layout, no rounding, with
thought for many tabs. The SDK makes that easy: a shell draws `state.tabs` however it lays
them out, marks its bars, and the SDK measures whatever moves. A strip that scrolls is
marked `shell`, with `drag` spaces beside it, as the fox shell's is; a list that wraps or
scrolls is the same. The spike's `mnshell` is not that design: it only proves the
API, with every control plain.

### What the spike changed

1. The kit became the SDK, with no framework in it, and the Vue shells use an adapter over
   it.
2. The extensions' state and the messages about them moved into the SDK, and the Vue
   toolbar draws from them.
3. Svelte joined the build, with its adapter. `svelte` 5 and `@sveltejs/vite-plugin-svelte`
   6 are new dependencies of `shell/`, so `npm install` there comes before the next build:
   the engine's build and the Android release both build from `shell/node_modules`.
4. `mnshell`, registered with `"example": true`, so it is offered nowhere.
5. The band became the shell's.
6. Rule 12, the basic controls, and the example's.

### Left for the owner

- **How the browser provides the SDK.** Bundled into each shell at build time, as it is
  now, or served once at `chrome://shell/sdk/sdk.js` for any shell to import, so a shell
  with no framework needs no build step. Serving needs no engine change: the engine
  already serves `chrome://shell/<id>/<id>.js`, so `sdk` would be an id no shell may take.
  It takes a library build of the SDK in `build.js`; every built shell, and the adapters,
  importing that file instead of a copy, since two copies would keep two states; and the
  SDK's names becoming a contract a shell from outside relies on. Recommendation: bundle
  it, as now, and serve it once a shell from outside the app is wanted, which is not in
  this step.
- **Svelte's `$state` and the SDK's `state`.** A Svelte component that imports `state`
  and also uses the `$state` rune gets a warning about the name. The spike's components
  don't need both: the extensions part takes what it needs as a prop. Recommendation: keep
  the one name in every adapter, and where a component needs both, import it under
  another name.
- **The extensions' hooks.** They are classes today, `.pbx-*`, beside a few labels. Roles
  and names would say the same more plainly, as the basic controls do: a menu named
  Extensions menu with its items, a pinned button pressed (`aria-pressed`) while its
  popup is open, a dialog named for its question. The real app's extensions tests and
  the check of a build without extensions would then find them as a screen reader does.
  Recommendation: move them to roles and names with the extensions session, after this.
- **`extensionsBuilt`.** One line in each of `src/extensions` and `src/extensions-off`,
  the only way a shell's own extensions buttons leave a build without extensions without
  Vue coming with them. Recommendation: keep it, named as the extensions session likes.
- **A tab that becomes active.** The standard, fox and terminal shells, and
  `mnshell`, each repeat one rule: a tab with nothing in it wants the address, one
  with a page wants its page. It names the address field, which is presentation, so the
  SDK leaves it to the shell. Recommendation: leave it there.
- **The standard shell's tab position.** `state.tabLayout` and `setTabLayout` are only the
  standard shell's. Recommendation: they go with that pref, as "Not in this step" says.
- **`minimal-js`'s drawing** (the shell is gone since). Recommendation: no library. `document.createElement` with a
  small helper that makes an element with its attributes and children, and the tab list
  drawn again by tab id. lit-html, about 3 KB gzipped, is the smallest library that would
  do the drawing again for it.
