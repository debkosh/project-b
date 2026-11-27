//go:build pbblock

// Package native is the blocking library the app links, libpbblock (core/blocking/lib),
// through cgo. Only the pbblock build tag builds it, which engine/build_go.py sets once it
// has built the library beside libcore; a plain go test of core needs no Rust.
package native

/*
#cgo CFLAGS: -I${SRCDIR}/../lib -I${SRCDIR}/../../../engine/abi
#cgo LDFLAGS: -lpbblock
#include <stdlib.h>
#include "pbblock.h"
#include "pb_abi.h"

// hooks is the engine's table for blocker b: the library's own functions, which the engine
// calls on any thread, and the watcher's script. C memory, kept for the process.
static PBBlockerHooks* hooks(PBBlocker* b, const char* watcher) {
  PBBlockerHooks* h = calloc(1, sizeof *h);
  h->blocker = b;
  h->request = (int32_t (*)(void*, const char*, const char*, const char*, char**))pbblock_request;
  h->page = (char* (*)(void*, const char*))pbblock_page;
  h->hide = (char* (*)(void*, const char*, const char*))pbblock_hide;
  h->free_string = pbblock_free;
  h->watcher_script = watcher;
  return h;
}

static int compile(uint8_t** texts, size_t* lens, uint8_t* trusted, size_t n, uint8_t** out, size_t* out_len) {
  return pbblock_compile((const uint8_t* const*)texts, lens, trusted, n, out, out_len);
}
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"

	"projectb/blocking"
	"projectb/blocking/page"
)

// Library is libpbblock.
type Library struct{}

func (Library) New() blocking.Blocker { return &blocker{b: C.pbblock_new()} }

func (Library) Compile(texts [][]byte, trusted []bool) ([]byte, error) {
	n := len(texts)
	// The texts are copied to C memory: cgo lets no Go pointer to Go pointers cross.
	ptrs := (*[1 << 28]*C.uint8_t)(C.calloc(C.size_t(max(n, 1)), C.size_t(unsafe.Sizeof((*C.uint8_t)(nil)))))[:n:n]
	lens := (*[1 << 28]C.size_t)(C.calloc(C.size_t(max(n, 1)), C.size_t(unsafe.Sizeof(C.size_t(0)))))[:n:n]
	defer func() {
		for _, p := range ptrs {
			C.free(unsafe.Pointer(p))
		}
		C.free(unsafe.Pointer(unsafe.SliceData(ptrs)))
		C.free(unsafe.Pointer(unsafe.SliceData(lens)))
	}()
	flags := make([]byte, max(n, 1))
	for i, t := range texts {
		ptrs[i] = (*C.uint8_t)(C.CBytes(t))
		lens[i] = C.size_t(len(t))
		if i < len(trusted) && trusted[i] {
			flags[i] = 1
		}
	}
	cflags := (*C.uint8_t)(C.CBytes(flags))
	defer C.free(unsafe.Pointer(cflags))
	var out *C.uint8_t
	var outLen C.size_t
	if C.compile(unsafe.SliceData(ptrs), unsafe.SliceData(lens), cflags, C.size_t(n), &out, &outLen) == 0 {
		return nil, errors.New("libpbblock compiled nothing")
	}
	defer C.pbblock_free_data(out, outLen)
	return C.GoBytes(unsafe.Pointer(out), C.int(outLen)), nil
}

type blocker struct {
	b     *C.PBBlocker
	once  sync.Once
	hooks *C.PBBlockerHooks
}

// watcher is the watcher's script as C memory, made once, for the life of the process.
var watcher = sync.OnceValue(func() *C.char { return C.CString(page.Watcher) })

func (b *blocker) Load(group string, engine []byte) bool {
	if len(engine) == 0 {
		return false
	}
	g := C.CString(group)
	defer C.free(unsafe.Pointer(g))
	return C.pbblock_load(b.b, g, (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(engine))), C.size_t(len(engine))) == 1
}

func (b *blocker) Unload(group string) {
	g := C.CString(group)
	defer C.free(unsafe.Pointer(g))
	C.pbblock_unload(b.b, g)
}

func (b *blocker) Resources(json []byte) bool {
	if len(json) == 0 {
		return false
	}
	return C.pbblock_resources(b.b, (*C.uint8_t)(unsafe.Pointer(unsafe.SliceData(json))), C.size_t(len(json))) == 1
}

func (b *blocker) Request(url, source, kind string) int {
	u, s, k := C.CString(url), C.CString(source), C.CString(kind)
	defer C.free(unsafe.Pointer(u))
	defer C.free(unsafe.Pointer(s))
	defer C.free(unsafe.Pointer(k))
	var redirect *C.char
	v := C.pbblock_request(b.b, u, s, k, &redirect)
	C.pbblock_free(redirect)
	return int(v)
}

func (b *blocker) Hooks() unsafe.Pointer {
	b.once.Do(func() { b.hooks = C.hooks(b.b, watcher()) })
	return unsafe.Pointer(b.hooks)
}
