//go:build linux && !android

package main

/*
#include "pb_abi.h"
static inline void call_set_system_dark(const PBEngineCalls* c, int dark) { c->set_system_dark(dark); }
*/
import "C"

import "projectb/system/platform"

// followDark has the engine follow the desktop's light or dark (pb_abi.h set_system_dark), from
// the Settings portal, on the main thread.
func followDark() {
	platform.New().WatchDark(func(dark bool) {
		d := 0
		if dark {
			d = 1
		}
		native.Post(func() { C.call_set_system_dark(native.calls, C.int(d)) })
	})
}
