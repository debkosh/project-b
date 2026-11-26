package abi

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"projectb/engine"
)

// pack lays Go values out the way the adapter does, so a test can fake a crossing.
// The generated structs declare their fields in the adapter's order.
func pack(values ...any) []Value {
	var out []Value
	for _, v := range values {
		out = append(out, packOne(reflect.ValueOf(v))...)
	}
	return out
}

func packOne(v reflect.Value) []Value {
	switch v.Kind() {
	case reflect.Bool:
		return []Value{yes(v.Bool())}
	case reflect.Int, reflect.Int32, reflect.Int64:
		return []Value{{I: v.Int()}}
	case reflect.Float64:
		return []Value{{D: v.Float()}}
	case reflect.String:
		return []Value{{S: v.String()}}
	case reflect.Slice:
		out := []Value{{I: int64(v.Len())}}
		for i := 0; i < v.Len(); i++ {
			out = append(out, packOne(v.Index(i))...)
		}
		return out
	case reflect.Struct:
		var out []Value
		for i := 0; i < v.NumField(); i++ {
			out = append(out, packOne(v.Field(i))...)
		}
		return out
	}
	panic("cannot pack " + v.Kind().String())
}

// adapterRecorder is an engine.Listener that writes down what it hears.
type adapterRecorder struct {
	got []string
	// answer is what NavigationStarting says.
	answer engine.Navigation
}

func (r *adapterRecorder) note(s string) { r.got = append(r.got, s) }
func (r *adapterRecorder) ShellMessage(w engine.WindowID, msg []byte) {
	r.note(fmt.Sprintf("shell %d %s", w, msg))
}
func (r *adapterRecorder) NavigationStarting(tab engine.TabID, nav int64, url string) engine.Navigation {
	r.note("starting " + url)
	return r.answer
}
func (r *adapterRecorder) NavigationFinished(tab engine.TabID, nav int64, url string) {
	r.note("finished " + url)
}
func (r *adapterRecorder) NavigationFailed(tab engine.TabID, nav int64, url string, reason engine.Failure, code string) {
	r.note("failed " + url + " " + string(reason))
}
func (r *adapterRecorder) HistoryChanged(tab engine.TabID, back, fwd bool) {
	if back {
		r.note("canGoBack")
	}
}
func (r *adapterRecorder) TitleChanged(tab engine.TabID, title string) { r.note("title " + title) }
func (r *adapterRecorder) LoadingChanged(tab engine.TabID, loading bool) {
	if loading {
		r.note("loading")
	} else {
		r.note("loaded")
	}
}
func (r *adapterRecorder) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	r.note(fmt.Sprintf("menu %d %s %s %s", tab, m.Link, m.Src, m.Media))
}
func (r *adapterRecorder) ScrollDirectionChanged(tab engine.TabID, down bool) {
	r.note(fmt.Sprintf("scroll %d %v", tab, down))
}

func (r *adapterRecorder) LoadProgressChanged(tab engine.TabID, progress float64) {
	r.note(fmt.Sprintf("progress %.2f", progress))
}
func (r *adapterRecorder) PageCrashed(tab engine.TabID) { r.note("crashed") }
func (r *adapterRecorder) FaviconChanged(tab engine.TabID, dataURL string) {
	r.note("favicon " + dataURL)
}
func (r *adapterRecorder) TabOpened(opener, tab engine.TabID, url, workspace string, foreground, newWindow, tapped bool) {
	r.note(fmt.Sprint("opened ", url, " in ", workspace, " new window ", newWindow, " tapped ", tapped))
}
func (r *adapterRecorder) CloseRequested(tab engine.TabID) { r.note(fmt.Sprintf("close %d", tab)) }
func (r *adapterRecorder) ActivateRequested(tab engine.TabID) {
	r.note(fmt.Sprintf("activate %d", tab))
}
func (r *adapterRecorder) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	r.note(fmt.Sprintf("key %d %d %d %d", k.Code, k.Mods, phase, in))
	return k.Mods&engine.ModCmd != 0
}
func (r *adapterRecorder) ModsChanged(m engine.Mods)        { r.note(fmt.Sprint("mods ", m)) }
func (r *adapterRecorder) MenuChosen(action string)         { r.note("menu " + action) }
func (r *adapterRecorder) MenuList(name string) engine.Menu { r.note("menu list " + name); return nil }
func (r *adapterRecorder) OSBindings(byTitle map[string]engine.Key) {
	r.note(fmt.Sprint("os ", byTitle))
}
func (r *adapterRecorder) PageMessage(tab engine.TabID, page engine.Page, msg []byte) {
	r.note(fmt.Sprintf("page %d %s %s", tab, page, msg))
}
func (r *adapterRecorder) PickerMessage(w engine.WindowID, msg []byte) {
	r.note(fmt.Sprintf("picker %d %s", w, msg))
}
func (r *adapterRecorder) WindowChanged(w engine.WindowID, s engine.WindowState) {
	r.note(fmt.Sprintf("window %d %gx%g r=%g full=%v", w, s.W, s.H, s.CornerRadius, s.FullScreen))
}
func (r *adapterRecorder) TabFocused(tab engine.TabID)    { r.note(fmt.Sprint("focused ", tab)) }
func (r *adapterRecorder) WindowClosed(w engine.WindowID) { r.note(fmt.Sprintf("window %d closed", w)) }
func (r *adapterRecorder) WindowFocused(w engine.WindowID) {
	r.note(fmt.Sprintf("window %d focused", w))
}
func (r *adapterRecorder) ShellCrashed(w engine.WindowID) { r.note(fmt.Sprintf("shell %d crashed", w)) }
func (r *adapterRecorder) ProcessCrashed(p engine.ProcessCrash) {
	r.note(fmt.Sprintf("%s %s %d %d %s", p.Type, p.Status, p.ExitCode, p.PID, p.URL))
}

