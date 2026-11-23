package media

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework AppKit
#include "media_darwin.h"
*/
import "C"

import "slices"

// mac is macOS's Privacy & Security, through AVCaptureDevice: it asks the person once for each
// kind, then answers from the settings.
type mac struct {
	asking  asking
	returns returns
}

var the = &mac{}

// New is this OS's Media.
func New() Media { return the }

func code(kind string) C.int {
	if kind == Microphone {
		return 1
	}
	return 0
}

func (m *mac) Allowed(kind string) bool { return C.oskit_media_allowed(code(kind)) != 0 }

func (m *mac) Ask(kinds []string, answered func(map[string]bool)) {
	camera, microphone := slices.Contains(kinds, Camera), slices.Contains(kinds, Microphone)
	m.asking.ask(answered, func() {
		yes := func(b bool) C.int {
			if b {
				return 1
			}
			return 0
		}
		C.oskit_media_ask(yes(camera), yes(microphone))
	})
}

func (m *mac) OpenSettings(kind string) { C.oskit_media_open_settings(code(kind)) }

func (m *mac) OnReturn(returned func()) {
	m.returns.set(returned)
	C.oskit_media_watch_return()
}

// oskitMediaAnswered is macOS's answer, both kinds as they stand now, on the main thread.
//
//export oskitMediaAnswered
func oskitMediaAnswered(camera, microphone C.int) {
	the.asking.answer(map[string]bool{Camera: camera != 0, Microphone: microphone != 0})
}

// oskitMediaReturned is the app becoming active, on the main thread.
//
//export oskitMediaReturned
func oskitMediaReturned() { the.returns.fire() }
