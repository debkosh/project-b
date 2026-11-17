package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include "pb_abi.h"

static inline void call_ask_notifications(const PBEngineCalls* c) { c->ask_notifications(); }
static inline int call_notifications_allowed(const PBEngineCalls* c) { return c->notifications_allowed(); }
*/
import "C"

// AskNotifications asks Android to let the app show its notifications, which asks the
// person; PB_EVENT_NOTIFICATIONS says how it ended.
func (e *nativeEngine) AskNotifications() { C.call_ask_notifications(e.calls) }

func (e *nativeEngine) NotificationsAllowed() bool { return C.call_notifications_allowed(e.calls) != 0 }
