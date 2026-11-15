package abi

import (
	"fmt"
	"reflect"
	"testing"

	"projectb/engine"
)

// recorder writes down the events it hears. The rest of engine.Listener is not called
// here.
type recorder struct {
	engine.Listener
	got        []string
	allow      bool
	fullScreen bool
}

func (r *recorder) note(format string, args ...any) {
	r.got = append(r.got, fmt.Sprintf(format, args...))
}
func (r *recorder) NavigationStarting(tab engine.TabID, nav int64, url string) bool {
	r.note("starting %d %d %s", tab, nav, url)
	return r.allow
}
func (r *recorder) NavigationFinished(tab engine.TabID, nav int64, url string) {
	r.note("finished %d %d %s", tab, nav, url)
}
func (r *recorder) NavigationFailed(tab engine.TabID, nav int64, url, reason string) {
	r.note("failed %d %d %s %s", tab, nav, url, reason)
}
func (r *recorder) HistoryChanged(tab engine.TabID, back, forward bool) {
	r.note("history %d %v %v", tab, back, forward)
}
func (r *recorder) TitleChanged(tab engine.TabID, title string) { r.note("title %d %s", tab, title) }
func (r *recorder) LoadingChanged(tab engine.TabID, loading bool) {
	r.note("loading %d %v", tab, loading)
}
func (r *recorder) LoadProgressChanged(tab engine.TabID, progress float64) {
	r.note("progress %d %.2f", tab, progress)
}
func (r *recorder) ContextMenu(tab engine.TabID, m engine.ContextMenu) {
	r.note("menu %d %+v", tab, m)
}
func (r *recorder) ScrollDirectionChanged(tab engine.TabID, down bool) {
	r.note("scroll %d %v", tab, down)
}
func (r *recorder) TabFocused(tab engine.TabID)        { r.note("focused %d", tab) }
func (r *recorder) PageCrashed(tab engine.TabID)       { r.note("crashed %d", tab) }
func (r *recorder) CloseRequested(tab engine.TabID)    { r.note("close %d", tab) }
func (r *recorder) ActivateRequested(tab engine.TabID) { r.note("activate %d", tab) }
func (r *recorder) Key(k engine.Key, phase engine.KeyPhase, in engine.Surface) bool {
	r.note("key %d %d %d %d", k.Code, k.Mods, phase, in)
	return k.Mods&engine.ModCmd != 0
}
func (r *recorder) FindReply(tab engine.TabID, request, matches, active int, final bool) {
	r.note("find %d %d %d/%d %v", tab, request, active, matches, final)
}
func (r *recorder) PageFullScreen(tab engine.TabID, on bool) {
	r.note("full screen %d %v", tab, on)
	r.fullScreen = on
}
func (r *recorder) IsPageFullScreen(engine.TabID) bool { return r.fullScreen }

func listening() (*Events, *recorder) {
	e, r := New(), &recorder{allow: true}
	e.Listen(r)
	return e, r
}

func i(n int64) Value   { return Value{I: n} }
func s(t string) Value  { return Value{S: t} }
func d(f float64) Value { return Value{D: f} }

// Each event that only informs becomes the Listener call of its name, with its values in
// the order pb_abi.h lists them.
func TestEachEventReachesTheListener(t *testing.T) {
	e, r := listening()
	for _, c := range []struct {
		id   Event
		args []Value
		want string
	}{
		{NavigationFinished, []Value{i(3), i(41), s("https://a.test/")}, "finished 3 41 https://a.test/"},
		{NavigationFailed, []Value{i(3), i(42), s("https://x.test/"), s("net::ERR_NAME_NOT_RESOLVED")}, "failed 3 42 https://x.test/ net::ERR_NAME_NOT_RESOLVED"},
		{HistoryChanged, []Value{i(3), i(1), i(0)}, "history 3 true false"},
		{TitleChanged, []Value{i(3), s("A")}, "title 3 A"},
		{LoadingChanged, []Value{i(3), i(1)}, "loading 3 true"},
		{LoadingChanged, []Value{i(3), i(0)}, "loading 3 false"},
		{LoadProgressChanged, []Value{i(2), d(0.4)}, "progress 2 0.40"},
		{ScrollDirectionChanged, []Value{i(2), i(1)}, "scroll 2 true"},
		{ScrollDirectionChanged, []Value{i(2), i(0)}, "scroll 2 false"},
		{TabFocused, []Value{i(5)}, "focused 5"},
		{PageCrashed, []Value{i(5)}, "crashed 5"},
		{CloseRequested, []Value{i(4)}, "close 4"},
		{ActivateRequested, []Value{i(5)}, "activate 5"},
		{FindReply, []Value{i(2), i(7), i(12), i(3), i(1)}, "find 2 7 3/12 true"},
		{PageFullScreen, []Value{i(2), i(1)}, "full screen 2 true"},
	} {
		r.got = nil
		if _, answered := e.Dispatch(c.id, c.args); answered {
			t.Errorf("%s was answered", names[c.id])
		}
		if !reflect.DeepEqual(r.got, []string{c.want}) {
			t.Errorf("%s: got %q, want %q", names[c.id], r.got, c.want)
		}
	}
}

