package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_abi.h"

static inline int call_delete_download(const PBEngineCalls* c, const char* path) { return c->delete_download(path); }
*/
import "C"

import "unsafe"

// DeleteDownload deletes a download's copy in the phone's shared Downloads (pb_abi.h
// delete_download).
func (e *nativeEngine) DeleteDownload(path string) bool {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return C.call_delete_download(e.calls, p) != 0
}
