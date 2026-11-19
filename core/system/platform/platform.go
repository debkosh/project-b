// Package platform is system.System on the OS the core runs on: the Mac's in Objective-C beside
// the Go, Android's through Java, which go_table.c reaches over JNI (docs/layers.md). Only the
// core's library, core/cmd/lib, links it; everything else in core/ sees system.System.
package platform

/*
#include <stdint.h>
*/
import "C"

import (
	"strings"

	"projectb/system"
)

// The questions the OS answers later, by the numbers that cross with the answer.
const (
	questionDefaultBrowser = 1
	questionNotifications  = 2
)

// System is this OS's system.System. There is one, as there is one OS: New returns it.
type System struct {
	defaultBrowser system.Question
	notifications  system.Question
	// clicked is OnNotificationClicked's, and opened OnOpenURLs', set once at start. Main
	// thread.
	clicked func(id string)
	opened  func(urls []string)
}

var the = &System{}

var _ system.System = the

// New is this OS's system.System.
func New() *System { return the }

func (s *System) OnNotificationClicked(clicked func(id string)) { s.clicked = clicked }

// PBSystemNotificationClicked is a click on a notification ShowNotification showed, on the
// main thread.
//
//export PBSystemNotificationClicked
func PBSystemNotificationClicked(id *C.char) {
	if the.clicked != nil {
		the.clicked(C.GoString(id))
	}
}

// PBSystemOpenURLs is the links and files the OS hands the app, one per line, on the main
// thread.
//
//export PBSystemOpenURLs
func PBSystemOpenURLs(urls *C.char) {
	if the.opened != nil {
		the.opened(strings.Split(C.GoString(urls), "\n"))
	}
}

// PBSystemAnswered is the OS answering a question the core asked, on the main thread:
// whether the app is the default browser now, or may show its notifications.
//
//export PBSystemAnswered
func PBSystemAnswered(question C.int32_t, yes C.int) {
	switch question {
	case questionDefaultBrowser:
		the.defaultBrowser.Answer(yes != 0)
	case questionNotifications:
		the.notifications.Answer(yes != 0)
	}
}
