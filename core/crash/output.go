//go:build !js

package crash

import (
	"os"
	"runtime/debug"
)

// setCrashOutput has the runtime write a crash's report to f as well as to standard error.
func setCrashOutput(f *os.File) error { return debug.SetCrashOutput(f, debug.CrashOptions{}) }
