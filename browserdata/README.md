# browserdata

Reads what other browsers on a Mac keep: which are there and their profiles, and from a
profile its bookmarks with their folders, its history and its search engine. It only
reads, each database from a copy, as the browser may have it open. What to do with what
it finds is the caller's: Project B's welcome and Settings' Import bring it over
([docs/onboarding.md](../docs/onboarding.md)).

It is a Go module of its own, with no import of Project B's `core`, so it could move to a
repo of its own as it is. It uses nothing but the standard library and macOS's own
`libsqlite3` and AppKit, through cgo; Safari's property lists are read by a small reader of
its own.

| Browser | Where | Files |
|---|---|---|
| Chrome | `~/Library/Application Support/Google/Chrome` | Chromium's: `Local State` names the profiles; each has `Bookmarks` (JSON), `History` (SQLite), `Preferences` (JSON) |
| Edge | `…/Microsoft Edge` | Chromium's |
| Brave | `…/BraveSoftware/Brave-Browser` | Chromium's |
| Arc | `…/Arc/User Data` | Chromium's |
| Firefox | `…/Firefox` | `profiles.ini` names the profiles; each has `places.sqlite` and `search.json.mozlz4` |
| Safari | `~/Library/Safari`, behind Full Disk Access | one profile: `Bookmarks.plist` (a binary property list), `History.db` (SQLite); the search engine is macOS's, `NSPreferredWebServices` in `~/Library/Preferences/.GlobalPreferences.plist` |

```go
for _, b := range browserdata.Find(home) {        // the browsers with a profile, and their app
	for _, p := range b.Profiles {
		if p.Locked() {                           // Safari's, until the app has Full Disk Access
			continue
		}
		c, _ := p.Counts()                        // bookmarks, folders, pages, search engine's name
		tree, _ := p.Bookmarks()                  // top folders, such as the bookmarks bar, with all they hold
		pages, _ := p.History()                   // each web page visited: title, visits, last visit
		s, _ := p.Search()                        // the engine chosen, or the browser's own
	}
	png, _ := browserdata.Icon(b.App, 64)         // the browser's own logo, from the app installed
}
```

- A profile's search engine is the one its person chose, or the browser's own when none
  was: Google for Chrome, Arc, Firefox and Safari, Bing for Edge, Brave Search for Brave.
- Safari's folder is there to see but not to read until the app has Full Disk Access:
  `Locked` says so. Safari has one profile, with no name.
- History is web pages alone (http and https), not hidden, visited at least once.
- Firefox's tags, separators and saved searches (`place:`) aren't bookmarks.
- SQLite and the icon need macOS; elsewhere they return `ErrNotOnThisSystem`.

`cmd/browserdata` prints what it finds, for checking against a real browser:

```
go run ./cmd/browserdata                          # each browser, profile and counts
go run ./cmd/browserdata -dump chrome/Default     # one profile in full, as JSON
go run ./cmd/browserdata -icon out.png chrome     # a browser's icon, 128 pixels square
go run ./cmd/browserdata -home testdata/home      # the test home, below
```

Tests: `go test ./...`, against `testdata/home`, a home folder with a Chrome of two
profiles, a Firefox of one and a Safari in the browsers' own formats, which `testdata/make.sh` makes
(it needs `sqlite3` and `python3`).
