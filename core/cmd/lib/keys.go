package main

import (
	"context"
	"errors"

	"github.com/sudiptadeb/oskit/secret"

	"projectb/keys"
)

// deviceKeyName is the device key's name (docs/sync.md): the keychain item's service on the
// Mac, the Keystore alias's on Android (project_b_device_key), the Secret portal key's on
// Linux ("project-b device key").
const deviceKeyName = "Project B"

// deviceKey is the OS's device key (oskit/secret), as keys wants it.
func deviceKey(create bool) ([]byte, error) {
	b, err := secret.New().DeviceKey(context.Background(), deviceKeyName, create)
	if errors.Is(err, secret.ErrNone) {
		return nil, keys.ErrNoDeviceKey
	}
	return b, err
}
