//go:build !android

// Package portal is what oskit's Linux halves share: the desktop portals (xdg-desktop-portal)
// on the session bus, which answer for the desktop, GNOME's or KDE's, from inside a Flatpak
// too, and a portal call that answers later through a Request.
package portal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

// The portals' one object.
const (
	Name = "org.freedesktop.portal.Desktop"
	Path = dbus.ObjectPath("/org/freedesktop/portal/desktop")
)

// Desktop is the session bus, one connection for the process, and the portals' object on it.
func Desktop() (*dbus.Conn, dbus.BusObject, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, nil, fmt.Errorf("no session bus: %w", err)
	}
	return conn, conn.Object(Name, Path), nil
}

// Request calls a portal method that answers later, through a Request object: call makes the
// call with the handle_token it is given, and Request waits for the Request's Response, or for
// ctx to end. It returns the response's code, 0 for done, 1 cancelled by the person, 2 other,
// and its results.
func Request(ctx context.Context, call func(conn *dbus.Conn, portal dbus.BusObject, token string) (dbus.ObjectPath, error)) (uint32, map[string]dbus.Variant, error) {
	conn, portal, err := Desktop()
	if err != nil {
		return 0, nil, err
	}
	b := make([]byte, 8)
	rand.Read(b)
	token := "oskit_" + hex.EncodeToString(b)
	// The portal answers at a path made from our name and the token; the match comes before
	// the call, so the answer can't come first.
	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")
	want := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sender + "/" + token)
	match := []dbus.MatchOption{dbus.WithMatchObjectPath(want),
		dbus.WithMatchInterface("org.freedesktop.portal.Request"), dbus.WithMatchMember("Response")}
	if err := conn.AddMatchSignal(match...); err != nil {
		return 0, nil, err
	}
	defer conn.RemoveMatchSignal(match...)
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	got, err := call(conn, portal, token)
	if err != nil {
		return 0, nil, err
	}
	if got != want {
		return 0, nil, fmt.Errorf("the portal answers at %s, not %s", got, want)
	}
	for {
		select {
		case s := <-signals:
			if s.Path != want || s.Name != "org.freedesktop.portal.Request.Response" || len(s.Body) != 2 {
				continue
			}
			code, _ := s.Body[0].(uint32)
			results, _ := s.Body[1].(map[string]dbus.Variant)
			return code, results, nil
		case <-ctx.Done():
			conn.Object(Name, want).Call("org.freedesktop.portal.Request.Close", 0)
			return 0, nil, ctx.Err()
		}
	}
}
