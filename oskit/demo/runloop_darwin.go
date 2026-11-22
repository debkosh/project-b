package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>
static void run_app(void) {
  [NSApplication sharedApplication];
  [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  [NSApp run];
}
*/
import "C"

import "runtime"

func init() { runtime.LockOSThread() }

// runMain runs the demo off the main thread, which AppKit's run loop keeps: oskit's answers
// come there.
func runMain(demo func()) {
	go demo()
	C.run_app()
}
