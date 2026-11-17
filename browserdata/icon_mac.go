//go:build darwin && cgo

package browserdata

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

// app_icon draws the icon macOS shows for the app at path at px pixels square, as a PNG.
static void *app_icon(const char *path, int px, int *len) {
	@autoreleasepool {
		NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:[NSString stringWithUTF8String:path]];
		NSRect rect = NSMakeRect(0, 0, px, px);
		CGImageRef image = [icon CGImageForProposedRect:&rect context:nil hints:nil];
		if (!image) return NULL;
		NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:image];
		[rep setSize:NSMakeSize(px, px)];
		NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
		if (!png) return NULL;
		*len = (int)png.length;
		void *out = malloc(png.length);
		memcpy(out, png.bytes, png.length);
		return out;
	}
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Icon is the icon macOS shows for the app at path, px pixels square, as a PNG: the
// browser's own logo, read from the app installed, so none of it ships with the caller.
func Icon(path string, px int) ([]byte, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var n C.int
	p := C.app_icon(cpath, C.int(px), &n)
	if p == nil {
		return nil, errors.New("browserdata: no icon for " + path)
	}
	defer C.free(p)
	return C.GoBytes(p, n), nil
}
