//go:build extensions

package abi

import (
	"encoding/json"

	"projectb/engine"
	"projectb/timeline"
)

// The adapter's extension events. The numbers and value layouts match PBExtensionEvent in
// engine/abi/pb_extensions_abi.h.
const (
	ExtensionsChanged      Event = 50
	ExtensionInstallPrompt Event = 51
	ExtensionInstallDone   Event = 52
	ExtensionCall          Event = 53
	ExtensionBlockedCount  Event = 54
	ExtensionPopupClosed   Event = 55
	PathChosen             Event = 56
)

// ListenExtensions turns the adapter's extension events into l's calls.
func (e *Events) ListenExtensions(l engine.ExtensionsListener) {
	on := func(id Event, name string, f func(r *reader)) {
		names[id] = name
		e.more[id] = func(r *reader) (Value, bool) {
			f(r)
			return Value{}, false
		}
	}
	on(ExtensionsChanged, "engine.ExtensionsChanged", func(*reader) { l.ExtensionsChanged() })
	on(ExtensionInstallPrompt, "engine.ExtensionInstallPrompt", func(r *reader) {
		p := engine.InstallPrompt{ID: r.int()}
		if err := json.Unmarshal([]byte(r.str()), &p); err != nil {
			timeline.Warnf("abi: bad install prompt: %v", err)
			return
		}
		l.ExtensionInstallPrompt(p)
	})
	on(ExtensionInstallDone, "engine.ExtensionInstallDone", func(r *reader) {
		l.ExtensionInstallDone(engine.InstallResult{Path: r.str(), Extension: r.str(), Error: r.str(), Cancelled: r.bool()})
	})
	on(ExtensionBlockedCount, "engine.ExtensionBlockedCount", func(r *reader) {
		l.ExtensionBlockedCount(r.str(), engine.TabID(r.int()), int(r.int()))
	})
	on(ExtensionPopupClosed, "engine.ExtensionPopupClosed", func(*reader) { l.ExtensionPopupClosed() })
	on(PathChosen, "engine.PathChosen", func(r *reader) { l.PathChosen(r.str()) })

	// The one event with an answer: {"result": ...}, {} for none, or {"error": "..."}.
	names[ExtensionCall] = "engine.ExtensionCall"
	e.more[ExtensionCall] = func(r *reader) (Value, bool) {
		result, failure := l.ExtensionCall(engine.ExtensionCall{Extension: r.str(), Name: r.str(), Args: r.str(), Tab: engine.TabID(r.int())})
		answer := struct {
			Result json.RawMessage `json:"result,omitempty"`
			Error  string          `json:"error,omitempty"`
		}{json.RawMessage(result), failure}
		b, _ := json.Marshal(answer)
		return Value{S: string(b)}, true
	}
}
