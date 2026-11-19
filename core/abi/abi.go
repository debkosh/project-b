// Package abi is where an engine's events arrive in Go: the numbered events of
// engine/abi/pb_abi.h, each a flat list of values, turned into the engine.Listener call
// it means. It knows nothing of any one engine. Whatever an engine is built on, its
// adapter says what happened in these events, and the app hears it here. Content's own
// hooks, from 100 up, are core/content's.
package abi

import (
	"runtime"

	"projectb/engine"
)

// Value is one value as it crosses: an integer, a float or a string, by layout.
type Value struct {
	I int64
	D float64
	S string
}

// Event is an event's number, PBEvent in pb_abi.h.
type Event int32

// The events of a tab's page and of keys. The numbers and value layouts match PBEvent.
const (
	NavigationStarting     Event = 60
	NavigationFinished     Event = 61
	NavigationFailed       Event = 62
	HistoryChanged         Event = 63
	TitleChanged           Event = 64
	LoadingChanged         Event = 65
	LoadProgressChanged    Event = 66
	ContextMenu            Event = 67
	ScrollDirectionChanged Event = 68
	TabFocused             Event = 69
	PageCrashed            Event = 70
	CloseRequested         Event = 71
	ActivateRequested      Event = 72
	Key                    Event = 73
	FindReply              Event = 74
	PageFullScreen         Event = 75
	IsPageFullScreen       Event = 76
	Thumbnail              Event = 77
	// 78 and 79 were the default browser's and notifications' answers, which core/system
	// has now.
	FolderChosen   Event = 80
	AppLink        Event = 81
	ImageSaved     Event = 82
	DownloadAction Event = 83
)

var names = map[Event]string{
	NavigationStarting:     "engine.NavigationStarting",
	NavigationFinished:     "engine.NavigationFinished",
	NavigationFailed:       "engine.NavigationFailed",
	HistoryChanged:         "engine.HistoryChanged",
	TitleChanged:           "engine.TitleChanged",
	LoadingChanged:         "engine.LoadingChanged",
	LoadProgressChanged:    "engine.LoadProgressChanged",
	ContextMenu:            "engine.ContextMenu",
	ScrollDirectionChanged: "engine.ScrollDirectionChanged",
	TabFocused:             "engine.TabFocused",
	PageCrashed:            "engine.PageCrashed",
	CloseRequested:         "engine.CloseRequested",
	ActivateRequested:      "engine.ActivateRequested",
	Key:                    "engine.Key",
	FindReply:              "engine.FindReply",
	PageFullScreen:         "engine.PageFullScreen",
	IsPageFullScreen:       "engine.IsPageFullScreen",
	Thumbnail:              "engine.Thumbnail",
	FolderChosen:           "engine.FolderChosen",
	AppLink:                "engine.AppLink",
	ImageSaved:             "engine.ImageSaved",
	DownloadAction:         "engine.DownloadNotificationCommand",
}

// linksAndImagesOnly is a phone's way: only a long press on a link or an image is the
// shell's, and one on text or in a field keeps the platform's own selection bar.
var linksAndImagesOnly = runtime.GOOS == "android"

// Events hands each event to the listener. There is one per process; cmd/lib makes it
// and gives it every crossing whose number Has holds.
type Events struct {
	l      engine.Listener
	counts map[Event]int
	// more are the events a build tag adds, by number, with their own listener.
	more map[Event]func(r *reader) (Value, bool)
}

// New returns an Events with nobody listening yet.
func New() *Events {
	return &Events{counts: map[Event]int{}, more: map[Event]func(r *reader) (Value, bool){}}
}

// Listen says who hears the events from now on: the picker, then the app.
func (e *Events) Listen(l engine.Listener) { e.l = l }

// Has reports whether id is an event decoded here: every number below content's hooks,
// which start at 100.
func Has(id int32) bool { return id > 0 && id < 100 }

