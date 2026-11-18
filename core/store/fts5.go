//go:build !sqlite_fts5

package store

// The history's search needs SQLite's FTS5, which github.com/mattn/go-sqlite3 builds in only
// with the sqlite_fts5 build tag: build and test with -tags sqlite_fts5, as every script in
// the repo does (docs/store.md). Without it this line stops the build, rather than the app
// starting with no history.
var _ int = "build with -tags sqlite_fts5"
