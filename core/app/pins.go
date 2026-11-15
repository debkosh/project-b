package app

import (
	"encoding/json"

	"projectb/engine"
	"projectb/shell"
	"projectb/tabs"
	"projectb/timeline"
)

// pinShellPart gives every shell the profile's pins in its snapshot, and takes the
// messages that pin a tab, unpin a pin and open one: pins are Go's, for every shell, which
// may draw them or not. A shell names a pin by "pin", since "id" is a tab's number.
func (a *App) pinShellPart() shell.Part {
	return shell.Part{
		Snapshot: func(into map[string]any) { into["pins"] = a.Pins.List() },
		Handle: func(w engine.WindowID, t string, msg []byte) bool {
			var c struct {
				ID  engine.TabID `json:"id"`
				Pin string       `json:"pin"`
			}
			json.Unmarshal(msg, &c)
			switch t {
			case "pinTab":
				a.pinTab(c.ID)
			case "unpin":
				a.unpin(c.Pin)
			case "openPin":
				a.openPin(w, c.Pin)
			default:
				return false
			}
			return true
		},
	}
}

// pinnable says whether a tab can be pinned: a web tab with an address, not pinned yet. A
// pin is a saved place, which one of our tabs and a new tab aren't.
func pinnable(t tabs.Tab) bool { return t.Page == "" && t.Pin == "" && t.URL != "" }

// pinTab pins a tab: a new pin from its address, title and icon, after its workspace's
// others, with the tab open for it.
func (a *App) pinTab(id engine.TabID) {
	t, _ := a.Tabs.Get(id)
	if !pinnable(t) {
		timeline.Warnf("pins: tab %d can't be pinned", id)
		return
	}
	p := a.Pins.Add(t.Workspace, t.URL, t.Title, t.Favicon)
	a.Tabs.Bind(id, p.ID)
}

// unpin removes a pin. The tabs open for it stay, as ordinary tabs.
func (a *App) unpin(pin string) {
	if err := a.Pins.Remove(pin); err != nil {
		timeline.Warnf("pins: %v", err)
		return
	}
	a.Tabs.Unbind(pin)
}

// openPin shows window w's tab for a pin, or opens one for it there.
func (a *App) openPin(w engine.WindowID, pin string) {
	p, ok := a.Pins.Get(pin)
	if !ok {
		timeline.Warnf("pins: no pin %s", pin)
		return
	}
	if id := a.Tabs.PinIn(w, pin); id != 0 {
		a.Tabs.Activate(id)
		return
	}
	a.Tabs.OpenPin(w, p.ID, p.Workspace, p.URL)
}

// togglePin is the Pin Tab action: it pins the active tab of the window in front, or unpins
// it when it is pinned.
func (a *App) togglePin() {
	if t := a.Tabs.Active(); t.Pin != "" {
		a.unpin(t.Pin)
	} else {
		a.pinTab(t.ID)
	}
}