// The gate's answer is the app's: 1 lets the navigation go on, 0 blocks it.
func TestTheGateAnswersWithTheAppsDecision(t *testing.T) {
	e, r := listening()
	args := []Value{i(2), i(41), s("https://blocked.test/")}
	if v, ok := e.Dispatch(NavigationStarting, args); !ok || v.I != 1 {
		t.Fatalf("allowed: %v %v", v, ok)
	}
	r.allow = false
	if v, ok := e.Dispatch(NavigationStarting, args); !ok || v.I != 0 {
		t.Fatalf("blocked: %v %v", v, ok)
	}
	if !reflect.DeepEqual(r.got, []string{"starting 2 41 https://blocked.test/", "starting 2 41 https://blocked.test/"}) {
		t.Fatalf("got %q", r.got)
	}
}

// The recorder takes any Cmd key: the app's answer is the event's.
func TestKeysReachTheAppWithItsAnswer(t *testing.T) {
	e, r := listening()
	key := func(code int64, mods engine.Mods, phase engine.KeyPhase, in engine.Surface) int64 {
		v, ok := e.Dispatch(Key, []Value{i(code), i(int64(mods)), i(int64(phase)), i(int64(in))})
		if !ok {
			t.Fatal("a key got no answer")
		}
		return v.I
	}
	if key('W', engine.ModCmd|engine.ModShift, engine.KeyBeforePage, engine.SurfacePage) != 1 {
		t.Error("Cmd+Shift+W before the page was not used")
	}
	if key('W', 0, engine.KeyBeforePage, engine.SurfacePage) != 0 {
		t.Error("a W the app didn't want was taken from the page")
	}
	if key('L', engine.ModCmd, engine.KeyUnhandled, engine.SurfaceSettings) != 1 {
		t.Error("Cmd+L after the settings page was not used")
	}
	want := []string{"key 87 12 1 1", "key 87 0 1 1", "key 76 8 2 3"}
	if !reflect.DeepEqual(r.got, want) {
		t.Fatalf("got %q, want %q", r.got, want)
	}
}

// A right click in a tab is the shell's to show, wherever it was, and the engine is told
// so.
func TestARightClickIsTheShells(t *testing.T) {
	e, r := listening()
	v, ok := e.Dispatch(ContextMenu, []Value{i(2), d(10), d(20), s("https://a.test/"), s("A"), s("https://a.test/i.png"), s("image"), s("words"), i(1)})
	if !ok || v.I != 1 {
		t.Fatalf("answer %v %v", v, ok)
	}
	want := fmt.Sprintf("menu 2 %+v", engine.ContextMenu{X: 10, Y: 20, Link: "https://a.test/", LinkText: "A", Src: "https://a.test/i.png", Media: "image", Selection: "words", Editable: true})
	if !reflect.DeepEqual(r.got, []string{want}) {
		t.Fatalf("got %q, want %q", r.got, want)
	}
	if v, ok := e.Dispatch(ContextMenu, []Value{i(2), d(0), d(0), s(""), s(""), s(""), s(""), s("words"), i(0)}); !ok || v.I != 1 {
		t.Fatalf("a right click on text: %v %v", v, ok)
	}
}

// On a phone only a long press on a link or an image is the shell's; one on text is left
// to the engine.
func TestALongPressOnTextStaysThePlatforms(t *testing.T) {
	linksAndImagesOnly = true
	t.Cleanup(func() { linksAndImagesOnly = false })
	e, r := listening()
	ask := func(link, src, selection string) int64 {
		v, ok := e.Dispatch(ContextMenu, []Value{i(2), d(0), d(0), s(link), s(""), s(src), s(""), s(selection), i(0)})
		if !ok {
			t.Fatal("no answer")
		}
		return v.I
	}
	if ask("https://a.test/", "", "") != 1 || ask("", "https://a.test/i.png", "") != 1 || ask("", "", "words") != 0 {
		t.Fatalf("answers wrong, got %q", r.got)
	}
	if len(r.got) != 2 {
		t.Fatalf("got %q", r.got)
	}
}

// The engine asks whether a page has the screen, and hears what the app last decided.
func TestTheEngineAsksWhetherAPageIsFullScreen(t *testing.T) {
	e, _ := listening()
	if v, ok := e.Dispatch(IsPageFullScreen, []Value{i(2)}); !ok || v.I != 0 {
		t.Fatalf("before: %v %v", v, ok)
	}
	e.Dispatch(PageFullScreen, []Value{i(2), i(1)})
	if v, ok := e.Dispatch(IsPageFullScreen, []Value{i(2)}); !ok || v.I != 1 {
		t.Fatalf("after: %v %v", v, ok)
	}
}

// An event before anyone listens is dropped, and a question then has no answer, so the
// engine does what it would have.
func TestEventsBeforeAListenerAreDropped(t *testing.T) {
	e := New()
	if _, ok := e.Dispatch(NavigationStarting, []Value{i(1), i(1), s("https://a.test/")}); ok {
		t.Fatal("answered with nobody listening")
	}
	e.Dispatch(TitleChanged, []Value{i(1), s("A")})
	if e.Counts()["engine.TitleChanged"] != 1 {
		t.Fatalf("counts %v", e.Counts())
	}
}

// Has holds for this package's numbers only: the earlier events and content's hooks are
// someone else's.
func TestHasIsTheRangeOfTheseEvents(t *testing.T) {
	for id, want := range map[int32]bool{1: false, 59: false, 60: true, 76: true, 79: true, 80: false, 100: false} {
		if Has(id) != want {
			t.Errorf("Has(%d) = %v", id, !want)
		}
	}
}
