// Command lib is the Go core as a c-archive, linked statically into the main binary.
// It implements engine.Engine on the C function table and hands every event from the
// adapter to the package that decodes it: abi for its events, content for content's hooks. Nothing else in core/ touches cgo, so everything
// else still builds and tests with no C++.
package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../../engine/abi
#include <stdlib.h>
#include "pb_abi.h"
#include "pb_content_abi.h"

static inline int64_t call_create_tab(const PBEngineCalls* c, const char* workspace) { return c->create_tab(workspace); }
static inline int64_t call_create_internal_tab(const PBEngineCalls* c) { return c->create_internal_tab(); }
static inline void call_set_first_workspace(const PBEngineCalls* c, const char* workspace) { c->set_first_workspace(workspace); }
static inline void call_erase_workspace(const PBEngineCalls* c, const char* workspace) { c->erase_workspace(workspace); }
static inline void call_close_tab(const PBEngineCalls* c, int64_t tab) { c->close_tab(tab); }
static inline void call_open_profile(const PBEngineCalls* c, const char* dir) { c->open_profile(dir); }
static inline void call_set_block_rules(const PBEngineCalls* c, const char* hosts) { c->set_block_rules(hosts); }
static inline void call_set_ad_rules(const PBEngineCalls* c, const char* hosts, const char* off) { c->set_ad_rules(hosts, off); }
static inline void call_set_window_page(const PBEngineCalls* c, int64_t window, const char* page) { c->set_window_page(window, page); }
static inline void call_create_window(const PBEngineCalls* c, int64_t window, int64_t near, const char* page, const PBWindowLayout* layout, const PBRegion* regions, size_t count, const PBVisibleTab* tabs, size_t tab_count) { c->create_window(window, near, page, layout, regions, count, tabs, tab_count); }
static inline void call_set_window_bounds(const PBEngineCalls* c, int64_t window, double x, double y, double w, double h) { c->set_window_bounds(window, x, y, w, h); }
static inline void call_set_window_mode(const PBEngineCalls* c, int64_t window, int32_t mode) { c->set_window_mode(window, mode); }
static inline void call_request_window(const PBEngineCalls* c, int64_t window, int32_t request) { c->request_window(window, request); }
static inline void call_load_url(const PBEngineCalls* c, int64_t tab, const char* url) { c->load_url(tab, url); }
static inline void call_go_back(const PBEngineCalls* c, int64_t tab) { c->go_back(tab); }
static inline void call_go_forward(const PBEngineCalls* c, int64_t tab) { c->go_forward(tab); }
static inline void call_reload(const PBEngineCalls* c, int64_t tab) { c->reload(tab); }
static inline void call_reload_bypassing_cache(const PBEngineCalls* c, int64_t tab) { c->reload_bypassing_cache(tab); }
static inline void call_stop(const PBEngineCalls* c, int64_t tab) { c->stop(tab); }
static inline void call_set_zoom(const PBEngineCalls* c, int64_t tab, double factor) { c->set_zoom(tab, factor); }
static inline double call_get_zoom(const PBEngineCalls* c, int64_t tab) { return c->get_zoom(tab); }
static inline int64_t call_duplicate_tab(const PBEngineCalls* c, int64_t tab) { return c->duplicate_tab(tab); }
static inline void call_edit(const PBEngineCalls* c, int32_t command) { c->edit(command); }
static inline void call_write_clipboard(const PBEngineCalls* c, const char* text, size_t length) { c->write_clipboard(text, length); }
static inline void call_download_url(const PBEngineCalls* c, int64_t tab, const char* url, int save_as) { c->download_url(tab, url, save_as); }
static inline void call_download_command(const PBEngineCalls* c, uint32_t download, int32_t command) { c->download_command(download, command); }
static inline void call_open_file(const PBEngineCalls* c, const char* path) { c->open_file(path); }
static inline void call_reveal_file(const PBEngineCalls* c, const char* path) { c->reveal_file(path); }
static inline void call_show_download(const PBEngineCalls* c, int64_t id, const char* name, const char* path, int32_t state, int64_t received, int64_t total, int paused) {
  if (c->show_download) c->show_download(id, name, path, state, received, total, paused);
}
static inline void call_find(const PBEngineCalls* c, int64_t tab, int32_t request, const char* text, int forward, int find_next) { c->find(tab, request, text, forward, find_next); }
static inline void call_stop_finding(const PBEngineCalls* c, int64_t tab) { c->stop_finding(tab); }
static inline void call_print(const PBEngineCalls* c, int64_t tab) { c->print(tab); }
static inline void call_capture_thumbnail(const PBEngineCalls* c, int64_t tab, int32_t max_width) { c->capture_thumbnail(tab, max_width); }
static inline void call_exit_page_full_screen(const PBEngineCalls* c, int64_t tab) { c->exit_page_full_screen(tab); }
static inline void call_answer_sign_in(const PBEngineCalls* c, int64_t dialog, int ok, const char* username, const char* password) { c->answer_sign_in(dialog, ok, username, password); }
static inline void call_answer_dialog(const PBEngineCalls* c, int64_t dialog, int ok, const char* text) { c->answer_dialog(dialog, ok, text); }
static inline void call_answer_permission(const PBEngineCalls* c, uint32_t id, const int32_t* statuses, size_t count) { c->answer_permission(id, statuses, count); }
static inline void call_send_to_page(const PBEngineCalls* c, int64_t tab, const char* json, size_t length) { c->send_to_page(tab, json, length); }
static inline void call_set_menu(const PBEngineCalls* c, const char* json, size_t length) { c->set_menu(json, length); }
static inline void call_app_command(const PBEngineCalls* c, int32_t command) { c->app_command(command); }
static inline void call_set_window_layout(const PBEngineCalls* c, int64_t window, const PBWindowLayout* layout, const PBRegion* regions, size_t count, const PBVisibleTab* tabs, size_t tab_count) { c->set_window_layout(window, layout, regions, count, tabs, tab_count); }
static inline void call_focus_tab(const PBEngineCalls* c, int64_t tab) { c->focus_tab(tab); }
static inline void call_focus_shell(const PBEngineCalls* c, int64_t window) { c->focus_shell(window); }
static inline void call_send_to_shell(const PBEngineCalls* c, int64_t window, const char* json, size_t length) { c->send_to_shell(window, json, length); }
static inline void call_set_shell_back(const PBEngineCalls* c, int64_t window, int on) { c->set_shell_back(window, on); }
static inline void call_save_file(const PBEngineCalls* c, const char* path) { c->save_file(path); }
static inline void call_share_file(const PBEngineCalls* c, int64_t window, const char* path, double x, double y, double w, double h) { c->share_file(window, path, x, y, w, h); }
static inline const PBContentHooks* call_content_hooks(const PBEngineCalls* c) { return c->content_hooks ? c->content_hooks() : NULL; }
static inline void call_set_subscribed(const PBContentHooks* h, int32_t hook, int on) { h->set_subscribed(hook, on); }
static inline void call_post_wakeup(const PBEngineCalls* c) { c->post_wakeup(); }
static inline void native_nil_deref(void) { *(volatile int*)0 = 1; }
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"projectb/abi"
	"projectb/app"
	"projectb/content"
	"projectb/engine"
	"projectb/flags"
	"projectb/instance"
	"projectb/profiles"
	"projectb/relaunch"
	"projectb/timeline"
)

