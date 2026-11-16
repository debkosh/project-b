package abi

import (
	"encoding/json"
	"errors"
	"strings"

	"projectb/engine"
	"projectb/timeline"
)

// The adapter's events of windows, our pages, menus, crashes, downloads, dialogs and
// permissions. The numbers and value layouts match PBEvent in engine/abi/pb_abi.h.
const (
	ShellMessage        Event = 1
	TabOpened           Event = 2
	FaviconChanged      Event = 3
	WindowChanged       Event = 4
	Ping                Event = 5 // The driver measures a crossing; nothing listens.
	ShellCrashed        Event = 6
	MenuChosen          Event = 8
	OSBindings          Event = 9
	ModsChanged         Event = 10
	ProcessCrashed      Event = 11
	DevToolsCommand     Event = 12
	WindowClosed        Event = 13
	WindowFocused       Event = 14
	MenuList            Event = 15
	PickerMessage       Event = 16
	DownloadTarget      Event = 17
	DownloadUpdated     Event = 18
	FileChooser         Event = 19
	FilesChosen         Event = 20
	DialogOpened        Event = 21
	DialogClosed        Event = 22
	PageMessage         Event = 23
	PermissionRequested Event = 40
	PermissionStatusOf  Event = 41
	NotificationShown   Event = 42
	NotificationClicked Event = 43
	ConnectionChanged   Event = 44
	CertificateError    Event = 45
	PrintRequested      Event = 46
	SparksFor           Event = 47
	AdBlocked           Event = 48
)

func init() {
	for id, name := range map[Event]string{
		ShellMessage: "engine.ShellMessage", TabOpened: "engine.TabOpened", FaviconChanged: "engine.FaviconChanged",
		WindowChanged: "engine.WindowChanged", Ping: "engine.Ping", ShellCrashed: "engine.ShellCrashed",
		MenuChosen: "engine.MenuChosen", OSBindings: "engine.OSBindings", ModsChanged: "engine.ModsChanged",
		ProcessCrashed: "engine.ProcessCrashed", DevToolsCommand: "engine.DevToolsCommand",
		WindowClosed: "engine.WindowClosed", WindowFocused: "engine.WindowFocused", MenuList: "engine.MenuList",
		PickerMessage: "engine.PickerMessage", DownloadTarget: "engine.DownloadTarget",
		DownloadUpdated: "engine.DownloadUpdated", FileChooser: "engine.FileChooser", FilesChosen: "engine.FilesChosen",
		DialogOpened: "engine.DialogOpened", DialogClosed: "engine.DialogClosed", PageMessage: "engine.PageMessage",
		PermissionRequested: "engine.PermissionRequested", PermissionStatusOf: "engine.PermissionStatusOf",
		NotificationShown: "engine.NotificationShown", NotificationClicked: "engine.NotificationClicked",
		ConnectionChanged: "engine.ConnectionChanged", CertificateError: "engine.CertificateError",
		SparksFor: "engine.SparksFor", AdBlocked: "engine.AdBlocked", PrintRequested: "engine.PrintRequested",
	} {
		names[id] = name
	}
}

// dialogKinds name PBDialogKind's values.
var dialogKinds = []string{"alert", "confirm", "prompt", "beforeunload", "signin"}

