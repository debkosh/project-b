# Bookmark folders

Slice 2 of the [onboarding](onboarding.md#what-it-needs-that-we-dont-have): bookmarks brought
over from another browser keep their folders, so ours need them first. **Proposed, 2026-11-17:
for the owner's answer, nothing built.** Mocks: [screenshots/bookmarks/mocks.html](screenshots/bookmarks/mocks.html).

## What the person sees

- **The Bookmarks page** gets the folders on its left: All bookmarks at the top, the folders
  under it, nested, each with how many it holds; New folder at the foot. The right shows the
  folder chosen: its folders first, then its bookmarks, a path above it back to All
  bookmarks. Search looks in every folder ([mock](screenshots/bookmarks/mock-b1.png)).
- **Moving:** a bookmark or a folder is dragged onto a folder on the left, or moved with the
  row's menu, Move to…, which lists the folders. The menu also has Open in new tab, Edit…
  and Delete ([mock](screenshots/bookmarks/mock-b2.png)).
- **Folders** are renamed in place, and deleted with what they hold; deleting one that
  holds anything asks first, saying how many.
- **The star** (Standard's address bar, Cmd+D in every shell) bookmarks the page into the
  folder last used, and Standard's star then shows a small panel to change the name or the
  folder, or remove it again; a press elsewhere closes it ([mock](screenshots/bookmarks/mock-b3.png)).
  The Mobile web shell's Bookmark item does the same with a sheet.
- **Imports** (slice 3) come in as a folder named after the browser, "From Chrome", holding
  that browser's own tree.

No bookmarks bar: no shell has one today, and it would be a shell's to draw.

## How it is kept

`core/bookmarks` keeps one tree. Each entry, bookmark or folder, has:

| Field | |
|---|---|
| `id` | random, 16 hex digits, made once and never changed, the same on every device a later sync reaches |
| `parent` | the folder's id, `""` for the top (All bookmarks) |
| `kind` | `bookmark` or `folder` |
| `title`, `url` | a folder has no url |
| `added`, `modified` | Unix milliseconds; `modified` changes with the title, the url, the parent or the place |
| `place` | its place among its folder's entries, a fractional key that sorts as text, so moving one entry changes only that entry, as a sync needs (project-b-70) |

The file is `bookmarks.json`, version 2. The bookmarks people have today are version 1, a flat
list with numbered ids: the first start of this version reads it, gives each a random id in
the top folder, newest first as now, and writes version 2. A deleted entry leaves no
tombstone yet; sync adds what it needs when its design is agreed.

The manager tells its subscribers which entries changed (`OnChanged(ids)`), so the pages and
the shells resend only what moved, and a sync can follow.

## Questions for the owner

1. The star: the small panel to pick the folder (as above, and as Chrome does), or bookmark
   silently as today and move it on the Bookmarks page?
2. Is a bookmarks bar wanted at all, in Standard, now or later?
