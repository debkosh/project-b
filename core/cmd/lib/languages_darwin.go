package main

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <stdlib.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>

// pb_languages is the person's languages, in their order, comma-separated: malloc'd.
static char* pb_languages(void) {
  CFArrayRef list = CFLocaleCopyPreferredLanguages();
  CFIndex n = list ? CFArrayGetCount(list) : 0;
  size_t size = 1 + (size_t)n * 64;
  char* out = calloc(1, size);
  for (CFIndex i = 0; i < n; i++) {
    char tag[60];
    if (CFStringGetCString((CFStringRef)CFArrayGetValueAtIndex(list, i), tag, sizeof tag, kCFStringEncodingUTF8)) {
      if (out[0]) strlcat(out, ",", size);
      strlcat(out, tag, size);
    }
  }
  if (list) CFRelease(list);
  return out;
}
*/
import "C"

import (
	"strings"
	"unsafe"
)

// languages are the languages the person chose in macOS, which turn a regional block list
// on by default (docs/blocking.md).
func languages() []string {
	c := C.pb_languages()
	defer C.free(unsafe.Pointer(c))
	return strings.Split(C.GoString(c), ",")
}
