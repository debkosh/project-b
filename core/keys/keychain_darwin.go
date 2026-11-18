package keys

/*
#cgo CFLAGS: -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <stdlib.h>
#include <string.h>
#include <Security/Security.h>

// query is the generic password of service and account. With no_ui, a call that would ask the
// person fails instead.
static CFMutableDictionaryRef query(const char* service, const char* account, int no_ui) {
	CFMutableDictionaryRef q = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFStringRef s = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef a = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
	CFDictionarySetValue(q, kSecClass, kSecClassGenericPassword);
	CFDictionarySetValue(q, kSecAttrService, s);
	CFDictionarySetValue(q, kSecAttrAccount, a);
	if (no_ui) {
		CFDictionarySetValue(q, kSecUseAuthenticationUI, kSecUseAuthenticationUIFail);
	}
	CFRelease(s);
	CFRelease(a);
	return q;
}

// keychain_read copies the item's data, up to *len bytes, into out, and sets *len.
static OSStatus keychain_read(const char* service, const char* account, int no_ui, unsigned char* out, int* len) {
	CFMutableDictionaryRef q = query(service, account, no_ui);
	CFDictionarySetValue(q, kSecReturnData, kCFBooleanTrue);
	CFDictionarySetValue(q, kSecMatchLimit, kSecMatchLimitOne);
	CFTypeRef data = NULL;
	OSStatus st = SecItemCopyMatching(q, &data);
	CFRelease(q);
	if (st != errSecSuccess) {
		return st;
	}
	CFIndex n = CFDataGetLength((CFDataRef)data);
	if (n > *len) {
		st = errSecDecode;
	} else {
		memcpy(out, CFDataGetBytePtr((CFDataRef)data), n);
		*len = (int)n;
	}
	CFRelease(data);
	return st;
}

static OSStatus keychain_add(const char* service, const char* account, const char* label, int no_ui, const unsigned char* b, int len) {
	CFMutableDictionaryRef q = query(service, account, no_ui);
	CFDataRef data = CFDataCreate(NULL, b, len);
	CFStringRef l = CFStringCreateWithCString(NULL, label, kCFStringEncodingUTF8);
	CFDictionarySetValue(q, kSecValueData, data);
	CFDictionarySetValue(q, kSecAttrLabel, l);
	OSStatus st = SecItemAdd(q, NULL);
	CFRelease(data);
	CFRelease(l);
	CFRelease(q);
	return st;
}

static OSStatus keychain_delete(const char* service, const char* account, int no_ui) {
	CFMutableDictionaryRef q = query(service, account, no_ui);
	OSStatus st = SecItemDelete(q);
	CFRelease(q);
	return st;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// keychainAccount is the device key's account in its service.
const keychainAccount = "device key"

// keychainPrompts says whether a keychain call may ask the person, as the app's may when the
// keychain asks to allow a new build. A test turns it off: such a call fails instead.
var keychainPrompts = true

// Keychain is the device key in the login keychain: an ordinary generic password of service,
// account "device key", which any app the person allows can read (docs/sync.md). One is made,
// 32 random bytes, only when create is true and there is none.
func Keychain(service string) Device {
	return func(create bool) ([]byte, error) {
		b, err := keychainRead(service)
		if err != ErrNoDeviceKey || !create {
			return b, err
		}
		b = random32()
		if err := keychainCall("add", service, func(s, a *C.char, noUI C.int) C.OSStatus {
			label := C.CString(service + " " + keychainAccount)
			defer C.free(unsafe.Pointer(label))
			return C.keychain_add(s, a, label, noUI, (*C.uchar)(unsafe.Pointer(&b[0])), C.int(len(b)))
		}); err != nil {
			return nil, err
		}
		return b, nil
	}
}

func keychainRead(service string) ([]byte, error) {
	out := make([]byte, 64)
	n := C.int(len(out))
	err := keychainCall("read", service, func(s, a *C.char, noUI C.int) C.OSStatus {
		return C.keychain_read(s, a, noUI, (*C.uchar)(unsafe.Pointer(&out[0])), &n)
	})
	if err != nil {
		return nil, err
	}
	if n != 32 {
		return nil, fmt.Errorf("keys: the keychain's %s %s is %d bytes", service, keychainAccount, n)
	}
	return out[:n], nil
}

// deleteKeychain deletes the device key of service, for a test to leave nothing behind.
func deleteKeychain(service string) error {
	return keychainCall("delete", service, func(s, a *C.char, noUI C.int) C.OSStatus {
		return C.keychain_delete(s, a, noUI)
	})
}

// keychainCall makes one call with the service and account as C strings: ErrNoDeviceKey for
// an item that isn't there.
func keychainCall(what, service string, call func(s, a *C.char, noUI C.int) C.OSStatus) error {
	s, a := C.CString(service), C.CString(keychainAccount)
	defer C.free(unsafe.Pointer(s))
	defer C.free(unsafe.Pointer(a))
	noUI := C.int(0)
	if !keychainPrompts {
		noUI = 1
	}
	switch st := call(s, a, noUI); st {
	case C.errSecSuccess:
		return nil
	case C.errSecItemNotFound:
		return ErrNoDeviceKey
	default:
		return fmt.Errorf("keys: the keychain's %s %s, %s: OSStatus %d", service, keychainAccount, what, int(st))
	}
}
