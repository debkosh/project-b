package content

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
		return []Value{boolValue(v.Bool())}
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

// recorder is an engine.Listener that writes down what it hears.
type recorder struct {
	got []string
	// answer is what NavigationStarting says.
	answer engine.Navigation
}

func (r *recorder) note(s string) { r.got = append(r.got, s) }
func (r *recorder) ShellMessage(w engine.WindowID, msg []byte) {
	r.note(fmt.Sprintf("shell %d %s", w, msg))
}
func (r *recorder) NavigationStarting(tab engine.TabID, nav int64, url string) engine.Navigation {
	r.note("starting " + url)
	return r.answer
}
func (r *recorder) NavigationFinished(tab engine.TabID, nav int64, url string) {
	r.note("finished " + url)
}
func (r *recorder) NavigationFailed(tab engine.TabID, nav int64, url string, reason engine.Failure, code string) {
	r.note("failed " + url + " " + string(reason))
}
func (r *recorder) HistoryChanged(tab engine.TabID, back, fwd bool) {
	if back {
		r.note("canGoBack")
	}
}
func (r *recorder) TitleChanged(tab engine.TabID, title string) { r.note("title " + title) }
func (r *recorder) LoadingChanged(tab engine.TabID, loading bool) {
	if loading {
		r.note("loading")
	} else {
		r.note("loaded")
	}
}
func (r *recorder) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	r.note(fmt.Sprintf("menu %d %s %s %s", tab, m.Link, m.Src, m.Media))
}
func (r *recorder) ScrollDirectionChanged(tab engine.TabID, down bool) {
	r.note(fmt.Sprintf("scroll %d %v", tab, down))
}

func (r *recorder) LoadProgressChanged(tab engine.TabID, progress float64) {
	r.note(fmt.Sprintf("progress %.2f", progress))
}
func (r *recorder) PageCrashed(tab engine.TabID)                    { r.note("crashed") }
func (r *recorder) FaviconChanged(tab engine.TabID, dataURL string) { r.note("favicon " + dataURL) }
func (r *recorder) TabOpened(opener, tab engine.TabID, url, workspace string, foreground, newWindow bool) {
	r.note(fmt.Sprint("opened ", url, " in ", workspace, " new window ", newWindow))
}
func (r *recorder) CloseRequested(tab engine.TabID)    { r.note(fmt.Sprintf("close %d", tab)) }
func (r *recorder) ActivateRequested(tab engine.TabID) { r.note(fmt.Sprintf("activate %d", tab)) }
func (r *recorder) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	r.note(fmt.Sprintf("key %d %d %d %d", k.Code, k.Mods, phase, in))
	return k.Mods&engine.ModCmd != 0
}
func (r *recorder) ModsChanged(m engine.Mods)                { r.note(fmt.Sprint("mods ", m)) }
func (r *recorder) MenuChosen(action string)                 { r.note("menu " + action) }
func (r *recorder) MenuList(name string) engine.Menu         { r.note("menu list " + name); return nil }
func (r *recorder) OSBindings(byTitle map[string]engine.Key) { r.note(fmt.Sprint("os ", byTitle)) }
func (r *recorder) SettingsMessage(tab engine.TabID, msg []byte) {
	r.note(fmt.Sprintf("settings %d %s", tab, msg))
}
func (r *recorder) PickerMessage(w engine.WindowID, msg []byte) {
	r.note(fmt.Sprintf("picker %d %s", w, msg))
}
func (r *recorder) WindowChanged(w engine.WindowID, s engine.WindowState) {
	r.note(fmt.Sprintf("window %d %gx%g r=%g full=%v", w, s.W, s.H, s.CornerRadius, s.FullScreen))
}
func (r *recorder) TabFocused(tab engine.TabID)     { r.note(fmt.Sprint("focused ", tab)) }
func (r *recorder) WindowClosed(w engine.WindowID)  { r.note(fmt.Sprintf("window %d closed", w)) }
func (r *recorder) WindowFocused(w engine.WindowID) { r.note(fmt.Sprintf("window %d focused", w)) }
func (r *recorder) ShellCrashed(w engine.WindowID)  { r.note(fmt.Sprintf("shell %d crashed", w)) }
func (r *recorder) ProcessCrashed(p engine.ProcessCrash) {
	r.note(fmt.Sprintf("%s %s %d %d %s", p.Type, p.Status, p.ExitCode, p.PID, p.URL))
}

func (r *recorder) DownloadTarget(d engine.DownloadRequest) engine.DownloadTarget {
	r.note(fmt.Sprintf("download %d %d %s %s", d.ID, d.Tab, d.URL, d.Name))
	return engine.DownloadTarget{Path: "/d/" + d.Name}
}
func (r *recorder) DownloadUpdated(d engine.Download) {
	r.note(fmt.Sprintf("downloaded %d %s %d/%d", d.ID, d.Path, d.Received, d.Total))
}

func (r *recorder) FileChooser(q engine.FileChooserRequest) engine.FileChooserAnswer {
	r.note(fmt.Sprintf("file chooser %d %d %v", q.Tab, q.Mode, q.Accept))
	return engine.FileChooserAnswer{Allow: true}
}
func (r *recorder) FilesChosen(tab engine.TabID, count int, folder string) {
	r.note(fmt.Sprintf("files chosen %d %d %s", tab, count, folder))
}

