//go:build !android

package platform

import (
	"github.com/godbus/dbus/v5"

	"projectb/timeline"
)

// The desktop portals (docs/research/linux.md, 4): xdg-desktop-portal's object on the session
// bus, which answers for the desktop, GNOME's or KDE's, from inside the Flatpak too.
const (
	portalName = "org.freedesktop.portal.Desktop"
	portalPath = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	settings   = "org.freedesktop.portal.Settings"
)

// The appearance's colour scheme, as the Settings portal gives it: 0 is no preference, 1
// dark, 2 light.
const (
	appearance  = "org.freedesktop.appearance"
	colorScheme = "color-scheme"
	schemeDark  = 1
)

// WatchDark calls changed with whether the desktop is dark, once it has said, and again at
// each change: the Settings portal's appearance colour scheme. A desktop with no portal, or
// one that doesn't say, is light, as the engine already is, and changed isn't called.
// changed runs on a goroutine of its own.
func (s *System) WatchDark(changed func(dark bool)) {
	conn, err := dbus.SessionBus()
	if err != nil {
		timeline.Warnf("system: no session bus, so the desktop's dark mode isn't followed: %v", err)
		return
	}
	go watchDark(conn, changed)
}

func watchDark(conn *dbus.Conn, changed func(dark bool)) {
	// The match comes before the first read, so no change falls between them.
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(portalPath), dbus.WithMatchInterface(settings),
		dbus.WithMatchMember("SettingChanged"), dbus.WithMatchArg(0, appearance)); err != nil {
		timeline.Warnf("system: the desktop's dark mode isn't followed: %v", err)
		return
	}
	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)

	last := -1
	tell := func(scheme uint32) {
		dark := 0
		if scheme == schemeDark {
			dark = 1
		}
		if dark != last {
			last = dark
			changed(dark == 1)
		}
	}
	if scheme, err := readScheme(conn.Object(portalName, portalPath)); err != nil {
		timeline.Infof("system: the desktop doesn't say whether it is dark: %v", err)
	} else {
		tell(scheme)
	}
	for sig := range signals {
		if sig.Path != portalPath || sig.Name != settings+".SettingChanged" || len(sig.Body) != 3 {
			continue
		}
		if ns, _ := sig.Body[0].(string); ns != appearance {
			continue
		}
		if key, _ := sig.Body[1].(string); key != colorScheme {
			continue
		}
		if v, ok := sig.Body[2].(dbus.Variant); ok {
			if scheme, ok := unwrap(v).(uint32); ok {
				tell(scheme)
			}
		}
	}
}

// readScheme is the colour scheme now: ReadOne, from the portal's version 2, or Read on an
// older portal, whose answer comes wrapped in a second variant.
func readScheme(portal dbus.BusObject) (uint32, error) {
	var v dbus.Variant
	err := portal.Call(settings+".ReadOne", 0, appearance, colorScheme).Store(&v)
	if err != nil {
		if err = portal.Call(settings+".Read", 0, appearance, colorScheme).Store(&v); err != nil {
			return 0, err
		}
	}
	scheme, ok := unwrap(v).(uint32)
	if !ok {
		return 0, &dbus.Error{Name: "pb.UnexpectedType", Body: []any{v.Signature().String()}}
	}
	return scheme, nil
}

// unwrap is what a variant holds, under any variants around it.
func unwrap(v dbus.Variant) any {
	value := v.Value()
	for {
		inner, ok := value.(dbus.Variant)
		if !ok {
			return value
		}
		value = inner.Value()
	}
}
