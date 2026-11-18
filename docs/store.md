# The store

One SQLite database per profile, `store.db` in the profile folder, for what Go keeps that
grows and is searched as one types: history and bookmarks today, saved logins and the rest
later, each a feature with its own tables. **Built, 2026-11-19.** A simple clean base first;
sync, keys and sealing come later, on top of it.

## His decisions

- 2026-11-18: "we need to design a simple clean base then add features later"
- 2026-11-18: "we need to think about speed of querying, so sqlite or bolt makes sense to
  store items in local... sync can be slower"
- 2026-11-19: "history will definitely need to stay in sqlite"
- 2026-11-19: "ok lets write it in sqlite db... each gets a table.... history is site visit
  count last visit.... and individual visits are not synced... for suggestion we do create a
  table which is basically typed urls... searching with say g.. should first show github.com
  only and then longer title matches show below it if the direct searches are less than the
  required count"
- 2026-11-19: "yes store visits but do not sync.... typed as you said". Typed is an address
  typed in the address field and entered, or a suggestion picked after typing; a link
  clicked and a bookmark opened don't count.
- 2026-11-19, on a store.db that can't be opened: "yes move the old file into backup and
  create a new one".

## What moves and what stays

| | Where |
|---|---|
| History, its visits, its sites, the typed addresses | `store.db` |
| Bookmarks and the folder last saved in | `store.db` |
| Prefs, flags, workspaces, pins, the session, the downloads list, the welcome, the shells' data | their JSON files, as before |
| The timeline, crash logs, the profile list, the update state | their files, as before |

The JSON files are small, read whole at start and written whole; nothing searches them as one
types, so they stay.

## How it works

`core/store` opens `store.db` with WAL and `synchronous=NORMAL`, through
`github.com/mattn/go-sqlite3`, which builds SQLite into the core with cgo. Its FTS5, which the
search needs, is built only with the `sqlite_fts5` tag: every build and test script passes it
(`engine/build_go.py`, `engine/android/build_go.py`, `tools/test.sh`, the e2e tests'
`go build`), and `core/store` does not compile without it, saying so.

**One worker.** Every read and write runs on one goroutine with the database's one
connection, never on the main thread, each job one transaction, in the order asked. So a read
sees every write asked before it, and the main thread never waits on a lock. A feature posts
the answer back to the main thread itself, as `core/history` does with the engine's `post`.
The main thread waits on the worker once, at start, for the bookmarks, which are few
(`Wait`).

**Schema steps.** Each feature gives its schema as numbered steps, in order, the first making
its tables. The store keeps one number per feature, in its table `schema`, and runs the steps
a profile has not had, in one transaction, before any other job of that feature: a new step
is added at the end and never changed once released. A feature's first step also reads its old
file once (below).

**For sync, later.** Every table that may sync one day has `id`, made once, and `edited`, Unix
milliseconds of its last change. A new table's id is a UUIDv7 as text (`store.NewID`); the
bookmarks keep the ids they have had since folders, 16 random hex digits. Nothing else is built for sync: deleting
deletes the row, with no tombstone. Visits and sites never sync and have neither.

**A damaged store.db.** When SQLite says store.db is not a database, or a damaged one, as it
is opened, the file goes aside with its `-wal` and `-shm` as `store.db.backup-<UTC date and
time>`, such as `store.db.backup-20261119T101500Z`, a new store.db is made, and the start
carries on; the timeline has one error line naming the backup. Nothing else is mended this
way: a folder that can't be written, or a new file that can't be opened either, still stops
the start, and a feature's schema step that fails is a bug in the feature, which rolls back
and leaves the file as it was. With new, empty tables, each feature's first step runs again,
so the one-time read of history.jsonl and bookmarks.json (below) brings back what the profile
had when the store was first made, rather than nothing, while those files are still there.

**Checkpoints.** SQLite's own: when the WAL passes 1000 pages the write that crosses it
copies them back, 5 to 15 ms on the worker for that one write. Nothing on the main thread
waits for it, so they are left on.

## History

| Table | One row per | Columns |
|---|---|---|
| `history` | address | `id`, `url`, `title`, `visits`, `last_visit` (Unix seconds, as history.jsonl had it), `edited`; `low_url` and `low_title`, lowercased by Go as the search compares them, since SQLite's `lower()` knows only ASCII; `bare` and `rank`, made by SQLite from them |
| `visits` | visit made here | `page` (the history row), `time` (Unix seconds). Local only |
| `sites` | site, a host without `www.` | its visits added up, and its page: the one visited most, then latest, then the lowest address |
| `typed` | address reached from the address field | `id`, `url`, `count`, `last` (Unix seconds), `edited` |
| `history_tri` | | an FTS5 trigram index of `low_url` and `low_title` |

Only `http` and `https` pages are kept. Triggers keep `sites` and `history_tri` in step with
every insert, change and delete of a `history` row, and a deleted row takes its `visits` and
its `typed` row with it, so deleting a page, a site, or clearing a time range takes it out of
all of them. Top sites read `sites` in order, a few microseconds at any size, rather than adding
up every page.

