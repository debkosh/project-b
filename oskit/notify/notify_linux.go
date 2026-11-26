//go:build !android

package notify

import (
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/debkosh/project-b/oskit"
	"github.com/debkosh/project-b/oskit/internal/portal"
)

const (
	notification = "org.freedesktop.portal.Notification"
	// defaultAction is the portal's name for a click on the note itself, action "" here.
	defaultAction = "default"
)

// linux is the Notification portal (org.freedesktop.portal.Notification), which a Flatpak
// must use and which works outside one too. It has nothing to ask: the person turns an app's
// notifications off in the desktop's settings. It has no subtitle, so Subtitle is the body's
// first line. Its calls go out in order on a goroutine of their own, so Show and Remove
// return at once and a Remove never overtakes the Show before it.
type linux struct {
	clicks clicks
	start  sync.Once
	calls  chan func(dbus.BusObject)
	listen sync.Once
}

var the = &linux{}

// New is this OS's Notifier.
func New() Notifier { return the }

func (l *linux) Allowed() bool { return true }

// Ask answers later, never inside the call that asked, as the Mac and Android do, even with
// oskit.Post unset.
func (l *linux) Ask(answered func(bool)) { go oskit.Post(func() { answered(true) }) }

func (l *linux) Show(n Note) {
	body := n.Body
	if n.Subtitle != "" {
		body = n.Subtitle + "\n" + n.Body
	}
	note := map[string]dbus.Variant{
		"title":          dbus.MakeVariant(n.Title),
		"body":           dbus.MakeVariant(body),
		"default-action": dbus.MakeVariant(defaultAction),
	}
	if len(n.Actions) > 0 {
		buttons := make([]map[string]dbus.Variant, len(n.Actions))
		for i, a := range n.Actions {
			buttons[i] = map[string]dbus.Variant{"label": dbus.MakeVariant(a.Label), "action": dbus.MakeVariant(a.ID)}
		}
		note["buttons"] = dbus.MakeVariant(buttons)
	}
	l.do(func(p dbus.BusObject) { p.Call(notification+".AddNotification", 0, n.ID, note) })
}

func (l *linux) Remove(id string) {
	l.do(func(p dbus.BusObject) { p.Call(notification+".RemoveNotification", 0, id) })
}

// OnClick listens for the portal's ActionInvoked: the note's own click is its default action.
func (l *linux) OnClick(clicked func(id, action string)) {
	l.clicks.set(clicked)
	l.listen.Do(func() {
		conn, _, err := portal.Desktop()
		if err != nil {
			return
		}
		if conn.AddMatchSignal(dbus.WithMatchObjectPath(portal.Path), dbus.WithMatchInterface(notification),
			dbus.WithMatchMember("ActionInvoked")) != nil {
			return
		}
		signals := make(chan *dbus.Signal, 16)
		conn.Signal(signals)
		go func() {
			for s := range signals {
				if s.Path != portal.Path || s.Name != notification+".ActionInvoked" || len(s.Body) < 2 {
					continue
				}
				id, _ := s.Body[0].(string)
				action, _ := s.Body[1].(string)
				if action == defaultAction {
					action = ""
				}
				l.clicks.click(id, action)
			}
		}()
	})
}

// do runs a call to the portal on the goroutine that keeps them in order. With no session
// bus there is no portal, and the call is dropped.
func (l *linux) do(call func(dbus.BusObject)) {
	l.start.Do(func() {
		l.calls = make(chan func(dbus.BusObject), 64)
		go func() {
			for c := range l.calls {
				if _, p, err := portal.Desktop(); err == nil {
					c(p)
				}
			}
		}()
	})
	l.calls <- call
}
