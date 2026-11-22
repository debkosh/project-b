// Package jni is oskit's way to Java on Android. The app's own JNI code calls OskitAttach once,
// from a Java thread, before Go asks anything of oskit: each package then finds its Java class
// and registers its native methods there, with the app's class loader.
package jni

/*
#include "jni_android.h"
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"
)

var (
	mu    sync.Mutex
	vm    *C.JavaVM
	hooks []func(env unsafe.Pointer)
)

// OnAttach has hook run with a Java thread's JNIEnv when the app attaches oskit: a package
// finds its class and registers its natives there. Call it from init.
func OnAttach(hook func(env unsafe.Pointer)) {
	mu.Lock()
	defer mu.Unlock()
	hooks = append(hooks, hook)
}

// OskitAttach is the app attaching oskit, with the JNIEnv of the Java thread it calls on.
//
//export OskitAttach
func OskitAttach(env unsafe.Pointer) {
	mu.Lock()
	vm = C.oskit_vm((*C.JNIEnv)(env))
	run := hooks
	mu.Unlock()
	for _, hook := range run {
		hook(env)
	}
}

// Do runs call with this thread's JNIEnv, attaching it to Java for the call when it isn't. It
// says false, and doesn't call, before OskitAttach or when the thread can't attach.
func Do(call func(env unsafe.Pointer)) bool {
	mu.Lock()
	v := vm
	mu.Unlock()
	if v == nil {
		return false
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var attached C.int
	env := C.oskit_env(v, &attached)
	if env == nil {
		return false
	}
	call(unsafe.Pointer(env))
	C.oskit_done(v, env, attached)
	return true
}
