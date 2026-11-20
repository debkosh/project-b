//go:build !js

package profiles

import (
	"os"
	"syscall"
)

// lockFile holds f's lock until f closes, waiting for another process's.
func lockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) }
