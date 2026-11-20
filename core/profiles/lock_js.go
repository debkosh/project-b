//go:build js

package profiles

import "os"

// lockFile has nothing to wait for in a web page, where one copy of the core runs alone.
func lockFile(*os.File) error { return nil }
