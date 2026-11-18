# How import works

This is how Project B brings bookmarks, history and extensions over from another
browser on the Mac. Written to be read aloud; the detail is in
[onboarding.md](onboarding.md#built) and [browserdata's README](../browserdata/README.md).

```
  other browsers' files        browserdata/            core/app/imports.go        our stores
  ─────────────────────        ────────────            ───────────────────        ──────────
  Chrome, Edge, Brave, Arc ─┐  Find(home)              look: list the rows   ─┐
  Firefox                  ─┼─ Bookmarks, History, ──▶ bring: read on a      ─┼─▶ bookmarks, in "From Chrome"
  Safari (Full Disk Access)─┘  Extensions              worker, add on the     │   history, merged
                               (reads only)            main thread           ─┘
                                                                                  extensions, by id from the store
```

**Two halves.** Reading is one Go module, `browserdata`, at the top of the repo. It has its
own go.mod, imports nothing from core, and only reads, so it could be a project of its own.
Deciding what to do with what it read is core's: `core/app/imports.go`, which the welcome's
Bring your things over and Settings › Import both talk to.

**Finding profiles.** Find looks under the home folder's Application Support for each
browser it knows, forks included: each is one line of data, a name, a folder and a family.
Chrome, Edge, Brave, Arc, Vivaldi, Opera and the rest share Chromium's layout: a file called
Local State names the profiles, and each profile is a folder. Firefox and its forks, Zen,
LibreWolf and others, list their profiles in profiles.ini. Safari has one, in
Library/Safari. A browser we don't know is a folder the person chooses, which we recognise
by the same files. Each browser shows with its logo, which ships with us, and each profile
with what it holds.

**Reading each format.** Chromium keeps bookmarks as JSON, history in SQLite, and
extensions in Secure Preferences with each one's manifest. Firefox keeps bookmarks and
history together in places.sqlite. Safari keeps bookmarks in a binary property list, which
a small reader of our own opens, with the Reading List as one of its folders, and history
in SQLite. The search engine doesn't come over: the welcome asks for it instead.

**Safari's lock.** macOS keeps Safari's folder behind Full Disk Access. Until the app has it,
the row says so; turning it on opens System Settings at the right switch, and Go tries the
folder every second. The moment it can read, the row comes back unlocked, with no restart.

**Locked databases.** The other browser may be running and holding its database open. So
each SQLite file is copied first, with its write-ahead log, and the copy is read. We never
write to another browser's files. SQLite is macOS's own library, so nothing extra ships.

**Mapping in.** Bookmarks go into a folder of their own at the end of the bookmarks, named
after the source, "From Chrome", or "From Chrome · Work" when a browser has several
profiles, keeping the browser's own folders inside. History joins as pages. Extensions, offered only
in Settings while the Extensions flag is on, are installed again from the Chrome Web Store
by their ids, one at a time, with no prompt, as ticking them is the yes; one that isn't
from that store, was loaded from a folder, or is Manifest V2 is left out, and says why.

**Passwords.** A Chrome-family browser seals each password with AES, its key made from a
secret it keeps in the Mac's login keychain; we ask the keychain for that secret, and macOS
asks you first, once per browser. Firefox seals them with a key kept in key4.db, itself
locked by your primary password if you set one, so the wizard asks for it. Safari's are in
iCloud Keychain, which no app can read, so they come as the file the Passwords app
exports. Passwords stay in memory and go straight into the passwords store; only their
count is shown.

**From a file.** Settings also takes a bookmarks file, the HTML every browser exports, and
a passwords file, the CSV a browser or the Passwords app exports.

**Importing twice.** An address already bookmarked isn't added again, and a second import
reuses the folders the first made. History keeps the larger visit count and the later
visit, so nothing is counted twice. An extension already installed is said to be, not
installed again.

**What came.** The wizard ticks each kind as it lands and then shows what came: bookmarks
and folders, Reading List items, pages of history, and each extension installed, installed
already, failed, or left out. Nothing about an import is kept afterwards.

**Android.** A phone keeps each app's data from the others, so there is nothing to read.
The package still builds there: its macOS-only parts answer "not on this system", and the
app finds no browsers, so the phone's welcome has no import step.
