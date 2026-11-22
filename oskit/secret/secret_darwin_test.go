package secret

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

// The device key in the login keychain, under a service of the test's own, which it deletes
// after. Where no keychain can be used without asking, it is skipped.
func TestKeychain(t *testing.T) {
	prompts = false
	defer func() { prompts = true }()
	b := make([]byte, 6)
	rand.Read(b)
	service := "oskit test " + hex.EncodeToString(b)
	ctx := context.Background()
	s := New()
	if _, err := s.DeviceKey(ctx, service, false); err != ErrNone {
		t.Skipf("no keychain to use: %v", err)
	}
	made, err := s.DeviceKey(ctx, service, true)
	if err != nil {
		t.Skipf("no keychain to write to: %v", err)
	}
	t.Cleanup(func() {
		if err := remove(service); err != nil {
			t.Errorf("deleting %s: %v", service, err)
		}
	})
	got, err := s.DeviceKey(ctx, service, false)
	if err != nil || !bytes.Equal(got, made) || len(made) != 32 {
		t.Fatalf("read back %x (%v), made %x", got, err, made)
	}
	if again, _ := s.DeviceKey(ctx, service, true); !bytes.Equal(again, made) {
		t.Fatal("a second key was made over the first")
	}
}
