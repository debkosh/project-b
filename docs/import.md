# How import works

This is how Project B brings bookmarks, history, the search engine and extensions over from another
browser on the Mac. Written to be read aloud; the detail is in
[onboarding.md](onboarding.md#built) and [browserdata's README](../browserdata/README.md).

```
  other browsers' files        browserdata/            core/app/imports.go        our stores
  ─────────────────────        ────────────            ───────────────────        ──────────
  Chrome, Edge, Brave, Arc ─┐  Find(home)              look: list the rows   ─┐
  Firefox                  ─┼─ Bookmarks, History, ──▶ bring: read on a      ─┼─▶ bookmarks, in "From Chrome"
  Safari (Full Disk Access)─┘  Search, Extensions      worker, add on the     │   history, merged
                               (reads only)            main thread           ─┘   search engine pref
                                                                                  extensions, by id from the store
```

**Two halves.** Reading is one Go module, `browserdata`, at the top of the repo. It has its
own go.mod, imports nothing from core, and only reads, so it could be a project of its own.
Deciding what to do with what it read is core's: `core/app/imports.go`, which the welcome's
Bring your things over and Settings › Import both talk to.

**Finding profiles.** Find looks under the home folder's Application Support for each
browser it knows. Chrome, Edge, Brave and Arc share Chromium's layout: a file called Local
State names the profiles, and each profile is a folder. Firefox lists its profiles in
profiles.ini. Safari has one, in Library/Safari. Each profile becomes a row, with what it
holds counted, and the browser's logo, which ships with us.

**Reading each format.** Chromium keeps bookmarks as JSON, history in SQLite, the search
engine in Preferences, and extensions in Secure Preferences with each one's manifest.
Firefox keeps bookmarks and history together in places.sqlite, and its search engine in a
compressed JSON file, which a small LZ4 reader opens. Safari keeps bookmarks in a binary
property list, which a small reader of our own opens, with the Reading List as one of its
folders, and history in SQLite; its search engine is macOS's own setting.

**Safari's lock.** macOS keeps Safari's folder behind Full Disk Access. Until the app has it,
the row says so; turning it on opens System Settings at the right switch, and Go tries the
folder every second. The moment it can read, the row comes back unlocked, with no restart.

**Locked databases.** The other browser may be running and holding its database open. So
each SQLite file is copied first, with its write-ahead log, and the copy is read. We never
write to another browser's files. SQLite is macOS's own library, so nothing extra ships.

**Mapping in.** Bookmarks go into a folder of their own at the end of the bookmarks, named
after the source, "From Chrome", or "From Chrome · Work" when a browser has several
profiles, keeping the browser's own folders inside. History joins as pages. The search
engine is taken when it is one we have, matched by its address. Extensions, offered only
in Settings while the Extensions flag is on, are installed again from the Chrome Web Store
by their ids, one at a time, with no prompt, as ticking them is the yes; one that isn't
from that store, was loaded from a folder, or is Manifest V2 is left out, and says why.

**Importing twice.** An address already bookmarked isn't added again, and a second import
reuses the folders the first made. History keeps the larger visit count and the later
visit, so nothing is counted twice. An extension already installed is said to be, not
installed again.

**What came.** Each import's result is kept in imports.json in the profile, and both pages
show it per browser: bookmarks and folders, Reading List items, pages of history, the
search engine, and each extension installed, installed already, failed, or left out.

**Android.** A phone keeps each app's data from the others, so there is nothing to read.
The package still builds there: its macOS-only parts answer "not on this system", and the
app finds no browsers, so the phone's welcome has no import step.
