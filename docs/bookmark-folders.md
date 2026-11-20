# Bookmark folders

Slice 2 of the [onboarding](onboarding.md#what-it-needs-that-we-dont-have): bookmarks brought
over from another browser keep their folders, so ours need them first. **Built, 2026-11-17.**
The proposal's mocks: [screenshots/bookmarks/mocks.html](screenshots/bookmarks/mocks.html).

The owner's answer on the star, 2026-11-17: a small popup with the last used folder already
chosen. Cmd+D (Ctrl+D) saves the bookmark at once, into the last used folder, or the top
when none was used, then shows the popup to edit it; Enter saves the edits, and Esc closes
it with the bookmark kept. So Cmd+D then Enter, or Cmd+D then Esc, both keep it.

## Where bookmarks show

The owner's answer on the four places mocked (2026-11-18, mocks in the UX work folder):
build them all, each a choice in Settings › Bookmarks, and the bar over pages a choice of
its own. Each is a pref Go keeps (`core/prefs`) and a shell reads from `prefs`; the
settings page sets them by name with `setPref`, as a shell does.

| Pref | Default | Standard | Mobile |
|---|---|---|---|
| `bookmarksNewTab` | on | the new tab shows the top folder as tiles, folders first; a folder opens in place, with Open all as a group | the same, on its new tab page |
| `bookmarksSidebar` | off | the folders as a tree under the tabs | the switcher has Tabs and Bookmarks |
| `bookmarksCommandBox` | on | `*` in the command box narrows it to bookmarks, the folders on its left | the address tapped on a page shows every bookmark as one row of tiles, and typed rows include bookmarks |
| `bookmarksBar`, `bookmarksBarFolder` | off, the top | a bar of the folder's bookmarks under the address bar; a folder in it drops down | a strip of them above the bars, which hides with them |

The bar's folder is a folder's id, `""` for the top; a shell shows the top when the folder
is gone.

How Standard shows them (`shell/src/apps/shell/standard/`, `bookmarks.js` for what they share):

- **The new tab** (`NewTabBookmarks.vue`): under the command box, which then lists only the
  sites visited most with nothing typed, as the tiles hold the bookmarks. A bookmark has no
  favicon of its own: a tile shows an open tab's, else its site's letter on its site's
  colour. A folder's tile shows four of its sites; it opens under its row, its folders and
  bookmarks as tiles with a path back up. A bookmark loads where the box's rows do. Open
  all as a group has no message of its own in Go: the first bookmark opens in a new tab,
  the rest join it with `newTab` and `groupWith`, which makes the group, and `editGroup`
  names it after the folder.
- **Beside the tabs** (`SidebarBookmarks.vue`): under the tabs, half the sidebar at most and
  scrolling on its own; a heading that folds it, and the tree, every folder folded at first.
  What is unfolded lasts while the shell runs. A bookmark shows the tab that has its address
  open, which marks it with a dot, else opens in a new tab. With the tabs on top there is no
  sidebar, so none shows. A tab dragged into a folder is not built.
- **In the command box** (`BookmarkFinder.vue`, the keys in `CommandBox.vue`): a `*` typed
  first narrows the box to bookmarks, a ★ Bookmarks chip before the text, and Go is asked
  nothing. The left lists All bookmarks, then the folders; the right, the chosen folder's
  bookmarks, its folders' too, whose title or address holds the text, then, once something is
  typed, those in other folders after a rule; one not in the chosen folder says which it is
  in. ↑ and ↓ choose a bookmark, ← and → a folder; Enter opens the bookmark where the box's
  rows open, and Shift+Enter the chosen folder's own bookmarks as a group, as the new tab's
  Open all does. Escape, or Backspace with nothing typed, leaves bookmarks with the box open.
  Tab in an empty box, ⌘1 to ⌘9 and ⌘D, in the mock, are not built.
- **The bar** (`BookmarkBar.vue`): a 32px strip under the top bar, in both layouts, at the
  top of the page's column (beside the sidebar when the tabs are on the side), so the page is
  laid out under it and its region starts below it. It shows the chosen folder's entries in
  their places: a bookmark as its mark and title, which loads in the active tab, or in a new
  one behind it on a middle press; a folder as a chip that drops down what it holds, its
  folders as submenus of every bookmark in them (a menu goes one submenu deep), then Open
  all as a group. What doesn't fit goes in a » menu at its end. The mock's folder per
  workspace is not built: the bar's folder is the one pref.

### On the Mobile web shell

- **The new tab page** (`NewTab.svelte`) shows the top folder under Bookmarks, after the
  sites, as tiles four to a row, its folders first. A folder's tile shows the first four of
  its bookmarks two by two, each the first letter of its name on a colour its host gives it.
  A tap on a folder opens it in place, in a card under its row: how many bookmarks it
  holds, Open all, and what it holds as tiles; a folder in it opens in the card, with Back,
  and a second tap on the folder closes the card. A bookmark loads in the tab. Open all
  loads the first bookmark in the tab and opens the rest behind it, in a group with it
  named after the folder (`newTab` with `groupWith`, then `editGroup` once the group shows);
  a tab already in a group takes them into its own. Off, the page shows no bookmarks.
- **The switcher** (`Switcher.svelte`) has Tabs, with how many, and Bookmarks at its top.
  Bookmarks shows a folder as a list (`BookmarkList.svelte`), the top to start with: the
  path to it, each folder on it a tap away, then its folders with how many bookmarks each
  holds, a tap going into one, then its bookmarks, each its letter, name and host. A tap on
  a bookmark opens it in a new tab, in the workspace the switcher shows, and closes the
  switcher. It opens on Tabs each time. Off, it has its tabs alone, as before.
- **The address** (`Address.svelte`), tapped on a page with nothing typed, shows every
  bookmark, of every folder, as one row of tiles that scrolls sideways, beside the page's
  own row: under it with the bar on top, over it with the bar at the foot. No folders: they
  live on the new tab page and in All bookmarks (owner, 2026-11-20, [daily-driver.md](daily-driver.md#what-the-address-field-suggests-and-when)).
  A tap on a tile loads it in the tab. Typing replaces them with the suggestions, which
  then include bookmark rows. Over a new tab the field shows nothing, the page having its
  own. Off, the page's row shows alone, and typed rows have no bookmarks.
- **The bookmarks bar** (`MarksBar.svelte`) is one more of the bars: a 48px row of chips,
  what `bookmarksBarFolder` holds, the top's when that folder is gone, its folders first,
  scrolling sideways. A bookmark is its letter and name, and a tap loads it in the tab; a
  folder is a folder and its name, and a tap opens a sheet of what it holds
  (`FolderSheet.svelte`, the same list as the switcher's, from that folder), where a
  bookmark loads in the tab too. With the address at the foot it lies over the toolbar,
  and over the group strip while the tab is in a group; with the address on top it lies
  under it. It hides and comes back with the bars, and the page's room counts it
  ([shells.md](shells.md#bars-that-hide-on-scroll)). A folder that holds nothing has no
  bar.

## What the person sees

- **The star** (Standard's address bar) and **Cmd+D** (every shell) bookmark the page into
  the folder last saved in, or find its bookmark when the page is one, and Standard shows
  the panel under the star: Bookmarked, or Edit bookmark for one that was already there; its
  name, chosen and selected; the folders, the bookmark's own chosen, once there is a folder;
  Remove and Done. Enter, anywhere in it but on a button, saves the name and the folder,
  and the next bookmark goes into that folder too. Escape or a press outside closes it with
  the bookmark kept as it was. The panel closes with another tab, and the page gets the
  focus back. ([mock](screenshots/bookmarks/mock-b3.png))
- **The Mobile shells'** Bookmark, in the menu sheet's row of actions after Forward, does the
  same, its star filled on a bookmarked page, and the panel is a sheet in the menu's place:
  Enter in the name, or Done, saves; the back key, a tap outside or a swipe down close it.
  The native Mobile shell draws the same sheet in Kotlin (`bookmarkSheet` in `Sheets.kt`),
  its folders in the icon colour, as it has no gulal for their tints.
- **Shells with no panel** (Glide, Fox, Minimal, Terminal) have Cmd+D save the bookmark all
  the same, into the folder last saved in.
- **The Bookmarks page** has the folders on its left: Bookmarks, the top, with the folders
  under it, nested, each with how many bookmarks it holds, in it and in its folders; New
  folder at the foot makes one in the folder shown. The right shows the folder chosen, its
  folders first, then its bookmarks, a path above it back to the top. Search looks in every
  folder, and says which folder each bookmark is in ([mock](screenshots/bookmarks/mock-b1.png)).
- **Moving:** an entry dragged onto a folder, on the left, in the list or in the path, moves
  there, first in it. Each row's menu (⋯, or a right-click) has Open in new tab (Open, for
  a folder), Edit… (Rename…), Move to…, which asks for a folder in a dialog, and Delete
  ([mock](screenshots/bookmarks/mock-b2.png)).
- **Delete:** a folder that holds anything asks first, saying how many bookmarks go with it.
- **On a phone** the page has no room for the left: the path leads back up, New folder is by
  the search, and each row's ⋯ shows at once.
- **Imports** (slice 3) come in as a folder named after the browser, "From Chrome", holding
  that browser's own tree.

## How it is kept

`core/bookmarks` keeps one tree. Each entry, bookmark or folder, has:

| Field | |
|---|---|
| `id` | random, 16 hex digits, made once and never changed, the same on every device a later sync reaches |
| `parent` | the folder's id, `""` for the top |
| `kind` | `bookmark` or `folder` |
| `title`, `url` | a folder has no url |
| `added`, `modified` | Unix milliseconds; `modified` changes with the title, the url or the folder |
| `place` | its place among its folder's entries, a key that sorts as text, so moving one entry changes that entry alone, as a sync needs (project-b-70) |

A place is base-36 digits with no trailing zero (`core/bookmarks/place.go`): there is
always a key between two, and at either end of a folder a key steps a digit, so a folder
whose entries keep going first grows its keys by a digit only every 35 or so. A new entry,
and one moved into a folder, goes first in it.

An address is bookmarked once: a second Add finds the bookmark, and an edit to an address
another bookmark has is refused.

The tree is the `bookmarks` table of the profile's store, `modified` its `edited` column, and
the folder a bookmark was last saved in from the panel is `bookmarks_last`
([store.md](store.md#bookmarks)). Go reads it whole at start and writes each entry that
changes. Before the store it was `bookmarks.json`, version 2: `{version, last, entries}`; and
before that version 1, a flat list with numbered ids. The first start with the store reads
either once: version 1's bookmarks get a random id each in the top folder, newest first as
before, with the times in milliseconds. A deleted entry leaves no tombstone yet; sync adds
what it needs when its design is agreed.

What a shell has and sends (`shell/src/sdk/bookmarks.js`, the tree's helpers in
`shell/src/ui/bookmarks.js`):

| | |
|---|---|
| `state.bookmarks` | the tree, a flat list in its places |
| `state.bookmarkPanel` | `{id, new}`, the panel Go asks this window to show; a new object each time |
| `bookmark(url, title)` | the star: `bookmark {url, title}` |
| `saveBookmark(id, title, parent)` | the panel's Enter: `saveBookmark {bookmark, title, parent}` |
| `removeBookmark(id)` | the panel's Remove: `removeBookmark {bookmark}` |

Cmd+D is the action `bookmarks.toggle`, Bookmark This Page…, which does what the star does
for the active tab of the window in front. Go gives the shell the focus with the panel, so
Enter and Escape reach it.

The Bookmarks page sends `editBookmark {id, title, url}`, `moveBookmark {id, parent}`,
`newFolder {parent, title}` and `removeBookmark {id}`; a refusal comes back as
`error {id, error}`.

## Tests

`core/bookmarks` (the tree, the moves, the folder last saved in, version 1 read, the
places), `core/app` (`TestBookmarksFromTheShellAndThePage`: Cmd+D, the star, the panel's
save, and the page's messages), `shell/e2e/pages.cjs` (the panel, Cmd+D, the folders, a drag,
Move to…, a search, a folder deleted with what it holds, the panel's folder kept for the
next), `shell/e2e/shells/mobile.cjs` (the sheet), `tools/drive_android.py bookmark_star`
(the star and its sheet on a phone, in both Mobile shells: the folder last saved in, Enter,
the back key, Remove) and `shell/e2e/shells/standard-bookmarks.cjs`
(Standard's places, each on and off with its pref: the new tab's tiles, a folder opened in
place and opened as a group; the sidebar's tree, a bookmark showing its tab or opening one; the command box narrowed by
`*`, its folders, its search and its keys; the bar, its folder, its menus, its » and the page
laid out under it in both layouts).
