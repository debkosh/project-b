package clip

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
#include "clip_darwin.h"
*/
import "C"

import "unsafe"

type mac struct{}

// New is this OS's Clipboard.
func New() Clipboard { return mac{} }

func (mac) CopySecret(text string) {
	c := C.CString(text)
	defer C.free(unsafe.Pointer(c))
	C.oskit_clip_copy_secret(c)
}
