//go:build linux && !android

package main

import (
	"errors"

	"github.com/debkosh/project-b/oskit/secret"
)

// noKeyring says err is the desktop's keyring giving no key, which the pages have a line for
// (keys.ErrKeyring).
func noKeyring(err error) bool { return errors.Is(err, secret.ErrNoPortal) }
