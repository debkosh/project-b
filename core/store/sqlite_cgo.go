//go:build !js

package store

import (
	"database/sql"
	"errors"
	"net/url"

	"github.com/mattn/go-sqlite3"
)

// openSQLite opens the SQLite database at path. The driver keeps each statement prepared once,
// so a query asked again isn't parsed again.
func openSQLite(path string) (*sql.DB, error) {
	return sql.Open("sqlite3", "file:"+(&url.URL{Path: path}).EscapedPath()+"?_journal_mode=WAL&_synchronous=NORMAL&_stmt_cache_size=64")
}

// damaged says SQLite found the file isn't a database, or is a damaged one: what a new file
// mends, and nothing else does.
func damaged(err error) bool {
	var e sqlite3.Error
	return errors.As(err, &e) && (e.Code == sqlite3.ErrNotADB || e.Code == sqlite3.ErrCorrupt)
}
