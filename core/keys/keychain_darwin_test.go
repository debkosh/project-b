package keys

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

// The device key in the login keychain, under a service of the test's own, which it deletes
// after. Where no keychain can be used without asking, it is skipped.
func TestKeychain(t *testing.T) {
	keychainPrompts = false
	defer func() { keychainPrompts = true }()
	b := make([]byte, 6)
	rand.Read(b)
	service := "Project B test " + hex.EncodeToString(b)
	device := Keychain(service)
	if _, err := device(false); err != ErrNoDeviceKey {
		t.Skipf("no keychain to use: %v", err)
	}
	made, err := device(true)
	if err != nil {
		t.Skipf("no keychain to write to: %v", err)
	}
	t.Cleanup(func() {
		if err := deleteKeychain(service); err != nil {
			t.Errorf("deleting %s: %v", service, err)
		}
	})
	got, err := device(false)
	if err != nil || !bytes.Equal(got, made) || len(made) != 32 {
		t.Fatalf("read back %x (%v), made %x", got, err, made)
	}
	if again, _ := device(true); !bytes.Equal(again, made) {
		t.Fatal("a second key was made over the first")
	}
}
