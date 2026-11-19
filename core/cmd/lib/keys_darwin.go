package main

import (
	"path/filepath"

	"projectb/keys"
	"projectb/profiles"
)

// deviceKeyFor is the device key of the profile c (docs/sync.md): for a listed profile,
// device.key in root, beside the profile list, one for every profile there; none for a folder
// named outright, which keeps its own key and never reads the app's.
func deviceKeyFor(c profiles.Choice, root string) keys.Device {
	if c.ID == "" {
		return nil
	}
	return keys.File(filepath.Join(root, "device.key"))
}
