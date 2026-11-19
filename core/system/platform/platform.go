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
	// checks are the checks of the person the OS has not answered, by the number that
	// crosses with its answer, and lastCheck the last number given. Main thread.
	checks    map[int32]func(system.Verdict)
	lastCheck int32
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

// VerifyPerson asks the OS to check the person, as verifyPerson does it on this OS.
func (s *System) VerifyPerson(check system.PersonCheck, answered func(system.Verdict)) (cancel func()) {
	if s.checks == nil {
		s.checks = map[int32]func(system.Verdict){}
	}
	s.lastCheck++
	id := s.lastCheck
	s.checks[id] = answered
	s.verifyPerson(id, check)
	return func() {
		if _, waiting := s.checks[id]; waiting {
			delete(s.checks, id)
			s.cancelPerson(id)
		}
	}
}

// PBSystemPersonVerified is the OS saying how check id of the person ended, on the main
// thread: 0 verified, 1 not, 2 no screen lock. One taken back meanwhile says nothing.
//
//export PBSystemPersonVerified
func PBSystemPersonVerified(id C.int32_t, verdict C.int) {
	if answered := the.checks[int32(id)]; answered != nil {
		delete(the.checks, int32(id))
		answered(system.Verdict(verdict))
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
