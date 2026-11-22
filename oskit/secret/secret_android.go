package secret

/*
#include <stdlib.h>
#include "secret_android.h"
*/
import "C"

import (
	"context"
	"errors"
	"unsafe"

	"github.com/sudiptadeb/oskit/internal/jni"
)

func init() {
	jni.OnAttach(func(env unsafe.Pointer) { C.oskit_secret_register((*C.JNIEnv)(env)) })
}

// android is the device key in the app's files, sealed by the Android Keystore (Secret.java).
// It never asks the person, so ctx isn't waited on.
type android struct{}

// New is this OS's Store.
func New() Store { return android{} }

func (android) DeviceKey(_ context.Context, name string, create bool) ([]byte, error) {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))
	out := make([]byte, 32)
	c := C.int(0)
	if create {
		c = 1
	}
	got := C.int(-1)
	if !jni.Do(func(env unsafe.Pointer) {
		got = C.oskit_secret_get((*C.JNIEnv)(env), n, c, (*C.uint8_t)(unsafe.Pointer(&out[0])))
	}) {
		return nil, errors.New("secret: oskit isn't attached to Java")
	}
	switch got {
	case 1:
		return out, nil
	case 0:
		return nil, ErrNone
	default:
		return nil, errors.New("secret: the Android Keystore's device key failed; adb logcat has why")
	}
}
