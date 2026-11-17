//go:build darwin && cgo

package browserdata

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"unsafe"
)

// row is one row of a query's result.
type row struct{ s *C.sqlite3_stmt }

func (r row) text(i int) string {
	p := C.sqlite3_column_text(r.s, C.int(i))
	if p == nil {
		return ""
	}
	return C.GoStringN((*C.char)(unsafe.Pointer(p)), C.sqlite3_column_bytes(r.s, C.int(i)))
}

func (r row) int(i int) int64 { return int64(C.sqlite3_column_int64(r.s, C.int(i))) }

// query runs sql on a copy of the database at path, with its write-ahead log, as the
// browser may have it open, and calls each with every row.
func query(path, sql string, each func(row)) error {
	tmp, err := os.MkdirTemp("", "browserdata-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	copyPath := filepath.Join(tmp, filepath.Base(path))
	if err := copyFile(path, copyPath); err != nil {
		return err
	}
	if err := copyFile(path+"-wal", copyPath+"-wal"); err != nil && !os.IsNotExist(err) {
		return err
	}
	cpath := C.CString(copyPath)
	defer C.free(unsafe.Pointer(cpath))
	var db *C.sqlite3
	if rc := C.sqlite3_open_v2(cpath, &db, C.SQLITE_OPEN_READWRITE, nil); rc != C.SQLITE_OK {
		C.sqlite3_close(db)
		return errors.New("browserdata: sqlite: can't open " + path)
	}
	defer C.sqlite3_close(db)
	csql := C.CString(sql)
	defer C.free(unsafe.Pointer(csql))
	var s *C.sqlite3_stmt
	if C.sqlite3_prepare_v2(db, csql, -1, &s, nil) != C.SQLITE_OK {
		return errors.New("browserdata: sqlite: " + C.GoString(C.sqlite3_errmsg(db)) + " in " + path)
	}
	defer C.sqlite3_finalize(s)
	for {
		switch C.sqlite3_step(s) {
		case C.SQLITE_ROW:
			each(row{s})
		case C.SQLITE_DONE:
			return nil
		default:
			return errors.New("browserdata: sqlite: " + C.GoString(C.sqlite3_errmsg(db)) + " in " + path)
		}
	}
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
