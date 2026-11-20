//go:build js

package store

import (
	"database/sql"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
	_ "github.com/ncruces/go-sqlite3/vfs/memdb"
)

// openSQLite opens the database at path in a web page, where the core runs as WebAssembly: SQLite
// compiled to Go, with FTS5 for the history's search, its database in memory, gone with
// the page.
func openSQLite(path string) (*sql.DB, error) {
	return driver.Open("file:/"+path+"?vfs=memdb", func(c *sqlite3.Conn) error { return fts5.Register(c) })
}

// damaged is never so for a database made new in memory.
func damaged(error) bool { return false }
