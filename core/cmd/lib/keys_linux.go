//go:build !android

package main

import (
	"projectb/keys"
	"projectb/profiles"
)

// deviceKeyFor is the device key of the profile c, as on the Mac: the desktop's Secret
// portal's for a listed profile, none for a folder named outright (--user-data-dir, as the
// tests' are), whose profile key its own folder.key wraps. A listed profile on a desktop with
// no Secret portal has no key, and sync and saved passwords say so.
func deviceKeyFor(c profiles.Choice) keys.Device {
	if c.ID == "" {
		return nil
	}
	return keys.SecretPortal()
}