// Dispatch takes one crossing and returns the answer, for an event that asks. An event
// that arrives before anyone listens is dropped, and one that asks gets no answer.
func (e *Events) Dispatch(id Event, args []Value) (Value, bool) {
	e.counts[id]++
	r := &reader{v: args}
	if f := e.more[id]; f != nil {
		return f(r)
	}
	l := e.l
	if l == nil {
		return Value{}, false
	}
	switch id {
	case NavigationStarting:
		return Value{I: navigationAnswers[l.NavigationStarting(engine.TabID(r.int()), r.int(), r.str())]}, true
	case NavigationFinished:
		l.NavigationFinished(engine.TabID(r.int()), r.int(), r.str())
	case NavigationFailed:
		l.NavigationFailed(engine.TabID(r.int()), r.int(), r.str(), engine.Failure(r.str()), r.str())
	case HistoryChanged:
		l.HistoryChanged(engine.TabID(r.int()), r.bool(), r.bool())
	case TitleChanged:
		l.TitleChanged(engine.TabID(r.int()), r.str())
	case LoadingChanged:
		l.LoadingChanged(engine.TabID(r.int()), r.bool())
	case LoadProgressChanged:
		l.LoadProgressChanged(engine.TabID(r.int()), r.float())
	case ContextMenu:
		tab := engine.TabID(r.int())
		m := engine.ContextMenu{X: r.float(), Y: r.float(), Link: r.str(), LinkText: r.str(),
			Src: r.str(), Media: r.str(), Selection: r.str(), Editable: r.bool()}
		if linksAndImagesOnly && m.Link == "" && m.Src == "" {
			return yes(false), true
		}
		l.ContextMenu(tab, m)
		return yes(true), true
	case ScrollDirectionChanged:
		l.ScrollDirectionChanged(engine.TabID(r.int()), r.bool())
	case TabFocused:
		l.TabFocused(engine.TabID(r.int()))
	case PageCrashed:
		l.PageCrashed(engine.TabID(r.int()))
	case CloseRequested:
		l.CloseRequested(engine.TabID(r.int()))
	case ActivateRequested:
		l.ActivateRequested(engine.TabID(r.int()))
	case Key:
		k := engine.Key{Code: int(r.int()), Mods: engine.Mods(r.int())}
		return yes(l.Key(k, engine.KeyPhase(r.int()), engine.Surface(r.int()))), true
	case FindReply:
		l.FindReply(engine.TabID(r.int()), int(r.int()), int(r.int()), int(r.int()), r.bool())
	case PageFullScreen:
		l.PageFullScreen(engine.TabID(r.int()), r.bool())
	case IsPageFullScreen:
		return yes(l.IsPageFullScreen(engine.TabID(r.int()))), true
	case Thumbnail:
		l.Thumbnail(engine.TabID(r.int()), r.int())
	case FolderChosen:
		l.FolderChosen(engine.WindowID(r.int()), r.str())
	case AppLink:
		l.AppLink(engine.TabID(r.int()), r.str())
	case ImageSaved:
		l.ImageSaved(r.str(), r.bool())
	case DownloadAction:
		l.DownloadNotificationCommand(int(r.int()), engine.DownloadCommand(r.int()))
	default:
		return e.adapter(id, l, r)
	}
	return Value{}, false
}

// Counts returns how often each event has crossed, by name. For stats.
func (e *Events) Counts() map[string]int {
	out := map[string]int{}
	for id, n := range e.counts {
		out[names[id]] = n
	}
	return out
}

// navigationAnswers are PBNavigation's numbers for the app's answers to a navigation.
var navigationAnswers = map[engine.Navigation]int64{
	engine.NavigationBlock:     0,
	engine.NavigationProceed:   1,
	engine.NavigationElsewhere: 2,
}

func yes(b bool) Value {
	if b {
		return Value{I: 1}
	}
	return Value{}
}

// reader walks an event's values in layout order.
type reader struct {
	v []Value
	i int
}

func (r *reader) next() Value {
	if r.i >= len(r.v) {
		return Value{}
	}
	v := r.v[r.i]
	r.i++
	return v
}

func (r *reader) int() int64     { return r.next().I }
func (r *reader) bool() bool     { return r.next().I != 0 }
func (r *reader) float() float64 { return r.next().D }
func (r *reader) str() string    { return r.next().S }
