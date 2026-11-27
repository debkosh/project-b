//go:build !pbblock

package main

import "projectb/blocking"

// blockingLibrary is Go's small one in a build without libpbblock, as go test's is: the app
// is always built with the pbblock tag (engine/build_go.py).
func blockingLibrary() blocking.Library { return blocking.Plain{} }
