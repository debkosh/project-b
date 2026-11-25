package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include "pb_abi.h"

static inline int call_exit_reason(const PBEngineCalls* c, long long pid) { return c->exit_reason(pid); }
*/
import "C"

// exitReasons are ApplicationExitInfo's REASON_ numbers, in words, and ExitReasons.java's
// own -2: Android kept no reason, and its crash log holds no crash of the process.
var exitReasons = map[int]string{
	-2: "no crash logged", 0: "unknown", 1: "exited", 2: "signaled", 3: "low memory", 4: "crash", 5: "native crash",
	6: "not responding", 7: "failed to start", 8: "permission change", 9: "excessive resource use",
	10: "user requested", 11: "user stopped", 12: "dependency died", 13: "other", 14: "frozen",
	15: "package state change", 16: "package updated",
}

// ExitReason asks Android why the process ended (pb_abi.h exit_reason).
func (e *nativeEngine) ExitReason(pid int) string {
	r := int(C.call_exit_reason(e.calls, C.longlong(pid)))
	if r == -1 {
		return ""
	}
	if w, ok := exitReasons[r]; ok {
		return w
	}
	return "other"
}
