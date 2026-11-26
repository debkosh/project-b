package open

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
#include "open_darwin.h"
*/
import "C"

import (
	"strings"
	"unsafe"

	"github.com/debkosh/project-b/oskit"
)

// mac is the Mac's: NSWorkspace opens and reveals, and is asked about and set as the default
// browser, which macOS asks the person; NSApp's delegate takes the links macOS hands the app.
type mac struct {
	asking oskit.Question
	links  links
}

var the = &mac{}

// New is this OS's Opener.
func New() Opener { return the }

func withPath(path string, call func(*C.char)) {
	c := C.CString(path)
	defer C.free(unsafe.Pointer(c))
	call(c)
}

func (m *mac) OpenFile(path string) { withPath(path, func(c *C.char) { C.oskit_open_file(c) }) }
func (m *mac) Reveal(path string)   { withPath(path, func(c *C.char) { C.oskit_open_reveal(c) }) }

func (m *mac) IsDefaultBrowser() bool { return C.oskit_open_is_default_browser() != 0 }

func (m *mac) AskDefaultBrowser(answered func(bool)) {
	m.asking.Ask(answered, func() { C.oskit_open_ask_default_browser() })
}

func (m *mac) OnOpenURLs(opened func(urls []string)) {
	m.links.set(opened)
	C.oskit_open_receive_links()
}

// oskitOpenDefaultBrowser is macOS's answer to AskDefaultBrowser, on the main thread.
//
//export oskitOpenDefaultBrowser
func oskitOpenDefaultBrowser(yes C.int) { the.asking.Answer(yes != 0) }

// oskitOpenURLs is the links and files macOS hands the app, one per line, on the main thread.
//
//export oskitOpenURLs
func oskitOpenURLs(urls *C.char) { the.links.open(strings.Split(C.GoString(urls), "\n")) }
