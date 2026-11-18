//go:build darwin && cgo

package browserdata

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#include <stdlib.h>

// keychain_secret reads the generic password service/account from the login keychain,
// which asks the person first unless they let this app always. The caller frees *out.
static OSStatus keychain_secret(const char* service, const char* account, void** out, int* len) {
	CFStringRef s = CFStringCreateWithCString(NULL, service, kCFStringEncodingUTF8);
	CFStringRef a = CFStringCreateWithCString(NULL, account, kCFStringEncodingUTF8);
	const void* keys[] = {kSecClass, kSecAttrService, kSecAttrAccount, kSecReturnData, kSecMatchLimit};
	const void* values[] = {kSecClassGenericPassword, s, a, kCFBooleanTrue, kSecMatchLimitOne};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, values, 5, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(s);
	CFRelease(a);
	if (status != errSecSuccess) {
		return status;
	}
	CFDataRef data = (CFDataRef)result;
	*len = (int)CFDataGetLength(data);
	*out = malloc(*len);
	CFDataGetBytes(data, CFRangeMake(0, *len), *out);
	CFRelease(result);
	return errSecSuccess;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// KeychainSecret reads a generic password from the login keychain: service is a Chromium
// browser's "<name> Safe Storage", account its name. macOS asks the person first, naming
// this app; one refused, or none kept, is ErrNoSecret.
func KeychainSecret(service, account string) ([]byte, error) {
	cs, ca := C.CString(service), C.CString(account)
	defer C.free(unsafe.Pointer(cs))
	defer C.free(unsafe.Pointer(ca))
	var out unsafe.Pointer
	var n C.int
	switch status := C.keychain_secret(cs, ca, &out, &n); status {
	case C.errSecSuccess:
		defer C.free(out)
		return C.GoBytes(out, n), nil
	case C.errSecItemNotFound, C.errSecUserCanceled, C.errSecAuthFailed:
		return nil, ErrNoSecret
	default:
		return nil, fmt.Errorf("browserdata: the keychain answered %d", int(status))
	}
}
