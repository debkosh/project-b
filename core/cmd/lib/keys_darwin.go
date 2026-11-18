package main

import (
	"projectb/keys"
	"projectb/profiles"
)

// keychainService is the device key's service in the login keychain (docs/sync.md).
const keychainService = "Project B"

// deviceKeyFor is the device key of the profile c: the keychain's for a listed profile, none
// for a folder named outright, which keeps its own key and never touches the keychain.
func deviceKeyFor(c profiles.Choice) keys.Device {
	if c.ID == "" {
		return nil
	}
	return keys.Keychain(keychainService)
}
