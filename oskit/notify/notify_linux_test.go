//go:build !android

package notify

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/sudiptadeb/oskit/internal/portal"
	"github.com/sudiptadeb/oskit/internal/portaltest"
)

func TestMain(m *testing.M) { os.Exit(portaltest.Run(m)) }

// fakePortal is the Notification portal, which says what it was asked.
type fakePortal struct{ got chan []any }

func (p *fakePortal) AddNotification(id string, note map[string]dbus.Variant) *dbus.Error {
	p.got <- []any{"add", id, note}
	return nil
}

func (p *fakePortal) RemoveNotification(id string) *dbus.Error {
	p.got <- []any{"remove", id}
	return nil
}

// A note goes to the portal with its subtitle as the body's first line and its buttons, a
// Remove after it, in order; the portal's ActionInvoked comes back as a click, the note's own
// as action "".
func TestTheNotificationPortal(t *testing.T) {
	portaltest.Need(t)
	p := &fakePortal{got: make(chan []any, 4)}
	if err := portaltest.Desktop.Export(p, portal.Path, notification); err != nil {
		t.Fatal(err)
	}
	next := func() []any {
		t.Helper()
		select {
		case c := <-p.got:
			return c
		case <-time.After(5 * time.Second):
			t.Fatal("the portal was asked nothing")
			return nil
		}
	}
	n := New()
	n.Show(Note{ID: "dl-1", Title: "report.pdf", Subtitle: "Downloaded", Body: "From example.com",
		Actions: []Action{{ID: "open", Label: "Open"}, {ID: "show", Label: "Show in Folder"}}})
	n.Remove("dl-1")
	add := next()
	note, _ := add[2].(map[string]dbus.Variant)
	if add[0] != "add" || add[1] != "dl-1" || note["title"].Value() != "report.pdf" ||
		note["body"].Value() != "Downloaded\nFrom example.com" || note["default-action"].Value() != "default" {
		t.Fatalf("added %v", add)
	}
	var buttons []map[string]dbus.Variant
	if err := note["buttons"].Store(&buttons); err != nil || len(buttons) != 2 ||
		buttons[1]["label"].Value() != "Show in Folder" || buttons[1]["action"].Value() != "show" {
		t.Fatalf("buttons %v, %v", note["buttons"], err)
	}
	if got := next(); !reflect.DeepEqual(got, []any{"remove", "dl-1"}) {
		t.Fatalf("then %v, want the remove", got)
	}

	clicks := make(chan [2]string, 2)
	n.OnClick(func(id, action string) { clicks <- [2]string{id, action} })
	time.Sleep(200 * time.Millisecond) // the match reaches the bus
	for _, c := range [][2]string{{"default", ""}, {"show", "show"}} {
		portaltest.Desktop.Emit(portal.Path, notification+".ActionInvoked", "dl-1", c[0], []dbus.Variant{})
		select {
		case got := <-clicks:
			if got != [2]string{"dl-1", c[1]} {
				t.Fatalf("clicked %v, want dl-1 %q", got, c[1])
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("the %s click never came", c[0])
		}
	}

	asked := make(chan bool, 1)
	n.Ask(func(ok bool) { asked <- ok })
	if !n.Allowed() || !<-asked {
		t.Fatal("Linux's notifications are allowed, and asking says so")
	}
}
