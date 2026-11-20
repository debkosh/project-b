//go:build js

package crash

import "os"

// setCrashOutput keeps nothing in a web page, where the core runs as WebAssembly
// (core/cmd/webdemo): Go's runtime there has no crash output, and a crash shows in the
// page's console.
func setCrashOutput(*os.File) error { return nil }
