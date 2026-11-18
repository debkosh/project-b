package main

import (
	"testing"

	"projectb/profiles"
)

// A folder named outright, as every test and driver run's is, gets no device key, so nothing
// of it reaches the keychain; a listed profile gets the keychain's.
func TestAFolderNamedOutrightGetsNoDeviceKey(t *testing.T) {
	if deviceKeyFor(profiles.Named(t.TempDir())) != nil {
		t.Fatal("a folder named outright has a device key")
	}
	if deviceKeyFor(profiles.Choice{Dir: t.TempDir(), ID: "p1"}) == nil {
		t.Fatal("a listed profile has no device key")
	}
}