func (r *recorder) DialogOpened(d engine.Dialog) {
	r.note(fmt.Sprintf("dialog %d %s %s", d.ID, d.Kind, d.Message))
}
func (r *recorder) DialogClosed(id engine.DialogID) { r.note(fmt.Sprintf("dialog closed %d", id)) }

func (r *recorder) FindReply(tab engine.TabID, request, matches, active int, final bool) {
	r.note(fmt.Sprintf("find %d %d %d/%d %v", tab, request, active, matches, final))
}

func (r *recorder) PageFullScreen(tab engine.TabID, on bool) {
	r.note(fmt.Sprintf("full screen %d %v", tab, on))
}
func (r *recorder) IsPageFullScreen(engine.TabID) bool { return false }

func (r *recorder) DevToolsCommand(method string, params []byte) (any, error) {
	r.note("devtools " + method + " " + string(params))
	switch method {
	case "ProjectB.echo":
		return json.RawMessage(params), nil
	case "ProjectB.fail":
		return nil, errors.New("it failed")
	}
	return nil, engine.ErrUnknownCommand
}

func wired(t *testing.T) (*Hooks, *recorder, map[Hook]bool) {
	t.Helper()
	subscribed := map[Hook]bool{}
	h := New(func(id Hook) { subscribed[id] = true })
	r := &recorder{}
	Wire(h, r)
	return h, r, subscribed
}

// The app is told through events alone, so wiring it lets no hook cross.
func TestTheAppSubscribesToNoHook(t *testing.T) {
	_, _, subscribed := wired(t)
	if len(subscribed) != 0 {
		t.Fatalf("subscribed to %v", subscribed)
	}
}

// A hook still crosses for whoever subscribes to it.
func TestASubscribedHookReachesItsHandler(t *testing.T) {
	h, _, subscribed := wired(t)
	var got []float64
	h.Observer.OnLoadProgressChanged(func(c WebContents, progress float64) { got = append(got, progress) })
	if !subscribed[ObserverLoadProgressChanged] {
		t.Fatal("not subscribed")
	}
	h.Dispatch(ObserverLoadProgressChanged, pack(WebContents{ID: 2}, 0.4))
	if !reflect.DeepEqual(got, []float64{0.4}) {
		t.Fatalf("got %v", got)
	}
}

func TestEngineEventsNeedNoSubscription(t *testing.T) {
	h, r, _ := wired(t)
	h.Dispatch(eventTabOpened, []Value{{I: 0}, {I: 2}, {S: "https://p.test/"}, {S: "a1b2c3d4"}, {I: 1}, {I: 1}})
	h.Dispatch(eventWindowChanged, []Value{{I: 2}, {D: 1100}, {D: 700}, {D: 1}, {D: 2}, {D: 3}, {D: 4}, {D: 20}, {I: 1}})
	h.Dispatch(eventShellCrashed, []Value{{I: 2}})
	h.Dispatch(eventShellMessage, []Value{{I: 2}, {S: `{"t":"ready"}`}})
	h.Dispatch(eventSettingsMsg, []Value{{I: 2}, {S: `{"t":"ready"}`}})
	h.Dispatch(eventPickerMessage, []Value{{I: 2}, {S: `{"t":"open"}`}})
	h.Dispatch(eventWindowFocused, []Value{{I: 2}})
	h.Dispatch(eventWindowClosed, []Value{{I: 2}})
	h.Dispatch(eventProcessCrashed, []Value{{S: "renderer"}, {S: "crashed"}, {I: 11}, {I: 42}, {S: "https://p.test/"}})
	if !reflect.DeepEqual(r.got, []string{"opened https://p.test/ in a1b2c3d4 new window true", "window 2 1100x700 r=20 full=true", "shell 2 crashed", `shell 2 {"t":"ready"}`, `settings 2 {"t":"ready"}`, `picker 2 {"t":"open"}`, "window 2 focused", "window 2 closed", "renderer crashed 11 42 https://p.test/"}) {
		t.Fatalf("got %v", r.got)
	}
	if h.Counts()["engine.TabOpened"] != 1 {
		t.Fatalf("counts %v", h.Counts())
	}
}

func TestDevToolsCommandsAnswerWithTheirResultOrError(t *testing.T) {
	h, _, _ := wired(t)
	for _, c := range []struct{ method, params, answer string }{
		{"ProjectB.echo", `{"a":1}`, `{"result":{"a":1}}`},
		{"ProjectB.fail", `{}`, `{"error":{"code":-32000,"message":"it failed"}}`},
	} {
		v, ok := h.Dispatch(eventDevTools, pack(c.method, c.params))
		if !ok || v.S != c.answer {
			t.Errorf("%s: %v %q, want %q", c.method, ok, v.S, c.answer)
		}
	}
	if _, ok := h.Dispatch(eventDevTools, pack("ProjectB.nothing", `{}`)); ok {
		t.Error("an unknown command was answered, not left to content")
	}
}