// Where releases are published. The project and the URL are the ones in amarit.json at the
// repo root, which tools/release.sh publishes with. updateConfig, beside this file for each
// platform, describes this build to the updater.
const (
	releaseProject  = "project-b"
	releaseManifest = "https://github.com/sudiptadeb/project-b/releases/latest/download/releases.json"
)

// nativeEngine is the real engine: every method calls the C++ adapter.
type nativeEngine struct {
	calls *C.PBEngineCalls

	mu     sync.Mutex
	posted []func()
}

func (e *nativeEngine) CreateTab(workspace string) engine.TabID {
	c := C.CString(workspace)
	defer C.free(unsafe.Pointer(c))
	return engine.TabID(C.call_create_tab(e.calls, c))
}

func (e *nativeEngine) CreateInternalTab() engine.TabID {
	return engine.TabID(C.call_create_internal_tab(e.calls))
}

func (e *nativeEngine) SetFirstWorkspace(workspace string) {
	c := C.CString(workspace)
	defer C.free(unsafe.Pointer(c))
	C.call_set_first_workspace(e.calls, c)
}

func (e *nativeEngine) EraseWorkspace(workspace string) {
	c := C.CString(workspace)
	defer C.free(unsafe.Pointer(c))
	C.call_erase_workspace(e.calls, c)
}

