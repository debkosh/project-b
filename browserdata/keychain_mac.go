//go:build darwin && cgo

package browserdata

/*
#cgo CFLAGS: -Wno-deprecated-declarations
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

// keychain_items lists the login keychain's generic passwords by their attributes alone,
// none's data, which macOS asks the person nothing about; one that would need asking fails
// instead. The caller releases *out.
static OSStatus keychain_items(CFArrayRef* out) {
	const void* keys[] = {kSecClass, kSecReturnAttributes, kSecMatchLimit, kSecUseAuthenticationUI};
	const void* values[] = {kSecClassGenericPassword, kCFBooleanTrue, kSecMatchLimitAll, kSecUseAuthenticationUIFail};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, values, 4, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus status = SecItemCopyMatching(query, &result);
	CFRelease(query);
	if (status == errSecSuccess && CFGetTypeID(result) != CFArrayGetTypeID()) {
		CFRelease(result);
		return errSecDecode;
	}
	*out = (CFArrayRef)result;
	return status;
}

static int items_count(CFArrayRef items) { return (int)CFArrayGetCount(items); }

// item_attr is item i's service (account 0) or account (1) in UTF-8, which the caller
// frees, or NULL when it has none.
static char* item_attr(CFArrayRef items, int i, int account) {
	CFDictionaryRef d = (CFDictionaryRef)CFArrayGetValueAtIndex(items, i);
	CFTypeRef v = CFDictionaryGetValue(d, account ? kSecAttrAccount : kSecAttrService);
	if (v == NULL || CFGetTypeID(v) != CFStringGetTypeID()) {
		return NULL;
	}
	CFStringRef s = (CFStringRef)v;
	CFIndex n = CFStringGetMaximumSizeForEncoding(CFStringGetLength(s), kCFStringEncodingUTF8) + 1;
	char* buf = malloc(n);
	if (!CFStringGetCString(s, buf, n, kCFStringEncodingUTF8)) {
		free(buf);
		return NULL;
	}
	return buf;
}
*/
import "C"

import (
	"fmt"
	"slices"
	"strings"
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

// KeychainItems lists the login keychain's "<name> Safe Storage" items, each Chromium
// browser's that has kept a password, by their service and account alone: listing them
// unlocks nothing, so macOS asks the person nothing.
func KeychainItems() ([]KeychainItem, error) {
	var items C.CFArrayRef
	switch status := C.keychain_items(&items); status {
	case C.errSecSuccess:
	case C.errSecItemNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("browserdata: the keychain's list answered %d", int(status))
	}
	defer C.CFRelease(C.CFTypeRef(items))
	attr := func(i, account int) string {
		p := C.item_attr(items, C.int(i), C.int(account))
		if p == nil {
			return ""
		}
		defer C.free(unsafe.Pointer(p))
		return C.GoString(p)
	}
	var out []KeychainItem
	for i := range int(C.items_count(items)) {
		item := KeychainItem{Service: attr(i, 0), Account: attr(i, 1)}
		if strings.HasSuffix(item.Service, safeStorage) && item.Service != safeStorage && !slices.Contains(out, item) {
			out = append(out, item)
		}
	}
	return out, nil
}
