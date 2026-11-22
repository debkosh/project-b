//go:build !android

package main

import (
	"projectb/keys"
	"projectb/profiles"
)

// deviceKeyFor is the device key of the profile c: the OS's for a listed profile, none for a
// folder named outright (--user-data-dir, as the tests' are), whose profile key its own
// folder.key wraps, so it never touches the keychain or the Secret portal.
func deviceKeyFor(c profiles.Choice) keys.Device {
	if c.ID == "" {
		return nil
	}
	return deviceKey
}
