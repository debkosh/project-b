// Package secret is the key a device keeps for the app, apart from the app's own files: what
// wraps the app's data so a copy of its folder alone opens nothing.
package secret

import (
	"context"
	"errors"
)

// ErrNone is the answer when there is no device key and none was to be made.
var ErrNone = errors.New("secret: no device key")

// Store is the OS's keeper of the device key.
type Store interface {
	// DeviceKey is the 32-byte key this device keeps for the app under name: the login
	// keychain on the Mac, a file wrapped by the Android Keystore, the Secret portal on Linux.
	// create makes one when there is none; else ErrNone. It may wait on the person (the Mac's
	// keychain, a Linux keyring to unlock, up to 2 minutes), so call it off the main thread.
	// Linux's portal makes its secret on the first call, so create changes nothing there.
	DeviceKey(ctx context.Context, name string, create bool) ([]byte, error)
}