func (r *adapterRecorder) DownloadTarget(d engine.DownloadRequest) engine.DownloadTarget {
	r.note(fmt.Sprintf("download %d %d %s %s", d.ID, d.Tab, d.URL, d.Name))
	return engine.DownloadTarget{Path: "/d/" + d.Name}
}
func (r *adapterRecorder) DownloadUpdated(d engine.Download) {
	r.note(fmt.Sprintf("downloaded %d %s %d/%d %q", d.ID, d.Path, d.Received, d.Total, d.Reason))
}

func (r *adapterRecorder) FileChooser(q engine.FileChooserRequest) engine.FileChooserAnswer {
	r.note(fmt.Sprintf("file chooser %d %d %v", q.Tab, q.Mode, q.Accept))
	return engine.FileChooserAnswer{Allow: true}
}
func (r *adapterRecorder) FilesChosen(tab engine.TabID, count int, folder string) {
	r.note(fmt.Sprintf("files chosen %d %d %s", tab, count, folder))
}

func (r *adapterRecorder) DialogOpened(d engine.Dialog) {
	r.note(fmt.Sprintf("dialog %d %s %s", d.ID, d.Kind, d.Message))
}
func (r *adapterRecorder) PasskeyStep(id engine.DialogID, step, carries string) {
	r.note(fmt.Sprintf("passkey %d %s %s", id, step, carries))
}
func (r *adapterRecorder) DialogClosed(id engine.DialogID) {
	r.note(fmt.Sprintf("dialog closed %d", id))
}

func (r *adapterRecorder) FindReply(tab engine.TabID, request, matches, active int, final bool) {
	r.note(fmt.Sprintf("find %d %d %d/%d %v", tab, request, active, matches, final))
}

func (r *adapterRecorder) PageFullScreen(tab engine.TabID, on bool) {
	r.note(fmt.Sprintf("full screen %d %v", tab, on))
}
func (r *adapterRecorder) Thumbnail(engine.TabID, int64)                           {}
func (r *adapterRecorder) FolderChosen(engine.WindowID, string)                    {}
func (r *adapterRecorder) AppLink(engine.TabID, string)                            {}
func (r *adapterRecorder) ImageSaved(string, bool)                                 {}
func (r *adapterRecorder) SecureDNSChecked(string, bool)                           {}
func (r *adapterRecorder) SiteDataListed(string, []engine.SiteData)                {}
func (r *adapterRecorder) IsPageFullScreen(engine.TabID) bool                      { return false }
func (r *adapterRecorder) DownloadNotificationCommand(int, engine.DownloadCommand) {}
func (r *adapterRecorder) DownloadPublished(string, string)                        {}

