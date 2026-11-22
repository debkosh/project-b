package main

import (
	"projectb/keys"
	"projectb/profiles"
)

// deviceKeyFor is the device key of the phone's one profile, the folder Android gives the app
// (docs/sync.md), named outright yet the app's own.
func deviceKeyFor(profiles.Choice) keys.Device { return deviceKey }
