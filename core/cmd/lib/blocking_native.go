//go:build pbblock

package main

import (
	"projectb/blocking"
	pbblock "projectb/blocking/native"
)

// blockingLibrary is libpbblock, which engine/build_go.py builds beside libcore and asks
// for with the pbblock tag.
func blockingLibrary() blocking.Library { return pbblock.Library{} }
