# browserdata

Reads what other browsers on a Mac keep: which are there and their profiles, and from a
profile its bookmarks with their folders, its history and the
extensions its person installed. It only
reads, each database from a copy, as the browser may have it open. What to do with what
it finds is the caller's: Project B's welcome and Settings' Import bring it over
([docs/onboarding.md](../docs/onboarding.md)).

It is a Go module of its own, with no import of Project B's `core`, so it could move to a
repo of its own as it is. It uses nothing but the standard library and macOS's own
`libsqlite3`, through cgo; Safari's property lists are read by a small reader of
its own.

| Browser | Where | Files |
|---|---|---|
| Chrome | `~/Library/Application Support/Google/Chrome` | Chromium's: `Local State` names the profiles; each has `Bookmarks` (JSON), `History` (SQLite), `Preferences` and `Secure Preferences` (JSON), `Extensions/` |
| Edge | `…/Microsoft Edge` | Chromium's |
| Brave | `…/BraveSoftware/Brave-Browser` | Chromium's |
| Arc | `…/Arc/User Data` | Chromium's |
| Firefox | `…/Firefox` | `profiles.ini` names the profiles; each has `places.sqlite` |
| Safari | `~/Library/Safari`, behind Full Disk Access | one profile: `Bookmarks.plist` (a binary property list), `History.db` (SQLite) |

```go
for _, b := range browserdata.Find(home) {        // the browsers with a profile
	for _, p := range b.Profiles {
		if p.Locked() {                           // Safari's, until the app has Full Disk Access
			continue
		}
		c, _ := p.Counts()                        // bookmarks, folders, pages, extensions
		tree, _ := p.Bookmarks()                  // top folders, such as the bookmarks bar, with all they hold
		pages, _ := p.History()                   // each web page visited: title, visits, last visit
		exts, _ := p.Extensions()                 // what its person installed, and why any can't come
	}
}
```

- Safari's folder is there to see but not to read until the app has Full Disk Access:
  `Locked` says so. Safari has one profile, with no name.
- Extensions are a Chromium profile's, those its person installed: what the browser, an
  administrator or Chrome itself installed is left out. Each says why it can't be installed
  again by its id from the Chrome Web Store, if it can't: `SkipUnpacked`, `SkipNotInStore`
  (another store's, or a file's), `SkipManifestV2`. A name its manifest takes from its
  messages (`__MSG_name__`) is looked up in its default locale. `Counts` counts those that
  can come.
- History is web pages alone (http and https), not hidden, visited at least once.
- Firefox's tags, separators and saved searches (`place:`) aren't bookmarks.
- SQLite needs macOS; elsewhere they return `ErrNotOnThisSystem`.

`cmd/browserdata` prints what it finds, for checking against a real browser:

```
go run ./cmd/browserdata                          # each browser, profile and counts
go run ./cmd/browserdata -dump chrome/Default     # one profile in full, as JSON
go run ./cmd/browserdata -home testdata/home      # the test home, below
```

Tests: `go test ./...`, against `testdata/home`, a home folder with a Chrome of two
profiles, a Firefox of one and a Safari in the browsers' own formats, which `testdata/make.sh` makes
(it needs `sqlite3` and `python3`).
