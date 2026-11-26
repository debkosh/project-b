package media

/*
#include "media_android.h"
*/
import "C"

import (
	"slices"
	"unsafe"

	"github.com/debkosh/project-b/oskit/internal/jni"
)

// android is Android's runtime permissions, CAMERA and RECORD_AUDIO, through oskit's
// Media.java: Android asks the person, and answers no at once for one refused for good.
type android struct {
	asking  asking
	returns returns
}

var the = &android{}

func init() {
	jni.OnAttach(func(env unsafe.Pointer) { C.oskit_media_register((*C.JNIEnv)(env)) })
}

// New is this OS's Media.
func New() Media { return the }

func code(kind string) C.int {
	if kind == Microphone {
		return 1
	}
	return 0
}

func (a *android) Allowed(kind string) bool {
	yes := false
	jni.Do(func(env unsafe.Pointer) { yes = C.oskit_media_allowed((*C.JNIEnv)(env), code(kind)) != 0 })
	return yes
}

func (a *android) Ask(kinds []string, answered func(map[string]bool)) {
	camera, microphone := C.int(0), C.int(0)
	if slices.Contains(kinds, Camera) {
		camera = 1
	}
	if slices.Contains(kinds, Microphone) {
		microphone = 1
	}
	a.asking.ask(answered, func() {
		if !jni.Do(func(env unsafe.Pointer) { C.oskit_media_ask((*C.JNIEnv)(env), camera, microphone) }) {
			a.asking.answer(map[string]bool{})
		}
	})
}

// OpenSettings opens the app's details in Android's settings, where both kinds are.
func (a *android) OpenSettings(string) {
	jni.Do(func(env unsafe.Pointer) { C.oskit_media_open_settings((*C.JNIEnv)(env)) })
}

func (a *android) OnReturn(returned func()) { a.returns.set(returned) }

// oskitMediaAnswered is Android's answer, both kinds as they stand now, on the main thread.
//
//export oskitMediaAnswered
func oskitMediaAnswered(camera, microphone C.int) {
	the.asking.answer(map[string]bool{Camera: camera != 0, Microphone: microphone != 0})
}

// oskitMediaReturned is the activity resuming, on the main thread.
//
//export oskitMediaReturned
func oskitMediaReturned() { the.returns.fire() }