// adapter decodes the events below 60.
func (e *Events) adapter(id Event, l engine.Listener, r *reader) (Value, bool) {
	switch id {
	case ShellMessage:
		l.ShellMessage(engine.WindowID(r.int()), []byte(r.str()))
	case PickerMessage:
		l.PickerMessage(engine.WindowID(r.int()), []byte(r.str()))
	case PageMessage:
		l.PageMessage(engine.TabID(r.int()), engine.Page(r.str()), []byte(r.str()))
	case TabOpened:
		l.TabOpened(engine.TabID(r.int()), engine.TabID(r.int()), r.str(), r.str(), r.bool(), r.bool())
	case FaviconChanged:
		l.FaviconChanged(engine.TabID(r.int()), r.str())
	case WindowChanged:
		l.WindowChanged(engine.WindowID(r.int()), engine.WindowState{
			W:            r.float(),
			H:            r.float(),
			Controls:     engine.Rect{X: r.float(), Y: r.float(), W: r.float(), H: r.float()},
			CornerRadius: r.float(),
			FullScreen:   r.bool(),
			Bounds:       engine.Rect{X: r.float(), Y: r.float(), W: r.float(), H: r.float()},
			Minimised:    r.bool(),
			Zoomed:       r.bool(),
		})
	case ShellCrashed:
		l.ShellCrashed(engine.WindowID(r.int()))
	case WindowClosed:
		l.WindowClosed(engine.WindowID(r.int()))
	case WindowFocused:
		l.WindowFocused(engine.WindowID(r.int()))
	case MenuChosen:
		l.MenuChosen(r.str())
	case ModsChanged:
		l.ModsChanged(engine.Mods(r.int()))
	case OSBindings:
		// JSON: each title with the key's Windows virtual-key code and our modifiers.
		var byTitle map[string]engine.Key
		if err := json.Unmarshal([]byte(r.str()), &byTitle); err != nil {
			timeline.Warnf("abi: bad OS bindings: %v", err)
			return Value{}, false
		}
		l.OSBindings(byTitle)
	case ProcessCrashed:
		l.ProcessCrashed(engine.ProcessCrash{Type: r.str(), Status: r.str(), ExitCode: int(r.int()), PID: int(r.int()), URL: r.str()})
	case MenuList:
		// The answer is the list's items now, as the menu bar's JSON has them.
		b, err := json.Marshal(l.MenuList(r.str()))
		if err != nil {
			timeline.Errorf("abi: a menu list: %v", err)
			return Value{}, false
		}
		return Value{S: string(b)}, true
	case DevToolsCommand:
		return devTools(l, r)
	case DownloadTarget:
		// The answer is where it goes, as JSON; none refuses it.
		t := l.DownloadTarget(engine.DownloadRequest{ID: engine.DownloadID(r.int()), Tab: engine.TabID(r.int()), URL: r.str(), Name: r.str(), Mime: r.str(), SaveAs: r.bool()})
		b, _ := json.Marshal(t)
		return Value{S: string(b)}, true
	case DownloadUpdated:
		l.DownloadUpdated(engine.Download{ID: engine.DownloadID(r.int()), Path: r.str(), URL: r.str(), Mime: r.str(),
			State: engine.DownloadState(r.int()), Received: r.int(), Total: r.int(), Paused: r.bool()})
	case FileChooser:
		// The answer is whether the panel opens, and where, as JSON; none refuses it.
		req := engine.FileChooserRequest{Tab: engine.TabID(r.int()), Mode: engine.FileChooserMode(r.int())}
		if accept := r.str(); accept != "" {
			req.Accept = strings.Split(accept, ",")
		}
		b, _ := json.Marshal(l.FileChooser(req))
		return Value{S: string(b)}, true
	case FilesChosen:
		l.FilesChosen(engine.TabID(r.int()), int(r.int()), r.str())
	case DialogOpened:
		d := engine.Dialog{ID: engine.DialogID(r.int()), Tab: engine.TabID(r.int())}
		if k := int(r.int()); k >= 0 && k < len(dialogKinds) {
			d.Kind = dialogKinds[k]
		}
		d.Message, d.DefaultText, d.Origin, d.Reload = r.str(), r.str(), r.str(), r.bool()
		l.DialogOpened(d)
	case DialogClosed:
		l.DialogClosed(engine.DialogID(r.int()))
	case PermissionRequested:
		l.PermissionRequested(engine.PermissionRequest{ID: engine.PermissionID(r.int()), Tab: engine.TabID(r.int()), Origin: r.str(), Kinds: strings.Split(r.str(), ","), Gesture: r.bool()})
	case PermissionStatusOf:
		return Value{I: int64(l.PermissionStatusOf(engine.TabID(r.int()), r.str(), r.str()))}, true
	case NotificationShown:
		n := engine.Notification{ID: r.str(), Origin: r.str(), Page: r.str(), Title: r.str(), Body: r.str()}
		return yes(l.NotificationShown(n)), true
	case NotificationClicked:
		l.NotificationClicked(r.str())
	case ConnectionChanged:
		l.ConnectionChanged(engine.TabID(r.int()), engine.Connection{State: engine.ConnectionState(r.int()), Reason: r.str()})
	case CertificateError:
		tab := engine.TabID(r.int())
		return yes(l.CertificateError(tab, engine.CertificateError{URL: r.str(), Reason: r.str(), MainFrame: r.bool(), Passable: r.bool()})), true
	case SparksFor:
		scripts := l.SparksFor(engine.TabID(r.int()), r.str(), r.bool())
		if len(scripts) == 0 {
			return Value{}, true
		}
		b, _ := json.Marshal(scripts)
		return Value{S: string(b)}, true
	case AdBlocked:
		l.AdBlocked(engine.TabID(r.int()))
	case PrintRequested:
		return yes(l.PrintRequested(engine.TabID(r.int()))), true
	}
	return Value{}, false
}

// devTools answers one of our DevTools commands with the reply's body as DevTools has
// it, its result or its error. No answer leaves an unknown method to the engine.
func devTools(l engine.Listener, r *reader) (Value, bool) {
	method, params := r.str(), r.str()
	result, err := l.DevToolsCommand(method, []byte(params))
	if errors.Is(err, engine.ErrUnknownCommand) {
		return Value{}, false
	}
	body := map[string]any{"result": result}
	if err != nil {
		body = map[string]any{"error": map[string]any{"code": -32000, "message": err.Error()}}
	}
	b, err := json.Marshal(body)
	if err != nil {
		b, _ = json.Marshal(map[string]any{"error": map[string]any{"code": -32603, "message": err.Error()}})
	}
	return Value{S: string(b)}, true
}
