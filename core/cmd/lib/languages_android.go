package main

/*
#include <stdlib.h>
#include <sys/system_properties.h>

static int pb_property(const char* name, char* value) { return __system_property_get(name, value); }
*/
import "C"

import (
	"strings"
	"unsafe"
)

// languages are the phone's, from its locale properties: the person's choice, then the
// one it came with. They turn a regional block list on by default (docs/blocking.md).
func languages() []string {
	var out []string
	buf := (*C.char)(C.malloc(C.PROP_VALUE_MAX))
	defer C.free(unsafe.Pointer(buf))
	for _, name := range []string{"persist.sys.locale", "ro.product.locale"} {
		n := C.CString(name)
		got := C.pb_property(n, buf)
		C.free(unsafe.Pointer(n))
		if got > 0 {
			out = append(out, strings.Split(C.GoString(buf), ",")...)
		}
	}
	return out
}
