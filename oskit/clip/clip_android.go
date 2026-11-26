package clip

/*
#include <stdlib.h>
#include "clip_android.h"
*/
import "C"

import (
	"unsafe"

	"github.com/debkosh/project-b/oskit/internal/jni"
)

func init() {
	jni.OnAttach(func(env unsafe.Pointer) { C.oskit_clip_register((*C.JNIEnv)(env)) })
}

type android struct{}

// New is this OS's Clipboard.
func New() Clipboard { return android{} }

func (android) CopySecret(text string) {
	c := C.CString(text)
	defer C.free(unsafe.Pointer(c))
	jni.Do(func(env unsafe.Pointer) { C.oskit_clip_copy_secret((*C.JNIEnv)(env), c) })
}
