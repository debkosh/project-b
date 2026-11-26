//go:build !js && !linux

package relaunch

import "os"

// sandboxed is the Flatpak's, on Linux.
func sandboxed() bool { return false }

// command is the command line that starts a copy of this program with args.
func command(args []string) ([]string, error) {
	exe, err := os.Executable()
	return append([]string{exe}, args...), err
}
