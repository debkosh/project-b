package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include "pb_abi.h"

static inline void call_choose_folder(const PBEngineCalls* c, int64_t window) { c->choose_folder(window); }
*/
import "C"

import "projectb/engine"

// ChooseFolder asks for a folder in the system's panel on the window; PB_EVENT_FOLDER_CHOSEN
// says which.
func (e *nativeEngine) ChooseFolder(w engine.WindowID) { C.call_choose_folder(e.calls, C.int64_t(w)) }
