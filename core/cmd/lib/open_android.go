package main

import "C"

// PBOpenURL opens a link another app sent, as a new tab in the window, as a link from
// another app does on the Mac. Android's main thread calls it, after PBStart.
//
//export PBOpenURL
func PBOpenURL(url *C.char) {
	owner.Open([]string{C.GoString(url)})
}
