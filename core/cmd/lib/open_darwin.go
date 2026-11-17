package main

import "C"

import "strings"

// PBOpenURLs opens the links and files macOS hands the app, one per line, as its default
// browser or a file opened with it: as a second copy's are, beside the window used last.
// A picker copy has no window to open them in. The main thread calls it, after PBStart.
//
//export PBOpenURLs
func PBOpenURLs(urls *C.char) {
	if theApp == nil {
		return
	}
	theApp.Handover(strings.Split(C.GoString(urls), "\n"))
}