func (r *adapterRecorder) PermissionRequested(p engine.PermissionRequest) {
	r.note(fmt.Sprintf("permission %d tab %d %s %v gesture=%v", p.ID, p.Tab, p.Origin, p.Kinds, p.Gesture))
}
func (r *adapterRecorder) PermissionStatusOf(tab engine.TabID, origin, kind string) engine.PermissionStatus {
	r.note(fmt.Sprintf("status tab %d %s %s", tab, origin, kind))
	return engine.PermissionAsk
}
func (r *adapterRecorder) NotificationShown(n engine.Notification) bool {
	r.note(fmt.Sprintf("notification %s %s %s %q %q", n.ID, n.Origin, n.Page, n.Title, n.Body))
	return true
}
func (r *adapterRecorder) NotificationClosed(id string) { r.note("closed " + id) }
func (r *adapterRecorder) CertificateError(tab engine.TabID, e engine.CertificateError) bool {
	r.note(fmt.Sprintf("certificate %d %s %s main=%v passable=%v", tab, e.URL, e.Reason, e.MainFrame, e.Passable))
	return false
}
func (r *adapterRecorder) SparksFor(tab engine.TabID, url string, mainFrame bool) []engine.Script {
	r.note(fmt.Sprintf("sparks %d %s main=%v", tab, url, mainFrame))
	return []engine.Script{{ID: "ab12cd34", Source: "1", RunAt: "document-end"}}
}
func (r *adapterRecorder) AdBlocked(tab engine.TabID) { r.note(fmt.Sprintf("ad %d", tab)) }
func (r *adapterRecorder) FormMessage(from engine.FormFrame, kind, payload string) {
	r.note(fmt.Sprintf("form %+v %s %s", from, kind, payload))
}

func (r *adapterRecorder) PrintRequested(tab engine.TabID) bool {
	r.note(fmt.Sprintf("print %d", tab))
	return true
}
func (r *adapterRecorder) ConnectionChanged(tab engine.TabID, c engine.Connection) {
	r.note(fmt.Sprintf("connection %d %d %s", tab, c.State, c.Reason))
}
func (r *adapterRecorder) DevToolsCommand(method string, params []byte) (any, error) {
	r.note("devtools " + method + " " + string(params))
	switch method {
	case "ProjectB.echo":
		return json.RawMessage(params), nil
	case "ProjectB.fail":
		return nil, errors.New("it failed")
	}
	return nil, engine.ErrUnknownCommand
}

func wired(t *testing.T) (*Events, *adapterRecorder) {
	t.Helper()
	e, r := New(), &adapterRecorder{}
	e.Listen(r)
	return e, r
}

// The adapter's events reach the listener with no subscription, and are counted.
func TestAdapterEventsReachTheListener(t *testing.T) {
	h, r := wired(t)
	h.Dispatch(TabOpened, []Value{{I: 0}, {I: 2}, {S: "https://p.test/"}, {S: "a1b2c3d4"}, {I: 1}, {I: 1}, {I: 0}})
	h.Dispatch(WindowChanged, []Value{{I: 2}, {D: 1100}, {D: 700}, {D: 1}, {D: 2}, {D: 3}, {D: 4}, {D: 20}, {I: 1}})
	h.Dispatch(ShellCrashed, []Value{{I: 2}})
	h.Dispatch(ShellMessage, []Value{{I: 2}, {S: `{"t":"ready"}`}})
	h.Dispatch(PageMessage, []Value{{I: 2}, {S: "history"}, {S: `{"t":"ready"}`}})
	h.Dispatch(PickerMessage, []Value{{I: 2}, {S: `{"t":"open"}`}})
	h.Dispatch(WindowFocused, []Value{{I: 2}})
	h.Dispatch(WindowClosed, []Value{{I: 2}})
	h.Dispatch(ProcessCrashed, []Value{{S: "renderer"}, {S: "crashed"}, {I: 11}, {I: 42}, {S: "https://p.test/"}})
	if !reflect.DeepEqual(r.got, []string{"opened https://p.test/ in a1b2c3d4 new window true tapped false", "window 2 1100x700 r=20 full=true", "shell 2 crashed", `shell 2 {"t":"ready"}`, `page 2 history {"t":"ready"}`, `picker 2 {"t":"open"}`, "window 2 focused", "window 2 closed", "renderer crashed 11 42 https://p.test/"}) {
		t.Fatalf("got %v", r.got)
	}
	if h.Counts()["engine.TabOpened"] != 1 {
		t.Fatalf("counts %v", h.Counts())
	}
}

// A download's news carries why a failed one failed, last.
func TestADownloadsFailureSaysWhy(t *testing.T) {
	h, r := wired(t)
	h.Dispatch(DownloadUpdated, pack(4, "/d/f.zip", "https://a.test/f.zip", "application/zip", 3, 500, 1000, false, "connection"))
	if want := []string{`downloaded 4 /d/f.zip 500/1000 "connection"`}; !reflect.DeepEqual(r.got, want) {
		t.Fatalf("got %v", r.got)
	}
}

