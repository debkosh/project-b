// Package platform is system.System on the OS the core runs on: the Mac's in Objective-C beside
// the Go, Android's through Java, which go_table.c reaches over JNI (docs/layers.md). Only the
// core's library, core/cmd/lib, links it; everything else in core/ sees system.System.
package platform

/*
#include <stdint.h>
*/
import "C"

import (
	"github.com/sudiptadeb/oskit/media"
	"github.com/sudiptadeb/oskit/notify"
	"github.com/sudiptadeb/oskit/open"

	"projectb/system"
)

// System is this OS's system.System. There is one, as there is one OS: New returns it.
type System struct {
	// notes are the OS's notifications (oskit/notify), and opener its opening of files and
	// links (oskit/open).
	notes  notify.Notifier
	opener open.Opener
	// media is the OS's own camera and microphone permission (oskit/media).
	media media.Media
	// checks are the checks of the person the OS has not answered, by the number that
	// crosses with its answer, and lastCheck the last number given. Main thread.
	checks    map[int32]func(system.Verdict)
	lastCheck int32
	// scans are the scans of a code the OS has not answered, as checks are.
	scans    map[int32]func(string, bool)
	lastScan int32
}

var the = &System{notes: notify.New(), opener: open.New(), media: media.New()}

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

func (s *System) OpenFile(path string)                  { s.opener.OpenFile(path) }
func (s *System) RevealFile(path string)                { s.opener.Reveal(path) }
func (s *System) IsDefaultBrowser() bool                { return s.opener.IsDefaultBrowser() }
func (s *System) AskDefaultBrowser(answered func(bool)) { s.opener.AskDefaultBrowser(answered) }
func (s *System) MediaAllowed(kind string) bool         { return s.media.Allowed(kind) }
func (s *System) AskMedia(kinds []string, answered func(map[string]bool)) {
	s.media.Ask(kinds, answered)
}
func (s *System) OpenMediaSettings(kind string)         { s.media.OpenSettings(kind) }
func (s *System) OnMediaReturn(returned func())         { s.media.OnReturn(returned) }
func (s *System) OnOpenURLs(opened func(urls []string)) { s.opener.OnOpenURLs(opened) }

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

// ScanCode opens the scanner, as scanCode does it on this OS.
func (s *System) ScanCode(prompt string, answered func(text string, ok bool)) {
	if s.scans == nil {
		s.scans = map[int32]func(string, bool){}
	}
	s.lastScan++
	s.scans[s.lastScan] = answered
	s.scanCode(s.lastScan, prompt)
}

// scanned ends scan id with what it read, or with ok false.
func (s *System) scanned(id int32, text string, ok bool) {
	if answered := s.scans[id]; answered != nil {
		delete(s.scans, id)
		answered(text, ok)
	}
}

// PBSystemCodeScanned is the OS saying how scan id ended, on the main thread: the code's
// text, or ok 0 when the person left the scanner or it couldn't open.
//
//export PBSystemCodeScanned
func PBSystemCodeScanned(id C.int32_t, text *C.char, ok C.int) {
	the.scanned(int32(id), C.GoString(text), ok != 0)
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
