package main

/*
#include <stdint.h>

typedef int (*device_key_reader)(int create, uint8_t* out);

static inline int call_device_key_reader(device_key_reader read, int create, uint8_t* out) { return read(create, out); }
*/
import "C"

import (
	"errors"
	"sync/atomic"
	"unsafe"

	"projectb/keys"
	"projectb/profiles"
)

// deviceKeyReader is go_table.c's way to Java's DeviceKey.get, set as the library is handed
// to Java, before Go starts.
var deviceKeyReader atomic.Pointer[byte]

// PBSetDeviceKeyReader is go_table.c giving Go its device key reader.
//
//export PBSetDeviceKeyReader
func PBSetDeviceKeyReader(read C.device_key_reader) {
	deviceKeyReader.Store((*byte)(unsafe.Pointer(read)))
}

// deviceKeyFor is the device key of the phone's one profile, the folder Android gives the app
// (docs/sync.md): Java keeps it in the app's files, wrapped by the Android Keystore.
func deviceKeyFor(profiles.Choice, string) keys.Device {
	return func(create bool) ([]byte, error) {
		read := C.device_key_reader(unsafe.Pointer(deviceKeyReader.Load()))
		if read == nil {
			return nil, errors.New("keys: no way to Java's device key")
		}
		out := make([]byte, 32)
		c := C.int(0)
		if create {
			c = 1
		}
		switch C.call_device_key_reader(read, c, (*C.uint8_t)(unsafe.Pointer(&out[0]))) {
		case 1:
			return out, nil
		case 0:
			return nil, keys.ErrNoDeviceKey
		default:
			return nil, errors.New("keys: the Android Keystore's device key failed; adb logcat has why")
		}
	}
}