func (e *nativeEngine) CloseTab(tab engine.TabID) { C.call_close_tab(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) RequestWindow(w engine.WindowID, r engine.WindowRequest) {
	C.call_request_window(e.calls, C.int64_t(w), C.int32_t(r))
}

func (e *nativeEngine) LoadURL(tab engine.TabID, url string) {
	c := C.CString(url)
	defer C.free(unsafe.Pointer(c))
	C.call_load_url(e.calls, C.int64_t(tab), c)
}

func (e *nativeEngine) GoBack(tab engine.TabID) { C.call_go_back(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) GoForward(tab engine.TabID) { C.call_go_forward(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) Reload(tab engine.TabID) { C.call_reload(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) ReloadBypassingCache(tab engine.TabID) {
	C.call_reload_bypassing_cache(e.calls, C.int64_t(tab))
}

func (e *nativeEngine) Stop(tab engine.TabID) { C.call_stop(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) Zoom(tab engine.TabID, factor float64) {
	C.call_set_zoom(e.calls, C.int64_t(tab), C.double(factor))
}

func (e *nativeEngine) ZoomOf(tab engine.TabID) float64 {
	return float64(C.call_get_zoom(e.calls, C.int64_t(tab)))
}

func (e *nativeEngine) DuplicateTab(tab engine.TabID) engine.TabID {
	return engine.TabID(C.call_duplicate_tab(e.calls, C.int64_t(tab)))
}

func (e *nativeEngine) Edit(c engine.EditCommand) { C.call_edit(e.calls, C.int32_t(c)) }

func (e *nativeEngine) WriteClipboard(text string) {
	cs := C.CString(text)
	defer C.free(unsafe.Pointer(cs))
	C.call_write_clipboard(e.calls, cs, C.size_t(len(text)))
}

func (e *nativeEngine) DownloadURL(tab engine.TabID, url string, saveAs bool) {
	cs := C.CString(url)
	defer C.free(unsafe.Pointer(cs))
	save := C.int(0)
	if saveAs {
		save = 1
	}
	C.call_download_url(e.calls, C.int64_t(tab), cs, save)
}

func (e *nativeEngine) DownloadCommand(id engine.DownloadID, c engine.DownloadCommand) {
	C.call_download_command(e.calls, C.uint32_t(id), C.int32_t(c))
}

func (e *nativeEngine) OpenFile(path string) {
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	C.call_open_file(e.calls, cs)
}

func (e *nativeEngine) ShowDownload(id int, d engine.Download) {
	name, path := C.CString(filepath.Base(d.Path)), C.CString(d.Path)
	defer C.free(unsafe.Pointer(name))
	defer C.free(unsafe.Pointer(path))
	paused := 0
	if d.Paused {
		paused = 1
	}
	C.call_show_download(e.calls, C.int64_t(id), name, path, C.int32_t(d.State), C.int64_t(d.Received), C.int64_t(d.Total), C.int(paused))
}

func (e *nativeEngine) RevealFile(path string) {
	cs := C.CString(path)
	defer C.free(unsafe.Pointer(cs))
	C.call_reveal_file(e.calls, cs)
}

func (e *nativeEngine) AnswerDialog(id engine.DialogID, ok bool, text string) {
	cs := C.CString(text)
	defer C.free(unsafe.Pointer(cs))
	yes := C.int(0)
	if ok {
		yes = 1
	}
	C.call_answer_dialog(e.calls, C.int64_t(id), yes, cs)
}

func (e *nativeEngine) Find(tab engine.TabID, request int, text string, forward, next bool) {
	cs := C.CString(text)
	defer C.free(unsafe.Pointer(cs))
	b := func(v bool) C.int {
		if v {
			return 1
		}
		return 0
	}
	C.call_find(e.calls, C.int64_t(tab), C.int32_t(request), cs, b(forward), b(next))
}

func (e *nativeEngine) StopFinding(tab engine.TabID) { C.call_stop_finding(e.calls, C.int64_t(tab)) }
func (e *nativeEngine) Print(tab engine.TabID)       { C.call_print(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) CaptureThumbnail(tab engine.TabID, maxWidth int) {
	C.call_capture_thumbnail(e.calls, C.int64_t(tab), C.int32_t(maxWidth))
}

// ThumbnailURL is where the engine serves a tab's picture to our pages; the revision only
// makes each picture's address its own.
func (e *nativeEngine) ThumbnailURL(tab engine.TabID, revision int64) string {
	return fmt.Sprintf("chrome://thumb/%d?v=%d", tab, revision)
}

func (e *nativeEngine) ExitPageFullScreen(tab engine.TabID) {
	C.call_exit_page_full_screen(e.calls, C.int64_t(tab))
}

func (e *nativeEngine) AnswerSignIn(id engine.DialogID, ok bool, username, password string) {
	u, p := C.CString(username), C.CString(password)
	defer C.free(unsafe.Pointer(u))
	defer C.free(unsafe.Pointer(p))
	yes := C.int(0)
	if ok {
		yes = 1
	}
	C.call_answer_sign_in(e.calls, C.int64_t(id), yes, u, p)
}

func (e *nativeEngine) AnswerPermission(id engine.PermissionID, statuses []engine.PermissionStatus) {
	s := make([]C.int32_t, len(statuses)+1) // never empty, so &s[0] is always there
	for i, v := range statuses {
		s[i] = C.int32_t(v)
	}
	C.call_answer_permission(e.calls, C.uint32_t(id), &s[0], C.size_t(len(statuses)))
}

func (e *nativeEngine) SendToPage(tab engine.TabID, msg []byte) {
	if len(msg) == 0 {
		return
	}
	C.call_send_to_page(e.calls, C.int64_t(tab), (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)))
}

func (e *nativeEngine) SetMenu(m engine.Menu) {
	b, err := json.Marshal(m)
	if err != nil {
		timeline.Errorf("menu: %v", err)
		return
	}
	C.call_set_menu(e.calls, (*C.char)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
}

func (e *nativeEngine) AppCommand(c engine.AppCommand) { C.call_app_command(e.calls, C.int32_t(c)) }

func (e *nativeEngine) SetWindowBounds(w engine.WindowID, r engine.Rect) {
	C.call_set_window_bounds(e.calls, C.int64_t(w), C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H))
}

func (e *nativeEngine) SetWindowMode(w engine.WindowID, m engine.WindowMode) {
	C.call_set_window_mode(e.calls, C.int64_t(w), C.int32_t(m))
}

func (e *nativeEngine) CreateWindow(w, near engine.WindowID, page engine.Page, l engine.WindowLayout) {
	p := C.CString(string(page))
	defer C.free(unsafe.Pointer(p))
	e.withLayout(l, func(layout *C.PBWindowLayout, regions *C.PBRegion, count C.size_t, tabs *C.PBVisibleTab, tabCount C.size_t) {
		C.call_create_window(e.calls, C.int64_t(w), C.int64_t(near), p, layout, regions, count, tabs, tabCount)
	})
}

func (e *nativeEngine) SetWindowPage(w engine.WindowID, page engine.Page) {
	p := C.CString(string(page))
	defer C.free(unsafe.Pointer(p))
	C.call_set_window_page(e.calls, C.int64_t(w), p)
}

func (e *nativeEngine) OpenProfile(dir string) {
	c := C.CString(dir)
	defer C.free(unsafe.Pointer(c))
	C.call_open_profile(e.calls, c)
}

func (e *nativeEngine) SetBlockRules(r engine.BlockRules) {
	c := C.CString(strings.Join(r.Hosts, "\n"))
	defer C.free(unsafe.Pointer(c))
	C.call_set_block_rules(e.calls, c)
}

func (e *nativeEngine) SetAdRules(r engine.AdRules) {
	hosts, off := C.CString(strings.Join(r.Hosts, "\n")), C.CString(strings.Join(r.Off, "\n"))
	defer C.free(unsafe.Pointer(hosts))
	defer C.free(unsafe.Pointer(off))
	C.call_set_ad_rules(e.calls, hosts, off)
}

func (e *nativeEngine) SetWindowLayout(w engine.WindowID, l engine.WindowLayout) {
	e.withLayout(l, func(layout *C.PBWindowLayout, regions *C.PBRegion, count C.size_t, tabs *C.PBVisibleTab, tabCount C.size_t) {
		C.call_set_window_layout(e.calls, C.int64_t(w), layout, regions, count, tabs, tabCount)
	})
}

// withLayout hands call a layout as the C side has it.
func (e *nativeEngine) withLayout(l engine.WindowLayout, call func(*C.PBWindowLayout, *C.PBRegion, C.size_t, *C.PBVisibleTab, C.size_t)) {
	rgb, err := strconv.ParseInt(strings.TrimPrefix(l.FrameColor, "#"), 16, 32)
	if err != nil {
		timeline.Errorf("window colour %q is not #rrggbb", l.FrameColor)
		return
	}
	regions := make([]C.PBRegion, len(l.Regions))
	for i, g := range l.Regions {
		regions[i] = C.PBRegion{x: C.double(g.Rect.X), y: C.double(g.Rect.Y), w: C.double(g.Rect.W), h: C.double(g.Rect.H), area: C.int32_t(g.Area)}
	}
	var first *C.PBRegion
	if len(regions) > 0 {
		first = &regions[0]
	}
	tabs := make([]C.PBVisibleTab, len(l.Tabs))
	for i, v := range l.Tabs {
		tabs[i] = C.PBVisibleTab{tab: C.int64_t(v.Tab), x: C.double(v.Rect.X), y: C.double(v.Rect.Y), w: C.double(v.Rect.W), h: C.double(v.Rect.H)}
	}
	var firstTab *C.PBVisibleTab
	if len(tabs) > 0 {
		firstTab = &tabs[0]
	}
	layout := C.PBWindowLayout{
		view_w: C.double(l.ViewW), view_h: C.double(l.ViewH),
		frame_rgb: C.int32_t(rgb), frame_width: C.double(l.FrameWidth),
		theme: C.int32_t(l.Theme),
	}
	call(&layout, first, C.size_t(len(regions)), firstTab, C.size_t(len(tabs)))
}

func (e *nativeEngine) FocusTab(tab engine.TabID) { C.call_focus_tab(e.calls, C.int64_t(tab)) }

func (e *nativeEngine) FocusShell(w engine.WindowID) { C.call_focus_shell(e.calls, C.int64_t(w)) }

func (e *nativeEngine) SetShellBack(w engine.WindowID, on bool) {
	n := 0
	if on {
		n = 1
	}
	C.call_set_shell_back(e.calls, C.int64_t(w), C.int(n))
}

func (e *nativeEngine) ShareFile(w engine.WindowID, path string, r engine.Rect) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	C.call_share_file(e.calls, C.int64_t(w), p, C.double(r.X), C.double(r.Y), C.double(r.W), C.double(r.H))
}

func (e *nativeEngine) SaveFile(path string) {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	C.call_save_file(e.calls, p)
}

func (e *nativeEngine) SendToShell(w engine.WindowID, msg []byte) {
	if len(msg) == 0 {
		return
	}
	C.call_send_to_shell(e.calls, C.int64_t(w), (*C.char)(unsafe.Pointer(&msg[0])), C.size_t(len(msg)))
}

// Our pages are WebUI apps, each at chrome://<page>/.
func (e *nativeEngine) PageURL(p engine.Page) string { return "chrome://" + string(p) + "/" }

func (e *nativeEngine) OurPage(u string) (engine.Page, bool) { return ourPageOf(u) }

func (e *nativeEngine) ViewSourceURL(u string) string { return "view-source:" + u }

// ourPageOf is the page at a chrome:// address, as PageURL makes them: its host and
// path, without the slash after.
func ourPageOf(raw string) (engine.Page, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "chrome" || u.Host == "" {
		return "", false
	}
	return engine.Page(strings.ToLower(u.Host) + strings.TrimSuffix(u.Path, "/")), true
}

// Post runs fn on the main thread. Any goroutine may call it.
func (e *nativeEngine) Post(fn func()) {
	e.mu.Lock()
	e.posted = append(e.posted, fn)
	first := len(e.posted) == 1
	e.mu.Unlock()
	if first {
		C.call_post_wakeup(e.calls)
	}
}

var (
	native *nativeEngine
	// What the adapter tells Go: its events, and content's hooks, which nothing in the app
	// subscribes to.
	events  *abi.Events
	hooks   *content.Hooks
	list    *profiles.Manager
	blocked []string
	// The profile this copy runs on, the folder it owns, and its managers. A copy started
	// to pick has none of them.
	profile profiles.Choice
	owner   *instance.Owner
	theApp  *app.App
	// The addresses the command line named, which this copy opens as it starts when it owns
	// its profile; otherwise they are handed to the copy that does.
	startURLs []string
)

// PBClaimProfile is the first call, before anything is written to the profile. It chooses
// the profile from the command line's --root, --profile and --user-data-dir, each "" when
// not given, and --picker, and returns the folder this copy keeps its files in, once this
// copy owns it: the profile's, or the root when the copy picks its profile later. It
// returns nil when another copy owns that folder and has been handed urls, one per line,
// so this copy quits.
//
//export PBClaimProfile
func PBClaimProfile(root, id, userDataDir *C.char, picker C.int, urls *C.char) *C.char {
	var handed []string
	if s := C.GoString(urls); s != "" {
		handed = strings.Split(s, "\n")
	}
	list = profiles.New(rootOf(C.GoString(root)))
	var err error
	if dir := C.GoString(userDataDir); dir != "" {
		profile = profiles.Named(dir)
	} else if profile, err = list.Choose(C.GoString(id), picker != 0); err != nil {
		fmt.Fprintf(os.Stderr, "the profile: %v\n", err)
		os.Exit(1)
	}
	// A picker copy owns the root, so a copy relaunching for an update finds it, and a
	// second picker copy hands over to it.
	dir := profile.Dir
	if profile.Pick {
		dir = list.Root()
	}
	o, err := instance.Claim(dir, handed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "the profile folder: %v\n", err)
		os.Exit(1)
	}
	if o == nil {
		return nil
	}
	owner = o
	startURLs = handed
	return C.CString(dir)
}

// rootOf is the folder that holds the profiles: the one --root names, else ~/.project-b.
func rootOf(root string) string {
	if root != "" {
		return root
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".project-b")
}

// Go's own work runs on the main thread, one event at a time, so the collector is what
// uses Go's other cores: at most 4 of them keeps it off the cores Chromium's renderers
// and GPU process need, with no change to how long an event takes.
func init() {
	runtime.GOMAXPROCS(min(4, runtime.NumCPU()))
}

// PBStart starts what a copy has before and apart from its profile, the engine and the
// list, then the profile's managers, or the picker when the copy has no profile yet.
//
//export PBStart
func PBStart(calls *C.PBEngineCalls, blockedHosts *C.char) {
	timeline.Mark("go.starting")
	// The tables are C memory that lives as long as the process.
	native = &nativeEngine{calls: calls}
	events = abi.New()
	// An engine with no content under it has no hooks, and nothing subscribes to one.
	hooks = content.New(func(content.Hook) {})
	if contentHooks := C.call_content_hooks(calls); contentHooks != nil {
		if table := C.GoString(contentHooks.table_hash); table != content.TableHash {
			timeline.Errorf("the library's hook table is %s, Go's is %s: regenerate with tools/gen_hooks.py and rebuild", table, content.TableHash)
			timeline.Close()
			os.Exit(1)
		}
		hooks = content.New(func(h content.Hook) { C.call_set_subscribed(contentHooks, C.int32_t(h), 1) })
	}
	if s := C.GoString(blockedHosts); s != "" {
		blocked = strings.Split(s, ",")
	}
	if profile.Pick {
		picker := app.StartPicker(native, list, func(id string) {
			relaunch.Copy("--root="+list.Root(), "--profile="+id)
		})
		events.Listen(picker)
		owner.Serve(func([]string) { native.Post(picker.Handover) }, quit)
	} else {
		startProfile(profile)
	}
	timeline.Mark("go.started")
}

// startProfile makes this copy the profile c's, whose folder it owns: its managers, which
// open the profile in the engine first.
func startProfile(c profiles.Choice) {
	if c.ID != "" {
		if err := list.MarkUsed(c.ID); err != nil {
			timeline.Errorf("profiles: %v", err)
		}
	}
	home, _ := os.UserHomeDir()
	theApp = app.Start(native, app.Config{
		ProfileDir:   c.Dir,
		Profiles:     list,
		Profile:      c.ID,
		URLs:         startURLs,
		BlockedHosts: blocked,
		Update:       updateConfig(c),
		CrashReports: filepath.Join(home, "Library/Logs/DiagnosticReports"),
		DownloadsDir: downloadsDir(home),
		System:       systemReport,
	})
	events.Listen(theApp)
	owner.Serve(func(urls []string) { native.Post(func() { theApp.Handover(urls) }) }, quit)
}

// quit ends this copy, as another copy relaunching for an update asks.
func quit() { native.Post(func() { native.AppCommand(engine.AppQuit) }) }

//export PBStop
func PBStop() {
	// A picker copy has no profile to stop.
	if theApp != nil {
		theApp.Stop()
	}
	owner.Close()
	timeline.Close()
}

// The timeline's kinds as they cross, PBTimelineKind in pb_abi.h.
var timelineKinds = []timeline.Kind{timeline.KindMark, timeline.KindBegin, timeline.KindEnd, timeline.KindInfo, timeline.KindWarn, timeline.KindError}

// PBTimelineStart starts the timeline: its file goes in the folder this copy keeps its
// files in, and time zero is processAge nanoseconds ago.
//
//export PBTimelineStart
func PBTimelineStart(profileDir *C.char, processAge C.int64_t) {
	timeline.Start(C.GoString(profileDir), time.Duration(processAge), printer())
}

// PBTimelineNow is the time on the timeline, in nanoseconds since process start. The
// adapter reads it once, to place its own clock on ours.
//
//export PBTimelineNow
func PBTimelineNow() C.int64_t { return C.int64_t(timeline.NowNanos()) }

// PBTimelineRecord records one entry from the adapter, on any thread. t is its time, or
// -1 for now.
//
//export PBTimelineRecord
func PBTimelineRecord(kind C.int32_t, t C.int64_t, name *C.char, nameLen C.size_t, text *C.char, textLen C.size_t) {
	timeline.Record(timelineKinds[kind], timeline.SrcEngine, C.GoStringN(name, C.int(nameLen)), C.GoStringN(text, C.int(textLen)), int64(t))
}

// PBTimelinePageOrigin says when one of our pages' own clock started: see
// timeline.PageOrigin.
//
//export PBTimelinePageOrigin
func PBTimelinePageOrigin(page *C.char, t C.int64_t) {
	timeline.PageOrigin(C.GoString(page), int64(t))
}

//export PBRunPosted
func PBRunPosted() {
	native.mu.Lock()
	fns := native.posted
	native.posted = nil
	native.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// PBHook is every event from the adapter. The values are copied out of C memory before
// anything runs, and a string answer is malloc'd for the adapter to free.
//
//export PBHook
func PBHook(id C.int32_t, args *C.PBValue, count C.size_t, result *C.PBValue) C.int {
	values := make([]content.Value, int(count))
	for i, a := range unsafe.Slice(args, int(count)) {
		values[i].I = int64(a.i)
		values[i].D = float64(a.d)
		if a.len > 0 {
			values[i].S = C.GoStringN(a.s, C.int(a.len))
		}
	}
	var v content.Value
	var ok bool
	if abi.Has(int32(id)) {
		v, ok = events.Dispatch(abi.Event(id), values)
	} else {
		v, ok = hooks.Dispatch(content.Hook(id), values)
	}
	if !ok {
		return 0
	}
	result.i = C.int64_t(v.I)
	result.d = C.double(v.D)
	if v.S != "" {
		result.s = C.CString(v.S)
		result.len = C.size_t(len(v.S))
	}
	return 1
}

// PBForceGC runs a full collection. The driver times it on the main thread.
//
//export PBForceGC
func PBForceGC() { runtime.GC() }

// PBRecoverNilDeref checks that a nil dereference in Go still becomes a panic that can
// be recovered, with Chromium's signal handlers installed in the same process.
//
//export PBRecoverNilDeref
func PBRecoverNilDeref() (result C.int) {
	defer func() {
		if recover() != nil {
			result = 1
		}
	}()
	var p *int
	*p = 1
	return 0
}

// PBCrash crashes on purpose, as pb_abi.h describes, for the crash log's tests.
//
//export PBCrash
func PBCrash(kind C.int32_t) {
	var p *int
	switch kind {
	case 0:
		*p = 1
	case 1:
		panic("PBCrash on the calling thread")
	case 2:
		go func() { *p = 1 }()
		select {}
	case 3:
		C.native_nil_deref()
	}
}

// PBGoStats returns runtime numbers as JSON. The caller frees the string.
//
//export PBGoStats
func PBGoStats() *C.char {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var gc debug.GCStats
	debug.ReadGCStats(&gc)
	var maxPauseNs int64
	for _, p := range gc.Pause {
		if p.Nanoseconds() > maxPauseNs {
			maxPauseNs = p.Nanoseconds()
		}
	}
	b, _ := json.Marshal(map[string]any{
		"goroutines":  runtime.NumGoroutine(),
		"heapAllocKB": m.HeapAlloc / 1024,
		"sysKB":       m.Sys / 1024,
		"numGC":       m.NumGC,
		"maxPauseUs":  maxPauseNs / 1000,
		"cgoCalls":    runtime.NumCgoCall(),
		"hooks":       crossings(),
	})
	return C.CString(string(b))
}

// crossings is how often each event and hook has crossed, by name.
func crossings() map[string]int {
	out := hooks.Counts()
	for name, n := range events.Counts() {
		out[name] = n
	}
	return out
}

// PBFree frees a string Go returned, for the engine, whose own free may be another
// allocator's.
//
//export PBFree
func PBFree(p unsafe.Pointer) { C.free(p) }

// PBVersion is what --version prints first: the release this build is, or "development
// build" for one that isn't a release. The caller frees the string.
//
//export PBVersion
func PBVersion() *C.char {
	v := appVersion()
	if v == "" {
		v = "development build"
	}
	return C.CString("Project B " + v)
}

// PBLaunchSwitches is the switches the flags saved in the profile ask for, one per line,
// for the adapter to add to the command line before content starts. The caller frees it.
//
//export PBLaunchSwitches
func PBLaunchSwitches(profileDir *C.char) *C.char {
	return C.CString(strings.Join(flags.New(C.GoString(profileDir)).Switches(), "\n"))
}

func main() {}
