package notify

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdlib.h>
#include "notify_darwin.h"
*/
import "C"

import (
	"unsafe"

	"github.com/sudiptadeb/oskit"
)

// mac is the Mac's notifications. A Mac's are the app's own to show: macOS asks the person
// the first time one is shown.
type mac struct {
	asking oskit.Question
	clicks clicks
}

var the = &mac{}

// New is this OS's Notifier.
func New() Notifier { return the }

func (m *mac) Allowed() bool { return true }

func (m *mac) Ask(answered func(bool)) { m.asking.Ask(answered, func() { C.oskit_notify_answer() }) }

func (m *mac) Show(n Note) {
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
		for i, a := range n.Actions {
			idv[i], labelv[i] = cs(a.ID), cs(a.Label)
		}
	}
	C.oskit_notify_show(cs(n.ID), cs(n.Title), cs(n.Subtitle), cs(n.Body), ids, labels, C.int(len(n.Actions)))
}

func (m *mac) Remove(id string) {
	c := C.CString(id)
	defer C.free(unsafe.Pointer(c))
	C.oskit_notify_remove(c)
}

func (m *mac) OnClick(clicked func(id, action string)) {
	m.clicks.set(clicked)
	C.oskit_notify_listen()
}

// oskitNotifyClicked is a click on a note, on the main thread.
//
//export oskitNotifyClicked
func oskitNotifyClicked(id, action *C.char) {
	the.clicks.click(C.GoString(id), C.GoString(action))
}

// oskitNotifyAnswered is the question whether the app may notify answered, on the main thread.
//
//export oskitNotifyAnswered
func oskitNotifyAnswered(yes C.int) { the.asking.Answer(yes != 0) }
