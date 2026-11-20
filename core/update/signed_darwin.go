//go:build darwin

package update

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <stdlib.h>
#include <string.h>

// pb_check_code checks the code at path as codesign --verify --deep --strict does, and,
// with a requirement, that it meets it. It answers the status, and the framework's words
// for a failure in msg. verdict says the status is the signature's own answer: the code is
// unsigned, altered, or signed by someone else. Any other failure is a check that couldn't
// be made, such as a file that couldn't be read.
static OSStatus pb_check_code(const char *path, const char *requirement, char *msg, size_t n, int *verdict) {
	*verdict = 0;
	msg[0] = 0;
	CFURLRef url = CFURLCreateFromFileSystemRepresentation(NULL, (const UInt8 *)path, strlen(path), true);
	if (url == NULL) return errSecParam;
	SecStaticCodeRef code = NULL;
	OSStatus s = SecStaticCodeCreateWithPath(url, kSecCSDefaultFlags, &code);
	CFRelease(url);
	if (s != errSecSuccess) return s;
	SecRequirementRef req = NULL;
	if (requirement != NULL) {
		CFStringRef text = CFStringCreateWithCString(NULL, requirement, kCFStringEncodingUTF8);
		s = SecRequirementCreateWithString(text, kSecCSDefaultFlags, &req);
		CFRelease(text);
		if (s != errSecSuccess) { CFRelease(code); return s; }
	}
	CFErrorRef err = NULL;
	s = SecStaticCodeCheckValidityWithErrors(code,
		kSecCSCheckAllArchitectures | kSecCSStrictValidate | kSecCSCheckNestedCode, req, &err);
	if (err != NULL) {
		CFStringRef d = CFErrorCopyDescription(err);
		if (d != NULL) { CFStringGetCString(d, msg, n, kCFStringEncodingUTF8); CFRelease(d); }
		CFRelease(err);
	}
	if (req != NULL) CFRelease(req);
	CFRelease(code);
	switch (s) {
	case errSecCSUnsigned:
	case errSecCSSignatureFailed:
	case errSecCSSignatureInvalid:
	case errSecCSSignatureNotVerifiable:
	case errSecCSSignatureUnsupported:
	case errSecCSReqFailed:
	case errSecCSBadResource:
	case errSecCSResourcesNotFound:
	case errSecCSResourcesNotSealed:
	case errSecCSResourcesInvalid:
	case errSecCSResourceRulesInvalid:
	case errSecCSBadObjectFormat:
	case errSecCSInfoPlistFailed:
	case errSecCSNoMainExecutable:
	case errSecCSBadBundleFormat:
	case errSecCSBadNestedCode:
	case errSecCSUnsealedAppRoot:
	case errSecCSUnsealedFrameworkRoot:
	case errSecCSInvalidSymlink:
	case errSecCSWeakResourceRules:
	case errSecCSWeakResourceEnvelope:
	case errSecCSStaticCodeChanged:
		*verdict = 1;
	}
	return s;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// SignedBy returns a check that a bundle's signature is intact and was made with the key
// whose certificate has this SHA-1: the two checks tools/package.sh makes before it ships
// one. It asks macOS's Security framework in this process rather than running codesign,
// which inherited the app's open files under Go's original limit for children, 256, and
// failed for want of more. A refusal wraps errRefused; a check that couldn't be made
// doesn't, and the release is tried again.
func SignedBy(leafSHA1 string) func(app string) error {
	return func(app string) error {
		return checkSigned(app, fmt.Sprintf(`certificate leaf = H"%s"`, leafSHA1))
	}
}

func checkSigned(app, requirement string) error {
	path := C.CString(app)
	defer C.free(unsafe.Pointer(path))
	var req *C.char
	if requirement != "" {
		req = C.CString(requirement)
		defer C.free(unsafe.Pointer(req))
	}
	msg := make([]byte, 512)
	var verdict C.int
	s := C.pb_check_code(path, req, (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)), &verdict)
	if s == 0 {
		return nil
	}
	words := C.GoString((*C.char)(unsafe.Pointer(&msg[0])))
	if words == "" {
		words = fmt.Sprintf("status %d", int(s))
	}
	switch {
	case verdict == 0:
		return fmt.Errorf("its signature couldn't be checked (%s); Try again, or it's tried at the next check", words)
	case s == C.errSecCSReqFailed:
		return fmt.Errorf("it isn't signed with Project B's key, %w", errRefused)
	default:
		return fmt.Errorf("its signature is broken (%s), %w", words, errRefused)
	}
}
