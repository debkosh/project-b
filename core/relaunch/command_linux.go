//go:build linux && !android

package relaunch

import "os"

// sandboxed is whether this copy runs in the Flatpak, whose sandbox is one PID namespace:
// when the process that started it ends, every other process in it is killed, so a copy
// started from inside would end with the one that started it.
func sandboxed() bool { return os.Getenv("FLATPAK_ID") != "" }

// command is the command line that starts a copy of this program with args. In the Flatpak
// it is flatpak-spawn, which asks the Flatpak portal to start the launcher in a sandbox of its
// own, as a start from the desktop has; it returns at once and the copy goes on alone.
func command(args []string) ([]string, error) {
	if sandboxed() {
		return append([]string{"flatpak-spawn", launcher}, args...), nil
	}
	exe, err := os.Executable()
	return append([]string{exe}, args...), err
}
