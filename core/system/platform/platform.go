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

	"github.com/sudiptadeb/oskit/notify"

	"projectb/system"
)

// The question the OS answers later, by the number that crosses with the answer.
const questionDefaultBrowser = 1

// System is this OS's system.System. There is one, as there is one OS: New returns it.
type System struct {
	defaultBrowser system.Question
	// notes are the OS's notifications (oskit/notify).
	notes notify.Notifier
	// opened is OnOpenURLs', set once at start. Main thread.
	opened func(urls []string)
	// checks are the checks of the person the OS has not answered, by the number that
	// crosses with its answer, and lastCheck the last number given. Main thread.
	checks    map[int32]func(system.Verdict)
	lastCheck int32
}

var the = &System{notes: notify.New()}

var _ system.System = the

// New is this OS's system.System.
func New() *System { return the }

func (s *System) NotificationsAllowed() bool { return s.notes.Allowed() }

func (s *System) AskNotifications(answered func(allowed bool)) { s.notes.Ask(answered) }

func (s *System) ShowNotification(n system.Notification) {
	s.notes.Show(notify.Note{ID: n.ID, Title: n.Title, Subtitle: n.Subtitle, Body: n.Body})
}

func (s *System) RemoveNotification(id string) { s.notes.Remove(id) }

// OnNotificationClicked takes the clicks on a page's notifications, a click on the note itself;
// on the Mac a press on the update's, which amarit posted, goes to the updater (otherNote).
func (s *System) OnNotificationClicked(clicked func(id string)) {
	s.notes.OnClick(func(id, action string) {
		if otherNote(id, action) {
			return
		}
		if action == "" {
			clicked(id)
		}
	})
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
// whether the app is the default browser now.
//
//export PBSystemAnswered
func PBSystemAnswered(question C.int32_t, yes C.int) {
	switch question {
	case questionDefaultBrowser:
		the.defaultBrowser.Answer(yes != 0)
	}
}
