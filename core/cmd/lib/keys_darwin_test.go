package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"projectb/keys"
	"projectb/profiles"
)

// A folder named outright, as every test and driver run's is, gets no device key, so it never
// reads the app's device.key; a listed profile gets device.key beside the list, made the first
// time it is asked to make one, readable by this user alone, and the same for every profile.
func TestAFolderNamedOutrightGetsNoDeviceKey(t *testing.T) {
	root := t.TempDir()
	if deviceKeyFor(profiles.Named(t.TempDir()), root) != nil {
		t.Fatal("a folder named outright has a device key")
	}
	device := deviceKeyFor(profiles.Choice{Dir: filepath.Join(root, "p1"), ID: "p1"}, root)
	if device == nil {
		t.Fatal("a listed profile has no device key")
	}
	if _, err := device(false); err != keys.ErrNoDeviceKey {
		t.Fatalf("before one is made: %v", err)
	}
	made, err := device(true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "device.key"))
	if err != nil || !bytes.Equal(b, made) || len(b) != 32 {
		t.Fatalf("device.key holds %x (%v), made %x", b, err, made)
	}
	if fi, _ := os.Stat(filepath.Join(root, "device.key")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("device.key's mode is %v", fi.Mode().Perm())
	}
	other := deviceKeyFor(profiles.Choice{Dir: filepath.Join(root, "p2"), ID: "p2"}, root)
	if again, err := other(true); err != nil || !bytes.Equal(again, made) {
		t.Fatalf("another profile's device key is %x (%v), not the one made", again, err)
	}
}
