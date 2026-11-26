package open

/*
#include <stdlib.h>
#include "open_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/debkosh/project-b/oskit"
	"github.com/debkosh/project-b/oskit/internal/jni"
)

// android is Android's, through oskit's Open.java: a content:// address opens in the app the
// person picks, Reveal shows the phone's Downloads, the browser role is the default browser,
// and links come from other apps' intents.
type android struct {
	asking oskit.Question
	links  links
}

var the = &android{}

func init() {
	jni.OnAttach(func(env unsafe.Pointer) { C.oskit_open_register((*C.JNIEnv)(env)) })
}

// New is this OS's Opener.
func New() Opener { return the }

func withPath(path string, call func(env *C.JNIEnv, c *C.char)) {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	jni.Do(func(env unsafe.Pointer) { call((*C.JNIEnv)(env), c) })
}

func (a *android) OpenFile(path string) {
	withPath(path, func(env *C.JNIEnv, c *C.char) { C.oskit_open_file(env, c) })
}

func (a *android) Reveal(path string) {
	withPath(path, func(env *C.JNIEnv, c *C.char) { C.oskit_open_reveal(env, c) })
}

func (a *android) IsDefaultBrowser() bool {
	yes := false
	jni.Do(func(env unsafe.Pointer) { yes = C.oskit_open_is_default_browser((*C.JNIEnv)(env)) != 0 })
	return yes
}

func (a *android) AskDefaultBrowser(answered func(bool)) {
	a.asking.Ask(answered, func() {
		if !jni.Do(func(env unsafe.Pointer) { C.oskit_open_ask_default_browser((*C.JNIEnv)(env)) }) {
			a.asking.Answer(false)
		}
	})
}

func (a *android) OnOpenURLs(opened func(urls []string)) { a.links.set(opened) }

// oskitOpenDefaultBrowser is Android's answer to AskDefaultBrowser, on the main thread.
//
//export oskitOpenDefaultBrowser
func oskitOpenDefaultBrowser(yes C.int) { the.asking.Answer(yes != 0) }

// oskitOpenURLs is a link another app sent, on the main thread.
//
//export oskitOpenURLs
func oskitOpenURLs(url *C.char) { the.links.open([]string{C.GoString(url)}) }