func TestDevToolsCommandsAnswerWithTheirResultOrError(t *testing.T) {
	h, _ := wired(t)
	for _, c := range []struct{ method, params, answer string }{
		{"ProjectB.echo", `{"a":1}`, `{"result":{"a":1}}`},
		{"ProjectB.fail", `{}`, `{"error":{"code":-32000,"message":"it failed"}}`},
	} {
		v, ok := h.Dispatch(DevToolsCommand, pack(c.method, c.params))
		if !ok || v.S != c.answer {
			t.Errorf("%s: %v %q, want %q", c.method, ok, v.S, c.answer)
		}
	}
	if _, ok := h.Dispatch(DevToolsCommand, pack("ProjectB.nothing", `{}`)); ok {
		t.Error("an unknown command was answered, not left to the engine")
	}
}

func TestPermissionRequestsAndReadsReachTheListener(t *testing.T) {
	h, r := wired(t)
	h.Dispatch(PermissionRequested, pack(7, 3, "https://meet.test", "camera,microphone", true))
	v, ok := h.Dispatch(PermissionStatusOf, pack(3, "https://meet.test", "notifications"))
	if !ok || v.I != int64(engine.PermissionAsk) {
		t.Fatalf("the status read answered %v %v", ok, v)
	}
	want := []string{"permission 7 tab 3 https://meet.test [camera microphone] gesture=true", "status tab 3 https://meet.test notifications"}
	if !reflect.DeepEqual(r.got, want) {
		t.Fatalf("got %v", r.got)
	}
}

func TestNotificationsReachTheListener(t *testing.T) {
	h, r := wired(t)
	v, ok := h.Dispatch(NotificationShown, pack("n1", "https://chat.test", "https://chat.test/room", "Ana", "hello"))
	if !ok || v.I != 1 {
		t.Fatalf("show answered %v %v", ok, v)
	}
	h.Dispatch(NotificationClosed, pack("n1"))
	want := []string{`notification n1 https://chat.test https://chat.test/room "Ana" "hello"`, "closed n1"}
	if !reflect.DeepEqual(r.got, want) {
		t.Fatalf("got %v", r.got)
	}
}

func TestAConnectionChangeReachesTheListener(t *testing.T) {
	h, r := wired(t)
	h.Dispatch(ConnectionChanged, pack(3, int(engine.ConnectionInsecure), "certificate"))
	if !reflect.DeepEqual(r.got, []string{"connection 3 2 certificate"}) {
		t.Fatalf("got %v", r.got)
	}
}

func TestACertificateErrorAsksTheListener(t *testing.T) {
	h, r := wired(t)
	v, ok := h.Dispatch(CertificateError, pack(3, "https://old.test/", "expired", true, true))
	if !ok || v.I != 0 {
		t.Fatalf("answered %v %v", ok, v)
	}
	if !reflect.DeepEqual(r.got, []string{"certificate 3 https://old.test/ expired main=true passable=true"}) {
		t.Fatalf("got %v", r.got)
	}
}

func TestAFrameAsksForItsSparksAndAnAdBlockIsTold(t *testing.T) {
	h, r := wired(t)
	v, ok := h.Dispatch(SparksFor, pack(3, "https://a.test/", true))
	if !ok || v.S != `[{"id":"ab12cd34","source":"1","runAt":"document-end"}]` {
		t.Fatalf("answered %v %q", ok, v.S)
	}
	h.Dispatch(AdBlocked, pack(3))
	if !reflect.DeepEqual(r.got, []string{"sparks 3 https://a.test/ main=true", "ad 3"}) {
		t.Fatalf("got %v", r.got)
	}
}

func TestAPageAsksToPrint(t *testing.T) {
	h, r := wired(t)
	v, ok := h.Dispatch(PrintRequested, pack(3))
	if !ok || v.I != 1 {
		t.Fatalf("answered %v %v", ok, v)
	}
	if !reflect.DeepEqual(r.got, []string{"print 3"}) {
		t.Fatalf("got %v", r.got)
	}
}

// A form script's message comes with the frame as the engine knows it: its bits are the
// tab's own frame, a frame of another origin above it, no origin of its own, and https
// with a certificate that checks.
func TestAFormScriptsMessageComesStamped(t *testing.T) {
	h, r := wired(t)
	h.Dispatch(FormMessage, pack(3, "frame-9", "doc-4", "https://pay.test", 2|8, 40, 300, "pressed", `{"ref":"f1"}`))
	want := `form {Tab:3 Frame:frame-9 Document:doc-4 Origin:https://pay.test Top:false CrossOrigin:true Opaque:false Secure:true X:40 Y:300} pressed {"ref":"f1"}`
	if !reflect.DeepEqual(r.got, []string{want}) {
		t.Fatalf("got %v", r.got)
	}
}