A visit is one transaction: the page's row (one more visit, the latest time, the title when
there is one), its `visits` row, and when it was typed its `typed` row. An import from another
browser adds pages with their visits counted but no `visits` rows, as their times are not
known.

**Typed.** Go records an address as typed when the navigation it started commits: Enter in an
address field or a command box with text that reads as an address (the shell sends
`navigate {text, typed: true}`, or `newTab {text, typed: true}`), or a row of the suggestion list
chosen after something was typed (the rows of such a list carry `typed: true`, and come back
with it in `chooseSuggestion`). A search typed into the field is not an address and is not
recorded, nor is a link, a bookmark, a tile or a row of the list for nothing typed. `core/tabs`
marks the navigation the load starts, and the page's address once it commits is the one
recorded, after any redirect.

## Suggestions

`history.Search(text, limit)` gives, for the text lowercased:

1. The typed addresses whose bare address (no scheme, no `www.`) starts with it, most typed
   first, then the latest typed. `g` shows github.com first once it is typed most.
2. Only if those are fewer than `limit`, history, scored as before: a bare address that starts
   with the text 1000, an address that holds it 100, a title that holds it 50, each plus twice
   its visits up to 50; then the latest visit, then the address. An address never shows twice.

`core/suggest` builds its list from that as before, its completion from the first row, so a
typed address completes first.

The history part is one query per tier, each with a `LIMIT`, so none sorts every match:

- **Starts with:** counts the matches in the index of `bare`, stopping at about √(8 × rows).
  Few, it reads them from the index and sorts them; many, it walks the index in score order
  (`rank`, the latest visit, the address), which meets enough of them within a few hundred rows.
- **Holds, three letters or more:** the trigram index gives the rows that hold the text in
  their address or title, both tiers at once, stopping at the same count; few, they are read and
  sorted; many, each tier counts its own column, then walks as above. One or two letters walk
  the score order.
- An address match scores 102 to 200 and a title match 52 to 150, so both are fetched and
  merged, and a title match is looked for only with a rank that can still reach the last
  address match kept.
- Text with a space skips the address tiers: addresses keep their spaces escaped.

The benchmark behind this (10,000, 50,000 and 200,000 addresses, 3,000 queries each) gave
today's top eight in the same order for every query, and was faster than the search in memory
at every length: at 200,000 addresses a letter took 0.10 ms at the median, four letters 0.16
ms, two words 1.2 ms, and the slowest one in a hundred 5.2 ms against 39 ms in memory. One
visit, with every index kept in step, took about 75 µs at the median. A separate table of the
pages visited most, or an index of them alone, gave other results in up to 37% of queries, and
is not built. The disk at 200,000 addresses is 162 MB, half of it the trigram index.

Measured on the store as built, on the benchmark's 50,000 addresses (Search answered back,
2,000 queries): 0.26 ms at the median, 3.0 ms for the slowest one in a hundred, 6.9 ms the
slowest; one letter 0.15 ms, two words 0.85 ms. A visit, asked and done on the worker, 0.16 ms
at the median and 7.8 ms for the slowest one in a hundred, a checkpoint among them. Top sites
0.02 ms. store.db 62 MB.

## Bookmarks

| Table | Columns |
|---|---|
| `bookmarks` | `id`, `parent`, `kind`, `title`, `url`, `added`, `edited` (the entry's `modified`), `place` |
| `bookmarks_last` | one row: the folder a bookmark was last saved in |

The tree stays in memory, read whole at start: the main thread reads it as before, and each
change writes the rows it changed, on the worker, one transaction a change. The ids and
places are as [bookmark-folders.md](bookmark-folders.md) has them. The bookmarks are made
before history at start, so the main thread's wait for them is not behind history's first
read of history.jsonl.

## The old files

`history.jsonl` and `bookmarks.json` are released formats. A feature's first schema step reads
its file once, in the same transaction that makes its tables: the last line for an address
wins in history.jsonl, and bookmarks.json is read in either of its versions. The file is then
left as it is, never written or deleted; a later start has the tables and never looks at it.
History's rows go in first, then its trigram index and its sites are made from them at once,
then the triggers: 50,000 pages take 1.4 s on the worker, against 6 s row by row through the
triggers. Until it is done, a search waits behind it.

## Tests

`core/store` (the steps run once each, in order, a failed step leaves nothing, a store.db
that is no database goes aside byte for byte and a new one is made, and a healthy one stays),
`core/history` (each tier and a row once, typed first, a non-ASCII title and address, top
sites, deleting a site, clearing a range from every table, the one-time read of
history.jsonl, again into a new store), `core/suggest` (typed first and its completion), `core/tabs` (which navigations
count as typed), `core/bookmarks` (the one-time read of both versions of bookmarks.json, and the
tree surviving a restart).
