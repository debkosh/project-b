package notify

/*
#include <stdlib.h>
#include "notify_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/sudiptadeb/oskit"
	"github.com/sudiptadeb/oskit/internal/jni"
)

// android is Android's notifications, through oskit's Notify.java: Android 13 and later ask
// the person before the app may show one.
type android struct {
	asking oskit.Question
	clicks clicks
}

var the = &android{}

func init() {
	jni.OnAttach(func(env unsafe.Pointer) { C.oskit_notify_register((*C.JNIEnv)(env)) })
}

// New is this OS's Notifier.
func New() Notifier { return the }

func (a *android) Allowed() bool {
	yes := false
	jni.Do(func(env unsafe.Pointer) { yes = C.oskit_notify_allowed((*C.JNIEnv)(env)) != 0 })
	return yes
}

func (a *android) Ask(answered func(bool)) {
	a.asking.Ask(answered, func() {
		if !jni.Do(func(env unsafe.Pointer) { C.oskit_notify_ask((*C.JNIEnv)(env)) }) {
			a.asking.Answer(false)
		}
	})
}

func (a *android) Show(n Note) {
	var free []unsafe.Pointer
	cs := func(s string) *C.char {
		c := C.CString(s)
		free = append(free, unsafe.Pointer(c))
		return c
	}
	defer func() {
		for _, p := range free {
			C.free(p)
		}
	}()
	var ids, labels **C.char
	if len(n.Actions) > 0 {
		size := C.size_t(len(n.Actions)) * C.size_t(unsafe.Sizeof((*C.char)(nil)))
		ids, labels = (**C.char)(C.malloc(size)), (**C.char)(C.malloc(size))
		free = append(free, unsafe.Pointer(ids), unsafe.Pointer(labels))
		idv := unsafe.Slice(ids, len(n.Actions))
		labelv := unsafe.Slice(labels, len(n.Actions))
		for i, act := range n.Actions {
			idv[i], labelv[i] = cs(act.ID), cs(act.Label)
		}
	}
	id, title, subtitle, body := cs(n.ID), cs(n.Title), cs(n.Subtitle), cs(n.Body)
	jni.Do(func(env unsafe.Pointer) {
		C.oskit_notify_show((*C.JNIEnv)(env), id, title, subtitle, body, ids, labels, C.int(len(n.Actions)))
	})
}

func (a *android) Remove(id string) {
	c := C.CString(id)
	defer C.free(unsafe.Pointer(c))
	jni.Do(func(env unsafe.Pointer) { C.oskit_notify_remove((*C.JNIEnv)(env), c) })
}

func (a *android) OnClick(clicked func(id, action string)) { a.clicks.set(clicked) }

// oskitNotifyClicked is a tap on a note or its button, on the main thread.
//
//export oskitNotifyClicked
func oskitNotifyClicked(id, action *C.char) {
	the.clicks.click(C.GoString(id), C.GoString(action))
}

// oskitNotifyAnswered is Android's question about notifications answered, on the main thread.
//
//export oskitNotifyAnswered
func oskitNotifyAnswered(yes C.int) { the.asking.Answer(yes != 0) }
